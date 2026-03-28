package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

type spokeConfig struct {
	Name       string
	BesuRPC    string
	HTLCAddr   common.Address
	GRPCTarget string // payment-orchestrator gRPC on the *counterpart* spoke
}

var logHTLCClaimedSig = crypto.Keccak256Hash([]byte("LogHTLCClaimed(bytes32,bytes32)"))

var claimedEventABI abi.Arguments

func init() {
	b32, _ := abi.NewType("bytes32", "", nil)
	claimedEventABI = abi.Arguments{{Name: "secret", Type: b32}}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	spokeA := spokeConfig{
		Name:     "spoke-a",
		BesuRPC:  requireEnv("SPOKE_A_BESU_RPC"),
		HTLCAddr: common.HexToAddress(requireEnv("SPOKE_A_HTLC_ADDRESS")),
	}
	spokeB := spokeConfig{
		Name:     "spoke-b",
		BesuRPC:  requireEnv("SPOKE_B_BESU_RPC"),
		HTLCAddr: common.HexToAddress(requireEnv("SPOKE_B_HTLC_ADDRESS")),
	}

	spokeA.GRPCTarget = requireEnv("SPOKE_B_PAYMENT_GRPC")
	spokeB.GRPCTarget = requireEnv("SPOKE_A_PAYMENT_GRPC")

	pollInterval := 3 * time.Second
	if v := os.Getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			pollInterval = d
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("relay starting",
		"spoke-a-rpc", spokeA.BesuRPC,
		"spoke-a-htlc", spokeA.HTLCAddr.Hex(),
		"spoke-b-rpc", spokeB.BesuRPC,
		"spoke-b-htlc", spokeB.HTLCAddr.Hex(),
		"poll-interval", pollInterval,
	)

	go pollSpoke(ctx, logger, spokeA, pollInterval)
	go pollSpoke(ctx, logger, spokeB, pollInterval)

	<-ctx.Done()
	logger.Info("relay shutting down")
}

func pollSpoke(ctx context.Context, logger *slog.Logger, cfg spokeConfig, interval time.Duration) {
	ethClient, err := ethclient.DialContext(ctx, cfg.BesuRPC)
	if err != nil {
		log.Fatalf("relay: dial %s besu: %v", cfg.Name, err)
	}
	defer ethClient.Close()

	conn, err := grpc.NewClient(cfg.GRPCTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("relay: dial %s grpc %s: %v", cfg.Name, cfg.GRPCTarget, err)
	}
	defer conn.Close()

	client := pb.NewPaymentOrchestratorServiceClient(conn)

	var fromBlock uint64
	header, err := ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("relay: get head on %s: %v", cfg.Name, err)
	}
	fromBlock = header.Number.Uint64()
	logger.Info("relay poll started", "spoke", cfg.Name, "fromBlock", fromBlock)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			toBlock, err := latestBlock(ctx, ethClient)
			if err != nil {
				logger.Warn("relay: get latest block", "spoke", cfg.Name, "error", err)
				continue
			}
			if toBlock < fromBlock {
				continue
			}

			logs, err := fetchClaimedLogs(ctx, ethClient, cfg.HTLCAddr, fromBlock, toBlock)
			if err != nil {
				logger.Warn("relay: fetch logs", "spoke", cfg.Name, "error", err)
				continue
			}

			for _, lg := range logs {
				contractID, secret, err := parseClaimedLog(lg)
				if err != nil {
					logger.Warn("relay: parse log", "spoke", cfg.Name, "error", err)
					continue
				}

				logger.Info("relay: LogHTLCClaimed detected",
					"spoke", cfg.Name,
					"contractId", contractID,
					"block", lg.BlockNumber,
					"txHash", lg.TxHash.Hex(),
				)

				settleCtx, settleCancel := context.WithTimeout(ctx, 30*time.Second)
				resp, err := client.SettleHTLC(settleCtx, &pb.SettleHTLCRequest{
					ContractId: contractID,
					Secret:     secret,
				})
				settleCancel()

				if err != nil {
					logger.Error("relay: settle on counterpart failed",
						"spoke", cfg.Name,
						"contractId", contractID,
						"error", err,
					)
					continue
				}

				logger.Info("relay: settled on counterpart",
					"spoke", cfg.Name,
					"contractId", contractID,
					"htlcTxHash", resp.HtlcTxHash,
					"zetoTxHash", resp.ZetoTxHash,
				)
			}

			fromBlock = toBlock + 1
		}
	}
}

func latestBlock(ctx context.Context, c *ethclient.Client) (uint64, error) {
	h, err := c.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return h.Number.Uint64(), nil
}

func fetchClaimedLogs(ctx context.Context, c *ethclient.Client, addr common.Address, from, to uint64) ([]types.Log, error) {
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{addr},
		Topics:    [][]common.Hash{{logHTLCClaimedSig}},
	}
	return c.FilterLogs(ctx, query)
}

func parseClaimedLog(lg types.Log) (contractID, secret string, err error) {
	if len(lg.Topics) < 2 {
		return "", "", fmt.Errorf("expected >=2 topics, got %d", len(lg.Topics))
	}
	contractID = strings.TrimPrefix(lg.Topics[1].Hex(), "0x")

	vals, err := claimedEventABI.UnpackValues(lg.Data)
	if err != nil {
		return "", "", fmt.Errorf("unpack secret: %w", err)
	}
	if len(vals) < 1 {
		return "", "", fmt.Errorf("no data values")
	}
	secretBytes, ok := vals[0].([32]byte)
	if !ok {
		return "", "", fmt.Errorf("unexpected secret type %T", vals[0])
	}
	secret = hex.EncodeToString(secretBytes[:])
	return contractID, secret, nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: %s is required", key)
	}
	return v
}
