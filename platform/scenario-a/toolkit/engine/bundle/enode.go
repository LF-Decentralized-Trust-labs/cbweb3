// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// EnodeToValidatorAddress derives the QBFT validator EVM address from an enode URI.
// The enode embeds the node's uncompressed secp256k1 public key (128 hex chars,
// no 0x04 prefix); the validator address is the last 20 bytes of its keccak256.
// Exported so the mode:join engine derives addresses with identical semantics.
func EnodeToValidatorAddress(enode string) (string, error) {
	if !strings.HasPrefix(enode, "enode://") {
		return "", fmt.Errorf("enode: missing enode:// prefix")
	}
	rest := strings.TrimPrefix(enode, "enode://")
	at := strings.Index(rest, "@")
	if at < 0 {
		return "", fmt.Errorf("enode: missing @ separator")
	}
	pubHex := rest[:at]
	if len(pubHex) != 128 {
		return "", fmt.Errorf("enode: expected 128 hex chars of pubkey, got %d", len(pubHex))
	}
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil {
		return "", fmt.Errorf("enode: decode pubkey: %w", err)
	}
	pub, err := crypto.UnmarshalPubkey(append([]byte{0x04}, pubBytes...))
	if err != nil {
		return "", fmt.Errorf("enode: unmarshal pubkey: %w", err)
	}
	return crypto.PubkeyToAddress(*pub).Hex(), nil
}

// parseAndRewriteEnode extracts the enode-id from raw (e.g. "enode://<id>@<host>:<port>")
// and rebuilds the enode using advertisedHost and p2pPort from the manifest.
// This ensures the bundle never exposes internal container addresses.
func parseAndRewriteEnode(raw, advertisedHost string, p2pPort int) (string, error) {
	if p2pPort <= 0 {
		return "", fmt.Errorf("%w: p2pPort must be > 0", ErrEnodeUnavailable)
	}
	if !strings.HasPrefix(raw, "enode://") {
		return "", fmt.Errorf("%w: missing enode:// prefix in %q", ErrEnodeUnavailable, raw)
	}
	atIdx := strings.LastIndex(raw, "@")
	if atIdx < 0 {
		return "", fmt.Errorf("%w: missing @ separator in %q", ErrEnodeUnavailable, raw)
	}
	enodePart := raw[:atIdx] // "enode://<id>"
	return fmt.Sprintf("%s@%s:%d", enodePart, advertisedHost, p2pPort), nil
}

// BesuEnodeProvider calls Besu's admin_nodeInfo JSON-RPC endpoint to retrieve the enode.
type BesuEnodeProvider struct {
	rpcURL     string
	httpClient *http.Client
}

// NewBesuEnodeProvider creates a provider that calls rpcURL via admin_nodeInfo.
// A nil httpClient uses a default client with a 10 s timeout.
func NewBesuEnodeProvider(rpcURL string, httpClient *http.Client) EnodeProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &BesuEnodeProvider{rpcURL: rpcURL, httpClient: httpClient}
}

type nodeInfoResponse struct {
	Result struct {
		Enode string `json:"enode"`
	} `json:"result"`
}

// NodeInfo calls admin_nodeInfo and returns the raw enode string.
func (p *BesuEnodeProvider) NodeInfo(ctx context.Context) (string, error) {
	body := []byte(`{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: build request: %v", ErrEnodeUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEnodeUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: HTTP %d from Besu", ErrEnodeUnavailable, resp.StatusCode)
	}

	var result nodeInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("%w: decode response: %v", ErrEnodeUnavailable, err)
	}
	if result.Result.Enode == "" {
		return "", fmt.Errorf("%w: result.enode absent in admin_nodeInfo response", ErrEnodeUnavailable)
	}
	return result.Result.Enode, nil
}
