// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"
)

func TestProvisionKeycloakStep_RendersRealmsAndEnv(t *testing.T) {
	requireDocker(t)
	s := newProvisionKeycloakStep("provision-keycloak", keycloakStepParams{
		EntityPrefix: "cbweb3-central-bank-brazil-test",
		NetName:      "cbweb3-central-bank-brazil-net",
		ComposePath:  "/nonexistent/keycloak-compose.yaml",
		KCDBURL:      "jdbc:postgresql://cbweb3-central-bank-brazil-postgres:5432/cbweb3_central_bank_brazil",
		KCUser:       "default",
		KCPassword:   "default",
		HostPort:     24645,
		Realms:       centralBankRealmPlans("central-bank-brazil", nil),
	}).(*provisionKeycloakStep)
	cleanupVolume(t, s.importVolume())

	// composeEnv carries the discriminating values. KEYCLOAK_IMPORT_DIR is no
	// longer set: the compose template mounts the named volume directly (no host
	// filesystem involved — see specs/026-tk4-compose-central-bank/plan.md addendum).
	env := strings.Join(s.composeEnv(), "\n")
	for _, want := range []string{
		"ENTITY_INFRA_PREFIX=cbweb3-central-bank-brazil-test",
		"KEYCLOAK_HOST_PORT=24645",
		"KC_DB_URL=jdbc:postgresql://cbweb3-central-bank-brazil-postgres:5432/cbweb3_central_bank_brazil",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}
	if strings.Contains(env, "KEYCLOAK_IMPORT_DIR=") {
		t.Error("composeEnv must not set KEYCLOAK_IMPORT_DIR (unused since the volume migration)")
	}

	// Run seeds realm JSONs into the volume even though the compose up will fail
	// (no compose file) — the seed happens before `docker compose up` runs.
	_ = s.Run(context.Background())
	for _, realm := range []string{"central-bank-brazil", "cbweb3"} {
		if _, err := readVolumeFile(context.Background(), s.importVolume(), realm+"-realm.json"); err != nil {
			t.Errorf("expected rendered realm %s in volume: %v", realm, err)
		}
	}
}
