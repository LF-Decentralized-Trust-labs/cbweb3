// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestRunApply_MissingFFlag verifies exit 1 when -f is omitted.
func TestRunApply_MissingFFlag(t *testing.T) {
	code := runApply([]string{})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_BadOutputFormat verifies exit 1 for unknown --output value.
func TestRunApply_BadOutputFormat(t *testing.T) {
	code := runApply([]string{"-f", "testdata/central-bank-brl.yaml", "--output", "toml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_ManifestNotFound verifies exit 1 when the manifest file does not exist.
func TestRunApply_ManifestNotFound(t *testing.T) {
	code := runApply([]string{"-f", "testdata/does-not-exist.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_InvalidSyntax verifies exit 1 for a YAML file with parse errors.
func TestRunApply_InvalidSyntax(t *testing.T) {
	code := runApply([]string{"-f", "testdata/invalid-syntax.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_MissingSpokeID verifies exit 1 for a valid YAML missing spec.spoke.id.
func TestRunApply_MissingSpokeID(t *testing.T) {
	code := runApply([]string{"-f", "testdata/missing-spoke-id.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_ModeJoin verifies exit 1 for mode: join (not yet supported).
func TestRunApply_ModeJoin(t *testing.T) {
	code := runApply([]string{"-f", "testdata/mode-join.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_DryRun_AllPending verifies dry-run outputs 10 pending steps and exits 0.
func TestRunApply_DryRun_AllPending(t *testing.T) {
	// Redirect stdout so we can capture the report.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	code := runApply([]string{
		"--dry-run",
		"-f", "testdata/central-bank-brl.yaml",
		"--output", "json",
	})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if code != 0 {
		t.Errorf("exit code = %d; want 0\noutput: %s", code, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, output)
	}

	if result["status"] != "dry-run" {
		t.Errorf("status = %v; want dry-run", result["status"])
	}
	if result["dryRun"] != true {
		t.Errorf("dryRun = %v; want true", result["dryRun"])
	}
	steps, ok := result["steps"].([]interface{})
	if !ok {
		t.Fatalf("steps is not an array: %v", result["steps"])
	}
	if len(steps) != 13 {
		t.Errorf("steps len = %d; want 13", len(steps))
	}
	for i, s := range steps {
		step := s.(map[string]interface{})
		if step["status"] != "pending" {
			t.Errorf("step[%d].status = %v; want pending", i, step["status"])
		}
	}
}

// TestRunApply_DryRun_AliasN verifies -n is equivalent to --dry-run.
func TestRunApply_DryRun_AliasN(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	code := runApply([]string{"-n", "-f", "testdata/central-bank-brl.yaml", "--output", "json"})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if code != 0 {
		t.Errorf("exit code = %d; want 0\noutput: %s", code, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json: %v\noutput: %s", err, output)
	}
	if result["status"] != "dry-run" {
		t.Errorf("status = %v; want dry-run", result["status"])
	}
}

// TestRunApply_DryRun_MissingManifest verifies exit 1 even with --dry-run flag.
func TestRunApply_DryRun_MissingManifest(t *testing.T) {
	code := runApply([]string{"--dry-run", "-f", "testdata/missing-spoke-id.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_DryRun_InvalidSyntax verifies exit 1 for YAML parse errors with --dry-run.
func TestRunApply_DryRun_InvalidSyntax(t *testing.T) {
	code := runApply([]string{"--dry-run", "-f", "testdata/invalid-syntax.yaml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
}

// TestRunApply_OutputYAML verifies dry-run with --output yaml emits valid YAML.
func TestRunApply_OutputYAML(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	code := runApply([]string{"--dry-run", "-f", "testdata/central-bank-brl.yaml", "--output", "yaml"})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if code != 0 {
		t.Errorf("exit code = %d; want 0\noutput: %s", code, output)
	}
	if !strings.Contains(output, "spoke: spoke-brl") {
		t.Errorf("yaml output missing spoke field:\n%s", output)
	}
	if !strings.Contains(output, "status: dry-run") {
		t.Errorf("yaml output missing status field:\n%s", output)
	}
}
