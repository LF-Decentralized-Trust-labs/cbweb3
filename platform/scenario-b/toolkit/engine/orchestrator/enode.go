package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// EnodeReader returns the node's enode URL. Injectable so found-spoke is
// testable without a live Besu.
type EnodeReader func(ctx context.Context, rpcURL string) (string, error)

// adminNodeInfoEnode queries admin_nodeInfo over JSON-RPC and returns the enode.
func adminNodeInfoEnode(ctx context.Context, rpcURL string) (string, error) {
	body := []byte(`{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			Enode string `json:"enode"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Result.Enode == "" {
		return "", fmt.Errorf("admin_nodeInfo returned empty enode from %s", rpcURL)
	}
	return out.Result.Enode, nil
}
