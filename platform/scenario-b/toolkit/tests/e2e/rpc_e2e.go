//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// envOr returns the env var value or the fallback when unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// hasCode reports whether the address has non-empty bytecode on-chain
// (eth_getCode != "0x").
func hasCode(t *testing.T, rpcURL, addr string) bool {
	t.Helper()
	reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getCode","params":["%s","latest"],"id":1}`, addr)
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		t.Fatalf("eth_getCode %s: %v", addr, err)
	}
	defer resp.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode eth_getCode: %v", err)
	}
	return out.Result != "" && out.Result != "0x"
}

// castCall runs `cast call <addr> <sig> <args...> --rpc-url <rpcURL>` and returns
// the trimmed stdout. Fatal on error.
func castCall(t *testing.T, rpcURL, addr, sig string, args ...string) string {
	t.Helper()
	cargs := append([]string{"call", addr, sig}, args...)
	cargs = append(cargs, "--rpc-url", rpcURL)
	out, err := exec.Command("cast", cargs...).CombinedOutput()
	if err != nil {
		t.Fatalf("cast call %s %s: %v\n%s", addr, sig, err, out)
	}
	return strings.TrimSpace(string(out))
}

// castCallErr is like castCall but returns the error instead of failing — used to
// assert that a call/tx reverts (e.g. a swap while the breaker is paused).
func castCallErr(rpcURL, key, addr, sig string, args ...string) error {
	cargs := []string{"send", addr, sig}
	cargs = append(cargs, args...)
	cargs = append(cargs, "--rpc-url", rpcURL, "--private-key", key)
	return exec.Command("cast", cargs...).Run()
}

// castSend runs `cast send <addr> <sig> <args...> --rpc-url <rpcURL> --private-key
// <key>`. Fatal on error.
func castSend(t *testing.T, rpcURL, key, addr, sig string, args ...string) {
	t.Helper()
	cargs := append([]string{"send", addr, sig}, args...)
	cargs = append(cargs, "--rpc-url", rpcURL, "--private-key", key)
	if out, err := exec.Command("cast", cargs...).CombinedOutput(); err != nil {
		t.Fatalf("cast send %s %s: %v\n%s", addr, sig, err, out)
	}
}
