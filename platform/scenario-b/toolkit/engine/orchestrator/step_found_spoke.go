package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/relayregistrar"
)

// SpokeConfig parametrizes the found-spoke mode. External gates/reads and the
// relay registrar are injectable so the step set is testable without a live
// Besu/Keycloak/relay.
type SpokeConfig struct {
	Runner         exec.CommandRunner
	ContractsDir   string
	TemplatesDir   string
	OutDir         string
	SpokeID        string
	SpokeChainID   uint64
	SpokeRPC       string
	SpokeWS        string
	CBAddress      string
	GenesisDir     string
	VolumePrefix   string // <p>_genesis, <p>_besu_data (node state in named volumes)
	ValidatorCount int
	BesuImage      string
	HubBundlePath  string
	HubRPC         string // hub RPC (from the bundle unless overridden)
	SpokeEnvFile   string
	KeycloakEnv    []string
	GatewayURL     string
	Registrar      relayregistrar.RelayRegistrar

	// Sovereign-pair tail (TK-B9): non-nil enables the soft open-sovereign-pair
	// / commit-liquidity / seed-oracle steps.
	Pair                *PairConfig
	Environment         string // seed-oracle is local-only; gates pending
	HubPairRegistry     string // from the hub bundle
	HubManualOracle     string // from the hub bundle
	HubIdentityRegistry string // from the hub bundle (AMM/LCR ctor, setCentralBankOf)
	RelayerAddr         string // grant CENTRAL_BANK_ROLE to the relayer on the sovereign tokens
	HubAdminKey         string // scaffolding acts (deploy/grants) — hub admin
	CBHubKey            string // current CB's sovereign act (propose/confirm/commit)

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
	EnodeReader      EnodeReader
	CBRegistered     func(ctx context.Context) (bool, error) // idempotency for register-cb
	PairStatus       func(ctx context.Context, pairID string) (status string, exists bool, err error)
}

// PairConfig carries the sovereign-pair block (spec.pair) plus the local-only
// inputs (rate, commit amounts) sourced from flags.
type PairConfig struct {
	ProposerCB         string
	ConfirmerCB        string
	ProposerCBAddress  string // EVM address of the proposer CB (setCentralBankOf tokenA)
	ConfirmerCBAddress string // EVM address of the confirmer CB (setCentralBankOf tokenB)
	SymbolA            string
	SymbolB            string
	CurrentCB          string // the CB of this spoke (discriminates the role this run)
	Rate               string // seed-oracle rate (local-only; from --pair-rate)
	AmountA            string // CB-A's commit amount (local-only; from --commit-amount-a)
	AmountB            string // CB-B's commit amount (local-only; from --commit-amount-b)
}

func (c *SpokeConfig) WithDefaults() {
	if c.ValidatorCount == 0 {
		c.ValidatorCount = 1
	}
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.GenesisDir == "" {
		c.GenesisDir = filepath.Join(c.OutDir, "genesis-"+c.SpokeID)
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = c.SpokeID + "_central-bank"
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.SpokeRPC, 60*time.Second) }
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(context.Context) error { return nil }
	}
	if c.EnodeReader == nil {
		c.EnodeReader = adminNodeInfoEnode
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(ctx context.Context) (string, error) {
			out, err := c.Runner.Run(ctx, "docker", "exec", c.SpokeID+"-keycloak", "cat", "/tmp/spoke-client-secret")
			return string(out), err
		}
	}
}

func (c SpokeConfig) genesisVolume() string  { return c.VolumePrefix + "_genesis" }
func (c SpokeConfig) besuDataVolume() string { return c.VolumePrefix + "_besu_data" }

func (c SpokeConfig) spokeTemplate(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c SpokeConfig) spokeBroadcastPath() string {
	return filepath.Join(c.ContractsDir, "broadcast", "CBWeb3Spoke.s.sol",
		fmt.Sprintf("%d", c.SpokeChainID), "run-latest.json")
}

