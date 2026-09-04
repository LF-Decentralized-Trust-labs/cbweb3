// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resolverStub answers ptx_resolveVerifier for the identities it knows and returns Paladin's real
// PD012100 error for everything else — the response a node gives for an identity that lives on
// another node, which is every cross-spoke party in an FX agreement.
type resolverStub struct {
	known map[string]string
	calls int
	// failWith replaces the default PD012100 answer for unknown identities, so the tests can
	// distinguish "this Paladin has never heard of that node" from every other failure.
	failWith *rpcError
	// emptyResult answers with a successful envelope carrying an empty address.
	emptyResult bool
}

func (s *resolverStub) handler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Method != "ptx_resolveVerifier" {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1,
			"error": &rpcError{Code: -32601, Message: "method not found: " + req.Method}})
		return
	}
	s.calls++
	var identity string
	_ = json.Unmarshal(req.Params[0], &identity)
	if addr, ok := s.known[identity]; ok {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": addr})
		return
	}
	if s.emptyResult {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": ""})
		return
	}
	if s.failWith != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "error": s.failWith})
		return
	}
	node := identity
	if at := strings.Index(identity, "@"); at >= 0 {
		node = identity[at+1:]
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1,
		"error": &rpcError{Code: -32000, Message: "PD012100: No entries found for node '" + node + "'"}})
}

