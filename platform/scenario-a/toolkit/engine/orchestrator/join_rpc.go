// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

// sharedJoinHTTPClient is reused across mode:join RPC steps so HTTP keep-alive
// connections to the same Besu RPC endpoint are pooled instead of each step
// opening fresh connections. http.Client is safe for concurrent use.
var sharedJoinHTTPClient = &http.Client{Timeout: 5 * time.Second}

// jsonRPCRequest is a minimal JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

// jsonRPCResponse is a minimal JSON-RPC 2.0 response envelope with raw result.
type jsonRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// rpcCall performs a single JSON-RPC POST to rpcURL and returns the raw result.
func rpcCall(ctx context.Context, client *http.Client, rpcURL, method string, params ...any) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
	}
	body, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()

	var parsed jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", method, err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("%s RPC error %d: %s", method, parsed.Error.Code, parsed.Error.Message)
	}
	return parsed.Result, nil
}

// ethBlockNumber returns the current block height via eth_blockNumber.
func ethBlockNumber(ctx context.Context, client *http.Client, rpcURL string) (uint64, error) {
	raw, err := rpcCall(ctx, client, rpcURL, "eth_blockNumber")
	if err != nil {
		return 0, err
	}
	var hexStr string
	if err := json.Unmarshal(raw, &hexStr); err != nil {
		return 0, fmt.Errorf("decode block number: %w", err)
	}
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimPrefix(hexStr, "0x"), 16); !ok {
		return 0, fmt.Errorf("parse block number %q", hexStr)
	}
	return n.Uint64(), nil
}

// netPeerCount returns the number of connected P2P peers via net_peerCount.
// Used to distinguish a node that is actually following the spoke (peers >= 1)
// from a misconfigured singleton that produces its own chain in isolation.
func netPeerCount(ctx context.Context, client *http.Client, rpcURL string) (uint64, error) {
	raw, err := rpcCall(ctx, client, rpcURL, "net_peerCount")
	if err != nil {
		return 0, err
	}
	var hexStr string
	if err := json.Unmarshal(raw, &hexStr); err != nil {
		return 0, fmt.Errorf("decode peer count: %w", err)
	}
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimPrefix(hexStr, "0x"), 16); !ok {
		return 0, fmt.Errorf("parse peer count %q", hexStr)
	}
	return n.Uint64(), nil
}

// qbftProposeValidatorVote casts a QBFT validator vote on the validator at rpcURL.
// vote=true proposes adding the address as a validator.
func qbftProposeValidatorVote(ctx context.Context, client *http.Client, rpcURL, address string, vote bool) error {
	raw, err := rpcCall(ctx, client, rpcURL, "qbft_proposeValidatorVote", address, vote)
	if err != nil {
		return err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("decode vote result: %w", err)
	}
	if !ok {
		return fmt.Errorf("qbft_proposeValidatorVote returned false for %s", address)
	}
	return nil
}

// qbftGetValidators returns the current validator set at the given block tag.
func qbftGetValidators(ctx context.Context, client *http.Client, rpcURL, blockTag string) ([]string, error) {
	raw, err := rpcCall(ctx, client, rpcURL, "qbft_getValidatorsByBlockNumber", blockTag)
	if err != nil {
		return nil, err
	}
	var validators []string
	if err := json.Unmarshal(raw, &validators); err != nil {
		return nil, fmt.Errorf("decode validator set: %w", err)
	}
	return validators, nil
}

// containsAddressFold reports whether addr is in set (case-insensitive hex compare).
func containsAddressFold(set []string, addr string) bool {
	for _, s := range set {
		if strings.EqualFold(s, addr) {
			return true
		}
	}
	return false
}

// adminNodeInfoEnode calls admin_nodeInfo and returns the node's enode string.
func adminNodeInfoEnode(ctx context.Context, client *http.Client, rpcURL string) (string, error) {
	raw, err := rpcCall(ctx, client, rpcURL, "admin_nodeInfo")
	if err != nil {
		return "", err
	}
	var info struct {
		Enode string `json:"enode"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", fmt.Errorf("decode admin_nodeInfo: %w", err)
	}
	if info.Enode == "" {
		return "", fmt.Errorf("admin_nodeInfo: empty enode")
	}
	return info.Enode, nil
}

// resolveJoinerAddress queries the joiner node's admin_nodeInfo and derives its
// QBFT validator address. Address derivation is shared with the bundle emitter
// (bundle.EnodeToValidatorAddress) so both sides agree on the same semantics.
func resolveJoinerAddress(ctx context.Context, client *http.Client, joinerRPC string) (string, error) {
	enode, err := adminNodeInfoEnode(ctx, client, joinerRPC)
	if err != nil {
		return "", err
	}
	return bundle.EnodeToValidatorAddress(enode)
}
