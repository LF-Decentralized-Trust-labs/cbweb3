// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// entityenv.go renders a per-entity backend .env (feature 034), parametrized by
// entity type/spoke and pointing at the entity's OWN dedicated infra (Postgres,
// Redis, and — when keycloak: per-entity — its own Keycloak). It mirrors the
// canonical backend/config/.env.infra.<entity> structure without reusing those
// fixed files (which only cover the 6 reference entities). No private keys are
// rendered except the local-profile CB bootstrap key (same as the reference local
// env); prod uses the KeyProvider/Docker secrets.

// EntityEnvData holds the values rendered into a per-entity backend .env.
type EntityEnvData struct {
	EntityName    string // e.g. "central-bank-brazil" / "bank-itau"
	IsCentralBank bool
	SpokeID       string
	FiatSymbol    string

	// Keycloak the entity authenticates against (CB's instance for CB/NOC/Governance;
	// the bank's own when keycloak: per-entity).
	KeycloakURL    string
	KCRealm        string
	KCClientID     string
	KCClientSecret string

	// Dedicated infra (per-entity).
	PostgresContainer string
	PostgresPort      int
	PostgresUser      string
	PostgresPassword  string
	DBName            string
	RedisContainer    string
	RedisPort         int
	RedisDB           int

	// PKI (local dev files).
	CACertFile string
	CAKeyFile  string

	// Blockchain.
	BesuRPCURL string
	ChainID    int

	// Contract addresses (from found .deployed-addrs.env / join bundle).
	ParticipantRegistryAddress string
	ZetoTokenAddress           string
	FiatTokenAddress           string
	HTLCAddress                string
	TokenAddress               string
	SpokeBridgeAddress         string

	// Central-bank only.
	CBPrivateKey     string
	GovernanceUserID string

	// Commercial-bank only.
	CentralBankAPIURL string

	RelaySecret string
	CORSOrigins string
}

const entityEnvTemplate = `# Rendered by cbweb3 toolkit (feature 034) — do NOT edit by hand.
# Entity: {{.EntityName}} (spoke {{.SpokeID}})

# Dedicated infra (per-entity)
POSTGRES_CONTAINER_NAME={{.PostgresContainer}}
POSTGRES_IMAGE_TAG=17-alpine
POSTGRES_PORT={{.PostgresPort}}
POSTGRES_USER={{.PostgresUser}}
POSTGRES_PASSWORD={{.PostgresPassword}}
DB_NAME={{.DBName}}
DATABASE_URL=postgres://{{.PostgresUser}}:{{.PostgresPassword}}@{{.PostgresContainer}}:5432/{{.DBName}}?sslmode=disable
POSTGRES_DSN=postgres://{{.PostgresUser}}:{{.PostgresPassword}}@{{.PostgresContainer}}:5432/{{.DBName}}?sslmode=disable

REDIS_CONTAINER_NAME={{.RedisContainer}}
REDIS_IMAGE_TAG=7-alpine
REDIS_PORT={{.RedisPort}}
REDIS_DB={{.RedisDB}}

# Keycloak (CB instance for CB/NOC/Governance; own when per-entity)
KC_BASE_PATH={{.KeycloakURL}}
KEYCLOAK_BASE_URL={{.KeycloakURL}}
KC_REALM={{.KCRealm}}
KC_CLIENT_ID={{.KCClientID}}
KC_CLIENT_SECRET={{.KCClientSecret}}
# Names the auth service reads for the admin (service-account) token + realm.
KEYCLOAK_REALM={{.KCRealm}}
KEYCLOAK_CLIENT_ID={{.KCClientID}}
KEYCLOAK_CLIENT_SECRET={{.KCClientSecret}}

# PKI (local dev)
CA_CERT_FILE={{.CACertFile}}
CA_KEY_FILE={{.CAKeyFile}}

# Blockchain
BLOCKCHAIN_CLIENT=besu
BESU_RPC_URL={{.BesuRPCURL}}
BESU_CHAIN_ID={{.ChainID}}

# Contracts
PARTICIPANT_REGISTRY_ADDRESS={{.ParticipantRegistryAddress}}
ZETO_TOKEN_ADDRESS={{.ZetoTokenAddress}}
FIAT_TOKEN_ADDRESS={{.FiatTokenAddress}}
HTLC_ADDRESS={{.HTLCAddress}}
TOKEN_ADDRESS={{.TokenAddress}}
SPOKE_BRIDGE_ADDRESS={{.SpokeBridgeAddress}}

# Interop
INTERNAL_RELAY_AUTH_SECRET={{.RelaySecret}}
FIAT_SYMBOL={{.FiatSymbol}}
CORS_ALLOW_ORIGINS={{.CORSOrigins}}
{{if .IsCentralBank}}
# Central bank only — governance bootstrap
CB_PRIVATE_KEY={{.CBPrivateKey}}
GOVERNANCE_USER_ID={{.GovernanceUserID}}
{{else}}
# Commercial bank only — points to the central bank's api-gateway for onboarding/proxy
CENTRAL_BANK_API_URL={{.CentralBankAPIURL}}
{{end}}`

// RenderEntityEnv renders the per-entity .env to outPath (creating parent dirs).
func RenderEntityEnv(data EntityEnvData, outPath string) error {
	tmpl, err := template.New("entityenv").Parse(entityEnvTemplate)
	if err != nil {
		return fmt.Errorf("parse entity env template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for entity env: %w", err)
	}
	f, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create entity env file: %w", err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("render entity env: %w", err)
	}
	return nil
}
