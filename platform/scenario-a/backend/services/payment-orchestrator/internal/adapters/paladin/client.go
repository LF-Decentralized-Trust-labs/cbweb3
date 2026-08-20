// SPDX-License-Identifier: Apache-2.0

// Package paladin provides the real ZetoOperator implementation that talks to
// Paladin sidecar nodes via their JSON-RPC HTTP API (POST /).
package paladin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.ZetoOperator = (*Client)(nil)

// Client implements ports.ZetoOperator by calling the Paladin sidecar HTTP API.
type Client struct {
	baseURL          string
	identity         string
	zetoTokenAddress string
	httpClient       *http.Client
	logger           *slog.Logger
}

// ClientConfig holds the configuration for a Paladin client.
type ClientConfig struct {
	BaseURL          string // e.g. "http://127.0.0.1:31648"
	Identity         string // Paladin identity, e.g. "funded_operator@spoke-a-cb"
	ZetoTokenAddress string // Deployed Zeto token instance address, e.g. "0x..."
}

// NewClient creates a new Paladin client.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client {
	return &Client{
		baseURL:          cfg.BaseURL,
		identity:         cfg.Identity,
		zetoTokenAddress: cfg.ZetoTokenAddress,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

type abiComponent struct {
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	InternalType string         `json:"internalType,omitempty"`
	Components   []abiComponent `json:"components,omitempty"`
}

type abiEntry struct {
	Type            string         `json:"type"`
	Name            string         `json:"name,omitempty"`
	StateMutability string         `json:"stateMutability,omitempty"`
	Inputs          []abiComponent `json:"inputs"`
	Outputs         []abiComponent `json:"outputs,omitempty"`
}

var transferParamComponents = []abiComponent{
	{Name: "to", Type: "string", InternalType: "string"},
	{Name: "amount", Type: "uint256", InternalType: "uint256"},
	{Name: "data", Type: "bytes", InternalType: "bytes"},
}

var zetoABI = []abiEntry{
	{
		Type: "function", Name: "mint",
		Inputs: []abiComponent{{Name: "mints", Type: "tuple[]", InternalType: "struct TransferParam[]", Components: transferParamComponents}},
	},
	{
		Type: "function", Name: "withdraw",
		Inputs: []abiComponent{{Name: "amount", Type: "uint256"}},
	},
	{
		Type: "function", Name: "transfer",
		Inputs: []abiComponent{{Name: "transfers", Type: "tuple[]", InternalType: "struct TransferParam[]", Components: transferParamComponents}},
	},
	{
		Type: "function", Name: "lock",
		Inputs: []abiComponent{
			{Name: "amount", Type: "uint256"},
			{Name: "delegate", Type: "address"},
		},
	},
	{
		Type: "function", Name: "transferLocked",
		Inputs: []abiComponent{
			{Name: "lockedInputs", Type: "uint256[]"},
			{Name: "delegate", Type: "string"},
			{Name: "transfers", Type: "tuple[]", InternalType: "struct TransferParam[]", Components: transferParamComponents},
		},
	},
	{
		Type: "function", Name: "balanceOf",
		StateMutability: "view",
		Inputs:          []abiComponent{{Name: "account", Type: "string"}},
		Outputs:         []abiComponent{{Name: "totalStates", Type: "uint256"}, {Name: "totalBalance", Type: "uint256"}, {Name: "overflow", Type: "bool"}},
	},
}

type paladinTx struct {
	Type     string      `json:"type"`
	Domain   string      `json:"domain,omitempty"`
	From     string      `json:"from"`
	To       string      `json:"to,omitempty"`
	ABI      []abiEntry  `json:"abi,omitempty"`
	Function string      `json:"function,omitempty"`
	Data     interface{} `json:"data"`
}

type txFullResult struct {
	Receipt *txReceipt `json:"receipt,omitempty"`
}

type txReceipt struct {
	ID              string `json:"id"`
	Success         bool   `json:"success"`
	ContractAddress string `json:"contractAddress,omitempty"`
	FailureMessage  string `json:"failureMessage,omitempty"`
}

func (c *Client) resolveVerifier(ctx context.Context, identity string) (string, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "ptx_resolveVerifier",
		Params:  []interface{}{identity, "ecdsa:secp256k1", "eth_address"},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST paladin: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", rpcResp.Error
	}
	var addr string
	if err := json.Unmarshal(rpcResp.Result, &addr); err != nil {
		return "", fmt.Errorf("unmarshal verifier address: %w", err)
	}
	return addr, nil
}

