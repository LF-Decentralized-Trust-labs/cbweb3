// SPDX-License-Identifier: Apache-2.0

//go:build perf

// Package perf holds the toolkit-native performance baseline (build tag `perf`,
// so it does not run in the default `go test ./...`). It measures quote and swap
// latency (p95) against a stack provisioned by the toolkit, using `cast`/RPC and
// the Go stdlib (`time`/`sort`). Informative only — no pass/fail threshold gate
// in this phase (production-grade gating is deferred). SKIPS WITH A WARNING when
// the environment is absent — never a false green.
//
//	Run: go test -tags perf ./tests/perf/... -run TestBaseline
//	Requires: cast, and:
//	  CBWEB3B_PERF_HUB_RPC = hub RPC URL
//	  CBWEB3B_PERF_AMM     = AMM address
//	  CBWEB3B_PERF_TOKEN_A / _TOKEN_B (quote args); optional CBWEB3B_PERF_N (iterations)
package perf

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"testing"
	"time"
)

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("perf skipped: %q not found in PATH", name)
	}
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("perf skipped: %s not set (provide a toolkit-provisioned stack)", key)
	}
	return v
}

// p95 returns the 95th-percentile duration in milliseconds.
func p95ms(durs []time.Duration) float64 {
	if len(durs) == 0 {
		return 0
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	idx := int(float64(len(durs))*0.95+0.999999) - 1 // ceil(0.95*N)-1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(durs) {
		idx = len(durs) - 1
	}
	return float64(durs[idx].Microseconds()) / 1000.0
}

// TestBaseline measures quote and swap p95 latency against the toolkit stack.
func TestBaseline(t *testing.T) {
	requireTool(t, "cast")
	rpc := requireEnv(t, "CBWEB3B_PERF_HUB_RPC")
	amm := requireEnv(t, "CBWEB3B_PERF_AMM")

	n := 50
	if v := os.Getenv("CBWEB3B_PERF_N"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}

	// quote: getAmountOut(amountIn, reserveIn, reserveOut, feeBps) is a pure view;
	// measure the round-trip latency of the read against the live node.
	quoteArgs := []string{"1000000000000000000", "1000000000000000000000", "1000000000000000000000", "30"}
	quotes := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		out, err := exec.Command("cast", append([]string{"call", amm,
			"getAmountOut(uint256,uint256,uint256,uint256)"}, append(quoteArgs, "--rpc-url", rpc)...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("quote call %d: %v\n%s", i, err, out)
		}
		quotes = append(quotes, time.Since(start))
	}
	t.Logf("quote_p95_ms=%.2f (n=%d)", p95ms(quotes), n)

	// swap: optional (needs a verified swapper key + tokens). When unset, only the
	// quote baseline is reported.
	swapKey := os.Getenv("CBWEB3B_PERF_SWAP_KEY")
	tokenA := os.Getenv("CBWEB3B_PERF_TOKEN_A")
	tokenB := os.Getenv("CBWEB3B_PERF_TOKEN_B")
	to := os.Getenv("CBWEB3B_PERF_SWAP_TO")
	if swapKey == "" || tokenA == "" || tokenB == "" || to == "" {
		t.Logf("swap baseline skipped: set CBWEB3B_PERF_SWAP_KEY/_TOKEN_A/_TOKEN_B/_SWAP_TO to measure swap p95")
		return
	}
	swaps := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		out, err := exec.Command("cast", "send", amm,
			"swapTokensForExactTokens(address,address,uint256,uint256,address)",
			tokenA, tokenB, "1", "1000000000000000000", to,
			"--rpc-url", rpc, "--private-key", swapKey).CombinedOutput()
		if err != nil {
			t.Fatalf("swap send %d: %v\n%s", i, err, out)
		}
		swaps = append(swaps, time.Since(start))
	}
	t.Logf("swap_p95_ms=%.2f (n=%d)", p95ms(swaps), n)
}
