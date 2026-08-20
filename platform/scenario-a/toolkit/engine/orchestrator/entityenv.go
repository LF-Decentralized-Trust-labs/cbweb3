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
	// KCAudience is the expected "aud" the auth service enforces (KEYCLOAK_AUDIENCE).
	// The realm's backend login clients carry a matching audience mapper (see
	// keycloak.go). Empty leaves aud enforcement off (iss is still enforced).
	KCAudience string

	// Dedicated infra (per-entity).
	PostgresContainer string
	PostgresPort      int
	PostgresUser      string
	PostgresPassword  string
	DBName            string
	RedisContainer    string
	RedisPort         int
	RedisDB           int
	// RedisPassword is required: this Redis holds the auth service's PKI login
	// nonces, and the service reads REDIS_PASSWORD from its environment.
	RedisPassword string

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
	// AMMAddress is the optional AutomatedMarketMaker address. Empty unless an AMM is
	// deployed and recorded; when set, the CB's compliance service drives the on-chain
	// circuit breaker instead of a database-only toggle.
	AMMAddress string

	// BesuOperatorKey is the hex private key (no 0x) the payment-orchestrator signs
	// Besu-layer transactions with (HTLC/fCeBM). Empty disables the Besu path. In
	// local it is the well-known public dev operator key exported from the
	// KeyProvider; prod wires the signing key via KMS/secrets, not this field.
	BesuOperatorKey string
	// EntityBesuAddress is this entity's on-chain wallet (derived from the operator
	// key in local). The api-gateway escrow proxy stamps it as requester_besu_address
	// on deposit/escrow/redeem, and it is the address fCeBM balances read against.
	EntityBesuAddress string
	// PaladinIdentity is this entity's Paladin identity (e.g. funded_operator@spoke-brl-bank-bradesco).
	// The api-gateway escrow proxy stamps it as requester_paladin_identity — the Zeto
	// mint recipient for reserve tokenisation (escrow). Also read by payment-orchestrator.
	PaladinIdentity string
	// CBPaladinIdentity is the central bank's Paladin identity — the Zeto transfer
	// receiver a commercial bank targets on redeem. Empty for the CB itself.
	CBPaladinIdentity string
	// PaladinIdentities is the comma-separated consortium FX-party roster rendered
	// as PALADIN_IDENTITIES (the manifest's fxPartyRoster). Empty leaves the gateway
	// to serve only live local Pente membership.
	PaladinIdentities string

	// Pente (bilateral private FX — feature 035). PenteEnabled turns on the on-chain
	// FXAgreement path in the payment-orchestrator; PenteBaseURL is the Paladin JSON-RPC
	// the Pente client calls. FXAgreementPenteAddress is the in-group FXAgreement address —
	// empty at render time (deploy-fxa runs in the join soft tail, after the backend starts),
	// resolved at runtime via the Pente context / A6 registration. See PLAN.md.
	PenteEnabled            bool
	PenteBaseURL            string
	FXAgreementPenteAddress string

	// Central-bank only.
	CBPrivateKey     string
	GovernanceUserID string

	// Commercial-bank only.
	CentralBankAPIURL string

	// KMS seed (commercial-bank only, local profile). Aligns the bank's onboarding
	// KMS wallet with its Besu operator key so the onboarded+verified participant is
	// the SAME address the payment-orchestrator signs HTLC txs with — otherwise the
	// HTLC lock reverts onlyVerified. KMSSeedKeyID is the bank code (CreateOnboardingKey
	// keys the KMS by bank code); KMSSeedPrivateKey is the bank's operator key. Empty
	// for the CB and in prod (banks custody their own keys).
	KMSSeedKeyID      string
	KMSSeedPrivateKey string

	RelaySecret string
	CORSOrigins string

	// RelayURL is the Cacti relay base URL reachable from inside the container
	// (host.docker.internal:4000 for a co-located relay). The api-gateway uses it
	// to federate the FX-party roster across every spoke in the relay's registry,
	// so cross-spoke identities need no static fxPartyRoster. Empty leaves the
	// gateway on local Pente membership only.
	RelayURL string
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
REDIS_PASSWORD={{.RedisPassword}}

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
# Token validation: iss is derived from KEYCLOAK_BASE_URL + realm (always enforced);
# aud is enforced when set. Backend login clients carry a matching audience mapper.
KEYCLOAK_AUDIENCE={{.KCAudience}}

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
# AutomatedMarketMaker (optional). When set, the CB compliance service drives the
# on-chain circuit breaker (pause 1-of-N / resume 2-of-N); empty -> DB-only toggle.
AMM_ADDRESS={{.AMMAddress}}

# Besu-layer signing key (local dev operator; empty in prod — see BesuOperatorKey)
BESU_OPERATOR_KEY={{.BesuOperatorKey}}
# This entity's Besu wallet — escrow proxy stamps it as requester_besu_address
ENTITY_BESU_ADDRESS={{.EntityBesuAddress}}
# Paladin identities — escrow proxy stamps requester_paladin_identity (Zeto mint
# recipient); CB identity is the redeem Zeto-transfer receiver.
PALADIN_IDENTITY={{.PaladinIdentity}}
CB_PALADIN_IDENTITY={{.CBPaladinIdentity}}
# Optional static override for the FX party roster (manifest's fxPartyRoster).
# The network-wide roster is normally federated via RELAY_URL (below); this only
# pins/augments it. Empty -> roster = live local membership ∪ federated peers.
PALADIN_IDENTITIES={{.PaladinIdentities}}

# Interop
INTERNAL_RELAY_AUTH_SECRET={{.RelaySecret}}
# Cacti relay base URL. When set, the api-gateway federates the FX-party roster
# across every spoke in the relay's registry (each spoke's CB gateway is queried
# for its local roster), so a new spoke appears network-wide with no manifest
# edit. Empty -> local Pente membership only.
RELAY_URL={{.RelayURL}}
FIAT_SYMBOL={{.FiatSymbol}}
CORS_ALLOW_ORIGINS={{.CORSOrigins}}

# Pente (bilateral private FXAgreement — feature 035)
PENTE_ENABLED={{.PenteEnabled}}
PENTE_BASE_URL={{.PenteBaseURL}}
FX_AGREEMENT_PENTE_CONTRACT_ADDRESS={{.FXAgreementPenteAddress}}
# FX indexer (feature 035): projects on-chain agreements from every bilateral Pente group this
# node belongs to into fx_agreements. Enabled for BOTH the central bank (aggregate view) and each
# commercial bank — a bank must see the destination-leg agreements proposed on_behalf into its
# CB↔bank group so they surface in its portal for acceptance. Chain-driven; only reads own groups.
FX_INDEXER_ENABLED=true
{{if .IsCentralBank}}
# Central bank only — governance bootstrap
CB_PRIVATE_KEY={{.CBPrivateKey}}
GOVERNANCE_USER_ID={{.GovernanceUserID}}
{{else}}
# Commercial bank only — points to the central bank's api-gateway for onboarding/proxy
CENTRAL_BANK_API_URL={{.CentralBankAPIURL}}
# KMS seed (local dev): make the onboarding wallet == the bank's Besu operator key so
# the onboarded+verified participant is the HTLC signer. auth reads these from env_file.
# Empty in prod (bank-custodied keys).
KMS_SEED_KEY_ID={{.KMSSeedKeyID}}
KMS_SEED_PRIVATE_KEY={{.KMSSeedPrivateKey}}
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
