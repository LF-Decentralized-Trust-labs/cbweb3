// Package paladin provides the real ZetoOperator implementation that talks to
// Paladin sidecar nodes via their JSON-RPC HTTP API (POST /api/v1).
package paladin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.ZetoOperator = (*Client)(nil)

// Client implements ports.ZetoOperator by calling the Paladin sidecar HTTP API.
type Client struct {
	baseURL    string
	identity   string
	httpClient *http.Client
	logger     *slog.Logger
}

// ClientConfig holds the configuration for a Paladin client.
type ClientConfig struct {
	BaseURL  string // e.g. "http://127.0.0.1:8548"
	Identity string // Paladin identity, e.g. "funded_operator@spoke-a-cb"
}

// NewClient creates a new Paladin client.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client {
	return &Client{
		baseURL:  cfg.BaseURL,
		identity: cfg.Identity,
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

type paladinTx struct {
	Type     string      `json:"type"`
	Domain   string      `json:"domain"`
	From     string      `json:"from"`
	To       string      `json:"to,omitempty"`
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1", bytes.NewReader(body))
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
				Method:  "ptx_getTransaction",
				Params:  []interface{}{txID},
			}
			body, _ := json.Marshal(reqBody)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1", bytes.NewReader(body))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.httpClient.Do(req)
			if err != nil {
				continue
			}
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

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
		Function: "mint",
		Data: map[string]interface{}{
			"to":     to,
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
		Function: "transfer",
		Data: map[string]interface{}{
			"transfers": []map[string]string{
				{"to": to, "amount": amount},
			},
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) Lock(ctx context.Context, amount string, delegate string) (*ports.ZetoLockResult, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		Function: "lock",
		Data: map[string]interface{}{
			"amount":   amount,
			"delegate": delegate,
		},
	}
	txHash, err := c.sendTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ports.ZetoLockResult{
		TxHash:      txHash,
		ZetoLockRef: txHash,
	}, nil
}

func (c *Client) Unlock(ctx context.Context, zetoLockRef string) (string, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		Function: "unlock",
		Data: map[string]interface{}{
			"lockId": zetoLockRef,
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) TransferLocked(ctx context.Context, zetoLockRef string, to string, _ string) (string, error) {
	tx := paladinTx{
		Type:     "private",
		Domain:   "zeto",
		From:     c.identity,
		Function: "transferLocked",
		Data: map[string]interface{}{
			"lockId": zetoLockRef,
			"transfers": []map[string]string{
				{"to": to},
			},
		},
	}
	return c.sendTx(ctx, tx)
}

func (c *Client) Balance(ctx context.Context, identity string) (string, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "ptx_call",
		Params: []interface{}{map[string]interface{}{
			"type":     "private",
			"domain":   "zeto",
			"from":     identity,
			"function": "getBalance",
			"data":     map[string]interface{}{},
		}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1", bytes.NewReader(body))
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

	var balance string
	if err := json.Unmarshal(rpcResp.Result, &balance); err != nil {
		return "", fmt.Errorf("unmarshal balance: %w", err)
	}
	return balance, nil
}
