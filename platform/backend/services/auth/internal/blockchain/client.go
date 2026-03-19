// Package blockchain provides an interface and implementations for interacting
// with the Hyperledger Besu node to manage on-chain participant registration.
package blockchain

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

// Client defines the on-chain operations required by the identity service.
type Client interface {
	// RegisterMember registers a participant on the ParticipantRegistry contract,
	// signing the transaction with the Central Bank private key (CB_PRIVATE_KEY).
	RegisterMember(ctx context.Context, memberAddr, role string) (txHash string, err error)

	// IsMemberAuthorized queries the contract to check if an address is active
	// and authorized on-chain.
	IsMemberAuthorized(ctx context.Context, address string) (bool, error)

	// GetMemberRole returns the role code registered for an address on-chain.
	GetMemberRole(ctx context.Context, address string) (uint8, error)
}

// --- NoopClient ---

// NoopClient is a no-op implementation used in local/test environments where
// no Besu node is available. All write operations succeed silently; reads
// return safe defaults.
type NoopClient struct{}

func (NoopClient) RegisterMember(_ context.Context, _, _ string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopClient) IsMemberAuthorized(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (NoopClient) GetMemberRole(_ context.Context, _ string) (uint8, error) {
	return 0, nil
}

// --- BesuClient ---

// BesuConfig holds the configuration required to connect to a Besu node and
// interact with the ParticipantRegistry contract.
type BesuConfig struct {
	RPCURL          string        // e.g. "http://besu-node:8545"
	RegistryAddress string        // deployed ParticipantRegistry address "0x..."
	CBPrivateKeyHex string        // CB_PRIVATE_KEY (hex, with or without 0x prefix)
	ChainID         int64         // Besu network chain ID
	RequestTimeout  time.Duration
}

// BesuClient implements Client by sending JSON-RPC calls to a Besu node.
// Transaction signing is performed locally with CB_PRIVATE_KEY using
// go-ethereum/crypto — no external KMS is involved for CB operations.
type BesuClient struct {
	cfg        BesuConfig
	privKey    *ecdsa.PrivateKey
	httpClient *http.Client
}

// NewBesuClient creates a BesuClient. Returns an error if any required
// configuration field is missing or the private key is invalid.
func NewBesuClient(cfg BesuConfig) (*BesuClient, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("blockchain: BESU_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.RegistryAddress) == "" {
		return nil, errors.New("blockchain: PARTICIPANT_REGISTRY_ADDRESS is required")
	}
	if strings.TrimSpace(cfg.CBPrivateKeyHex) == "" {
		return nil, errors.New("blockchain: CB_PRIVATE_KEY is required")
	}

	privKeyBytes, err := hex.DecodeString(strings.TrimPrefix(cfg.CBPrivateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("blockchain: decoding CB_PRIVATE_KEY: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("blockchain: parsing CB_PRIVATE_KEY: %w", err)
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

// RegisterMember encodes a call to registerMember(address,uint8) on the
// ParticipantRegistry contract, signs the transaction with CB_PRIVATE_KEY, and
// sends it via eth_sendRawTransaction. Returns the transaction hash.
func (b *BesuClient) RegisterMember(ctx context.Context, memberAddr, role string) (string, error) {
	cbAddress := gethcrypto.PubkeyToAddress(b.privKey.PublicKey).Hex()

	nonce, err := b.getNonce(ctx, cbAddress)
	if err != nil {
		return "", fmt.Errorf("blockchain: fetching nonce: %w", err)
	}

	gasPrice, err := b.getGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("blockchain: fetching gas price: %w", err)
	}

	roleCode := roleToCode(role)
	data := encodeRegisterMember(memberAddr, roleCode)
	dataBytes, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		return "", fmt.Errorf("blockchain: encoding tx data: %w", err)
	}

	registryAddr := common.HexToAddress(b.cfg.RegistryAddress)

	tx := types.NewTransaction(
		nonce,
		registryAddr,
		big.NewInt(0), // value = 0
		300000,        // gas limit
		gasPrice,
		dataBytes,
	)

	chainID := big.NewInt(b.cfg.ChainID)
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, b.privKey)
	if err != nil {
		return "", fmt.Errorf("blockchain: signing transaction: %w", err)
	}

	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("blockchain: marshaling signed tx: %w", err)
	}

	return b.sendRawTransaction(ctx, "0x"+hex.EncodeToString(rawTx))
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
	body, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return "", fmt.Errorf("blockchain: marshaling RPC request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.cfg.RPCURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("blockchain: building HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("blockchain: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("blockchain: reading response body: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "", fmt.Errorf("blockchain: unmarshaling RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("blockchain: RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
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
		return 0, fmt.Errorf("blockchain: parsing nonce %q: %w", result, err)
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

// --- ABI encoding helpers ---

// roleToCode converts a role string to the uint8 used in the ParticipantRegistry contract.
func roleToCode(role string) uint8 {
	switch role {
	case "ROLE_COMMERCIAL_BANK":
		return 1
	case "ROLE_NOC":
		return 2
	case "ROLE_SUPERVISOR":
		return 3
	case "ROLE_TREASURY":
		return 4
	case "ROLE_GOVERNANCE":
		return 5
	default:
		return 0
	}
}

// encodeRegisterMember ABI-encodes registerMember(address _member, uint8 _role).
// Selector computed from: keccak256("registerMember(address,uint8)")[0:4]
func encodeRegisterMember(memberAddr string, roleCode uint8) string {
	const selector = "5c0c4057"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(memberAddr), "0x"))
	paddedRole := fmt.Sprintf("%064x", roleCode)
	return "0x" + selector + addrHex + paddedRole
}

// encodeIsAuthorized ABI-encodes isAuthorized(address _member).
// Selector computed from: keccak256("isAuthorized(address)")[0:4]
func encodeIsAuthorized(address string) string {
	const selector = "fe9fbb80"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(address), "0x"))
	return "0x" + selector + addrHex
}

// encodeGetRole ABI-encodes getRole(address _member).
// Selector computed from: keccak256("getRole(address)")[0:4]
func encodeGetRole(address string) string {
	const selector = "b5dce8cd"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(address), "0x"))
	return "0x" + selector + addrHex
}
