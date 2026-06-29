// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

func TestBankNodeName_DerivedFromSpokeAndBank(t *testing.T) {
	if got := bankNodeName("spoke-brl", "bank-itau"); got != "spoke-brl-bank-itau" {
		t.Errorf("bankNodeName = %q; want spoke-brl-bank-itau", got)
	}
	if got := bankGrpcHostname("spoke-cop", "bank-davivienda"); got != "paladin-spoke-cop-bank-davivienda" {
		t.Errorf("bankGrpcHostname = %q; want paladin-spoke-cop-bank-davivienda", got)
	}
}

func TestGenTLSJoinStep_GeneratesBankCert(t *testing.T) {
	dir := t.TempDir()
	step := newGenTLSJoinStep("spoke-brl", "bank-itau", dir)

	done, _ := step.Check(context.Background())
	if done {
		t.Fatal("Check should be false before Run")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	certPath := filepath.Join(dir, "paladin", "bank-itau", "tls.crt")
	keyPath := filepath.Join(dir, "paladin", "bank-itau", "tls.key")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("tls.key missing: %v", err)
	}

	// CN/SAN must derive from the bank id — no hardcoded bank name.
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("cert is not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	want := "paladin-spoke-brl-bank-itau"
	if cert.Subject.CommonName != want {
		t.Errorf("CN = %q; want %q", cert.Subject.CommonName, want)
	}
	found := false
	for _, d := range cert.DNSNames {
		if d == want {
			found = true
		}
	}
	if !found {
		t.Errorf("SAN missing %q; got %v", want, cert.DNSNames)
	}

	done, _ = step.Check(context.Background())
	if !done {
		t.Error("Check should be true after Run (idempotent)")
	}
}

func TestRenderConfigJoinStep_Check(t *testing.T) {
	dir := t.TempDir()
	step := newRenderConfigJoinStep("spoke-brl", "bank-itau", dir, 8746, 8756, "0xREG", "0xZF", "0xPF", "/tmpl")
	done, _ := step.Check(context.Background())
	if done {
		t.Error("Check should be false when config.yaml absent")
	}
	cfgDir := filepath.Join(dir, "paladin", "bank-itau")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("x"), 0o644)
	done, _ = step.Check(context.Background())
	if !done {
		t.Error("Check should be true when config.yaml present")
	}
}

func TestStartPaladinJoinStep_ComposeEnvAndPorts(t *testing.T) {
	step := newStartPaladinJoinStep("spoke-brl", "bank-itau", t.TempDir(), "/nonexistent/compose.yaml",
		"paladin:test", 8746, 0, 0).(*startPaladinJoinStep)

	// Ports derived from the bank Besu RPC port (8746), each in its own +1000 band:
	// RPC=+19000, WS=+20000, gRPC=+21000.
	if !strings.Contains(step.paladinRPCURL, "27746") {
		t.Errorf("paladinRPCURL = %q; want port 27746", step.paladinRPCURL)
	}
	env := strings.Join(step.composeEnv(), "\n")
	for _, want := range []string{
		"SPOKE_ID=spoke-brl",
		"BANK_ID=bank-itau",
		"PALADIN_IMAGE=paladin:test",
		"PALADIN_BANK_RPC_PORT=27746",
		"PALADIN_BANK_WS_PORT=28746",
		"PALADIN_BANK_GRPC_PORT=29746",
		"SPOKE_NETWORK_NAME=cbweb3-spoke-brl-besu",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}
}

func TestRegisterPaladinNodeStep_Check_StateDriven(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterPaladinNodeStep("spoke-brl", "bank-itau", dir, "http://localhost:8746", "0xREG", keyprovider.NewLocalKeyProviderSeeded(), 0)
	done, _ := step.Check(context.Background())
	if done {
		t.Error("Check should be false with no state")
	}
	state := ProvisioningState{SpokeID: "spoke-brl"}
	state = markStep(state, StepRegisterPaladinNode, "done", "2026-06-29T00:00:00Z")
	saveState(dir, state)
	done, _ = step.Check(context.Background())
	if !done {
		t.Error("Check should be true when register-paladin-node is done")
	}
}

func TestRegisterPaladinNodeStep_Run_ErrorsOnMissingCert(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterPaladinNodeStep("spoke-brl", "bank-itau", dir, "http://localhost:8746", "0xREG", keyprovider.NewLocalKeyProviderSeeded(), 0)
	if err := step.Run(context.Background()); err == nil {
		t.Error("Run should error when the bank Paladin cert is missing")
	}
}
