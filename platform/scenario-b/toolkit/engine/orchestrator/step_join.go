package orchestrator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/pki"
)

// JoinConfig parametrizes the join mode (a commercial bank attaching to its
// spoke as a non-validating full node). External gates/reads are injectable so
// the step set is testable without a live Besu/Keycloak.
type JoinConfig struct {
	Runner          exec.CommandRunner
	TemplatesDir    string
	OutDir          string
	BankID          string
	Institution     string
	SpokeID         string
	SpokeChainID    uint64
	BankRPC         string // RPC of the bank's own node (wait-sync gate)
	SpokeBundlePath string
	GenesisDir      string
	DataDir         string
	BankEnvFile     string
	KeycloakEnv     []string
	VolumePrefix    string // <p>_genesis, <p>_besu_data (node state in named volumes)
	ContainerPrefix string // container name prefix (<p>-<entity>-besu, ...)
	NetPrefix       string // docker network name prefix
	Entity          string // compose ENTITY label (e.g. the bank id)
	RPCPort         int    // host port -> besu 8545; other service ports derive by offset
	WSPort          int    // host port -> besu 8546
	P2PPort         int    // host port -> besu 30303
	HubRPC          string // hub RPC (spoke backend reaches the hub via host.docker.internal:<port>)
	BesuImage       string
	GatewayURL      string

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitSync         func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
	EthSyncing       EthSyncing
}

func (c *JoinConfig) WithDefaults() {
	if c.GenesisDir == "" {
		c.GenesisDir = filepath.Join(c.DataDir, "genesis")
	}
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = c.BankID
	}
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "cbweb3-" + c.BankID
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.VolumePrefix
	}
	if c.Entity == "" {
		c.Entity = c.BankID
	}
	if c.RPCPort == 0 {
		c.RPCPort = 10545
	}
	if c.WSPort == 0 {
		c.WSPort = c.RPCPort + 1
	}
	if c.P2PPort == 0 {
		c.P2PPort = 30303
	}
	if c.GatewayURL == "" {
		c.GatewayURL = fmt.Sprintf("http://localhost:%d", c.RPCPort+8000)
	}
	if c.EthSyncing == nil {
		c.EthSyncing = ethSyncing
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.BankRPC, 60*time.Second) }
	}
	if c.WaitSync == nil {
		c.WaitSync = func(ctx context.Context) error {
			return waitSync(ctx, c.BankRPC, 300*time.Second, 2*time.Second, c.EthSyncing)
		}
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(ctx context.Context) error {
			return waitHTTPOK(ctx, fmt.Sprintf("http://localhost:%d/realms/master", c.keycloakPort()), 180*time.Second)
		}
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(context.Context) (string, error) { return bankKeycloakSecret, nil }
	}
}

func (c JoinConfig) genesisVolume() string { return c.VolumePrefix + "_genesis" }
func (c JoinConfig) keycloakPort() int     { return c.RPCPort + 7000 }

// keycloakContainer matches entity-keycloak.compose.yaml's container_name.
func (c JoinConfig) keycloakContainer() string {
	return c.ContainerPrefix + "-" + c.Entity + "-keycloak"
}

// Local Keycloak realm/client provisioned by join (OIDC for the bank backend).
const (
	bankKeycloakRealm  = "cbweb3"
	bankKeycloakClient = "bank-backend"
	bankKeycloakSecret = "bank-backend-local-secret" // local-only, not a real secret
)

// provisionKeycloakRealm creates the realm + confidential client via kcadm
// (idempotent: create failures are ignored).
func (c JoinConfig) provisionKeycloakRealm(ctx context.Context) error {
	kc := "/opt/keycloak/bin/kcadm.sh"
	script := fmt.Sprintf(
		"%[1]s config credentials --server http://localhost:8080 --realm master --user %[2]s --password %[3]s && "+
			"(%[1]s create realms -s realm=%[4]s -s enabled=true || true) && "+
			"(%[1]s create clients -r %[4]s -s clientId=%[5]s -s secret=%[6]s -s enabled=true "+
			"-s publicClient=false -s serviceAccountsEnabled=true -s directAccessGrantsEnabled=true || true)",
		kc, "admin", "admin", bankKeycloakRealm, bankKeycloakClient, bankKeycloakSecret)
	_, err := c.Runner.Run(ctx, "docker", "exec", c.keycloakContainer(), "bash", "-c", script)
	return err
}

