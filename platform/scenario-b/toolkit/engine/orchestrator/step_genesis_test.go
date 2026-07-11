package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

func genesisStep(t *testing.T, cfg HubConfig) Step {
	t.Helper()
	for _, s := range FoundHubSteps(cfg) {
		if s.Name == "gen-genesis-hub" {
			return s
		}
	}
	t.Fatal("gen-genesis-hub step not found")
	return Step{}
}

// gen-genesis-hub is idempotent: skipped when genesis.json already exists.
func TestGenGenesisIdempotent(t *testing.T) {
	cfg := testHubConfig(t, &exec.FakeRunner{})
	cfg.GenesisDir = filepath.Join(t.TempDir(), "genesis")
	step := genesisStep(t, cfg)

	ok, _ := step.Check(context.Background())
	if ok {
		t.Fatal("Check should be false before genesis exists")
	}
	// Seed genesis.json → Check true (idempotent skip).
	_ = os.MkdirAll(cfg.GenesisDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfg.GenesisDir, "genesis.json"), []byte("{}"), 0o644)
	ok, _ = step.Check(context.Background())
	if !ok {
		t.Fatal("Check should be true after genesis.json exists (idempotent)")
	}
}

// Run writes the QBFT config, invokes besu operator generate-blockchain-config,
// and seeds genesis.json into GenesisDir.
func TestGenGenesisRunInvokesBesu(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.GenesisDir = filepath.Join(t.TempDir(), "genesis")
	cfg.ChainID = 1337
	step := genesisStep(t, cfg)

	// Simulate besu output: pre-create the networkFiles/genesis.json the runner
	// would produce (FakeRunner does not execute).
	work := filepath.Join(cfg.GenesisDir, ".work", "networkFiles")
	_ = os.MkdirAll(work, 0o755)
	_ = os.WriteFile(filepath.Join(work, "genesis.json"), []byte(`{"config":{"chainId":1337}}`), 0o644)

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("gen-genesis Run: %v", err)
	}

	// besu operator generate-blockchain-config was invoked.
	found := false
	for _, c := range fake.Calls {
		if c.Name == "docker" && strings.Contains(strings.Join(c.Args, " "), "operator generate-blockchain-config") {
			found = true
		}
	}
	if !found {
		t.Fatalf("besu genesis generation not invoked; calls=%+v", fake.Calls)
	}
	// QBFT config carries the chainId.
	cfgBytes, err := os.ReadFile(filepath.Join(cfg.GenesisDir, ".work", "qbftConfig.json"))
	if err != nil || !strings.Contains(string(cfgBytes), "\"chainId\": 1337") {
		t.Fatalf("qbftConfig missing chainId: %v\n%s", err, cfgBytes)
	}
	// genesis.json seeded into GenesisDir.
	if _, err := os.Stat(filepath.Join(cfg.GenesisDir, "genesis.json")); err != nil {
		t.Fatalf("genesis.json not seeded: %v", err)
	}
}

// start-besu-hub now depends on gen-genesis-hub (genesis before node).
func TestGenesisBeforeBesu(t *testing.T) {
	ordered, err := topoSort(FoundHubSteps(testHubConfig(t, &exec.FakeRunner{})))
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, s := range ordered {
		pos[s.Name] = i
	}
	if pos["gen-genesis-hub"] >= pos["start-besu-hub"] {
		t.Fatalf("gen-genesis-hub must come before start-besu-hub (%d vs %d)", pos["gen-genesis-hub"], pos["start-besu-hub"])
	}
}