func newResolverClient(t *testing.T, self string, known map[string]string) (*PenteClient, *resolverStub, *bytes.Buffer) {
	t.Helper()
	stub := &resolverStub{known: known}
	srv := httptest.NewServer(http.HandlerFunc(stub.handler))
	t.Cleanup(srv.Close)
	var logs bytes.Buffer
	c := NewPenteClient(PenteClientConfig{
		BaseURL:  srv.URL,
		Identity: self,
		Logger:   slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return c, stub, &logs
}

func TestResolvePartyAddr_UsesResolvedAddressWhenAvailable(t *testing.T) {
	const itau = "funded_operator@spoke-brl-bank-itau"
	const real = "0xad6e7a6de8d8591c3f5f2a2ba56383a012566912"
	c, _, logs := newResolverClient(t, itau, map[string]string{itau: real})

	got := c.resolvePartyAddr(context.Background(), itau, "0xdeadbeef")
	if got != real {
		t.Fatalf("resolvePartyAddr = %q, want the resolved %q", got, real)
	}
	// A successful resolution must not log a degradation.
	if strings.Contains(logs.String(), "not resolved") {
		t.Errorf("logged a fallback for a successful resolution:\n%s", logs.String())
	}
}

// TestResolvePartyAddr_RemotePartyFallsBackAtDebug pins the measured reality: a remote identity
// cannot resolve (PD012100), so the placeholder is the normal path and must not shout. Logging it
// at warn would make every cross-spoke propose look broken.
func TestResolvePartyAddr_RemotePartyFallsBackAtDebug(t *testing.T) {
	const itau = "funded_operator@spoke-brl-bank-itau"
	const remote = "funded_operator@spoke-cop-bank-bancolombia"
	const placeholder = "0xd87da0f7a2c3bed49ebe21f51effae6c5c07b3c5"
	c, stub, logs := newResolverClient(t, itau, map[string]string{itau: "0xad6e"})

	got := c.resolvePartyAddr(context.Background(), remote, placeholder)
	if got != placeholder {
		t.Fatalf("resolvePartyAddr = %q, want the placeholder %q", got, placeholder)
	}
	if stub.calls != 1 {
		t.Errorf("expected exactly one resolve attempt, got %d", stub.calls)
	}
	out := logs.String()
	if !strings.Contains(out, "not resolved") {
		t.Errorf("the fallback was silent; a degradation must be observable:\n%s", out)
	}
	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("remote fallback should log at DEBUG, got:\n%s", out)
	}
	if !strings.Contains(out, remote) || !strings.Contains(out, placeholder) {
		t.Errorf("the log must name the identity and the placeholder written on-chain:\n%s", out)
	}
	if !strings.Contains(out, "PD012100") {
		t.Errorf("the log must carry the underlying error:\n%s", out)
	}
	if !strings.Contains(out, "expected=true") {
		t.Errorf("a cross-spoke miss must be marked expected:\n%s", out)
	}
}

// TestResolvePartyAddr_LocalFailureWarns is the case that matters operationally: this node cannot
// address its OWN signer, so a placeholder is about to be written into an immutable record and the
// party will not be able to submit its own accept/reject. Before this change it looked exactly
// like the expected remote case.
func TestResolvePartyAddr_LocalFailureWarns(t *testing.T) {
	const itau = "funded_operator@spoke-brl-bank-itau"
	c, _, logs := newResolverClient(t, itau, nil) // knows nobody — the local identity fails too

	if got := c.resolvePartyAddr(context.Background(), itau, "0xfallback"); got != "0xfallback" {
		t.Fatalf("resolvePartyAddr = %q, want the fallback", got)
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("a local identity failing to resolve must warn, got:\n%s", out)
	}
	if !strings.Contains(out, "local=true") {
		t.Errorf("the log must say the identity was local:\n%s", out)
	}
}

func TestResolvePartyAddr_EmptyIdentitySkipsTheCall(t *testing.T) {
	c, stub, logs := newResolverClient(t, "funded_operator@spoke-brl-cb", nil)

	if got := c.resolvePartyAddr(context.Background(), "", "0xzero"); got != "0xzero" {
		t.Fatalf("resolvePartyAddr(\"\") = %q, want the fallback", got)
	}
	if stub.calls != 0 {
		t.Errorf("an empty identity must not hit the RPC, got %d calls", stub.calls)
	}
	// An absent optional party is not a degradation, so it must not log one.
	if strings.Contains(logs.String(), "not resolved") {
		t.Errorf("logged a fallback for an empty identity:\n%s", logs.String())
	}
}

func TestIsLocalNode(t *testing.T) {
	c := &PenteClient{identity: "funded_operator@spoke-costa-rica-cb"}
	for _, tc := range []struct {
		identity string
		want     bool
	}{
		{"funded_operator@spoke-costa-rica-cb", true},
		{"other_key@spoke-costa-rica-cb", true},         // same node, different key
		{"funded_operator@spoke-costa-rica-cb1", false}, // a bank on the same spoke is a DIFFERENT node
		{"funded_operator@spoke-peru-cb", false},
		{"funded_operator", false}, // no node part
		{"", false},
		{"funded_operator@ spoke-costa-rica-cb ", true}, // whitespace tolerated, as elsewhere
	} {
		if got := c.isLocalNode(tc.identity); got != tc.want {
			t.Errorf("isLocalNode(%q) = %v, want %v", tc.identity, got, tc.want)
		}
	}
	// A client with no identity configured cannot claim anything is local, or every unresolvable
	// party would be reported as a local failure.
	empty := &PenteClient{identity: ""}
	for _, id := range []string{"", "funded_operator@spoke-peru-cb", "funded_operator"} {
		if empty.isLocalNode(id) {
			t.Errorf("isLocalNode(%q) on an identity-less client returned true", id)
		}
	}
}

// TestResolvePartyAddr_NonRegistryErrorWarns is the case the old silence hid completely: the node
// IS reachable, and resolution failed anyway. Measured on the live stack, every same-spoke peer
// resolves, so a failure here is a real fault — not the routine cross-spoke miss — and it must not
// be filed under the same debug line. The identity is remote, so shape alone cannot tell them
// apart; only Paladin's error can.
func TestResolvePartyAddr_NonRegistryErrorWarns(t *testing.T) {
	const remote = "funded_operator@spoke-cop-bank-bancolombia"
	c, stub, logs := newResolverClient(t, "funded_operator@spoke-brl-bank-itau", nil)
	stub.failWith = &rpcError{Code: -32603, Message: "PD020000: internal error"}

	if got := c.resolvePartyAddr(context.Background(), remote, "0xplaceholder"); got != "0xplaceholder" {
		t.Fatalf("resolvePartyAddr = %q, want the fallback", got)
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("a non-registry failure must warn, got:\n%s", out)
	}
	if !strings.Contains(out, "expected=false") {
		t.Errorf("a non-registry failure is not expected, got:\n%s", out)
	}
}

// TestResolvePartyAddr_EmptyAddressWarns covers a successful call that returns nothing. err is nil,
// so an error-only check would treat it as routine; it is a fault.
func TestResolvePartyAddr_EmptyAddressWarns(t *testing.T) {
	c, stub, logs := newResolverClient(t, "funded_operator@spoke-brl-bank-itau", nil)
	stub.emptyResult = true

	if got := c.resolvePartyAddr(context.Background(), "funded_operator@spoke-cop-cb", "0xfb"); got != "0xfb" {
		t.Fatalf("resolvePartyAddr = %q, want the fallback", got)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") {
		t.Errorf("an empty resolved address must warn, got:\n%s", out)
	}
}

func TestIsNodeUnknown(t *testing.T) {
	if isNodeUnknown(nil) {
		t.Error("a nil error is not a registry miss — the call succeeded and returned nothing")
	}
	if !isNodeUnknown(&rpcError{Code: -32000, Message: "PD012100: No entries found for node 'spoke-cop-cb'"}) {
		t.Error("PD012100 must be recognised as a registry miss")
	}
	for _, e := range []error{
		&rpcError{Code: -32603, Message: "PD020000: internal error"},
		errors.New("call ptx_resolveVerifier: dial tcp: connection refused"),
		errors.New("decode ptx_resolveVerifier response: unexpected EOF"),
	} {
		if isNodeUnknown(e) {
			t.Errorf("%v must NOT be treated as a routine registry miss", e)
		}
	}
}