// FoundSpokeSteps builds the ordered found-spoke step set.
func FoundSpokeSteps(c SpokeConfig) []Step {
	c.WithDefaults()
	var capturedEnode string // written by start-besu-spoke, read by emit-spoke-bundle

	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.spokeTemplate(tmpl), "--env-file", c.SpokeEnvFile, "up", "-d")
			return err
		}
	}

	steps := []Step{
		{
			Name: "consume-hub-bundle",
			Run: func(context.Context) error {
				_, err := bundle.LoadHub(c.HubBundlePath)
				return err
			},
		},
		{
			Name: "register-cb",
			Deps: []string{"consume-hub-bundle"},
			Check: func(ctx context.Context) (bool, error) {
				if c.CBRegistered != nil {
					return c.CBRegistered(ctx)
				}
				return false, nil
			},
			Run: func(ctx context.Context) error {
				hub, err := bundle.LoadHub(c.HubBundlePath)
				if err != nil {
					return err
				}
				// (a) registerParticipant CENTRAL_BANK — fatal.
				if _, err := c.Runner.Run(ctx, "forge", "script",
					"script/RegisterParticipants.s.sol:RegisterParticipants",
					"--root", c.ContractsDir, "--rpc-url", c.HubRPC, "--broadcast"); err != nil {
					return err
				}
				// (b) grantLiquidityProvider — attempted automatically; non-fatal if no permission.
				if _, err := c.Runner.Run(ctx, "cast", "send", hub.Contracts["identityRegistry"],
					"grantLiquidityProvider(address)", c.CBAddress, "--rpc-url", c.HubRPC); err != nil {
					fmt.Printf("[found-spoke] grantLiquidityProvider pending (needs hub admin/governance): %v\n", err)
				}
				return nil
			},
		},
		{
			Name:  "build-contracts",
			Check: func(context.Context) (bool, error) { return dirNonEmpty(filepath.Join(c.ContractsDir, "out")), nil },
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "forge", "build", "--root", c.ContractsDir)
				return err
			},
		},
		genGenesisStep("gen-genesis-spoke", c.SpokeChainID, c.genesisVolume(), c.besuDataVolume(), c.ValidatorCount, c.Runner, c.BesuImage),
		{
			Name: "start-besu-spoke",
			Deps: []string{"gen-genesis-spoke"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.spokeTemplate("entity-besu"), "--env-file", c.SpokeEnvFile, "up", "-d"); err != nil {
					return err
				}
				if err := c.WaitRPC(ctx); err != nil {
					return err
				}
				enode, err := c.EnodeReader(ctx, c.SpokeRPC)
				if err != nil {
					return err
				}
				capturedEnode = enode
				return nil
			},
		},
		{
			Name: "deploy-spoke-contracts",
			Deps: []string{"build-contracts", "start-besu-spoke"},
			Check: func(context.Context) (bool, error) {
				_, err := addrs.ParseBroadcastList(c.spokeBroadcastPath())
				return err == nil, nil
			},
			Run: func(ctx context.Context) error {
				if err := c.WaitRPC(ctx); err != nil {
					return err
				}
				_, err := c.Runner.Run(ctx, "forge", "script",
					"script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke",
					"--root", c.ContractsDir, "--rpc-url", c.SpokeRPC, "--broadcast")
				return err
			},
		},
		{
			Name: "wire-hub-addresses",
			Deps: []string{"consume-hub-bundle"},
			Run: func(context.Context) error {
				hub, err := bundle.LoadHub(c.HubBundlePath)
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"HUB_IDENTITY_REGISTRY_ADDRESS":  hub.Contracts["identityRegistry"],
					"HUB_TOKEN_A_ADDRESS":            hub.Contracts["tCeBM_BRL"],
					"HUB_TOKEN_B_ADDRESS":            hub.Contracts["tCeBM_EUR"],
					"FX_AGREEMENT_CONTRACT_ADDRESS":  hub.Contracts["fxAgreement"],
					"PAIR_REGISTRY_CONTRACT_ADDRESS": hub.Contracts["pairRegistry"],
				} {
					if err := addrs.AppendAddr(c.SpokeEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "provision-keycloak-spoke",
			Deps: []string{"deploy-spoke-contracts"},
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
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-f", c.spokeTemplate("entity-keycloak"), "--env-file", c.SpokeEnvFile, "up", "-d"); err != nil {
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
		{
			Name: "render-spoke-env",
			Deps: []string{"deploy-spoke-contracts"},
			Run: func(context.Context) error {
				m, err := spokeContractMap(c.spokeBroadcastPath())
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"SPOKE_IDENTITY_REGISTRY_ADDRESS": m["identityRegistry"],
					"SPOKE_TCEBM_ADDRESS":             m["tCeBM"],
					"SPOKE_BRIDGE_ADDRESS":            m["spokeBridge"],
					"SPOKE_FCEBM_ADDRESS":             m["fCeBM"],
				} {
					if err := addrs.AppendAddr(c.SpokeEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{Name: "start-spoke-infra", Deps: []string{"render-spoke-env"}, Run: compose("entity-infra")},
		{Name: "start-spoke-backend", Deps: []string{"start-spoke-infra", "render-spoke-env"}, Run: compose("entity-backend")},
		{Name: "start-spoke-frontend", Deps: []string{"start-spoke-backend"}, Run: compose("entity-frontend")},
		{
			Name: "register-relay-spoke",
			Deps: []string{"start-besu-spoke"},
			Run: func(ctx context.Context) error {
				if c.Registrar == nil {
					return fmt.Errorf("register-relay-spoke: no RelayRegistrar configured")
				}
				return c.Registrar.Register(ctx, relayregistrar.Spoke{
					ID: c.SpokeID, BesuRPC: c.SpokeRPC, BesuWS: c.SpokeWS, GatewayURL: c.GatewayURL,
				})
			},
		},
		{
			Name: "add-noc-agent",
			Deps: []string{"start-besu-spoke"},
			Soft: true, // observability — non-blocking
			Run:  compose("noc"),
		},
	}

	// TK-B9: sovereign-pair tail (soft) — only when spec.pair is present. Ordered
	// after add-noc-agent and before emit-spoke-bundle (insertion order preserves
	// this among independents in topoSort).
	emitDeps := []string{"deploy-spoke-contracts", "start-besu-spoke"}
	if c.Pair != nil {
		steps = append(steps, sovereignPairSteps(c)...)
		emitDeps = append(emitDeps, "open-sovereign-pair", "commit-liquidity", "seed-oracle")
	}

	steps = append(steps, Step{
		Name: "emit-spoke-bundle",
		Deps: emitDeps,
		Check: func(context.Context) (bool, error) {
			_, err := os.Stat(filepath.Join(c.OutDir, "bundles", "spoke-"+c.SpokeID+".bundle.yaml"))
			return err == nil, nil
		},
		Run: func(ctx context.Context) error {
			m, err := spokeContractMap(c.spokeBroadcastPath())
			if err != nil {
				return err
			}
			genesisBytes, err := readVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json")
			if err != nil {
				return err
			}
			b := bundle.SpokeBundle{
				SpokeID: c.SpokeID, ChainID: c.SpokeChainID, Enode: capturedEnode,
				SpokeRPC: c.SpokeRPC, SpokeWS: c.SpokeWS, Genesis: string(genesisBytes), Contracts: m,
			}
			_, err = bundle.EmitSpoke(b, c.OutDir)
			return err
		},
	})
	return steps
}

// spokeContractMap maps the CBWeb3Spoke broadcast deployments to bundle keys.
func spokeContractMap(broadcastPath string) (map[string]string, error) {
	list, err := addrs.ParseBroadcastList(broadcastPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, d := range list {
		switch d.Name {
		case "IdentityRegistry":
			out["identityRegistry"] = d.Address
		case "TokenizedCentralBankMoney":
			out["tCeBM"] = d.Address
		case "SpokeBridge":
			out["spokeBridge"] = d.Address
		case "FiatCentralBankMoney":
			out["fCeBM"] = d.Address
		}
	}
	return out, nil
}
