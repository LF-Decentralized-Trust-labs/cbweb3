// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// This file instruments the Scenario B live happy-path harness so every step
// records the real on-chain artifacts the D12 plan promises (pp. 15, 38): the
// transaction hash returned by the API, plus the block number, gas used, and
// receipt status resolved from the Besu RPC via eth_getTransactionReceipt. The
// captured steps are written to a harness evidence file that
// tools/gen_evidence_bundles.py folds into the machine-readable evidence bundle
// — so tx_hash / block_number are populated from a live run instead of null.
//
// Capture is best-effort and never fails the functional test: if the RPC is
// unreachable, or a hash is a privacy-layer (Paladin/Zeto) id rather than an EVM
// tx, the on-chain fields are recorded as unresolved with a reason, never faked.

// ── Besu JSON-RPC (read-only receipt lookups) ────────────────────────────────

type besuRPC struct {
	url    string
	client *http.Client
}

func newBesuRPC(url string) *besuRPC {
	return &besuRPC{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type txReceipt struct {
	BlockNumber string `json:"blockNumber"`
	GasUsed     string `json:"gasUsed"`
	Status      string `json:"status"`
	BlockHash   string `json:"blockHash"`
}

var errNoReceipt = fmt.Errorf("no receipt (null result)")

func (r *besuRPC) call(method string, params []interface{}, out interface{}) error {
	if r == nil || r.url == "" {
		return fmt.Errorf("besu rpc not configured")
	}
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	resp, err := r.client.Post(r.url, "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode rpc response: %w (body: %s)", err, string(raw))
	}
	if env.Error != nil {
		return fmt.Errorf("rpc error %d: %s", env.Error.Code, env.Error.Message)
	}
	if len(env.Result) == 0 || string(env.Result) == "null" {
		return errNoReceipt
	}
	return json.Unmarshal(env.Result, out)
}

func (r *besuRPC) receipt(txHash string) (*txReceipt, error) {
	var rec txReceipt
	if err := r.call("eth_getTransactionReceipt", []interface{}{txHash}, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// isEVMTxHash reports whether s is a 0x-prefixed 32-byte hex string, i.e. a hash
// that eth_getTransactionReceipt can resolve. Paladin/Zeto private-transaction
// ids (often UUIDs) are not, so we skip the RPC for them.
func isEVMTxHash(s string) bool {
	if len(s) != 66 || s[:2] != "0x" {
		return false
	}
	for _, c := range s[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func hexToUint(h string) uint64 {
	h = strings.TrimPrefix(strings.TrimSpace(h), "0x")
	if h == "" {
		return 0
	}
	n, err := strconv.ParseUint(h, 16, 64)
	if err != nil {
		return 0
	}
	return n
}

// ── Evidence recorder ────────────────────────────────────────────────────────

type txRef struct {
	network string // "hub" | "spoke-b"
	label   string
	hash    string
}

type onChainTx struct {
	Label       string  `json:"label"`
	Network     string  `json:"network"`
	TxHash      string  `json:"tx_hash"`
	BlockNumber *uint64 `json:"block_number"`
	GasUsed     *uint64 `json:"gas_used"`
	Status      string  `json:"status,omitempty"`
	Resolved    bool    `json:"resolved"`
	Unresolved  string  `json:"unresolved,omitempty"`
}

type evidenceStep struct {
	ScenarioID    string      `json:"scenario_id"`
	Step          string      `json:"step"`
	HTTPStatus    *int        `json:"http_status"`
	TxHash        *string     `json:"tx_hash"`
	BlockNumber   *uint64     `json:"block_number"`
	GasUsed       *uint64     `json:"gas_used"`
	LatencyMS     *int64      `json:"latency_ms"`
	Passed        *bool       `json:"passed"`
	Skipped       bool        `json:"skipped,omitempty"`
	CorrelationID string      `json:"correlation_id,omitempty"`
	Txs           []onChainTx `json:"txs,omitempty"`
}

type evidenceRecorder struct {
	mu        sync.Mutex
	steps     []evidenceStep
	rpc       map[string]*besuRPC
	startedAt time.Time
}

func newEvidenceRecorder(rpcByNetwork map[string]string) *evidenceRecorder {
	rpc := make(map[string]*besuRPC, len(rpcByNetwork))
	for net, url := range rpcByNetwork {
		rpc[net] = newBesuRPC(url)
	}
	return &evidenceRecorder{rpc: rpc, startedAt: time.Now()}
}

func (e *evidenceRecorder) resolve(refs []txRef) []onChainTx {
	out := make([]onChainTx, 0, len(refs))
	for _, ref := range refs {
		if ref.hash == "" {
			continue
		}
		oc := onChainTx{Label: ref.label, Network: ref.network, TxHash: ref.hash}
		if !isEVMTxHash(ref.hash) {
			// Paladin/Zeto private-transaction id — settled in the privacy domain,
			// not on the EVM chain, so there is no receipt to resolve.
			oc.Unresolved = "privacy-layer id (Paladin/Zeto), not an EVM tx"
			out = append(out, oc)
			continue
		}
		rpc := e.rpc[ref.network]
		rec, err := rpc.receipt(ref.hash)
		switch {
		case err == errNoReceipt:
			oc.Unresolved = "no EVM receipt (unknown or pending tx)"
		case err != nil:
			oc.Unresolved = "rpc lookup failed: " + err.Error()
		default:
			bn := hexToUint(rec.BlockNumber)
			gu := hexToUint(rec.GasUsed)
			oc.BlockNumber = &bn
			oc.GasUsed = &gu
			oc.Resolved = true
			if rec.Status == "0x1" {
				oc.Status = "success"
			} else {
				oc.Status = "reverted"
			}
		}
		out = append(out, oc)
	}
	return out
}

func (e *evidenceRecorder) record(scenarioID, step string, httpStatus int, latency time.Duration, passed bool, corrID string, refs ...txRef) {
	txs := e.resolve(refs)

	s := evidenceStep{ScenarioID: scenarioID, Step: step, Passed: &passed, CorrelationID: corrID, Txs: txs}
	if httpStatus >= 0 {
		s.HTTPStatus = &httpStatus
	}
	ms := latency.Milliseconds()
	s.LatencyMS = &ms
	for i := range txs {
		if txs[i].Resolved {
			h := txs[i].TxHash
			s.TxHash = &h
			s.BlockNumber = txs[i].BlockNumber
			s.GasUsed = txs[i].GasUsed
			break
		}
	}
	if s.TxHash == nil {
		for i := range txs {
			if txs[i].TxHash != "" {
				h := txs[i].TxHash
				s.TxHash = &h
				break
			}
		}
	}

	e.mu.Lock()
	e.steps = append(e.steps, s)
	e.mu.Unlock()
}

func (e *evidenceRecorder) recordSkipped(scenarioID, step string) {
	e.mu.Lock()
	e.steps = append(e.steps, evidenceStep{ScenarioID: scenarioID, Step: step, Skipped: true})
	e.mu.Unlock()
}

// flush writes the harness evidence file consumed by gen_evidence_bundles.py.
// Destination: $EVIDENCE_DIR/<bundleKey>.json (default <repo>/evidence-bundles/_harness/).
func (e *evidenceRecorder) flush(bundleKey, suite string, passed bool) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	dir := os.Getenv("EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join(scenarioBRoot(), "..", "evidence-bundles", "_harness")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	resolved := 0
	for _, s := range e.steps {
		for _, tx := range s.Txs {
			if tx.Resolved {
				resolved++
			}
		}
	}
	doc := map[string]interface{}{
		"bundle_key":         bundleKey,
		"suite":              suite,
		"passed":             passed,
		"total_duration_ms":  time.Since(e.startedAt).Milliseconds(),
		"timestamp":          time.Now().UTC().Format("2006-01-02"),
		"on_chain_txs_found": resolved,
		"steps":              e.steps,
	}
	path := filepath.Join(dir, bundleKey+".json")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	return path, nil
}

func newCorrID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
