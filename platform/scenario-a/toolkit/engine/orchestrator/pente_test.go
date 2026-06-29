// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakePaladin routes pgroup_*/ptx_* JSON-RPC calls for tests.
func fakePaladin(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "pgroup_createGroup":
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"id":"0xabc","genesisTransaction":"tx-genesis","contractAddress":null}}`)
		case "pgroup_getGroupById":
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"id":"0xabc","contractAddress":"0xCtx"}}`)
		case "pgroup_sendTransaction":
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"tx-deploy"}`)
		case "ptx_getTransactionFull":
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"receipt":{"success":true,"contractAddress":"0xFXA"}}}`)
		default:
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"unknown method"}}`)
		}
	}))
}

func TestPaladinIdentity(t *testing.T) {
	if got := paladinIdentity("spoke-brl-cb"); got != "funded_operator@spoke-brl-cb" {
		t.Errorf("paladinIdentity = %q", got)
	}
}

func TestCreatePenteGroup_ReturnsGroupID(t *testing.T) {
	srv := fakePaladin(t)
	defer srv.Close()
	id, err := createPenteGroup(context.Background(), srv.URL, "fx-test",
		[]string{"funded_operator@spoke-brl-cb", "funded_operator@spoke-brl-bank-itau"})
	if err != nil {
		t.Fatalf("createPenteGroup: %v", err)
	}
	if id != "0xabc" {
		t.Errorf("group id = %q; want 0xabc", id)
	}
}

func TestWaitPenteGroupReady_OK(t *testing.T) {
	srv := fakePaladin(t)
	defer srv.Close()
	if err := waitPenteGroupReady(context.Background(), srv.URL, "0xabc", 2*time.Second); err != nil {
		t.Errorf("waitPenteGroupReady: %v", err)
	}
}

func TestDeployFXAInPente_ReturnsAddress(t *testing.T) {
	srv := fakePaladin(t)
	defer srv.Close()

	// Minimal FXAgreement artifact with a constructor and bytecode.
	dir := t.TempDir()
	artifact := filepath.Join(dir, "FXAgreement.json")
	os.WriteFile(artifact, []byte(`{"abi":[{"type":"constructor","inputs":[{"name":"_identityRegistry","type":"address"}]}],"bytecode":{"object":"0x6080"}}`), 0o644)

	addr, err := deployFXAInPente(context.Background(), srv.URL, "0xabc",
		"funded_operator@spoke-brl-bank-itau", artifact, "0xREG")
	if err != nil {
		t.Fatalf("deployFXAInPente: %v", err)
	}
	if addr != "0xFXA" {
		t.Errorf("FXA address = %q; want 0xFXA", addr)
	}
}

func TestDeployFXAInPente_ErrorsOnMissingArtifact(t *testing.T) {
	srv := fakePaladin(t)
	defer srv.Close()
	_, err := deployFXAInPente(context.Background(), srv.URL, "0xabc", "id", "/nonexistent.json", "0xREG")
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

func TestCreatePenteJoinStep_Check_StateDriven(t *testing.T) {
	dir := t.TempDir()
	step := newCreatePenteJoinStep("spoke-brl", "bank-itau", dir, "http://x", 0)
	if done, _ := step.Check(context.Background()); done {
		t.Error("Check should be false with no group id")
	}
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("PENTE_CONTEXT_GROUP_ID=0xabc\n"), 0o644)
	if done, _ := step.Check(context.Background()); !done {
		t.Error("Check should be true once PENTE_CONTEXT_GROUP_ID is set")
	}
}

func TestDeployFXAJoinStep_Check_StateDriven(t *testing.T) {
	dir := t.TempDir()
	step := newDeployFXAJoinStep("spoke-brl", "bank-itau", dir, "http://x", "/a.json", "0xREG", 0)
	if done, _ := step.Check(context.Background()); done {
		t.Error("Check should be false before deploy")
	}
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("FX_AGREEMENT_DEPLOYED_AT=0xFXA\n"), 0o644)
	if done, _ := step.Check(context.Background()); !done {
		t.Error("Check should be true once FX_AGREEMENT_DEPLOYED_AT is set")
	}
}
