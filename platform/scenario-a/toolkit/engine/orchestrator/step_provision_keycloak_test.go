// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionKeycloakStep_RendersRealmsAndEnv(t *testing.T) {
	dir := t.TempDir()
	s := newProvisionKeycloakStep("provision-keycloak", keycloakStepParams{
		EntityPrefix: "cbweb3-central-bank-brazil",
		NetName:      "cbweb3-central-bank-brazil-net",
		DataDir:      dir,
		ComposePath:  "/nonexistent/keycloak-compose.yaml",
		KCDBURL:      "jdbc:postgresql://cbweb3-central-bank-brazil-postgres:5432/cbweb3_central_bank_brazil",
		KCUser:       "default",
		KCPassword:   "default",
		HostPort:     24645,
		Realms:       centralBankRealmPlans("central-bank-brazil"),
	}).(*provisionKeycloakStep)

	// composeEnv carries the discriminating values.
	env := strings.Join(s.composeEnv(), "\n")
	for _, want := range []string{
		"ENTITY_INFRA_PREFIX=cbweb3-central-bank-brazil",
		"KEYCLOAK_HOST_PORT=24645",
		"KC_DB_URL=jdbc:postgresql://cbweb3-central-bank-brazil-postgres:5432/cbweb3_central_bank_brazil",
		"KEYCLOAK_IMPORT_DIR=" + filepath.Join(dir, "keycloak-import"),
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}

	// Run renders realm JSONs even though the compose up will fail (no compose file).
	_ = s.Run(context.Background())
	for _, realm := range []string{"central-bank-brazil", "cbweb3"} {
		p := filepath.Join(dir, "keycloak-import", realm+"-realm.json")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected rendered realm %s: %v", realm, err)
		}
	}
}
