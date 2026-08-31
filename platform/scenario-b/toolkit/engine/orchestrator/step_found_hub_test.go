// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// writeBroadcast seeds a CBWeb3Hub broadcast run-latest.json under contractsDir.
func writeBroadcast(t *testing.T, contractsDir string, chainID uint64) {
	t.Helper()
	dir := filepath.Join(contractsDir, "broadcast", "CBWeb3Hub.s.sol", "1337")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	json := `{"transactions":[
	 {"transactionType":"CREATE","contractName":"IdentityRegistry","contractAddress":"0xa1"},
	 {"transactionType":"CREATE","contractName":"FXAgreement","contractAddress":"0xc1"},
	 {"transactionType":"CREATE","contractName":"PairRegistry","contractAddress":"0xd1"},
	 {"transactionType":"CREATE","contractName":"CurrencyRegistry","contractAddress":"0xe1"},
	 {"transactionType":"CREATE","contractName":"ManualOracle","contractAddress":"0xf1"},
	 {"transactionType":"CREATE","contractName":"LiquidityCommitRegistry","contractAddress":"0xf2"}
	]}`
	if err := os.WriteFile(filepath.Join(dir, "run-latest.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testHubConfig(t *testing.T, fake *exec.FakeRunner) HubConfig {
	t.Helper()
	dir := t.TempDir()
	contracts := filepath.Join(dir, "contracts")
	_ = os.MkdirAll(filepath.Join(contracts, "out"), 0o755) // build already satisfied
	_ = os.WriteFile(filepath.Join(contracts, "out", "x.json"), []byte("{}"), 0o644)
	envFile := filepath.Join(dir, ".env.hub")
	return HubConfig{
		Runner:           fake,
		ContractsDir:     contracts,
		TemplatesDir:     filepath.Join(dir, "templates"),
		OutDir:           dir,
		ChainID:          1337,
		HubRPC:           "http://hub:8545",
		HubWS:            "ws://hub:8546",
		HubEnvFile:       envFile,
		KeycloakEnv:      []string{envFile},
		WaitRPC:          func(context.Context) error { return nil },
		WaitKeycloak:     func(context.Context) error { return nil },
		ReadClientSecret: func(context.Context) (string, error) { return "s3cr3t", nil },
	}
}

// The found-hub step set is ordered with contracts before render/relay/noc/bundle.
func TestFoundHubStepOrder(t *testing.T) {
	steps := FoundHubSteps(testHubConfig(t, &exec.FakeRunner{}))
	ordered, err := topoSort(steps)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, s := range ordered {
		pos[s.Name] = i
	}
	must := func(a, b string) {
		if pos[a] >= pos[b] {
			t.Fatalf("%s must come before %s (pos %d vs %d)", a, b, pos[a], pos[b])
		}
	}
	must("build-contracts", "deploy-hub-contracts")
	must("render-hub-compose-env", "start-besu-hub")
	must("start-besu-hub", "deploy-hub-contracts")
	must("deploy-hub-contracts", "emit-hub-bundle")
	// Infra creates the network/DB that keycloak + backend join → before them.
	must("start-hub-infra", "provision-keycloak-hub")
	must("start-hub-infra", "start-hub-backend")
	must("provision-keycloak-hub", "start-hub-backend")
	must("start-besu-hub", "add-noc-agent")
}

// deploy-hub-contracts invokes the single CBWeb3Hub forge script.
func TestDeployInvokesCBWeb3HubScript(t *testing.T) {
	fake := &exec.FakeRunner{}
	steps := FoundHubSteps(testHubConfig(t, fake))
	var deploy Step
	for _, s := range steps {
		if s.Name == "deploy-hub-contracts" {
			deploy = s
		}
	}
	if err := deploy.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range fake.Calls {
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, "CBWeb3Hub.s.sol:DeployCBWeb3Hub") && strings.Contains(joined, "--legacy") {
			found = true
		}
	}
	if !found {
		t.Fatalf("deploy did not invoke the CBWeb3Hub forge script with --legacy; calls=%+v", fake.Calls)
	}
}

// Keycloak write-back is idempotent: Run writes the secret; re-run Check skips.
func TestKeycloakWriteBackIdempotent(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	var kc Step
	for _, s := range FoundHubSteps(cfg) {
		if s.Name == "provision-keycloak-hub" {
			kc = s
		}
	}
	// Before: Check false (secret not written yet).
	ok, _ := kc.Check(context.Background())
	if ok {
		t.Fatal("Check should be false before write-back")
	}
	if err := kc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !addrs.HasAddr(cfg.HubEnvFile, "KEYCLOAK_CLIENT_SECRET", "s3cr3t") {
		t.Fatal("secret not written back")
	}
	// After: Check true (idempotent skip on re-run).
	ok, _ = kc.Check(context.Background())
	if !ok {
		t.Fatal("Check should be true after write-back (idempotent)")
	}
}

// emit-hub-bundle produces a valid bundle with the two tCeBM distinguished.
// When AdvertisedHost is set, public RPC/WS/gateway URLs use that host (not localhost).
func TestEmitHubBundleFromBroadcast(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.AdvertisedHost = "10.10.0.20"
	cfg.RPCPort = 8845
	cfg.WSPort = 8846
	writeBroadcast(t, cfg.ContractsDir, cfg.ChainID)
	var emit Step
	for _, s := range FoundHubSteps(cfg) {
		if s.Name == "emit-hub-bundle" {
			emit = s
		}
	}
	if err := emit.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.LoadHub(filepath.Join(cfg.OutDir, "bundles", "hub.bundle.yaml"))
	if err != nil {
		t.Fatalf("load emitted bundle: %v", err)
	}
	if b.Contracts["identityRegistry"] != "0xa1" || b.Contracts["manualOracle"] != "0xf1" {
		t.Fatalf("contract mapping wrong: %+v", b.Contracts)
	}
	// The hub deploys no tCeBM: mirrored tokens are created per CB currency
	// registration and resolved from the PairRegistry, so no tCeBM key of ANY
	// currency may reach the bundle (a per-currency key would bake a fixed
	// bilateral pair into a multi-currency hub).
	for k := range b.Contracts {
		if strings.HasPrefix(k, "tCeBM") {
			t.Fatalf("no tCeBM key may be in the hub bundle, got %q: %+v", k, b.Contracts)
		}
	}
	if b.Contracts["liquidityCommitRegistry"] != "0xf2" {
		t.Fatalf("liquidityCommitRegistry mapping wrong: %+v", b.Contracts)
	}
	if b.HubRPC != "http://10.10.0.20:8845" {
		t.Fatalf("HubRPC want http://10.10.0.20:8845, got %q", b.HubRPC)
	}
	if b.HubWS != "ws://10.10.0.20:8846" {
		t.Fatalf("HubWS want ws://10.10.0.20:8846, got %q", b.HubWS)
	}
	if b.HubGateway != "http://10.10.0.20:16845" {
		t.Fatalf("HubGateway want http://10.10.0.20:16845, got %q", b.HubGateway)
	}
}

// emit-hub-bundle Check re-runs when an on-disk bundle still has localhost but
// AdvertisedHost is set (stale LNET artifact from an older toolkit).
func TestEmitHubBundleCheckRewritesLocalhost(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.RPCPort = 8845
	cfg.WSPort = 8846
	writeBroadcast(t, cfg.ContractsDir, cfg.ChainID)

	// Emit a localhost bundle (no AdvertisedHost), then set AdvertisedHost and
	// expect Check to force a re-emit.
	var emit Step
	for _, s := range FoundHubSteps(cfg) {
		if s.Name == "emit-hub-bundle" {
			emit = s
		}
	}
	if err := emit.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg.AdvertisedHost = "18.216.221.177"
	for _, s := range FoundHubSteps(cfg) {
		if s.Name == "emit-hub-bundle" {
			emit = s
		}
	}
	ok, err := emit.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Check should be false for stale localhost bundle when AdvertisedHost is set")
	}
}
