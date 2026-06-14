// Package paladin provides a lightweight JSON-RPC client for querying private
// transaction state from a Paladin node. Used by the supervisor decrypt endpoint.
package paladin

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DecryptedTx holds the decrypted fields extracted from a Zeto private state.
type DecryptedTx struct {
	Amount   string
	Currency string
	Sender   string
	Receiver string
}

type Client struct {
	url        string
	httpClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type rpcReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("paladin rpc error %d: %s", e.Code, e.Message) }

func (c *Client) call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(rpcReq{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST paladin: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result rpcResp
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal rpc response: %w", err)
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return result.Result, nil
}

// GetDecryptedTx fetches the confirmed private states for a Paladin transaction
// and extracts the Zeto token fields (amount, sender/owner, receiver).
//
// txID must be the Paladin internal transaction UUID (e.g. "3e5f16b1-071c-4f8a-9aa3-8f289782bb26"),
// as returned by ptx_sendTransaction and stored in HTLCRecord.ZetoTxHash. This is NOT the same as
// the on-chain zeto_lock_ref bytes32 field, which is sha256(state_ids) and cannot be used here.
func (c *Client) GetDecryptedTx(ctx context.Context, txID string) (*DecryptedTx, error) {
	// Strip 0x prefix if present — Paladin expects plain UUIDs without prefix.
	id := strings.TrimPrefix(txID, "0x")

	raw, err := c.call(ctx, "ptx_getStateReceipt", []interface{}{id})
	if err != nil {
		return nil, fmt.Errorf("ptx_getStateReceipt(%s): %w", id, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("no state receipt for tx %s", id)
	}

	var receipt struct {
		Confirmed []struct {
			ID   string                 `json:"id"`
			Data map[string]interface{} `json:"data"`
		} `json:"confirmed"`
		Spent []struct {
			ID   string                 `json:"id"`
			Data map[string]interface{} `json:"data"`
		} `json:"spent"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("unmarshal state receipt: %w", err)
	}

	// Collect all states (confirmed outputs + spent inputs) for field extraction.
	states := receipt.Confirmed
	states = append(states, receipt.Spent...)
	if len(states) == 0 {
		return nil, fmt.Errorf("tx %s has no readable states (not accessible from this node)", id)
	}

	dec := &DecryptedTx{Currency: "tCeBM"}

	// Extract fields from the first state that has usable data.
	// Zeto state schemas use: amount/value, owner/sender, receiver/to.
	for _, s := range states {
		if s.Data == nil {
			continue
		}
		if dec.Amount == "" {
			dec.Amount = firstString(s.Data, "amount", "value")
		}
		if dec.Sender == "" {
			dec.Sender = firstString(s.Data, "owner", "sender", "from")
		}
		if dec.Receiver == "" {
			dec.Receiver = firstString(s.Data, "receiver", "to", "delegate")
		}
	}
	if dec.Amount == "" && dec.Sender == "" {
		return nil, fmt.Errorf("tx %s state data does not contain recognisable Zeto fields", id)
	}
	return dec, nil
}

// UUIDFromBytes32 extracts the Paladin transaction UUID from an on-chain zeto_lock_ref bytes32.
// The first 16 bytes encode the UUID as raw bytes (written by uuidToRawBytes in the payment-orchestrator).
// Input hex32 may have or omit the "0x" prefix and must be at least 32 hex chars (16 bytes).
func UUIDFromBytes32(hex32 string) (string, error) {
	s := strings.TrimPrefix(hex32, "0x")
	if len(s) < 32 {
		return "", fmt.Errorf("zeto_lock_ref %q too short to contain a UUID", hex32)
	}
	b, err := hex.DecodeString(s[:32])
	if err != nil {
		return "", fmt.Errorf("decode zeto_lock_ref hex: %w", err)
	}
	// RFC 4122 UUID: 8-4-4-4-12 hex groups
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func firstString(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if n, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", n)
			}
		}
	}
	return ""
}
