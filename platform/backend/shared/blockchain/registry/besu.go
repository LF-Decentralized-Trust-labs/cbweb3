package registry

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// BesuConfig holds the configuration required to connect to a Besu node and
// interact with the ParticipantRegistry contract.
//
// Environment variables used by services:
//
//	BLOCKCHAIN_CLIENT          — "besu" or "noop" (default: "noop")
//	BESU_RPC_URL               — e.g. "http://besu-node:8545"
//	PARTICIPANT_REGISTRY_ADDRESS — deployed ParticipantRegistry address "0x..."
//	CB_PRIVATE_KEY             — Central Bank private key (hex, with or without 0x prefix)
//	BESU_CHAIN_ID              — Besu network chain ID (default: 1337)
//	BLOCKCHAIN_REQUEST_TIMEOUT_SEC — HTTP timeout in seconds (default: 15)
type BesuConfig struct {
	RPCURL          string
	RegistryAddress string
	CBPrivateKeyHex string
	ChainID         int64
	RequestTimeout  time.Duration
}

// BesuClient implements RegistryWriter and RegistryReader by sending JSON-RPC
// calls to a Besu node. Transaction signing is performed locally with
// CB_PRIVATE_KEY using go-ethereum/crypto — no external KMS involved.
type BesuClient struct {
	cfg        BesuConfig
	privKey    *ecdsa.PrivateKey
	httpClient *http.Client
}

// NewBesuClient creates a BesuClient. Returns an error if any required
// configuration field is missing or the private key is invalid.
func NewBesuClient(cfg BesuConfig) (*BesuClient, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("registry: BESU_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.RegistryAddress) == "" {
		return nil, errors.New("registry: PARTICIPANT_REGISTRY_ADDRESS is required")
	}
	if strings.TrimSpace(cfg.CBPrivateKeyHex) == "" {
		return nil, errors.New("registry: CB_PRIVATE_KEY is required")
	}

	privKeyBytes, err := hex.DecodeString(strings.TrimPrefix(cfg.CBPrivateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("registry: decoding CB_PRIVATE_KEY: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("registry: parsing CB_PRIVATE_KEY: %w", err)
	}

	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &BesuClient{
		cfg:        cfg,
		privKey:    privKey,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// SetParticipant activates (active=true) or deactivates (active=false) a
// participant on the ParticipantRegistry contract.
func (b *BesuClient) SetParticipant(ctx context.Context, wallet, role string, active bool) (string, error) {
	roleCode := roleToCode(role)
	if !active {
		roleCode = 0
	}
	return b.sendRegisterMember(ctx, wallet, roleCode)
}

// IsMemberAuthorized calls isAuthorized(address) on the contract via eth_call.
func (b *BesuClient) IsMemberAuthorized(ctx context.Context, address string) (bool, error) {
	data := encodeIsAuthorized(address)
	result, err := b.ethCall(ctx, b.cfg.RegistryAddress, data)
	if err != nil {
		return false, err
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(result, "0x"))
	if err != nil || len(decoded) < 32 {
		return false, nil
	}
	return decoded[31] == 0x01, nil
}

// GetMemberRole calls getRole(address) on the contract via eth_call.
func (b *BesuClient) GetMemberRole(ctx context.Context, address string) (uint8, error) {
	data := encodeGetRole(address)
	result, err := b.ethCall(ctx, b.cfg.RegistryAddress, data)
	if err != nil {
		return 0, err
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(result, "0x"))
	if err != nil || len(decoded) < 32 {
		return 0, nil
	}
	return decoded[31], nil
}

func (b *BesuClient) sendRegisterMember(ctx context.Context, wallet string, roleCode uint8) (string, error) {
	cbAddress := gethcrypto.PubkeyToAddress(b.privKey.PublicKey).Hex()

	nonce, err := b.getNonce(ctx, cbAddress)
	if err != nil {
		return "", fmt.Errorf("registry: fetching nonce: %w", err)
	}
	gasPrice, err := b.getGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("registry: fetching gas price: %w", err)
	}

	data := encodeRegisterMember(wallet, roleCode)
	dataBytes, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		return "", fmt.Errorf("registry: encoding tx data: %w", err)
	}

	registryAddr := common.HexToAddress(b.cfg.RegistryAddress)
	tx := types.NewTransaction(nonce, registryAddr, big.NewInt(0), 300000, gasPrice, dataBytes)

	signer := types.NewEIP155Signer(big.NewInt(b.cfg.ChainID))
	signedTx, err := types.SignTx(tx, signer, b.privKey)
	if err != nil {
		return "", fmt.Errorf("registry: signing transaction: %w", err)
	}

	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("registry: marshaling signed tx: %w", err)
	}

	return b.sendRawTransaction(ctx, "0x"+hex.EncodeToString(rawTx))
}

// --- JSON-RPC helpers ---

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type jsonRPCResponse struct {
	Result string        `json:"result"`
	Error  *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (b *BesuClient) doRPC(ctx context.Context, method string, params []interface{}) (string, error) {
	body, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	if err != nil {
		return "", fmt.Errorf("registry: marshaling RPC request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.cfg.RPCURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("registry: building HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("registry: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("registry: reading response body: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "", fmt.Errorf("registry: unmarshaling RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("registry: RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (b *BesuClient) getNonce(ctx context.Context, address string) (uint64, error) {
	result, err := b.doRPC(ctx, "eth_getTransactionCount", []interface{}{address, "pending"})
	if err != nil {
		return 0, err
	}
	var nonce uint64
	if _, err = fmt.Sscanf(result, "0x%x", &nonce); err != nil {
		return 0, fmt.Errorf("registry: parsing nonce %q: %w", result, err)
	}
	return nonce, nil
}

func (b *BesuClient) getGasPrice(ctx context.Context) (*big.Int, error) {
	result, err := b.doRPC(ctx, "eth_gasPrice", []interface{}{})
	if err != nil {
		return big.NewInt(0), err
	}
	price := new(big.Int)
	price.SetString(strings.TrimPrefix(result, "0x"), 16)
	return price, nil
}

func (b *BesuClient) ethCall(ctx context.Context, to, data string) (string, error) {
	callObj := map[string]string{"to": to, "data": data}
	return b.doRPC(ctx, "eth_call", []interface{}{callObj, "latest"})
}

func (b *BesuClient) sendRawTransaction(ctx context.Context, rawTx string) (string, error) {
	return b.doRPC(ctx, "eth_sendRawTransaction", []interface{}{rawTx})
}
