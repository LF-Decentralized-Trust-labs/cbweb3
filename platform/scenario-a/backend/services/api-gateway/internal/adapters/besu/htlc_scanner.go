// SPDX-License-Identifier: Apache-2.0

package besu

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// logChunkSize is the block range per FilterLogs call. Besu's default --rpc-max-logs-range
// is 1000; we stay at 999 to avoid off-by-one issues on nodes with the default config.
const logChunkSize = uint64(999)

// HTLCScanResult is the on-chain representation of an HTLC derived from event logs.
// CounterpartyLocked is always false: it is off-chain state tracked by the
// payment-orchestrator and cannot be derived from Besu events.
type HTLCScanResult struct {
	ContractID         string `json:"contract_id"`
	Sender             string `json:"sender"`
	Receiver           string `json:"receiver"`
	// SenderName/ReceiverName are the resolved institution names for the
	// Sender/Receiver EVM addresses. Populated best-effort by the api-gateway
	// handler from the compliance participant registry; empty when unresolved.
	SenderName         string `json:"sender_name,omitempty"`
	ReceiverName       string `json:"receiver_name,omitempty"`
	HashLock           string `json:"hash_lock"`
	TimeLock           uint64 `json:"time_lock"`
	ZetoLockRef        string `json:"zeto_lock_ref"`
	State              string `json:"state"`
	CounterpartyLocked bool   `json:"counterparty_locked"`
}

// HTLCScanner reads HTLC state from Besu event logs via eth_getLogs.
type HTLCScanner struct {
	client   *ethclient.Client
	contract common.Address
	abi      abi.ABI
}

// NewHTLCScanner creates a scanner connected to rpcURL targeting the given HTLC contract address.
func NewHTLCScanner(rpcURL, contractAddr string) (*HTLCScanner, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu at %s: %w", rpcURL, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(htlcEventsABI))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("parse htlc abi: %w", err)
	}
	return &HTLCScanner{
		client:   client,
		contract: common.HexToAddress(contractAddr),
		abi:      parsedABI,
	}, nil
}

// Close terminates the underlying Besu connection.
func (s *HTLCScanner) Close() error {
	s.client.Close()
	return nil
}

// filterLogsChunked paginates eth_getLogs in blocks of logChunkSize to stay within the
// node's --rpc-max-logs-range limit (Besu default: 1000 blocks).
func (s *HTLCScanner) filterLogsChunked(ctx context.Context, topic common.Hash) ([]types.Log, error) {
	latest, err := s.client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("get block number: %w", err)
	}

	var all []types.Log
	for from := uint64(0); from <= latest; from += logChunkSize {
		to := from + logChunkSize - 1
		if to > latest {
			to = latest
		}
		chunk, err := s.client.FilterLogs(ctx, ethereum.FilterQuery{
			Addresses: []common.Address{s.contract},
			Topics:    [][]common.Hash{{topic}},
			FromBlock: new(big.Int).SetUint64(from),
			ToBlock:   new(big.Int).SetUint64(to),
		})
		if err != nil {
			return nil, err
		}
		all = append(all, chunk...)
	}
	return all, nil
}

// ScanAllHTLCs returns all HTLCs ever created on the contract, with state derived from
// the full event history. Results are sorted by timeLock descending (soonest expiry first).
func (s *HTLCScanner) ScanAllHTLCs(ctx context.Context) ([]HTLCScanResult, error) {
	lockedTopic := s.abi.Events["LogHTLCLocked"].ID
	claimedTopic := s.abi.Events["LogHTLCClaimed"].ID
	refundedTopic := s.abi.Events["LogHTLCRefunded"].ID

	// Step 1: all Lock events → seed the index.
	lockedLogs, err := s.filterLogsChunked(ctx, lockedTopic)
	if err != nil {
		return nil, fmt.Errorf("filter LogHTLCLocked: %w", err)
	}

	index := make(map[[32]byte]*HTLCScanResult, len(lockedLogs))
	order := make([][32]byte, 0, len(lockedLogs))

	for _, l := range lockedLogs {
		// Topics: [0]=event sig, [1]=contractId, [2]=sender, [3]=receiver
		// Data:   hashLock(32) | timeLock(32) | zetoLockRef(32)
		if len(l.Topics) < 4 || len(l.Data) < 96 {
			continue
		}
		var contractID [32]byte
		copy(contractID[:], l.Topics[1].Bytes())

		sender := common.BytesToAddress(l.Topics[2].Bytes())
		receiver := common.BytesToAddress(l.Topics[3].Bytes())
		hashLock := "0x" + hex.EncodeToString(l.Data[0:32])
		timeLock := new(big.Int).SetBytes(l.Data[32:64]).Uint64()
		zetoLockRef := "0x" + hex.EncodeToString(l.Data[64:96])

		index[contractID] = &HTLCScanResult{
			ContractID:  "0x" + hex.EncodeToString(contractID[:]),
			Sender:      sender.Hex(),
			Receiver:    receiver.Hex(),
			HashLock:    hashLock,
			TimeLock:    timeLock,
			ZetoLockRef: zetoLockRef,
			State:       "HTLC_STATE_LOCKED",
		}
		order = append(order, contractID)
	}

	// Step 2: Claimed events → mark as SETTLED.
	claimedLogs, err := s.filterLogsChunked(ctx, claimedTopic)
	if err != nil {
		return nil, fmt.Errorf("filter LogHTLCClaimed: %w", err)
	}
	for _, l := range claimedLogs {
		if len(l.Topics) < 2 {
			continue
		}
		var contractID [32]byte
		copy(contractID[:], l.Topics[1].Bytes())
		if r, ok := index[contractID]; ok {
			r.State = "HTLC_STATE_SETTLED"
		}
	}

	// Step 3: Refunded events → mark as REFUNDED.
	refundedLogs, err := s.filterLogsChunked(ctx, refundedTopic)
	if err != nil {
		return nil, fmt.Errorf("filter LogHTLCRefunded: %w", err)
	}
	for _, l := range refundedLogs {
		if len(l.Topics) < 2 {
			continue
		}
		var contractID [32]byte
		copy(contractID[:], l.Topics[1].Bytes())
		if r, ok := index[contractID]; ok {
			r.State = "HTLC_STATE_REFUNDED"
		}
	}

	// Build ordered, deduplicated result slice.
	results := make([]HTLCScanResult, 0, len(order))
	seen := make(map[[32]byte]bool, len(order))
	for _, id := range order {
		if seen[id] {
			continue
		}
		seen[id] = true
		if r, ok := index[id]; ok {
			results = append(results, *r)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].TimeLock > results[j].TimeLock
	})

	return results, nil
}

// GetByContractID scans all HTLCs and returns the one matching contractID.
// The lookup is case-insensitive and accepts IDs with or without the "0x" prefix.
func (s *HTLCScanner) GetByContractID(ctx context.Context, contractID string) (*HTLCScanResult, error) {
	results, err := s.ScanAllHTLCs(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimPrefix(contractID, "0x"))
	for i := range results {
		if strings.ToLower(strings.TrimPrefix(results[i].ContractID, "0x")) == needle {
			return &results[i], nil
		}
	}
	return nil, fmt.Errorf("HTLC %s not found on-chain", contractID)
}
