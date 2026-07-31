package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// writtenFile returns the /t/<path> a writeVolumeFile call targeted, or "".
func writtenFile(c exec.Call) string {
	for _, a := range c.Args {
		if i := strings.Index(a, "> /t/"); i >= 0 {
			rest := a[i+len("> /t/"):]
			if j := strings.IndexByte(rest, ' '); j >= 0 {
				return rest[:j]
			}
			return rest
		}
	}
	return ""
}

func TestGenServiceTLS_WritesCAandLeafPairsForEveryService(t *testing.T) {
	r := &exec.FakeRunner{}
	if err := genServiceTLS(context.Background(), r, "hub_svc_tls"); err != nil {
		t.Fatalf("genServiceTLS: %v", err)
	}

	var written []string
	for _, c := range r.Calls {
		if f := writtenFile(c); f != "" {
			written = append(written, f)
		}
	}

	// Expect: <svc>.crt + <svc>.key for each service, plus svc-ca.crt (9 files).
	want := map[string]bool{"svc-ca.crt": false}
	for _, s := range serviceMeshServices {
		want[s+".crt"] = false
		want[s+".key"] = false
	}
	if len(written) != len(want) {
		t.Fatalf("expected %d files written, got %d: %v", len(want), len(written), written)
	}
	for _, f := range written {
		if _, ok := want[f]; !ok {
			t.Fatalf("unexpected file written: %q", f)
		}
		want[f] = true
	}
	for f, seen := range want {
		if !seen {
			t.Fatalf("expected file %q was not written", f)
		}
	}

	// The CA cert is the idempotency marker and MUST be written last.
	if last := written[len(written)-1]; last != "svc-ca.crt" {
		t.Fatalf("svc-ca.crt must be written last (idempotency marker), got %q last", last)
	}
}

func TestGenServiceTLS_IsIdempotentWhenCAPresent(t *testing.T) {
	// FakeRunner echoes "YES" for the volumeHasFile probe → CA already present.
	r := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("YES\n")}}
	if err := genServiceTLS(context.Background(), r, "hub_svc_tls"); err != nil {
		t.Fatalf("genServiceTLS: %v", err)
	}
	// Only the probe should have run; no writes.
	for _, c := range r.Calls {
		if writtenFile(c) != "" {
			t.Fatalf("expected no writes when CA already present, but wrote %q", writtenFile(c))
		}
	}
}
