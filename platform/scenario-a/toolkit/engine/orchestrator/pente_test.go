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
		case "ptx_resolveVerifier":
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"0xVERIFIER"}`)
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

func TestIsTransientTransportErr(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"PD011206: TRANSPORT grpc returned error: PD030015: ... Unavailable", true},
		{"connection refused", true},
		{"transport: authentication handshake failed", true},
		// A node-identity mismatch is permanent — must NOT be retried.
		{"PD011206: ... PD030011: the TLS identity of the node 'x' does not match", false},
		{"PD012345: some unrelated domain error", false},
	}
	for _, c := range cases {
		if got := isTransientTransportErr(&penteRPCError{Message: c.msg}); got != c.want {
			t.Errorf("isTransientTransportErr(%q) = %v; want %v", c.msg, got, c.want)
		}
	}
	if isTransientTransportErr(nil) {
		t.Error("nil error must not be transient")
	}
}

func TestWaitPentePeersReady_ResolvesWhenUp(t *testing.T) {
	srv := fakePaladin(t)
	defer srv.Close()
	err := waitPentePeersReady(context.Background(), srv.URL,
		[]string{"funded_operator@spoke-brl-cb", "funded_operator@spoke-brl-bank-itau"}, time.Millisecond, nil)
	if err != nil {
		t.Fatalf("waitPentePeersReady: %v", err)
	}
}

func TestWaitPentePeersReady_SurfacesPermanentError(t *testing.T) {
	// A node-identity mismatch (PD030011) must fail fast, not spin until the deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"PD011206: TRANSPORT grpc returned error: PD030011: the TLS identity of the node 'paladin-x' does not match the expected node 'x'"}}`)
	}))
	defer srv.Close()
	err := waitPentePeersReady(context.Background(), srv.URL, []string{"funded_operator@x"}, time.Millisecond, nil)
	if err == nil {
		t.Fatal("expected permanent error to be surfaced")
	}
}

func TestWaitPentePeersReady_TimesOutOnTransient(t *testing.T) {
	// A peer that never connects (transient error forever) must time out via ctx.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"PD011206: TRANSPORT grpc returned error: connection refused"}}`)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := waitPentePeersReady(ctx, srv.URL, []string{"funded_operator@x"}, time.Millisecond, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCreatePenteJoinStep_Check_StateDriven(t *testing.T) {
	dir := t.TempDir()
	step := newCreatePenteJoinStep("spoke-brl", "bank-itau", dir, "http://x", 0, 0, nil)
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
