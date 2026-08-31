// SPDX-License-Identifier: Apache-2.0

//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the cbweb3 binary into dir and returns the binary path.
func buildBinary(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "cbweb3")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return bin
}

func TestIntegration_DryRun_YAML(t *testing.T) {
	bin := buildBinary(t, t.TempDir())

	out, err := exec.Command(bin, "apply", "--dry-run", "-f", "testdata/central-bank-brl.yaml", "--output", "yaml").Output()
	if err != nil {
		t.Fatalf("cbweb3 apply --dry-run: %v\noutput: %s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dryRun: true") {
		t.Errorf("yaml output missing dryRun: true\n%s", output)
	}
	if !strings.Contains(output, "status: dry-run") {
		t.Errorf("yaml output missing status: dry-run\n%s", output)
	}
}

func TestIntegration_DryRun_JSON(t *testing.T) {
	bin := buildBinary(t, t.TempDir())

	out, err := exec.Command(bin, "apply", "--dry-run", "-f", "testdata/central-bank-brl.yaml", "--output", "json").Output()
	if err != nil {
		t.Fatalf("cbweb3 apply --dry-run --output json: %v\noutput: %s", err, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, out)
	}
	if result["status"] != "dry-run" {
		t.Errorf("status = %v; want dry-run", result["status"])
	}
	steps, _ := result["steps"].([]interface{})
	if len(steps) != 9 {
		t.Errorf("steps len = %d; want 9", len(steps))
	}
}

func TestIntegration_InvalidSyntax_Exit1(t *testing.T) {
	bin := buildBinary(t, t.TempDir())

	cmd := exec.Command(bin, "apply", "-f", "testdata/invalid-syntax.yaml")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected exit 1 for invalid syntax, got 0\noutput: %s", out)
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("exit code = %d; want 1", cmd.ProcessState.ExitCode())
	}
	// stdout should be empty (error goes to stderr only)
	stdout, _ := exec.Command(bin, "apply", "-f", "testdata/invalid-syntax.yaml").Output()
	if len(stdout) != 0 {
		t.Errorf("stdout should be empty for pre-execution error, got: %s", stdout)
	}
}

func TestIntegration_MissingF_Exit1(t *testing.T) {
	bin := buildBinary(t, t.TempDir())

	cmd := exec.Command(bin, "apply")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected exit 1 when -f is missing, got 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("exit code = %d; want 1", cmd.ProcessState.ExitCode())
	}
}
