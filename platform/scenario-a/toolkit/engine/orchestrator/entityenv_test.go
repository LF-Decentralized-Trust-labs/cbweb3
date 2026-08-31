// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderToString(t *testing.T, data EntityEnvData) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), ".env.infra")
	if err := RenderEntityEnv(data, out); err != nil {
		t.Fatalf("RenderEntityEnv: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestRenderEntityEnv_CentralBank(t *testing.T) {
	env := renderToString(t, EntityEnvData{
		EntityName:        "central-bank-brazil",
		IsCentralBank:     true,
		SpokeID:           "spoke-brl",
		FiatSymbol:        "BRL",
		KeycloakURL:       "http://cbweb3-central-bank-brazil-keycloak:8080",
		KCRealm:           "central-bank-brazil",
		PostgresContainer: "cbweb3-central-bank-brazil-postgres",
		PostgresPort:      22645,
		PostgresUser:      "default",
		PostgresPassword:  "default",
		DBName:            "cbweb3_central_bank_brazil",
		RedisContainer:    "cbweb3-central-bank-brazil-redis",
		RedisPort:         23645,
		BesuRPCURL:        "http://localhost:8645",
		ChainID:           1337,
		CBPrivateKey:      "deadbeef",
		GovernanceUserID:  "service-account-central-bank-brazil-client",
	})

	mustContain(t, env, "DATABASE_URL=postgres://default:default@cbweb3-central-bank-brazil-postgres:5432/cbweb3_central_bank_brazil")
	mustContain(t, env, "KC_REALM=central-bank-brazil")
	mustContain(t, env, "CB_PRIVATE_KEY=deadbeef")
	mustContain(t, env, "GOVERNANCE_USER_ID=service-account-central-bank-brazil-client")
	if strings.Contains(env, "CENTRAL_BANK_API_URL=") {
		t.Error("CB env must NOT contain CENTRAL_BANK_API_URL")
	}
}

func TestRenderEntityEnv_CommercialBank(t *testing.T) {
	env := renderToString(t, EntityEnvData{
		EntityName:        "bank-itau",
		IsCentralBank:     false,
		SpokeID:           "spoke-brl",
		KCRealm:           "bank-itau",
		PostgresContainer: "cbweb3-bank-itau-postgres",
		DBName:            "cbweb3_bank_itau",
		BesuRPCURL:        "http://localhost:8646",
		CentralBankAPIURL: "http://cbweb3-api-gateway.central-bank-brazil:8080",
		PaladinIdentities: "funded_operator@spoke-brl-cb,funded_operator@spoke-cop-cb",
	})

	mustContain(t, env, "CENTRAL_BANK_API_URL=http://cbweb3-api-gateway.central-bank-brazil:8080")
	mustContain(t, env, "DB_NAME=cbweb3_bank_itau")
	// Consortium FX roster (cross-spoke identities) is rendered as PALADIN_IDENTITIES.
	mustContain(t, env, "PALADIN_IDENTITIES=funded_operator@spoke-brl-cb,funded_operator@spoke-cop-cb")
	if strings.Contains(env, "CB_PRIVATE_KEY=") {
		t.Error("commercial-bank env must NOT contain CB_PRIVATE_KEY")
	}
	if strings.Contains(env, "GOVERNANCE_USER_ID=") {
		t.Error("commercial-bank env must NOT contain GOVERNANCE_USER_ID")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("rendered env missing:\n  %s", needle)
	}
}
