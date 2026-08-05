// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EthSyncing reports a node's sync state: whether it is still syncing and its
// current block height. Injectable so the join wait-sync gate is testable
// without a live Besu.
type EthSyncing func(ctx context.Context, rpcURL string) (syncing bool, block uint64, err error)

// waitSync blocks until the node reports not-syncing with a non-zero block, or
// the timeout elapses. It is the join readiness gate (FR-004): a node that is up
// on RPC but not yet synced must not be treated as ready.
func waitSync(ctx context.Context, rpcURL string, timeout, poll time.Duration, probe EthSyncing) error {
	deadline := time.Now().Add(timeout)
	var lastSyncing bool
	var lastBlock uint64
	var lastErr error
	for {
		syncing, block, err := probe(ctx, rpcURL)
		lastSyncing, lastBlock, lastErr = syncing, block, err
		if err == nil && !syncing && block > 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("wait-sync: node %s not synced within %s (syncing=%v block=%d lastErr=%v)",
				rpcURL, timeout, lastSyncing, lastBlock, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}

// ethSyncing is the default EthSyncing: it queries eth_syncing (false when
// caught up, or a progress object while syncing) and eth_blockNumber.
func ethSyncing(ctx context.Context, rpcURL string) (bool, uint64, error) {
	var syncRaw json.RawMessage
	if err := rpcCall(ctx, rpcURL, "eth_syncing", &syncRaw); err != nil {
		return false, 0, err
	}
	// eth_syncing returns literal false when caught up, otherwise a progress object.
	syncing := strings.TrimSpace(string(syncRaw)) != "false"

	var blockHex string
	if err := rpcCall(ctx, rpcURL, "eth_blockNumber", &blockHex); err != nil {
		return syncing, 0, err
	}
	block, err := strconv.ParseUint(strings.TrimPrefix(blockHex, "0x"), 16, 64)
	if err != nil {
		return syncing, 0, fmt.Errorf("wait-sync: bad blockNumber %q: %w", blockHex, err)
	}
	return syncing, block, nil
}

// rpcCall performs a JSON-RPC call and decodes result into out.
func rpcCall(ctx context.Context, rpcURL, method string, out any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": []any{}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("%s: %s", method, envelope.Error.Message)
	}
	return json.Unmarshal(envelope.Result, out)
}
