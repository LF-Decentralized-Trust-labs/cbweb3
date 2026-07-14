// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestCBCORSOrigins_AllFourPortals locks in that a central bank's CORS allow-list
// carries an origin for every portal it serves (governance, treasury, supervisor,
// noc). Omitting supervisor/noc previously made those portals fail CORS at login.
func TestCBCORSOrigins_AllFourPortals(t *testing.T) {
	ports := entityPorts(8645) // Brazil CB sample Besu RPC port
	got := cbCORSOrigins(ports, "")

	for _, want := range []string{
		"http://localhost:25645", // governance (FrontendPrimary, +17000)
		"http://localhost:26645", // treasury   (FrontendSecondary, +18000)
		"http://localhost:30645", // supervisor (FrontendSupervisor, +22000)
		"http://localhost:32645", // noc        (FrontendNOC, +24000)
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cbCORSOrigins missing %q; got %q", want, got)
		}
	}
}

// TestCBCORSOrigins_FrontendHostAddsRemoteOrigins verifica que quando frontendHost é
// um IP público, as origens remotas são adicionadas às locais.
func TestCBCORSOrigins_FrontendHostAddsRemoteOrigins(t *testing.T) {
	ports := entityPorts(8645)
	got := cbCORSOrigins(ports, "1.2.3.4")

	for _, want := range []string{
		"http://localhost:25645",
		"http://localhost:32645",
		"http://1.2.3.4:25645",
		"http://1.2.3.4:26645",
		"http://1.2.3.4:30645",
		"http://1.2.3.4:32645",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cbCORSOrigins missing %q; got %q", want, got)
		}
	}
}

// TestRenderCBEnvStep_CORSCoversAllPortals renders a CB backend env and asserts the
// CORS_ALLOW_ORIGINS line lists all four portal origins (concrete, never wildcard).
func TestRenderCBEnvStep_CORSCoversAllPortals(t *testing.T) {
	dir := t.TempDir()
	s := newRenderCBEnvStep("spoke-brl", "central-bank-brazil", "BRL", 8645, 1337, dir, "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63", "", nil)

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(cbEnvPath(dir, "central-bank-brazil"))
	if err != nil {
		t.Fatalf("read rendered env: %v", err)
	}
	env := string(data)

	want := "CORS_ALLOW_ORIGINS=http://localhost:25645,http://localhost:26645,http://localhost:30645,http://localhost:32645"
	if !strings.Contains(env, want) {
		t.Errorf("rendered env missing %q", want)
	}
	if strings.Contains(env, "CORS_ALLOW_ORIGINS=*") {
		t.Error("CORS origins must be concrete, not a wildcard")
	}
	// The operator key must be rendered so the payment-orchestrator can sign
	// Besu-layer (HTLC/fCeBM) transactions in local.
	if !strings.Contains(env, "BESU_OPERATOR_KEY=8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63") {
		t.Error("rendered env missing BESU_OPERATOR_KEY")
	}
	// ENTITY_BESU_ADDRESS is derived from the operator key; the escrow proxy stamps
	// it as requester_besu_address. "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63" is a valid secp256k1 key, so a
	// non-empty 0x address must be rendered.
	if !strings.Contains(env, "ENTITY_BESU_ADDRESS=0x") {
		t.Error("rendered env missing ENTITY_BESU_ADDRESS")
	}
	// The Besu-signing path needs the real chain id (0 makes go-ethereum's
	// NewKeyedTransactorWithChainID panic).
	if !strings.Contains(env, "BESU_CHAIN_ID=1337") {
		t.Error("rendered env missing BESU_CHAIN_ID=1337")
	}
}
