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

	GenesisDir     string // legacy; node state now lives in named volumes
	ValidatorCount int    // QBFT validators (default 1)
	BesuImage      string // besu image for genesis generation (default hyperledger/besu:25.8.0)

	// Compose interpolation for the hub template (rendered into HubEnvFile before
	// start-besu-hub). Node state is seeded into the `<VolumePrefix>_*` volumes.
	VolumePrefix    string // e.g. "hub-cbweb3"  → <p>_genesis, <p>_besu_data
	ContainerPrefix string // e.g. "cbweb3-hub"  → HUB_CONTAINER_PREFIX
	NetPrefix       string // e.g. "hub-cbweb3"  → HUB_NET_PREFIX
	RPCPort         int    // HUB_RPC_PORT (host)
	WSPort          int    // HUB_WS_PORT (host)
	P2PPort         int    // HUB_P2P_PORT (host)

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
	if c.VolumePrefix == "" {
		c.VolumePrefix = "hub-cbweb3"
	}
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "cbweb3-hub"
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.VolumePrefix
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

func (c HubConfig) genesisVolume() string  { return c.VolumePrefix + "_genesis" }
func (c HubConfig) besuDataVolume() string { return c.VolumePrefix + "_besu_data" }

// renderHubComposeEnv writes ALL hub compose-template interpolation vars into
// HubEnvFile (besu + infra + keycloak + backend/frontend/relay/noc), so every
// `docker compose --env-file` step resolves its ${...}. Ports derive from the
// besu RPC port by fixed offsets (single hub per host). Local dev creds only.
func (c HubConfig) renderHubComposeEnv() error {
	e := c.ContainerPrefix
	vars := map[string]string{
		// besu (hub template)
		"BESU_IMAGE":           c.BesuImage,
		"HUB_CONTAINER_PREFIX": c.ContainerPrefix,
		"HUB_NET_PREFIX":       c.NetPrefix,
		"HUB_VOLUME_PREFIX":    c.VolumePrefix,
		"HUB_RPC_PORT":         itoa(c.RPCPort),
		"HUB_WS_PORT":          itoa(c.WSPort),
		"HUB_P2P_PORT":         itoa(c.P2PPort),
		// shared entity vars (infra/keycloak/backend/frontend/noc)
		"CONTAINER_PREFIX":     c.ContainerPrefix,
		"ENTITY":               "hub",
		"ENTITY_NET_PREFIX":    c.NetPrefix,
		"ENTITY_VOLUME_PREFIX": c.VolumePrefix,
		"ENTITY_RPC_PORT":      itoa(c.RPCPort),
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER":     "cbweb3",
		"POSTGRES_PASSWORD": "cbweb3",
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak (joins the entity infra network; DB is the infra postgres)
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": "admin",
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-hub-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.RPCPort + 7000),
		// backend / frontend / relay / noc (images must be pre-built locally)
		"GATEWAY_PORT":         itoa(c.RPCPort + 8000),
		"GATEWAY_URL":          fmt.Sprintf("http://localhost:%d", c.RPCPort+8000),
		"BACKEND_IMAGE":        "cbweb3b-hub-backend:local",
		"FRONTEND_IMAGE":       "cbweb3b-hub-frontend:local",
		"FRONTEND_PORT":        itoa(c.RPCPort + 9000),
		"RELAY_IMAGE":          "cbweb3b-relay:local",
		"RELAY_CONTAINER_NAME": e + "-relay",
		"RELAY_NET_PREFIX":     c.NetPrefix,
		"RELAY_VOLUME_PREFIX":  c.VolumePrefix,
		"RELAY_PORT":           "4000",
		"NOC_AGENT_BESU_RPC":   fmt.Sprintf("http://%s-hub-validator:8545", e),
		"NOC_AGENT_ENTITY":     "hub",
		"NOC_AGENT_IMAGE":      "cbweb3b-noc-agent:local",
		"NOC_BACKEND_IMAGE":    "cbweb3b-noc-backend:local",
		"NOC_BACKEND_PORT":     itoa(c.RPCPort + 11000),
		"NOC_DB_NAME":          "noc",
		"NOC_DB_USER":          "cbweb3",
		"NOC_DB_PASSWORD":      "cbweb3",
		"NOC_NET_PREFIX":       c.NetPrefix,
		"NOC_PORTAL_IMAGE":     "cbweb3b-noc-portal:local",
		"NOC_PORTAL_PORT":      itoa(c.RPCPort + 12000),
		"NOC_VOLUME_PREFIX":    c.VolumePrefix,
	}
	for k, v := range vars {
		if err := addrs.AppendAddr(c.HubEnvFile, k, v); err != nil {
			return err
		}
	}
	return nil
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
			Name: "render-hub-compose-env",
			Deps: []string{"gen-genesis-hub"},
			Run:  func(context.Context) error { return c.renderHubComposeEnv() },
		},
		{
			Name: "start-besu-hub",
			Deps: []string{"render-hub-compose-env"},
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
				// Single forge script (the 7 contracts' order is internal to
				// Solidity). Runs inside the contracts project, with the local dev
				// deployer/admin/CB accounts, and --legacy for the zero-gas chain.
				cmd := fmt.Sprintf("cd %q && "+
					"DEPLOYER_PRIVATE_KEY=%s ADMIN_PRIVATE_KEY=%s ADMIN_ADDRESS=%s "+
					"CENTRAL_BANK_ADDRESS=%s CENTRAL_BANK_B_ADDRESS=%s "+
					"forge script script/CBWeb3Hub.s.sol:DeployCBWeb3Hub --rpc-url %s --broadcast --legacy",
					c.ContractsDir, devDeployerKey, devDeployerKey, devDeployerAddr,
					devDeployerAddr, devCBBAddr, c.HubRPC)
				_, err := c.Runner.Run(ctx, "sh", "-c", cmd)
				return err
			},
		},
		// Infra (postgres + redis) creates the entity_infra_network + DB that
		// keycloak/backend join — must come BEFORE them.
		{Name: "start-hub-infra", Deps: []string{"render-hub-compose-env"}, Run: compose("entity-infra")},
		{
			Name: "provision-keycloak-hub",
			Deps: []string{"start-hub-infra"},
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
			Deps: []string{"deploy-hub-contracts"},
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
		{Name: "start-hub-backend", Deps: []string{"start-hub-infra", "render-hub-env", "provision-keycloak-hub"}, Run: compose("entity-backend")},
		{Name: "start-hub-frontend", Deps: []string{"start-hub-backend"}, Run: compose("entity-frontend")},
		{Name: "start-relay", Deps: []string{"deploy-hub-contracts"}, Run: compose("relay")},
		{Name: "start-noc", Deps: []string{"start-relay", "start-hub-infra"}, Run: compose("noc")},
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
