// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"
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
	step := newGenTLSJoinStep("spoke-test-gentlsjoin", "bank-test-gentlsjoin", "").(*genTLSJoinStep)
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

// TestGenTLSJoinStep_RoutableHostInSAN asserts the cross-VM path: a routable
// advertisedHost is carried in the cert as an IP SAN so the CB's reply-leg dial to
// dns:///<advertisedHost>:9000 validates. Regression guard for the one-directional
// routable wiring that hung create-pente-context across VMs.
func TestGenTLSJoinStep_RoutableHostInSAN(t *testing.T) {
	requireDocker(t)
	step := newGenTLSJoinStep("spoke-test-gentlssan", "bank-test-gentlssan", "10.10.0.22").(*genTLSJoinStep)
	cleanupVolume(t, step.paladinConfigVolume())

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	certPEM, err := readVolumeFile(context.Background(), step.paladinConfigVolume(), "tls.crt")
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("cert is not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	foundIP := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "10.10.0.22" {
			foundIP = true
		}
	}
	if !foundIP {
		t.Errorf("cert IP SANs = %v; want them to include the routable advertisedHost 10.10.0.22", cert.IPAddresses)
	}
	// The container-name DNS SAN must remain (single-host / extra_hosts dial still valid).
	wantDNS := "paladin-spoke-test-gentlssan-bank-test-gentlssan"
	foundDNS := false
	for _, d := range cert.DNSNames {
		if d == wantDNS {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Errorf("cert DNS SANs = %v; want them to still include %q", cert.DNSNames, wantDNS)
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
	// Single-host bank (empty advertisedHost): gRPC host port stays in the +21000 band.
	step := newStartPaladinJoinStep("spoke-brl", "bank-itau", t.TempDir(), "/nonexistent/compose.yaml",
		"", "paladin:test", 8746, 0, 0).(*startPaladinJoinStep)

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

// TestStartPaladinJoinStep_RoutablePublishesGRPC9000 guards the cross-VM fix: a
// routable bank must publish its Paladin gRPC on host port 9000 (matching the
// dns:///<advertisedHost>:9000 endpoint it registers), else the CB's resolve-reply
// dial gets "connection refused" and create-pente-context hangs.
func TestStartPaladinJoinStep_RoutablePublishesGRPC9000(t *testing.T) {
	step := newStartPaladinJoinStep("spoke-brl", "bank-itau", t.TempDir(), "/nonexistent/compose.yaml",
		"10.10.0.22", "paladin:test", 8746, 0, 0).(*startPaladinJoinStep)
	env := strings.Join(step.composeEnv(), "\n")
	if !strings.Contains(env, "PALADIN_BANK_GRPC_PORT=9000") {
		t.Errorf("routable bank must publish gRPC on 9000; got:\n%s", env)
	}
	// RPC/WS stay in their per-bank bands (only the peer gRPC port is fixed to 9000).
	if !strings.Contains(env, "PALADIN_BANK_RPC_PORT=27746") {
		t.Errorf("routable bank RPC port should stay 27746; got:\n%s", env)
	}
}

// markRegisterPaladinDone writes the state file entry the step's Check reads first.
func markRegisterPaladinDone(t *testing.T, dir string) {
	t.Helper()
	state := markStep(ProvisioningState{SpokeID: "spoke-brl"}, StepRegisterPaladinNode, "done", "2026-06-29T00:00:00Z")
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}
}

// Check is NOT state-driven alone (it was until 69e44bcf). It is done only when
// the state says so AND the transport cert in the Paladin config volume still
// matches the fingerprint published on the last successful Run. gen-tls-join may
// regenerate that cert on a later run; without the drift check the step would stay
// "done" while Paladin presents a cert the CB no longer trusts, surfacing as
// `tls: bad certificate` on create-pente-context.
//
// These two cases need no Docker: both must be false.
func TestRegisterPaladinNodeStep_Check_FalseWithoutFingerprintEvidence(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterPaladinNodeStep("spoke-brl", "bank-itau", dir, "http://localhost:8746", "0xREG", "", keyprovider.NewLocalKeyProviderSeeded(), 0)

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check with no state: %v", err)
	}
	if done {
		t.Error("Check should be false with no state")
	}

	// State alone is not enough: with the config volume absent there is no cert to
	// compare, so the step must re-run once gen-tls-join recreates it.
	markRegisterPaladinDone(t, dir)
	done, err = step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check with state but no volume: %v", err)
	}
	if done {
		t.Error("Check should be false when the state says done but the Paladin config volume is absent")
	}
}

// The volume-backed half of the contract: a matching fingerprint means done, a
// stale one means the cert drifted and the node must republish. This is the
// regression 69e44bcf fixed, and it had no test until now.
func TestRegisterPaladinNodeStep_Check_TLSFingerprintDrift(t *testing.T) {
	requireDocker(t)
	dir := t.TempDir()
	const bankID = "bank-itau-fingerprint"
	volume := "spoke-brl_" + bankID + "_paladin_config"
	cleanupVolume(t, volume)

	step := newRegisterPaladinNodeStep("spoke-brl", bankID, dir, "http://localhost:8746", "0xREG", "", keyprovider.NewLocalKeyProviderSeeded(), 0).(*registerPaladinNodeStep)
	markRegisterPaladinDone(t, dir)

	certPEM := []byte("-----BEGIN CERTIFICATE-----\nfingerprint-fixture\n-----END CERTIFICATE-----\n")
	if err := dockervolume.WriteFile(context.Background(), volume, "tls.crt", certPEM, "0644"); err != nil {
		t.Fatalf("seed volume cert: %v", err)
	}

	// A cert in the volume with no recorded fingerprint forces one republish.
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check with no recorded fingerprint: %v", err)
	}
	if done {
		t.Error("Check should be false before any fingerprint has been recorded")
	}

	sum := sha256.Sum256(certPEM)
	if err := os.WriteFile(step.paladinTLSFingerprintFile(), []byte(hex.EncodeToString(sum[:])), 0o644); err != nil {
		t.Fatalf("write fingerprint: %v", err)
	}
	done, err = step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check with matching fingerprint: %v", err)
	}
	if !done {
		t.Error("Check should be true when the state is done and the volume cert matches the recorded fingerprint")
	}

	// gen-tls-join regenerates the cert → the recorded fingerprint no longer matches.
	if err := dockervolume.WriteFile(context.Background(), volume, "tls.crt",
		[]byte("-----BEGIN CERTIFICATE-----\nrotated\n-----END CERTIFICATE-----\n"), "0644"); err != nil {
		t.Fatalf("rotate volume cert: %v", err)
	}
	done, err = step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check after cert rotation: %v", err)
	}
	if done {
		t.Error("Check should be false after the volume cert drifted from the recorded fingerprint")
	}
}

func TestRegisterPaladinNodeStep_Run_ErrorsOnMissingCert(t *testing.T) {
	requireDocker(t)
	dir := t.TempDir()
	cleanupVolume(t, "spoke-brl_bank-itau-missingcert_paladin_config")
	step := newRegisterPaladinNodeStep("spoke-brl", "bank-itau-missingcert", dir, "http://localhost:8746", "0xREG", "", keyprovider.NewLocalKeyProviderSeeded(), 0)
	if err := step.Run(context.Background()); err == nil {
		t.Error("Run should error when the bank Paladin cert is missing")
	}
}
