// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
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
	besuRPCURL                 string
	dataDir                    string
	centralBankAPIURL          string
	zetoTokenAddress           string
	participantRegistryAddress string
}

func newRenderBankEnvStep(p bankEnvParams) Step {
	return &renderBankEnvStep{
		spokeID:                    p.SpokeID,
		bankCode:                   p.BankCode,
		currency:                   p.Currency,
		besuRPCPort:                p.BesuRPCPort,
		besuRPCURL:                 p.BesuRPCURL,
		dataDir:                    p.DataDir,
		centralBankAPIURL:          p.CentralBankAPIURL,
		zetoTokenAddress:           p.ZetoTokenAddress,
		participantRegistryAddress: p.ParticipantRegistryAddress,
	}
}

// bankEnvParams groups the inputs for renderBankEnvStep.
type bankEnvParams struct {
	SpokeID                    string
	BankCode                   string
	Currency                   string
	BesuRPCPort                int
	BesuRPCURL                 string
	DataDir                    string
	CentralBankAPIURL          string
	ZetoTokenAddress           string
	ParticipantRegistryAddress string
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
		ChainID:    0,

		ParticipantRegistryAddress: s.participantRegistryAddress,
		ZetoTokenAddress:           s.zetoTokenAddress,

		// Commercial bank: points at the central bank's api-gateway for onboarding/proxy.
		CentralBankAPIURL: s.centralBankAPIURL,

		RelaySecret: "cbweb3-relay-shared-secret",
		CORSOrigins: fmt.Sprintf("http://localhost:%d,http://localhost:%d", ports.FrontendPrimary, ports.FrontendSecondary),
	}
	return RenderEntityEnv(data, cbEnvPath(s.dataDir, s.bankCode))
}
