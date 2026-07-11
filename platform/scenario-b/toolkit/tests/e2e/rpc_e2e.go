//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

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
