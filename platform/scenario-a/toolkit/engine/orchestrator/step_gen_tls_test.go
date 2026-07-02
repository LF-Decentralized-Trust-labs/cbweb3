// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGenTLSStep_Check_False_NoCert(t *testing.T) {
	dir := t.TempDir()
	step := newGenTLSStep("spoke-test", dir, nil, nil)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when central-bank.crt does not exist")
	}
}

func TestGenTLSStep_Check_True_CertExists(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte("CERT"), 0o644)
	step := newGenTLSStep("spoke-test", dir, nil, nil)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when central-bank.crt exists")
	}
}

func TestGenTLSStep_Run_CreatesCertAndKey(t *testing.T) {
	dir := t.TempDir()
	step := newGenTLSStep("spoke-test", dir, nil, nil)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	certPath := filepath.Join(dir, "tls", "central-bank.crt")
	keyPath := filepath.Join(dir, "tls", "central-bank.key")

	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("central-bank.crt not created: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("central-bank.key not created: %v", err)
	}
}

func TestGenTLSStep_Run_CertIsPEM(t *testing.T) {
	dir := t.TempDir()
	step := newGenTLSStep("spoke-test", dir, nil, nil)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tls", "central-bank.crt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("central-bank.crt is empty")
	}
	if string(data[:27]) != "-----BEGIN CERTIFICATE-----" {
		t.Errorf("central-bank.crt does not start with PEM header: %s", data[:27])
	}
}

func TestGenTLSStep_Run_Idempotent_Check_TrueAfterRun(t *testing.T) {
	dir := t.TempDir()
	step := newGenTLSStep("spoke-test", dir, nil, nil)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check after Run: %v", err)
	}
	if !done {
		t.Error("Check should return true after successful Run")
	}
}
