// SPDX-License-Identifier: Apache-2.0

//go:build integration

package orchestrator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/certsource"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// TestRunFound_Integration executes the full 10-step provisioning sequence against a real
// Besu node and Docker environment. Run with:
//
//	go test -race -tags integration -timeout 15m ./engine/orchestrator/...
//
// Required environment variables:
//
//	INTEGRATION_DATA_DIR       — absolute path to spoke data directory (must have genesis.json)
//	INTEGRATION_BESU_RPC_URL   — Besu HTTP RPC URL (e.g. http://localhost:8645)
//	INTEGRATION_PALADIN_CB_URL — Paladin CB HTTP RPC URL (e.g. http://localhost:31648)
//	INTEGRATION_SCRIPTS_DIR    — absolute path to deploy/local/paladin/scripts/
//	INTEGRATION_COMPOSE_PATH   — absolute path to provisioning/templates/central-bank/paladin-compose.yaml
//	INTEGRATION_CONFIG_TPL_DIR — absolute path to provisioning/templates/central-bank/paladin-config/
func TestRunFound_Integration(t *testing.T) {
	dataDir := requireEnv(t, "INTEGRATION_DATA_DIR")
	besuRPCURL := requireEnv(t, "INTEGRATION_BESU_RPC_URL")
	paladinCBURL := requireEnv(t, "INTEGRATION_PALADIN_CB_URL")
	scriptsDir := requireEnv(t, "INTEGRATION_SCRIPTS_DIR")
	composePath := requireEnv(t, "INTEGRATION_COMPOSE_PATH")
	configTplDir := requireEnv(t, "INTEGRATION_CONFIG_TPL_DIR")

	m := &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "integration-cb"},
		Spec: manifest.Spec{
			Scenario: "a",
			Role:     "central-bank",
			Mode:     "found",
			Spoke: manifest.Spoke{
				ID:       "spoke-integration",
				ChainID:  1337,
				Currency: "INT",
			},
			Node: manifest.Node{
				AdvertisedHost: "integration.host",
				DataDir:        dataDir,
			},
			Image:       "hyperledger/besu:25.8.0",
			KeyProvider: "kms://local",
			CertSource:  "self-signed://local",
		},
	}

	kprov, err := keyprovider.NewLocalKeyProvider()
	if err != nil {
		t.Fatalf("create keyprovider: %v", err)
	}
	csrc, err := certsource.NewLocalCertSource(m.Spec.Spoke.ID)
	if err != nil {
		t.Fatalf("create certsource: %v", err)
	}

	deps := Deps{
		KeyProvider:              kprov,
		CertSource:               csrc,
		RelayRegistrar:           NoOpRelayRegistrar{},
		ScriptsDir:               scriptsDir,
		ComposeTemplatePath:      composePath,
		PaladinConfigTemplateDir: configTplDir,
		BesuRPCURL:               besuRPCURL,
		PaladinCBURL:             paladinCBURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// SC-001: First run provisions successfully.
	if err := RunFound(ctx, m, deps); err != nil {
		t.Fatalf("RunFound (first run) failed: %v", err)
	}

	// SC-003: Second run is idempotent and fast.
	start := time.Now()
	if err := RunFound(ctx, m, deps); err != nil {
		t.Fatalf("RunFound (idempotent run) failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Errorf("idempotent run took %s; want < 2s", elapsed)
	}

	// SC-002: State file exists with all steps done.
	state, err := LoadState(dataDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Steps) != 9 {
		t.Errorf("expected 9 steps in state, got %d", len(state.Steps))
	}
	for _, s := range state.Steps {
		if s.Step != StepRegisterRelay && s.Status != "done" {
			t.Errorf("step %q: status = %q; want done", s.Step, s.Status)
		}
	}
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("integration test skipped: %s not set", key)
	}
	return v
}