// ResolveIdentity implements ports.ZetoOperator — public wrapper around resolveVerifier.
func (c *Client) ResolveIdentity(ctx context.Context, identity string) (string, error) {
	return c.resolveVerifier(ctx, identity)
}

func (c *Client) getLockedStateIDs(ctx context.Context, txID string) ([]string, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "ptx_getStateReceipt",
		Params:  []interface{}{txID},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for locked states of %s", txID)
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.httpClient.Do(req)
			if err != nil {
				continue
			}
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			var rpcResp jsonRPCResponse
			if err := json.Unmarshal(respBody, &rpcResp); err != nil {
				continue
			}
			if rpcResp.Error != nil {
				continue
			}
			if rpcResp.Result == nil {
				continue
			}

			var stateReceipt struct {
				Confirmed []struct {
					ID   string                 `json:"id"`
					Data map[string]interface{} `json:"data"`
				} `json:"confirmed"`
			}
			if err := json.Unmarshal(rpcResp.Result, &stateReceipt); err != nil {
				continue
			}

			var lockedIDs []string
			for _, s := range stateReceipt.Confirmed {
				if locked, ok := s.Data["locked"]; ok {
					if b, isBool := locked.(bool); isBool && b {
						lockedIDs = append(lockedIDs, s.ID)
					}
				}
			}
			if len(lockedIDs) > 0 {
				c.logger.Info("found locked state IDs", "txID", txID, "lockedIDs", lockedIDs)
				return lockedIDs, nil
			}
		}
	}
}

func (c *Client) sendTx(ctx context.Context, tx paladinTx) (string, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ptx_sendTransaction",
		Params:  []interface{}{tx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	c.logger.Debug("sending Paladin transaction", "method", tx.Function, "to", tx.To)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST paladin: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", rpcResp.Error
	}

	var txID string
	if err := json.Unmarshal(rpcResp.Result, &txID); err != nil {
		return "", fmt.Errorf("unmarshal tx ID: %w", err)
	}

	receipt, err := c.pollReceipt(ctx, txID)
	if err != nil {
		return "", fmt.Errorf("poll receipt: %w", err)
	}
	if !receipt.Success {
		return "", fmt.Errorf("transaction failed: %s", receipt.FailureMessage)
	}

	return txID, nil
}

func (c *Client) pollReceipt(ctx context.Context, txID string) (*txReceipt, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for receipt of %s", txID)
		case <-ticker.C:
			reqBody := jsonRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "ptx_getTransactionFull",
				Params:  []interface{}{txID},
			}
			body, _ := json.Marshal(reqBody)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.httpClient.Do(req)
			if err != nil {
				continue
			}
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			var rpcResp jsonRPCResponse
			if err := json.Unmarshal(respBody, &rpcResp); err != nil {
				continue
			}

			var txResult txFullResult
			if err := json.Unmarshal(rpcResp.Result, &txResult); err != nil {
				continue
			}
			if txResult.Receipt != nil {
				return txResult.Receipt, nil
			}
		}
	}
}

// --- ZetoOperator interface implementation ---

func (c *Client) Mint(ctx context.Context, to string, amount string) (string, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		To:       c.zetoTokenAddress,
		ABI:      zetoABI,
		Function: "mint",
		Data: map[string]interface{}{
			"mints": []map[string]interface{}{
				{"to": to, "amount": amount, "data": "0x"},
			},
		},
	}
	return c.sendTx(ctx, tx)
}

