package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// HubConfig parametrizes the found-hub mode. External readiness gates and the
// Keycloak client-secret read are injectable so the step set is testable with a
// FakeRunner and no live Besu/Keycloak.
type HubConfig struct {
	Runner       exec.CommandRunner
	ContractsDir string // <repo>/scenario-b/contracts
	TemplatesDir string // <repo>/scenario-b/provisioning/templates
	OutDir       string // bundle destination
	ChainID      uint64
	HubRPC       string
	HubWS        string
	HubEnvFile   string   // hub .env (render target)
	KeycloakEnv  []string // backend .env files for client-secret write-back

	GenesisDir     string // where genesis.json is seeded (mounted by the hub template)
	ValidatorCount int    // QBFT validators (default 1)
	BesuImage      string // besu image for genesis generation (default hyperledger/besu:25.8.0)

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
}

// WithDefaults fills real implementations for the injectable seams when unset.
func (c *HubConfig) WithDefaults() {
	if c.ValidatorCount == 0 {
		c.ValidatorCount = 1
	}
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.GenesisDir == "" {
		c.GenesisDir = filepath.Join(c.OutDir, "genesis")
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.HubRPC, 60*time.Second) }
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(ctx context.Context) error { return nil } // template healthcheck gates it
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(ctx context.Context) (string, error) {
			out, err := c.Runner.Run(ctx, "docker", "exec", "keycloak", "cat", "/tmp/hub-client-secret")
			return string(bytes.TrimSpace(out)), err
		}
	}
}

func (c HubConfig) broadcastPath() string {
	return filepath.Join(c.ContractsDir, "broadcast", "CBWeb3Hub.s.sol",
		fmt.Sprintf("%d", c.ChainID), "run-latest.json")
}

func (c HubConfig) template(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

// FoundHubSteps builds the ordered found-hub step set.
func FoundHubSteps(c HubConfig) []Step {
	c.WithDefaults()
	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.template(tmpl), "--env-file", c.HubEnvFile, "up", "-d")
			return err
		}
	}

	return []Step{
		{
			Name:  "build-contracts",
			Check: func(context.Context) (bool, error) { return dirNonEmpty(filepath.Join(c.ContractsDir, "out")), nil },
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "forge", "build", "--root", c.ContractsDir)
				return err
			},
		},
		genGenesisHubStep(c),
		{
			Name: "start-besu-hub",
			Deps: []string{"gen-genesis-hub"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.template("hub"), "--env-file", c.HubEnvFile, "up", "-d"); err != nil {
					return err
				}
				return c.WaitRPC(ctx)
			},
		},
		{
			Name: "deploy-hub-contracts",
			Deps: []string{"build-contracts", "start-besu-hub"},
			Check: func(context.Context) (bool, error) {
				_, err := addrs.ParseBroadcastList(c.broadcastPath())
				return err == nil, nil
			},
			Run: func(ctx context.Context) error {
				if err := c.WaitRPC(ctx); err != nil { // readiness gate
					return err
				}
				// Single forge script; the 7 contracts' order is internal to Solidity.
				_, err := c.Runner.Run(ctx, "forge", "script",
					"script/CBWeb3Hub.s.sol:DeployCBWeb3Hub",
					"--root", c.ContractsDir, "--rpc-url", c.HubRPC, "--broadcast")
				return err
			},
		},
		{
			Name: "provision-keycloak-hub",
			Deps: []string{"deploy-hub-contracts"},
			Check: func(context.Context) (bool, error) {
				// idempotent: skip if the secret is already written to all targets
				if len(c.KeycloakEnv) == 0 {
					return false, nil
				}
				for _, env := range c.KeycloakEnv {
					if !addrs.HasAddrKey(env, "KEYCLOAK_CLIENT_SECRET") {
						return false, nil
					}
				}
				return true, nil
			},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.template("entity-keycloak"), "--env-file", c.HubEnvFile, "up", "-d"); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
					return err
				}
				secret, err := c.ReadClientSecret(ctx)
				if err != nil {
					return err
				}
				for _, env := range c.KeycloakEnv { // write-back
					if err := addrs.AppendAddr(env, "KEYCLOAK_CLIENT_SECRET", secret); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "render-hub-env",
			Deps: []string{"deploy-hub-contracts", "provision-keycloak-hub"},
			Run: func(context.Context) error {
				m, err := hubContractMap(c.broadcastPath())
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"HUB_IDENTITY_REGISTRY_ADDRESS":  m["identityRegistry"],
					"HUB_TOKEN_A_ADDRESS":            m["tCeBM_BRL"],
					"HUB_TOKEN_B_ADDRESS":            m["tCeBM_EUR"],
					"FX_AGREEMENT_CONTRACT_ADDRESS":  m["fxAgreement"],
					"PAIR_REGISTRY_CONTRACT_ADDRESS": m["pairRegistry"],
					"CURRENCY_REGISTRY_ADDRESS":      m["currencyRegistry"],
					"MANUAL_ORACLE_ADDRESS":          m["manualOracle"],
				} {
					if err := addrs.AppendAddr(c.HubEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{Name: "start-hub-infra", Deps: []string{"render-hub-env"}, Run: compose("entity-infra")},
		{Name: "start-hub-backend", Deps: []string{"start-hub-infra", "render-hub-env"}, Run: compose("entity-backend")},
		{Name: "start-hub-frontend", Deps: []string{"start-hub-backend"}, Run: compose("entity-frontend")},
		{Name: "start-relay", Deps: []string{"deploy-hub-contracts"}, Run: compose("relay")},
		{Name: "start-noc", Deps: []string{"start-relay"}, Run: compose("noc")},
		{
			Name: "emit-hub-bundle",
			Deps: []string{"deploy-hub-contracts"},
			Check: func(context.Context) (bool, error) {
				_, err := os.Stat(filepath.Join(c.OutDir, "bundles", "hub.bundle.yaml"))
				return err == nil, nil
			},
			Run: func(context.Context) error {
				m, err := hubContractMap(c.broadcastPath())
				if err != nil {
					return err
				}
				b := bundle.HubBundle{ChainID: c.ChainID, HubRPC: c.HubRPC, HubWS: c.HubWS, Contracts: m}
				_, err = bundle.EmitHub(b, c.OutDir)
				return err
			},
		},
	}
}

// hubContractMap maps the CBWeb3Hub broadcast deployments to bundle keys. The
// two TokenizedCentralBankMoney deployments are BRL then EUR (Solidity order).
func hubContractMap(broadcastPath string) (map[string]string, error) {
	list, err := addrs.ParseBroadcastList(broadcastPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	tcebmSeen := 0
	for _, d := range list {
		switch d.Name {
		case "IdentityRegistry":
			out["identityRegistry"] = d.Address
		case "TokenizedCentralBankMoney":
			if tcebmSeen == 0 {
				out["tCeBM_BRL"] = d.Address
			} else {
				out["tCeBM_EUR"] = d.Address
			}
			tcebmSeen++
		case "FXAgreement":
			out["fxAgreement"] = d.Address
		case "PairRegistry":
			out["pairRegistry"] = d.Address
		case "CurrencyRegistry":
			out["currencyRegistry"] = d.Address
		case "ManualOracle":
			out["manualOracle"] = d.Address
		}
	}
	return out, nil
}

func dirNonEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// waitRPC polls eth_blockNumber until the node answers or the timeout elapses.
func waitRPC(ctx context.Context, rpcURL string, timeout time.Duration) error {
	if rpcURL == "" {
		return nil
	}
	deadline := time.Now().Add(timeout)
	body := []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hub RPC %s not ready within %s", rpcURL, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
