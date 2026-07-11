package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/relayregistrar"
)

func findStep(steps []Step, name string) Step {
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	return Step{}
}

func writeSpokeBroadcast(t *testing.T, contractsDir string) {
	t.Helper()
	dir := filepath.Join(contractsDir, "broadcast", "CBWeb3Spoke.s.sol", "1338")
	_ = os.MkdirAll(dir, 0o755)
	j := `{"transactions":[
	 {"transactionType":"CREATE","contractName":"IdentityRegistry","contractAddress":"0xs1"},
	 {"transactionType":"CREATE","contractName":"TokenizedCentralBankMoney","contractAddress":"0xs2"},
	 {"transactionType":"CREATE","contractName":"SpokeBridge","contractAddress":"0xs3"},
	 {"transactionType":"CREATE","contractName":"FiatCentralBankMoney","contractAddress":"0xs4"}
	]}`
	_ = os.WriteFile(filepath.Join(dir, "run-latest.json"), []byte(j), 0o644)
}

func testSpokeCfg(t *testing.T, fake *exec.FakeRunner) SpokeConfig {
	t.Helper()
	dir := t.TempDir()
	hubOut := filepath.Join(dir, "hubout")
	hb := bundle.HubBundle{
		ChainID: 1337, HubRPC: "http://hub:8545", HubWS: "ws://hub:8546",
		Contracts: map[string]string{
			"identityRegistry": "0xh1", "tCeBM_BRL": "0xh2", "tCeBM_EUR": "0xh3",
			"fxAgreement": "0xh4", "pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
		},
	}
	hp, err := bundle.EmitHub(hb, hubOut)
	if err != nil {
		t.Fatal(err)
	}
	contracts := filepath.Join(dir, "contracts")
	_ = os.MkdirAll(filepath.Join(contracts, "out"), 0o755)
	_ = os.WriteFile(filepath.Join(contracts, "out", "x.json"), []byte("{}"), 0o644)
	env := filepath.Join(dir, ".env.spoke")
	reg, _ := relayregistrar.New("local")
	return SpokeConfig{
		Runner: fake, ContractsDir: contracts, TemplatesDir: filepath.Join(dir, "tmpl"), OutDir: dir,
		SpokeID: "spoke-a", SpokeChainID: 1338, SpokeRPC: "http://spoke:8545", SpokeWS: "ws://spoke:8546",
		CBAddress: "0xCB", GenesisDir: filepath.Join(dir, "genesis"), HubBundlePath: hp, HubRPC: "http://hub:8545",
		SpokeEnvFile: env, KeycloakEnv: []string{env}, GatewayURL: "http://gw", Registrar: reg,
		WaitRPC:          func(context.Context) error { return nil },
		WaitKeycloak:     func(context.Context) error { return nil },
		ReadClientSecret: func(context.Context) (string, error) { return "s3cr3t", nil },
		EnodeReader:      func(context.Context, string) (string, error) { return "enode://cb@spoke:30303", nil },
	}
}

// US1: register-cb invokes RegisterParticipants + grantLiquidityProvider; a failing grant is non-fatal.
func TestRegisterCBActionsAndGrantBestEffort(t *testing.T) {
	fake := &exec.FakeRunner{Errs: map[string]error{"cast": context.DeadlineExceeded}} // grant fails
	step := findStep(FoundSpokeSteps(testSpokeCfg(t, fake)), "register-cb")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("register-cb must not fail when only the grant fails: %v", err)
	}
	var forgeOK, castOK bool
	for _, c := range fake.Calls {
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, "RegisterParticipants.s.sol:RegisterParticipants") {
			forgeOK = true
		}
		if c.Name == "cast" && strings.Contains(joined, "grantLiquidityProvider") {
			castOK = true
		}
	}
	if !forgeOK || !castOK {
		t.Fatalf("register-cb must invoke RegisterParticipants and grantLiquidityProvider; calls=%+v", fake.Calls)
	}
}

