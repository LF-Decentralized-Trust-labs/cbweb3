package registry

import (
	"bytes"
	"context"
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
)

// BesuConfig holds the configuration required to connect to a Besu node and
// interact with the ParticipantRegistry contract.
//
// Environment variables used by services:
//
//	BLOCKCHAIN_CLIENT           — "besu" or "noop" (default: "noop")
//	BESU_RPC_URL                — e.g. "http://besu-node:8545"
//	PARTICIPANT_REGISTRY_ADDRESS — deployed ParticipantRegistry address "0x..."
//	CB_PRIVATE_KEY              — Central Bank private key (hex); only required for Central Bank nodes
//	BESU_CHAIN_ID               — Besu network chain ID (default: 1337)
//	BLOCKCHAIN_REQUEST_TIMEOUT_SEC — HTTP timeout in seconds (default: 15)
type BesuConfig struct {
	RPCURL          string
	RegistryAddress string
	ChainID         int64
	RequestTimeout  time.Duration
}

// BesuClient implements RegistryWriter and RegistryReader by sending JSON-RPC
// calls to a Besu node.
//
// Read operations (IsMemberAuthorized, GetMemberRole, GetCertFingerprint) always
// work — they use eth_call and require no signing key.
//
// Write operations (SetParticipant, SetCertFingerprint) require a
// TransactionSigner. When signer is nil, writes return ErrNoSigner.
type BesuClient struct {
	cfg        BesuConfig
	signer     TransactionSigner
	httpClient *http.Client
}

// NewBesuClient creates a BesuClient. The signer is optional: pass nil for a
// read-only client where write operations will return ErrNoSigner.
func NewBesuClient(cfg BesuConfig, signer TransactionSigner) (*BesuClient, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("registry: BESU_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.RegistryAddress) == "" {
		return nil, errors.New("registry: PARTICIPANT_REGISTRY_ADDRESS is required")
	}

	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &BesuClient{
		cfg:        cfg,
		signer:     signer,
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

// SetCertFingerprint calls setCertFingerprint(address, bytes32) on-chain.
func (b *BesuClient) SetCertFingerprint(ctx context.Context, wallet string, fingerprint [32]byte) (string, error) {
	if b.signer == nil {
		return "", ErrNoSigner
	}

	signerAddr, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("registry: resolving signer address: %w", err)
	}

	nonce, err := b.getNonce(ctx, signerAddr)
	if err != nil {
		return "", fmt.Errorf("registry: fetching nonce: %w", err)
	}
	gasPrice, err := b.getGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("registry: fetching gas price: %w", err)
	}

	data := encodeSetCertFingerprint(wallet, fingerprint)
	dataBytes, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		return "", fmt.Errorf("registry: encoding cert fingerprint tx data: %w", err)
	}

	registryAddr := common.HexToAddress(b.cfg.RegistryAddress)
	tx := types.NewTransaction(nonce, registryAddr, big.NewInt(0), 200000, gasPrice, dataBytes)

	signedTx, err := b.signer.SignTx(ctx, tx, big.NewInt(b.cfg.ChainID))
	if err != nil {
		return "", fmt.Errorf("registry: signing cert fingerprint tx: %w", err)
	}

	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("registry: marshaling cert fingerprint tx: %w", err)
	}

	return b.sendRawTransaction(ctx, "0x"+hex.EncodeToString(rawTx))
}

// GetCertFingerprint calls getCertFingerprint(address) on the contract via eth_call.
func (b *BesuClient) GetCertFingerprint(ctx context.Context, address string) ([32]byte, error) {
	data := encodeGetCertFingerprint(address)
	result, err := b.ethCall(ctx, b.cfg.RegistryAddress, data)
	if err != nil {
		return [32]byte{}, err
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(result, "0x"))
	if err != nil || len(decoded) < 32 {
		return [32]byte{}, nil
	}
	var fp [32]byte
	copy(fp[:], decoded[:32])
	return fp, nil
}

func (b *BesuClient) sendRegisterMember(ctx context.Context, wallet string, roleCode uint8) (string, error) {
	if b.signer == nil {
		return "", ErrNoSigner
	}

	signerAddr, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("registry: resolving signer address: %w", err)
	}

	nonce, err := b.getNonce(ctx, signerAddr)
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

	signedTx, err := b.signer.SignTx(ctx, tx, big.NewInt(b.cfg.ChainID))
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
