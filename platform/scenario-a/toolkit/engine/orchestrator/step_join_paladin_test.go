// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/x509"
	"encoding/pem"
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
	requireDocker(t)
	// Distinct spokeID/bankID from any real sample/demo entity (e.g. "spoke-brl" /
	// "bank-itau") — those map to real named volumes that may be live and mounted
	// by a running Paladin container; colliding here would risk clobbering (or, if
	// this test's Check() short-circuits like it once did, silently no-op'ing
	// against) a real deployment's cert.
	step := newGenTLSJoinStep("spoke-test-gentlsjoin", "bank-test-gentlsjoin").(*genTLSJoinStep)
	cleanupVolume(t, step.paladinConfigVolume())

	done, _ := step.Check(context.Background())
	if done {
		t.Fatal("Check should be false before Run")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	certPEM, err := readVolumeFile(context.Background(), step.paladinConfigVolume(), "tls.crt")
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	if _, err := readVolumeFile(context.Background(), step.paladinConfigVolume(), "tls.key"); err != nil {
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
	// CN must be the registered NODE NAME (not the container hostname): the Paladin
	// gRPC transport matches the peer's TLS identity against the expected node name
	// (PD030011). The container hostname stays in the SAN for the dns:/// dial.
	wantCN := "spoke-test-gentlsjoin-bank-test-gentlsjoin"
	if cert.Subject.CommonName != wantCN {
		t.Errorf("CN = %q; want %q", cert.Subject.CommonName, wantCN)
	}
	for _, want := range []string{wantCN, "paladin-" + wantCN} {
		found := false
		for _, d := range cert.DNSNames {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Errorf("SAN missing %q; got %v", want, cert.DNSNames)
		}
	}

	done, _ = step.Check(context.Background())
	if !done {
		t.Error("Check should be true after Run (idempotent)")
	}
}

func TestRenderConfigJoinStep_Check(t *testing.T) {
	requireDocker(t)
	step := newRenderConfigJoinStep("spoke-brl", "bank-itau-renderconfig", 8746, 8756, "0xREG", "0xZF", "0xPF", "/tmpl").(*renderConfigJoinStep)
	cleanupVolume(t, step.paladinConfigVolume())

	done, _ := step.Check(context.Background())
	if done {
		t.Error("Check should be false when config.yaml absent")
	}
	if err := writeVolumeFile(context.Background(), step.paladinConfigVolume(), "config.yaml", []byte("x"), "0644"); err != nil {
		t.Fatal(err)
	}
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
	requireDocker(t)
	dir := t.TempDir()
	cleanupVolume(t, "spoke-brl_bank-itau-missingcert_paladin_config")
	step := newRegisterPaladinNodeStep("spoke-brl", "bank-itau-missingcert", dir, "http://localhost:8746", "0xREG", keyprovider.NewLocalKeyProviderSeeded(), 0)
	if err := step.Run(context.Background()); err == nil {
		t.Error("Run should error when the bank Paladin cert is missing")
	}
}
