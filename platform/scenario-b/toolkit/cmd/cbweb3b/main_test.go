// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureDir = "../../engine/manifest/testdata"

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestValidateValidManifest(t *testing.T) {
	code, out, _ := runCLI(t, "validate", "-f", filepath.Join(fixtureDir, "found-hub.yaml"))
	if code != exitValid {
		t.Fatalf("expected exit 0, got %d (out=%s)", code, out)
	}
	if !strings.Contains(out, "valid: true") {
		t.Errorf("expected valid: true in output, got:\n%s", out)
	}
}

func TestValidateInvalidManifest(t *testing.T) {
	// A found-spoke stripped of hubBundleRef must fail with exit 1.
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	src, err := os.ReadFile(filepath.Join(fixtureDir, "found-spoke.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Drop the hubBundleRef line.
	var b strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "hubBundleRef:") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, "validate", "-f", tmp)
	if code != exitInvalid {
		t.Fatalf("expected exit 1, got %d (out=%s)", code, out)
	}
	if !strings.Contains(out, "hubBundleRef") {
		t.Errorf("expected hubBundleRef named in output, got:\n%s", out)
	}
}

func TestJSONOutput(t *testing.T) {
	code, out, _ := runCLI(t, "validate", "-f", filepath.Join(fixtureDir, "join.yaml"), "-o", "json")
	if code != exitValid {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "\"valid\": true") {
		t.Errorf("expected JSON valid field, got:\n%s", out)
	}
}

func TestUsageErrors(t *testing.T) {
	if code, _, _ := runCLI(t); code != exitUsage {
		t.Errorf("no args: expected exit 2, got %d", code)
	}
	if code, _, _ := runCLI(t, "validate"); code != exitUsage {
		t.Errorf("no -f: expected exit 2, got %d", code)
	}
	if code, _, _ := runCLI(t, "validate", "-f", "/no/such/file.yaml"); code != exitUsage {
		t.Errorf("missing file: expected exit 2, got %d", code)
	}
	if code, _, _ := runCLI(t, "validate", "-f", filepath.Join(fixtureDir, "join.yaml"), "-o", "xml"); code != exitUsage {
		t.Errorf("bad -o: expected exit 2, got %d", code)
	}
}

// TK-B6: `apply --dry-run` plans the found-hub steps (exit 0, no effects).
func TestApplyFoundHubDryRun(t *testing.T) {
	f := filepath.Join(fixtureDir, "found-hub.yaml")
	code, out, errb := runCLI(t, "apply", "-f", f, "--dry-run", "--data-dir", t.TempDir())
	if code != exitValid {
		t.Fatalf("apply --dry-run found-hub: expected exit 0, got %d (err=%s)", code, errb)
	}
	if !strings.Contains(out, "planned") && !strings.Contains(out, "skipped") {
		t.Errorf("expected a planned/skipped report, got:\n%s", out)
	}
}

// TK-B8: found-hub, found-spoke and join are all supported; join dry-run plans
// the steps (exit 0).
func TestApplyJoinDryRunCLI(t *testing.T) {
	f := filepath.Join(fixtureDir, "join.yaml")
	code, out, errb := runCLI(t, "apply", "-f", f, "--dry-run", "--data-dir", t.TempDir())
	if code != exitValid {
		t.Fatalf("apply join dry-run: expected 0, got %d (err=%s)", code, errb)
	}
	if !strings.Contains(out, "join") {
		t.Errorf("expected join report, got:\n%s", out)
	}
}

// TK-B7: found-spoke dry-run plans the steps (exit 0).
func TestApplyFoundSpokeDryRunCLI(t *testing.T) {
	f := filepath.Join(fixtureDir, "found-spoke.yaml")
	code, out, errb := runCLI(t, "apply", "-f", f, "--dry-run", "--data-dir", t.TempDir())
	if code != exitValid {
		t.Fatalf("apply found-spoke dry-run: expected 0, got %d (err=%s)", code, errb)
	}
	if !strings.Contains(out, "found-spoke") {
		t.Errorf("expected found-spoke report, got:\n%s", out)
	}
}

// apply requires exactly one -f.
func TestApplyRequiresOneFile(t *testing.T) {
	if code, _, _ := runCLI(t, "apply", "--dry-run"); code != exitUsage {
		t.Errorf("apply without -f: expected exit 2, got %d", code)
	}
}

func TestSetCollisionExit(t *testing.T) {
	// found-spoke + join on the same spoke is a valid set (exit 0).
	code, _, _ := runCLI(t, "validate",
		"-f", filepath.Join(fixtureDir, "found-spoke.yaml"),
		"-f", filepath.Join(fixtureDir, "join.yaml"))
	if code != exitValid {
		t.Errorf("founder+joiner set: expected exit 0, got %d", code)
	}
}
