// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// renderBankEnvStep renders a commercial bank's backend .env (feature 034 US2)
// into the entity data dir. Unlike the CB step, the contract addresses come from
// the join bundle (not a local .deployed-addrs.env), and the env points at the
// bank's OWN dedicated infra and (per-entity) Keycloak. CB_PRIVATE_KEY is never
// rendered — a commercial bank holds no governance key.
type renderBankEnvStep struct {
	spokeID                    string
	bankCode                   string
	currency                   string
	besuRPCPort                int
	chainID                    int
	besuRPCURL                 string
	dataDir                    string
	centralBankAPIURL          string
	zetoTokenAddress           string
	participantRegistryAddress string
	fiatTokenAddress           string
	htlcAddress                string
	besuOperatorKey            string
	frontendHost               string
	fxPartyRoster              []string
}

func newRenderBankEnvStep(p bankEnvParams) Step {
	return &renderBankEnvStep{
		spokeID:                    p.SpokeID,
		bankCode:                   p.BankCode,
		currency:                   p.Currency,
		besuRPCPort:                p.BesuRPCPort,
		chainID:                    p.ChainID,
		besuRPCURL:                 p.BesuRPCURL,
		dataDir:                    p.DataDir,
		centralBankAPIURL:          p.CentralBankAPIURL,
		zetoTokenAddress:           p.ZetoTokenAddress,
		participantRegistryAddress: p.ParticipantRegistryAddress,
		fiatTokenAddress:           p.FiatTokenAddress,
		htlcAddress:                p.HTLCAddress,
		besuOperatorKey:            p.BesuOperatorKey,
		frontendHost:               p.FrontendHost,
		fxPartyRoster:              p.FXPartyRoster,
	}
}

// bankEnvParams groups the inputs for renderBankEnvStep.
type bankEnvParams struct {
	SpokeID                    string
	BankCode                   string
	Currency                   string
	BesuRPCPort                int
	ChainID                    int
	BesuRPCURL                 string
	DataDir                    string
	CentralBankAPIURL          string
	ZetoTokenAddress           string
	ParticipantRegistryAddress string
	FiatTokenAddress           string
	HTLCAddress                string
	BesuOperatorKey            string
	FrontendHost               string
	FXPartyRoster              []string
}

func (s *renderBankEnvStep) Name() string { return StepRenderBankEnv }

func (s *renderBankEnvStep) Check(_ context.Context) (bool, error) {
	if _, err := os.Stat(cbEnvPath(s.dataDir, s.bankCode)); err == nil {
		return true, nil
	}
	return false, nil
}

func (s *renderBankEnvStep) Run(_ context.Context) error {
	prefix := entityContainerPrefix(s.bankCode)
	ports := entityPorts(s.besuRPCPort)

	data := EntityEnvData{
		EntityName:    s.bankCode,
		IsCentralBank: false,
		SpokeID:       s.spokeID,
		FiatSymbol:    s.currency,
		// Per-entity Keycloak: the bank runs its own instance on its own network.
		KeycloakURL:    fmt.Sprintf("http://%s-keycloak:8080", prefix),
		KCRealm:        s.bankCode,
		KCClientID:     s.bankCode + "-client",
		KCClientSecret: s.bankCode + "-local-secret",

		PostgresContainer: prefix + "-postgres",
		PostgresPort:      ports.Postgres,
		PostgresUser:      "default",
		PostgresPassword:  "default",
		DBName:            entityDBName(s.bankCode),
		RedisContainer:    prefix + "-redis",
		RedisPort:         ports.Redis,
		RedisDB:           0,

		// A commercial bank does not issue participant certificates (that is the
		// CB's role), so no CA is mounted; compliance runs without a CA.
		CACertFile: "",
		CAKeyFile:  "",

		// Reached from inside the container; the bank's Besu is published on the host.
		BesuRPCURL: hostInternalURL(s.besuRPCURL),
		// The backend's Besu-signing path (HTLC/fCeBM) needs the real chain id:
		// go-ethereum's NewKeyedTransactorWithChainID panics on chainID 0.
		ChainID: s.chainID,

		ParticipantRegistryAddress: s.participantRegistryAddress,
		ZetoTokenAddress:           s.zetoTokenAddress,
		FiatTokenAddress:           s.fiatTokenAddress,
		HTLCAddress:                s.htlcAddress,
		// Local: operator key signs the bank's Besu-layer txs (HTLC/fCeBM). Empty
		// leaves the Besu path off. EntityBesuAddress is the bank's wallet, stamped by
		// the escrow proxy as requester_besu_address on deposit/escrow/redeem.
		BesuOperatorKey:   s.besuOperatorKey,
		EntityBesuAddress: operatorAddressFromHex(s.besuOperatorKey),
		// The bank's Paladin identity is stamped as requester_paladin_identity (Zeto
		// mint recipient for reserve tokenisation); CB identity is the redeem receiver.
		PaladinIdentity:   paladinIdentity(bankNodeName(s.spokeID, s.bankCode)),
		CBPaladinIdentity: paladinIdentity(cbNodeName(s.spokeID)),
		// Consortium FX-party roster (cross-spoke identities) from the manifest.
		PaladinIdentities: strings.Join(s.fxPartyRoster, ","),

		// Commercial bank: points at the central bank's api-gateway for onboarding/proxy.
		CentralBankAPIURL: s.centralBankAPIURL,

		// Seed the bank's auth KMS with its operator key so onboarding (CreateOnboardingKey,
		// keyed by bank code) returns the operator address. That address becomes the
		// participant the CB verifies on KYC approval, so the HTLC signer passes onlyVerified.
		KMSSeedKeyID:      s.bankCode,
		KMSSeedPrivateKey: s.besuOperatorKey,

		// On-chain FXAgreement (Pente): the bank proposes into the CB↔bank group via its
		// own Paladin. The in-group FXAgreement address is resolved at runtime (deploy-fxa
		// runs after the backend starts), so it is left empty here.
		PenteEnabled: true,
		PenteBaseURL: hostInternalURL(bankPaladinURL(s.besuRPCPort)),

		RelaySecret: "cbweb3-relay-shared-secret",
		CORSOrigins: bankCORSOrigins(ports, s.frontendHost),
	}
	return RenderEntityEnv(data, cbEnvPath(s.dataDir, s.bankCode))
}
