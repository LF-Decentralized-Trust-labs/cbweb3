package orchestrator

import (
	"context"
	"crypto/sha256"
	"fmt"
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
		c.WaitKeycloak = func(context.Context) error { return nil }
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(ctx context.Context) (string, error) {
			out, err := c.Runner.Run(ctx, "docker", "exec", c.BankID+"-keycloak", "cat", "/tmp/bank-client-secret")
			return string(out), err
		}
	}
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
			_, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.bankTemplate(tmpl), "--env-file", c.BankEnvFile, "up", "-d")
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
			Name: "write-genesis",
			Deps: []string{"consume-spoke-bundle"},
			// Non-destructive guard: skip when the on-disk genesis already matches
			// the bundle's; a divergent genesis is a hard error (the bank must run
			// exactly the spoke genesis — same chainId, CB-only QBFT validator set).
			Check: func(context.Context) (bool, error) {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return false, err
				}
				existing, err := os.ReadFile(c.genesisPath())
				if os.IsNotExist(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				if sha256.Sum256(existing) == sha256.Sum256([]byte(b.Genesis)) {
					return true, nil
				}
				return false, fmt.Errorf("write-genesis: on-disk genesis at %s diverges from the spoke bundle; "+
					"the bank must run the spoke genesis", c.genesisPath())
			},
			Run: func(context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(c.GenesisDir, 0o755); err != nil {
					return err
				}
				return writeFileAtomic(c.genesisPath(), []byte(b.Genesis), 0o644)
			},
		},
		{
			// Non-validating full node: the node is not in the spoke's QBFT
			// validator set (genesis lists only the CB), so it syncs without
			// producing blocks — no besu "non-validator" flag is needed.
			Name: "start-besu-join",
			Deps: []string{"write-genesis"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.bankTemplate("entity-besu"), "--env-file", c.BankEnvFile, "up", "-d"); err != nil {
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
			Deps: []string{"wait-sync"},
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
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.bankTemplate("entity-keycloak"), "--env-file", c.BankEnvFile, "up", "-d"); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
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
		{Name: "render-bank-env", Deps: []string{"wait-sync", "wire-addresses"}, Run: func(context.Context) error { return nil }},
		{Name: "start-bank-infra", Deps: []string{"render-bank-env"}, Run: compose("entity-infra")},
		{Name: "start-bank-backend", Deps: []string{"start-bank-infra", "render-bank-env"}, Run: compose("entity-backend")},
		{Name: "start-bank-frontend", Deps: []string{"start-bank-backend"}, Run: compose("entity-frontend")},
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
