// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRenderBankEnvStep_CommercialBankMode(t *testing.T) {
	dir := t.TempDir()
	s := newRenderBankEnvStep(bankEnvParams{
		SpokeID:                    "spoke-brl",
		BankCode:                   "bank-itau",
		Currency:                   "BRL",
		BesuRPCPort:                8646,
		BesuRPCURL:                 "http://localhost:8646",
		DataDir:                    dir,
		CentralBankAPIURL:          "http://cbweb3-api-gateway.central-bank-brazil:8080",
		ZetoTokenAddress:           "0xZETO",
		ParticipantRegistryAddress: "0xREG",
	})

	if done, _ := s.Check(context.Background()); done {
		t.Fatal("Check should be false before Run")
	}
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if done, _ := s.Check(context.Background()); !done {
		t.Fatal("Check should be true after Run")
	}

	data, err := os.ReadFile(cbEnvPath(dir, "bank-itau"))
	if err != nil {
		t.Fatalf("read rendered env: %v", err)
	}
	env := string(data)

	for _, want := range []string{
		"KEYCLOAK_BASE_URL=http://cbweb3-bank-itau-keycloak:8080",
		"KC_REALM=bank-itau",
		"ZETO_TOKEN_ADDRESS=0xZETO",
		"PARTICIPANT_REGISTRY_ADDRESS=0xREG",
		"CENTRAL_BANK_API_URL=http://cbweb3-api-gateway.central-bank-brazil:8080",
		"BESU_RPC_URL=http://host.docker.internal:8646",
		"POSTGRES_DSN=postgres://default:",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("rendered env missing %q", want)
		}
	}

	// The DSN must carry a generated password, not the constant every entity used to
	// share. Asserting the absence is the point: a future "simplification" back to a
	// literal would still satisfy a shape-only check.
	if strings.Contains(env, "postgres://default:default@") {
		t.Error("DSN still carries the shared literal password")
	}
	if !strings.Contains(env, "@cbweb3-bank-itau-postgres") {
		t.Error("DSN lost its postgres host")
	}

	// A commercial bank holds no governance key and issues no certificates.
	if strings.Contains(env, "CB_PRIVATE_KEY=") {
		t.Error("commercial-bank env must not contain CB_PRIVATE_KEY")
	}
	if strings.Contains(env, "CORS_ALLOW_ORIGINS=*") {
		t.Error("CORS origins must be concrete, not a wildcard")
	}
}

func TestCentralBankAPIURL_StripsCredentialPath(t *testing.T) {
	got := centralBankAPIURL("http://cbweb3-api-gateway.central-bank-brazil:8080/api/v1/onboarding/credential-request")
	want := "http://cbweb3-api-gateway.central-bank-brazil:8080"
	if got != want {
		t.Errorf("centralBankAPIURL = %q; want %q", got, want)
	}
}
