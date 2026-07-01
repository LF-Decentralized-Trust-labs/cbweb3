// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// fxaFake is a stateful fake Paladin for setupBilateralFXAContext: it resolves identities to
// addresses, records pgroup_sendTransaction calls, and hands deploys sequential contract
// addresses via their receipts.
type fxaFake struct {
	resolve    map[string]string // identity -> address
	deployAddr []string          // addresses handed to successive deploys
	deployIdx  int
	sends      []map[string]any  // recorded pgroup_sendTransaction inputs (in order)
	txIsDeploy map[string]string // txID -> contractAddress ("" for non-deploy)
	seq        int
}

func (f *fxaFake) server(t *testing.T) *httptest.Server {
	t.Helper()
	if f.txIsDeploy == nil {
		f.txIsDeploy = map[string]string{}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		reply := func(result any) {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		}
		switch req.Method {
		case "ptx_resolveVerifier":
			var identity string
			_ = json.Unmarshal(req.Params[0], &identity)
			addr := f.resolve[identity]
			if addr == "" {
				addr = "0xUNKNOWN"
			}
			reply(addr)
		case "pgroup_sendTransaction":
			var tx map[string]any
			_ = json.Unmarshal(req.Params[0], &tx)
			f.sends = append(f.sends, tx)
			f.seq++
			txID := fmt.Sprintf("tx-%d", f.seq)
			if _, isDeploy := tx["bytecode"]; isDeploy {
				addr := ""
				if f.deployIdx < len(f.deployAddr) {
					addr = f.deployAddr[f.deployIdx]
				}
				f.deployIdx++
				f.txIsDeploy[txID] = addr
			} else {
				f.txIsDeploy[txID] = ""
			}
			reply(txID)
		case "ptx_getTransactionFull":
			// Base receipt confirms success; Pente deploys do NOT carry contractAddress here.
			reply(map[string]any{"receipt": map[string]any{"success": true}})
		case "ptx_getDomainReceipt":
			var txID string // params: [domain, txID]
			_ = json.Unmarshal(req.Params[1], &txID)
			reply(map[string]any{"receipt": map[string]any{"contractAddress": f.txIsDeploy[txID]}})
		default:
			reply(nil)
		}
	}))
}

func writeArtifact(t *testing.T, dir, name, ctorInput string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	body := fmt.Sprintf(`{"abi":[{"type":"constructor","inputs":[%s]}],"bytecode":{"object":"0x60016000"}}`, ctorInput)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return path
}

func TestSetupBilateralFXAContext(t *testing.T) {
	f := &fxaFake{
		resolve:    map[string]string{"cb@brl": "0xCB", "itau@brl": "0xITAU"},
		deployAddr: []string{"0xREG", "0xFXA"}, // registry first, then FXAgreement
	}
	srv := f.server(t)
	defer srv.Close()

	dir := t.TempDir()
	regArtifact := writeArtifact(t, dir, "IdentityRegistry.json", `{"name":"admin","type":"address"}`)
	fxaArtifact := writeArtifact(t, dir, "FXAgreement.json", `{"name":"_identityRegistry","type":"address"}`)

	members := []penteMember{
		{Identity: "cb@brl", Name: "Central Bank", Role: roleCentralBank},
		{Identity: "itau@brl", Name: "Itau", Role: roleCommercialBank},
	}
	reg, fxa, err := setupBilateralFXAContext(context.Background(), srv.URL, "0xGROUP", "cb@brl", regArtifact, fxaArtifact, members)
	if err != nil {
		t.Fatalf("setupBilateralFXAContext: %v", err)
	}
	if reg != "0xREG" || fxa != "0xFXA" {
		t.Fatalf("addresses: reg=%q fxa=%q, want 0xREG / 0xFXA", reg, fxa)
	}

	// Expect exactly: registry deploy, 2 registrations, FXAgreement deploy — in order.
	if len(f.sends) != 4 {
		t.Fatalf("expected 4 pgroup_sendTransaction calls, got %d", len(f.sends))
	}

	// 1) registry deploy: admin = deployer's resolved address.
	if _, ok := f.sends[0]["bytecode"]; !ok {
		t.Errorf("call[0] should be a deploy (has bytecode)")
	}
	if in := f.sends[0]["input"].(map[string]any); in["admin"] != "0xCB" {
		t.Errorf("registry admin = %v, want deployer addr 0xCB", in["admin"])
	}

	// 2) register central bank as CENTRAL_BANK(3).
	assertRegister(t, f.sends[1], "0xCB", "3")
	// 3) register commercial bank as COMMERCIAL_BANK(4).
	assertRegister(t, f.sends[2], "0xITAU", "4")

	// 4) FXAgreement deploy wired to the in-group registry address.
	if _, ok := f.sends[3]["bytecode"]; !ok {
		t.Errorf("call[3] should be a deploy (has bytecode)")
	}
	if in := f.sends[3]["input"].(map[string]any); in["_identityRegistry"] != "0xREG" {
		t.Errorf("FXAgreement _identityRegistry = %v, want in-group registry 0xREG", in["_identityRegistry"])
	}

	// Routing sanity on a representative call.
	if f.sends[0]["group"] != "0xGROUP" || f.sends[0]["from"] != "cb@brl" {
		t.Errorf("deploy routed wrong: group=%v from=%v", f.sends[0]["group"], f.sends[0]["from"])
	}
}

// TestSetupBilateralFXAContext_SharedKey covers the local case where both members resolve to
// the SAME address (shared dev operator key): they must be registered ONCE, with the governing
// role (CENTRAL_BANK), so canGovern() holds.
func TestSetupBilateralFXAContext_SharedKey(t *testing.T) {
	f := &fxaFake{
		resolve:    map[string]string{"cb@brl": "0xSAME", "itau@brl": "0xSAME"},
		deployAddr: []string{"0xREG", "0xFXA"},
	}
	srv := f.server(t)
	defer srv.Close()
	dir := t.TempDir()
	regArtifact := writeArtifact(t, dir, "IdentityRegistry.json", `{"name":"admin","type":"address"}`)
	fxaArtifact := writeArtifact(t, dir, "FXAgreement.json", `{"name":"_identityRegistry","type":"address"}`)
	members := []penteMember{
		{Identity: "cb@brl", Name: "Central Bank", Role: roleCentralBank},
		{Identity: "itau@brl", Name: "Itau", Role: roleCommercialBank},
	}
	if _, _, err := setupBilateralFXAContext(context.Background(), srv.URL, "0xGROUP", "cb@brl", regArtifact, fxaArtifact, members); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// registry deploy + ONE register + FXAgreement deploy = 3 sends (not 4).
	if len(f.sends) != 3 {
		t.Fatalf("expected 3 sends (deduped register), got %d", len(f.sends))
	}
	assertRegister(t, f.sends[1], "0xSAME", "3") // CENTRAL_BANK wins the collision
}

func assertRegister(t *testing.T, tx map[string]any, wantAccount, wantRole string) {
	t.Helper()
	fn, _ := tx["function"].(map[string]any)
	if fn == nil || fn["name"] != "registerParticipant" {
		t.Errorf("expected registerParticipant, got function %v", tx["function"])
		return
	}
	in := tx["input"].(map[string]any)
	if in["account"] != wantAccount {
		t.Errorf("register account = %v, want %s", in["account"], wantAccount)
	}
	if in["role"] != wantRole {
		t.Errorf("register role = %v, want %s", in["role"], wantRole)
	}
}
