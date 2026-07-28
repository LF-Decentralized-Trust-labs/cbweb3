package addrs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBroadcast(t *testing.T) {
	m, err := ParseBroadcast(filepath.Join("testdata", "run-latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m["IdentityRegistry"] != "0x1000000000000000000000000000000000000001" {
		t.Fatalf("IdentityRegistry = %q", m["IdentityRegistry"])
	}
	if m["FXAgreement"] == "" {
		t.Fatal("FXAgreement missing")
	}
	// CALL transactions are not deployments.
	if len(m) != 3 {
		t.Fatalf("got %d CREATE contracts, want 3", len(m))
	}
}

func TestAppendAddrIdempotent(t *testing.T) {
	env := filepath.Join(t.TempDir(), ".env.infra")
	if err := AppendAddr(env, "HUB_IDENTITY_REGISTRY_ADDRESS", "0xabc"); err != nil {
		t.Fatal(err)
	}
	// re-set same key with a new value → replaces, no duplicate line
	if err := AppendAddr(env, "HUB_IDENTITY_REGISTRY_ADDRESS", "0xdef"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(env)
	count := 0
	for _, line := range splitLines(string(b)) {
		if len(line) >= 27 && line[:27] == "HUB_IDENTITY_REGISTRY_ADDRE" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 line for the key, got %d\n%s", count, b)
	}
	if !HasAddr(env, "HUB_IDENTITY_REGISTRY_ADDRESS", "0xdef") {
		t.Fatal("HasAddr should find the upserted value")
	}
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