// renderBankComposeEnv writes ALL bank compose-template interpolation vars into
// BankEnvFile (join besu + infra + keycloak + backend/frontend). The founding
// CB's advertised enode (from the spoke bundle) is the --bootnodes value. Ports
// derive from RPCPort by fixed offsets. Local dev creds only — no secrets.
func (c JoinConfig) renderBankComposeEnv() error {
	b, err := bundle.LoadSpoke(c.SpokeBundlePath)
	if err != nil {
		return err
	}
	e := c.ContainerPrefix
	hubPort := "8545"
	if u, err := url.Parse(c.HubRPC); err == nil && u.Port() != "" {
		hubPort = u.Port()
	}
	vars := map[string]string{
		// besu (join template)
		"BESU_IMAGE":           c.BesuImage,
		"CONTAINER_PREFIX":     c.ContainerPrefix,
		"ENTITY":               c.Entity,
		"ENTITY_NET_PREFIX":    c.NetPrefix,
		"ENTITY_VOLUME_PREFIX": c.VolumePrefix,
		"ENTITY_RPC_PORT":      itoa(c.RPCPort),
		"ENTITY_WS_PORT":       itoa(c.WSPort),
		"ENTITY_P2P_PORT":      itoa(c.P2PPort),
		"BOOTNODE_ENODE":       b.Enode,
		// hub RPC (bank backend → hub via host.docker.internal:<HUB_RPC_PORT>)
		"HUB_RPC_PORT": hubPort,
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER":     "cbweb3",
		"POSTGRES_PASSWORD": "cbweb3",
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": "admin",
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-" + c.Entity + "-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.keycloakPort()),
		// backend / frontend (api-gateway image shared; must be pre-built)
		"GATEWAY_PORT":   itoa(c.RPCPort + 8000),
		"GATEWAY_URL":    c.GatewayURL,
		"BACKEND_IMAGE":  hubBackendImage,
		"FRONTEND_IMAGE": spokeFrontendImage,
		"FRONTEND_PORT":  itoa(c.RPCPort + 9000),
	}
	for k, v := range vars {
		if err := addrs.AppendAddr(c.BankEnvFile, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (c JoinConfig) bankTemplate(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c JoinConfig) genesisPath() string {
	return filepath.Join(c.GenesisDir, "genesis.json")
}

// JoinSteps builds the ordered join step set (canonical flow, roadmap §6):
// no relay/noc step — the spoke chain is already observed since found-spoke.
func JoinSteps(c JoinConfig) []Step {
	c.WithDefaults()

	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.bankTemplate(tmpl), "--env-file", c.BankEnvFile, "up", "-d")
			return err
		}
	}

	return []Step{
		{
			Name: "consume-spoke-bundle",
			Run: func(context.Context) error {
				_, err := bundle.LoadSpoke(c.SpokeBundlePath)
				return err
			},
		},
		{
			Name: "render-bank-compose-env",
			Deps: []string{"consume-spoke-bundle"},
			// Always re-render (never state-skipped): config/ports/images/bootnode
			// must be fresh in the .env before every compose; AppendAddr upserts.
			Check: func(context.Context) (bool, error) { return false, nil },
			Run:   func(context.Context) error { return c.renderBankComposeEnv() },
		},
		{
			Name: "write-genesis",
			Deps: []string{"consume-spoke-bundle"},
			// Node state lives in a named volume (roadmap §7), so the bundle genesis
			// is seeded into <prefix>_genesis, not the host. Non-destructive guard:
			// skip when the volume genesis already matches the bundle; a divergent
			// genesis is a hard error (the bank must run exactly the spoke genesis —
			// same chainId, CB-only QBFT validator set).
			Check: func(ctx context.Context) (bool, error) {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return false, err
				}
				existing, err := readVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json")
				if err != nil || len(existing) == 0 {
					return false, nil // absent/unreadable → seed
				}
				if sha256.Sum256(existing) == sha256.Sum256([]byte(b.Genesis)) {
					return true, nil
				}
				return false, fmt.Errorf("write-genesis: genesis in volume %s diverges from the spoke bundle; "+
					"the bank must run the spoke genesis", c.genesisVolume())
			},
			Run: func(ctx context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(c.GenesisDir, 0o755); err != nil {
					return err
				}
				if err := writeFileAtomic(c.genesisPath(), []byte(b.Genesis), 0o644); err != nil {
					return err
				}
				return copyHostFileToVolume(ctx, c.Runner, c.GenesisDir, "genesis.json", c.genesisVolume(), "genesis.json", "0644")
			},
		},
		{
			// Non-validating full node: the node is not in the spoke's QBFT
			// validator set (genesis lists only the CB), so it syncs without
			// producing blocks — no besu "non-validator" flag is needed. Uses the
			// join template (--bootnodes = the CB's advertised enode).
			Name: "start-besu-join",
			Deps: []string{"write-genesis", "render-bank-compose-env"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.bankTemplate("entity-besu-join"), "--env-file", c.BankEnvFile, "up", "-d"); err != nil {
					return err
				}
				return c.WaitRPC(ctx)
			},
		},
		{
			Name: "wait-sync",
			Deps: []string{"start-besu-join"},
			Run:  func(ctx context.Context) error { return c.WaitSync(ctx) },
		},
		{
			Name: "wire-addresses",
			Deps: []string{"consume-spoke-bundle"},
			Run: func(context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				for key, name := range map[string]string{
					"SPOKE_IDENTITY_REGISTRY_ADDRESS": "identityRegistry",
					"SPOKE_TCEBM_ADDRESS":             "tCeBM",
					"SPOKE_BRIDGE_ADDRESS":            "spokeBridge",
					"SPOKE_FCEBM_ADDRESS":             "fCeBM",
				} {
					if err := addrs.AppendAddr(c.BankEnvFile, key, b.Contracts[name]); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "provision-keycloak-bank",
			// Infra creates the external infra network + postgres that keycloak joins.
			Deps: []string{"start-bank-infra"},
			Check: func(context.Context) (bool, error) {
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
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.bankTemplate("entity-keycloak"), "--env-file", c.BankEnvFile, "up", "-d"); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
					return err
				}
				if err := c.provisionKeycloakRealm(ctx); err != nil {
					return err
				}
				secret, err := c.ReadClientSecret(ctx)
				if err != nil {
					return err
				}
				for _, env := range c.KeycloakEnv {
					if err := addrs.AppendAddr(env, "KEYCLOAK_CLIENT_SECRET", secret); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{Name: "start-bank-infra", Deps: []string{"render-bank-compose-env", "wait-sync"}, Run: compose("entity-infra")},
		{Name: "start-bank-backend", Deps: []string{"start-bank-infra", "wire-addresses", "provision-keycloak-bank"}, Run: compose("entity-backend")},
		{Name: "start-bank-frontend", Deps: []string{"start-bank-backend"}, Soft: true, Run: compose("entity-frontend")},
		{
			// Deferred PKI tail: the ONLY PKI step of the toolkit. Generates the
			// bank keypair + CSR locally (key 0600, never transmitted). The
			// toolkit never signs the CSR, never generates CA material, and never
			// registers on-chain — those are runtime (FR-009).
			Name: "gen-csr",
			Check: func(context.Context) (bool, error) {
				keyOK := fileExists(filepath.Join(c.pkiDir(), c.BankID+".key"))
				csrOK := fileExists(filepath.Join(c.pkiDir(), c.BankID+".csr"))
				return keyOK && csrOK, nil
			},
			Run: func(context.Context) error {
				// FR-010: pre-create the pki dir as the host user before any
				// bind-mount, or Docker creates it root-owned and gen-csr fails.
				if err := os.MkdirAll(c.pkiDir(), 0o700); err != nil {
					return err
				}
				_, _, err := pki.GenerateBankCSR(c.BankID, c.Institution, c.pkiDir())
				return err
			},
		},
	}
}

func (c JoinConfig) pkiDir() string { return filepath.Join(c.DataDir, "pki") }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// writeFileAtomic writes via a temp file + rename.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