// Burn removes Zeto tokens from circulation via Zeto.withdraw, which converts
// Zeto private states back into the underlying ERC-20 and burns them. The
// from parameter is currently informational; the on-chain caller is the
// configured Paladin identity.
func (c *Client) Burn(ctx context.Context, _ string, amount string) (string, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		To:       c.zetoTokenAddress,
		ABI:      zetoABI,
		Function: "withdraw",
		Data: map[string]interface{}{
			"amount": amount,
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) Transfer(ctx context.Context, to string, amount string) (string, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		To:       c.zetoTokenAddress,
		ABI:      zetoABI,
		Function: "transfer",
		Data: map[string]interface{}{
			"transfers": []map[string]interface{}{
				{"to": to, "amount": amount, "data": "0x"},
			},
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) Lock(ctx context.Context, amount string, delegate string) (*ports.ZetoLockResult, error) {
	delegateAddr, err := c.resolveVerifier(ctx, c.identity)
	if err != nil {
		return nil, fmt.Errorf("resolve delegate address: %w", err)
	}
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		To:       c.zetoTokenAddress,
		ABI:      zetoABI,
		Function: "lock",
		Data: map[string]interface{}{
			"amount":   amount,
			"delegate": delegateAddr,
		},
	}
	txHash, err := c.sendTx(ctx, tx)
	if err != nil {
		return nil, err
	}

	lockedIDs, err := c.getLockedStateIDs(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("get locked state IDs: %w", err)
	}

	// Refuse a lock this build cannot settle later. The caller aborts before the
	// public HTLC record exists (server.go LockHTLC returns on this error), so no
	// cross-spoke commitment is created and the relay never sees an event it would
	// retry forever. See locked_state_id.go for the reproduction and the evidence.
	if bad, found := unsettleableLockedStateID(lockedIDs); found {
		// The amount is recorded on purpose, against the general rule that settlement
		// amounts stay out of the logs. These tokens are locked on-chain and cannot be
		// released, so this line is the only trace that a specific sum became
		// unrecoverable — an incident record, not routine settlement traffic, and one
		// reconciliation has no other way to explain. A durable audit row would be
		// better; this service has no audit sink, which is its own follow-up.
		c.logger.Error("refusing an unsettleable Zeto lock; the locked amount cannot be released",
			"txHash", txHash, "stateID", bad, "lockedStateIDs", lockedIDs,
			"amount", amount, "delegate", delegate)
		return nil, errUnsettleableLock(bad)
	}

	return &ports.ZetoLockResult{
		TxHash:         txHash,
		ZetoLockRef:    txHash,
		LockedStateIDs: lockedIDs,
	}, nil
}

func (c *Client) TransferLocked(ctx context.Context, zetoLockRef string, to string, amount string) (string, error) {
	lockedInputs := strings.Split(zetoLockRef, ",")
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		To:       c.zetoTokenAddress,
		ABI:      zetoABI,
		Function: "transferLocked",
		Data: map[string]interface{}{
			"lockedInputs": lockedInputs,
			"delegate":     c.identity,
			"transfers": []map[string]interface{}{
				{"to": to, "amount": amount, "data": "0x"},
			},
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) Balance(ctx context.Context) (string, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "ptx_call",
		Params: []interface{}{map[string]interface{}{
			"type":     "private",
			"domain":   "zeto",
			"from":     c.identity,
			"to":       c.zetoTokenAddress,
			"abi":      zetoABI,
			"function": "balanceOf",
			"data":     map[string]interface{}{"account": c.identity},
		}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST paladin: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", rpcResp.Error
	}

	var result struct {
		TotalBalance string `json:"totalBalance"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return "", fmt.Errorf("unmarshal balance: %w", err)
	}
	return result.TotalBalance, nil
}
