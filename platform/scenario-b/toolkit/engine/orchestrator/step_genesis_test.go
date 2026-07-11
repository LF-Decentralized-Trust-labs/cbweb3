package orchestrator

import (
	"context"
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

// gen-genesis-hub is idempotent: skipped when genesis.json already exists in the
// `<prefix>_genesis` named volume (probed via the runner, so it is neutralized by
// the DryRunner). The probe emits "YES" when the file exists.
func TestGenGenesisIdempotent(t *testing.T) {
	// Absent: the probe returns no output → Check false.
	absent := genesisStep(t, testHubConfig(t, &exec.FakeRunner{}))
	if ok, _ := absent.Check(context.Background()); ok {
		t.Fatal("Check should be false when the volume has no genesis.json")
	}
	// Present: the probe returns "YES" → Check true (idempotent skip).
	present := genesisStep(t, testHubConfig(t, &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("YES\n")}}))
	if ok, _ := present.Check(context.Background()); !ok {
		t.Fatal("Check should be true when the volume already has genesis.json")
	}
}

// The zero-gas QBFT genesis carries the chainId, zeroBaseFee, and all forks at 0.
// It pre-funds NO dev wallets (scenario-a parity): the network is zero-gas, so
// the deployer/CB/each bank's runtime key transacts without a genesis balance.
func TestQBFTConfigZeroGas(t *testing.T) {
	out, err := qbftConfig(1337, 1)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"\"chainId\": 1337",
		"\"zeroBaseFee\": true",
		"\"shanghaiTime\": 0",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("qbftConfig missing %q:\n%s", want, s)
		}
	}
	// No pre-funded dev wallets — the deployer dev account must NOT be in alloc.
	if strings.Contains(s, devDeployerAddr[2:]) {
		t.Errorf("qbftConfig must not pre-fund dev wallets, found %s:\n%s", devDeployerAddr, s)
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