func TestConsumeHubBundleFailsOnInvalid(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.HubBundlePath = filepath.Join(t.TempDir(), "missing.yaml")
	step := findStep(FoundSpokeSteps(cfg), "consume-hub-bundle")
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("consume-hub-bundle should fail on missing/invalid bundle")
	}
}

// US2: deploy-spoke-contracts invokes CBWeb3Spoke; start-besu-spoke captures the enode.
func TestDeploySpokeInvokesScript(t *testing.T) {
	fake := &exec.FakeRunner{}
	step := findStep(FoundSpokeSteps(testSpokeCfg(t, fake)), "deploy-spoke-contracts")
	_ = step.Run(context.Background())
	found := false
	for _, c := range fake.Calls {
		if c.Name == "forge" && strings.Contains(strings.Join(c.Args, " "), "CBWeb3Spoke.s.sol:DeployCBWeb3Spoke") {
			found = true
		}
	}
	if !found {
		t.Fatalf("deploy-spoke-contracts must invoke CBWeb3Spoke; calls=%+v", fake.Calls)
	}
}

// US3: wire-hub-addresses writes hub addresses idempotently.
func TestWireHubAddresses(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	step := findStep(FoundSpokeSteps(cfg), "wire-hub-addresses")
	if err := step.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := step.Run(context.Background()); err != nil { // idempotent
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg.SpokeEnvFile)
	if !strings.Contains(string(b), "HUB_IDENTITY_REGISTRY_ADDRESS=0xh1") {
		t.Fatalf("hub address not wired:\n%s", b)
	}
}

// US4: register-relay-spoke registers via the local registrar; add-noc-agent is Soft.
func TestRegisterRelaySpokeAndSoftNoc(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	steps := FoundSpokeSteps(cfg)
	if err := findStep(steps, "register-relay-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	lr := cfg.Registrar.(relayregistrar.LocalRegistry)
	if len(lr.List()) != 1 || lr.List()[0].ID != "spoke-a" {
		t.Fatalf("relay registry = %+v", lr.List())
	}
	if !findStep(steps, "add-noc-agent").Soft {
		t.Fatal("add-noc-agent must be Soft")
	}
}

// US5: emit-spoke-bundle produces a valid spoke bundle (genesis + enode + contracts).
func TestEmitSpokeBundle(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testSpokeCfg(t, fake)
	writeSpokeBroadcast(t, cfg.ContractsDir)
	_ = os.MkdirAll(cfg.GenesisDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfg.GenesisDir, "genesis.json"), []byte(`{"config":{"chainId":1338}}`), 0o644)

	steps := FoundSpokeSteps(cfg)
	// start-besu-spoke captures the enode (shared closure) before emit.
	if err := findStep(steps, "start-besu-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := findStep(steps, "emit-spoke-bundle").Run(context.Background()); err != nil {
		t.Fatalf("emit-spoke-bundle: %v", err)
	}
	b, err := bundle.LoadSpoke(filepath.Join(cfg.OutDir, "bundles", "spoke-spoke-a.bundle.yaml"))
	if err != nil {
		t.Fatalf("load spoke bundle: %v", err)
	}
	if b.Enode != "enode://cb@spoke:30303" || b.ChainID != 1338 || b.Contracts["spokeBridge"] != "0xs3" {
		t.Fatalf("spoke bundle mismatch: %+v", b)
	}
}

// Integration: FoundSpokeSteps assembles in dependency order.
func TestFoundSpokeOrder(t *testing.T) {
	ordered, err := topoSort(FoundSpokeSteps(testSpokeCfg(t, &exec.FakeRunner{})))
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, s := range ordered {
		pos[s.Name] = i
	}
	must := func(a, b string) {
		if pos[a] >= pos[b] {
			t.Fatalf("%s must precede %s", a, b)
		}
	}
	must("consume-hub-bundle", "register-cb")
	must("gen-genesis-spoke", "start-besu-spoke")
	must("start-besu-spoke", "deploy-spoke-contracts")
	must("deploy-spoke-contracts", "emit-spoke-bundle")
	must("start-besu-spoke", "register-relay-spoke")
}
