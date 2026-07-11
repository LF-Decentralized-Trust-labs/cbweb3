package orchestrator

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

func testPairCfg(t *testing.T, fake *exec.FakeRunner, currentCB string) SpokeConfig {
	t.Helper()
	c := testSpokeCfg(t, fake)
	c.Environment = "local"
	c.HubPairRegistry = "0xPR"
	c.HubManualOracle = "0xORC"
	c.HubIdentityRegistry = "0xIR"
	c.RelayerAddr = "0xREL"
	c.HubAdminKey = "0xADMIN"
	c.CBHubKey = "0xCBKEY"
	c.Pair = &PairConfig{
		ProposerCB: "central-bank-a", ConfirmerCB: "central-bank-b",
		SymbolA: "BRL", SymbolB: "ARS", CurrentCB: currentCB,
		Rate: "1000000", AmountA: "100", AmountB: "200",
	}
	return c
}

func callsJoined(fake *exec.FakeRunner) string {
	var b strings.Builder
	for _, c := range fake.Calls {
		b.WriteString(c.Name + " " + strings.Join(c.Args, " ") + "\n")
	}
	return b.String()
}

// T005: FoundSpokeSteps appends the soft sovereign tail only when Pair != nil.
func TestFoundSpokeAppendsSovereignTail(t *testing.T) {
	with := testPairCfg(t, &exec.FakeRunner{}, "central-bank-a")
	names := map[string]bool{}
	for _, s := range FoundSpokeSteps(with) {
		names[s.Name] = true
	}
	for _, want := range []string{"open-sovereign-pair", "commit-liquidity", "seed-oracle"} {
		if !names[want] {
			t.Errorf("with spec.pair: missing %q", want)
		}
	}
	// All three must be Soft.
	for _, s := range FoundSpokeSteps(with) {
		if (s.Name == "open-sovereign-pair" || s.Name == "commit-liquidity" || s.Name == "seed-oracle") && !s.Soft {
			t.Errorf("%q must be Soft", s.Name)
		}
	}
	// No cycle.
	if _, err := topoSort(FoundSpokeSteps(with)); err != nil {
		t.Fatalf("topoSort with tail: %v", err)
	}

	without := testSpokeCfg(t, &exec.FakeRunner{}) // Pair == nil
	for _, s := range FoundSpokeSteps(without) {
		if s.Name == "open-sovereign-pair" || s.Name == "commit-liquidity" || s.Name == "seed-oracle" {
			t.Errorf("without spec.pair: %q must not be appended", s.Name)
		}
	}
}

// T007: proponent scaffolds (W-tokens dedup / AMM / LCR + grants) + proposePair; never confirmPair.
func TestOpenPairProposer(t *testing.T) {
	fake := &exec.FakeRunner{Outputs: map[string][]byte{
		"forge": []byte("Deployed to: 0x1111111111111111111111111111111111111111"),
	}}
	cfg := testPairCfg(t, fake, "central-bank-a")
	cfg.CBAddress = "0xCBA"
	cfg.Pair.ConfirmerCBAddress = "0xCBB"
	cfg.PairStatus = func(context.Context, string) (string, bool, error) { return "", false, nil } // not proposed yet
	if err := findStep(sovereignPairSteps(cfg), "open-sovereign-pair").Run(context.Background()); err != nil {
		t.Fatalf("proposer run: %v", err)
	}
	j := callsJoined(fake)
	if !strings.Contains(j, "forge create") {
		t.Errorf("proposer must scaffold via forge create; calls:\n%s", j)
	}
	// FR-003: BOTH W-tokens mapped to their CB (tokenA→proposer, tokenB→confirmer).
	if strings.Count(j, "setCentralBankOf(address,address)") < 2 {
		t.Errorf("both W-tokens must be mapped via setCentralBankOf; calls:\n%s", j)
	}
	if !strings.Contains(j, "0xCBA") || !strings.Contains(j, "0xCBB") {
		t.Errorf("setCentralBankOf must map tokenA→0xCBA and tokenB→0xCBB; calls:\n%s", j)
	}
	if !strings.Contains(j, "proposePair(string,address,address,address)") {
		t.Errorf("proposer must call proposePair; calls:\n%s", j)
	}
	if strings.Contains(j, "confirmPair") {
		t.Errorf("proposer must NOT confirm (no counterparty key); calls:\n%s", j)
	}
}

// T007 (dedup): a currency whose W-token already exists is not redeployed.
func TestOpenPairDedupWToken(t *testing.T) {
	fake := &exec.FakeRunner{Outputs: map[string][]byte{
		"forge": []byte("Deployed to: 0x2222222222222222222222222222222222222222"),
	}}
	cfg := testPairCfg(t, fake, "central-bank-a")
	cfg.PairStatus = func(context.Context, string) (string, bool, error) { return "", false, nil }
	// Pre-seed the BRL W-token so it is reused, not redeployed.
	_ = os.WriteFile(cfg.SpokeEnvFile, []byte("WTOKEN_BRL_ADDRESS=0xexisting\n"), 0o644)
	if err := findStep(sovereignPairSteps(cfg), "open-sovereign-pair").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	j := callsJoined(fake)
	if strings.Contains(j, "--constructor-args W-BRL W-BRL") {
		t.Errorf("BRL W-token must be reused (dedup), not redeployed; calls:\n%s", j)
	}
	if !strings.Contains(j, "--constructor-args W-ARS W-ARS") {
		t.Errorf("ARS W-token must be deployed; calls:\n%s", j)
	}
}

// T008: confirmer confirms only when PROPOSED; ACTIVE skips; not-yet-proposed is pending (soft error).
func TestOpenPairConfirmer(t *testing.T) {
	// PROPOSED → confirmPair.
	fake := &exec.FakeRunner{}
	cfg := testPairCfg(t, fake, "central-bank-b")
	cfg.PairStatus = func(context.Context, string) (string, bool, error) { return "PROPOSED", true, nil }
	if err := findStep(sovereignPairSteps(cfg), "open-sovereign-pair").Run(context.Background()); err != nil {
		t.Fatalf("confirmer run: %v", err)
	}
	if !strings.Contains(callsJoined(fake), "confirmPair(string)") {
		t.Errorf("confirmer must call confirmPair; calls:\n%s", callsJoined(fake))
	}

	// ACTIVE → Check skips.
	cfgActive := testPairCfg(t, &exec.FakeRunner{}, "central-bank-b")
	cfgActive.PairStatus = func(context.Context, string) (string, bool, error) { return "ACTIVE", true, nil }
	if skip, _ := findStep(sovereignPairSteps(cfgActive), "open-sovereign-pair").Check(context.Background()); !skip {
		t.Error("ACTIVE pair must skip")
	}

	// Not yet proposed → pending (soft error, non-fatal via engine).
	cfgPending := testPairCfg(t, &exec.FakeRunner{}, "central-bank-b")
	cfgPending.PairStatus = func(context.Context, string) (string, bool, error) { return "", false, nil }
	if err := findStep(sovereignPairSteps(cfgPending), "open-sovereign-pair").Run(context.Background()); err == nil {
		t.Error("confirmer with no proposal yet must return a pending error")
	}
}

// T010: commit-liquidity registers only the current CB's side; idempotent Check.
func TestCommitLiquidity(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testPairCfg(t, fake, "central-bank-a") // proposer → side 0 (BRL)
	_ = os.WriteFile(cfg.SpokeEnvFile, []byte("WTOKEN_BRL_ADDRESS=0xtokA\nLIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR\n"), 0o644)
	if err := findStep(sovereignPairSteps(cfg), "commit-liquidity").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	j := callsJoined(fake)
	if !strings.Contains(j, "registerCommit(string,uint8,uint256,address) W-BRL-ARS 0 100 0xtokA") {
		t.Errorf("commit must register CB-A's own side (BRL, side 0, amount 100); calls:\n%s", j)
	}

	// Idempotent: a non-zero pending commit → skip.
	fake2 := &exec.FakeRunner{Outputs: map[string][]byte{"cast": []byte("0x00000000000000000000000000000000000000000000000000000000000000ab")}}
	cfg2 := testPairCfg(t, fake2, "central-bank-a")
	_ = os.WriteFile(cfg2.SpokeEnvFile, []byte("LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR\n"), 0o644)
	if skip, _ := findStep(sovereignPairSteps(cfg2), "commit-liquidity").Check(context.Background()); !skip {
		t.Error("existing pending commit must skip")
	}
}

// T012: seed-oracle sets the rate in local; skips outside local.
func TestSeedOracle(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testPairCfg(t, fake, "central-bank-a")
	_ = os.WriteFile(cfg.SpokeEnvFile, []byte("WTOKEN_BRL_ADDRESS=0xA\nWTOKEN_ARS_ADDRESS=0xB\n"), 0o644)
	step := findStep(sovereignPairSteps(cfg), "seed-oracle")
	if skip, _ := step.Check(context.Background()); skip {
		t.Error("local must not skip seed-oracle")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(callsJoined(fake), "setRate(address,address,uint256) 0xA 0xB 1000000") {
		t.Errorf("seed-oracle must setRate; calls:\n%s", callsJoined(fake))
	}

	// Non-local → skip.
	cfgProd := testPairCfg(t, &exec.FakeRunner{}, "central-bank-a")
	cfgProd.Environment = "staging"
	if skip, _ := findStep(sovereignPairSteps(cfgProd), "seed-oracle").Check(context.Background()); !skip {
		t.Error("non-local must skip seed-oracle")
	}
}
