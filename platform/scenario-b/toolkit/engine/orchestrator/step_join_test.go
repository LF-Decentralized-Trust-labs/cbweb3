package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

const testGenesis = `{"config":{"chainId":1338,"qbft":{}},"alloc":{}}`

func testJoinCfg(t *testing.T, fake *exec.FakeRunner) JoinConfig {
	t.Helper()
	dir := t.TempDir()
	sb := bundle.SpokeBundle{
		SpokeID: "spoke-a", ChainID: 1338, Enode: "enode://cb@spoke:31303",
		SpokeRPC: "http://spoke:8645", SpokeWS: "ws://spoke:8655", Genesis: testGenesis,
		Contracts: map[string]string{
			"identityRegistry": "0xs1", "tCeBM": "0xs2", "spokeBridge": "0xs3", "fCeBM": "0xs4",
		},
	}
	bp, err := bundle.EmitSpoke(sb, filepath.Join(dir, "bundleout"))
	if err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(dir, ".env.bank")
	return JoinConfig{
		Runner: fake, TemplatesDir: filepath.Join(dir, "tmpl"), OutDir: dir,
		BankID: "bank-a", Institution: "Bank A", SpokeID: "spoke-a", SpokeChainID: 1338,
		BankRPC: "http://bank:8646", SpokeBundlePath: bp, GenesisDir: filepath.Join(dir, "genesis"),
		DataDir: dir, BankEnvFile: env, KeycloakEnv: []string{env},
		WaitRPC:          func(context.Context) error { return nil },
		WaitSync:         func(context.Context) error { return nil },
		WaitKeycloak:     func(context.Context) error { return nil },
		ReadClientSecret: func(context.Context) (string, error) { return "s3cr3t", nil },
	}
}

// US1: consume-spoke-bundle fails on an invalid bundle.
func TestConsumeSpokeBundleFailsOnInvalid(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	cfg.SpokeBundlePath = filepath.Join(t.TempDir(), "missing.yaml")
	if err := findStep(JoinSteps(cfg), "consume-spoke-bundle").Run(context.Background()); err == nil {
		t.Fatal("consume-spoke-bundle should fail on missing/invalid bundle")
	}
}

// US1/SC-002: write-genesis writes the bundle genesis; re-run with matching
// content skips (Check true); a divergent on-disk genesis is a hard error.
func TestWriteGenesisGuard(t *testing.T) {
	// Node state lives in a named volume: Run stages the bundle genesis to the host
	// scratch then copies it into the volume; Check reads it back via the runner
	// (docker cat → fake Outputs["docker"]).
	fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(testGenesis)}}
	cfg := testJoinCfg(t, fake)
	step := findStep(JoinSteps(cfg), "write-genesis")

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("write-genesis run: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(cfg.GenesisDir, "genesis.json"))
	if string(got) != testGenesis {
		t.Fatalf("genesis not staged from bundle:\n%s", got)
	}
	// Matching content in the volume → Check skips.
	skip, err := step.Check(context.Background())
	if err != nil || !skip {
		t.Fatalf("matching genesis should skip: skip=%v err=%v", skip, err)
	}
	// Divergent volume content → Check errors (non-destructive guard).
	fake.Outputs["docker"] = []byte(`{"config":{"chainId":9999}}`)
	if _, err := step.Check(context.Background()); err == nil {
		t.Fatal("divergent genesis must be a hard error")
	}
}

// US1: start-besu-join composes entity-besu and waits on RPC; wait-sync uses WaitSync.
func TestStartBesuJoinAndWaitSync(t *testing.T) {
	fake := &exec.FakeRunner{}
	steps := JoinSteps(testJoinCfg(t, fake))
	if err := findStep(steps, "start-besu-join").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	composed := false
	for _, c := range fake.Calls {
		if c.Name == "docker" && strings.Contains(strings.Join(c.Args, " "), "entity-besu") {
			composed = true
		}
	}
	if !composed {
		t.Fatalf("start-besu-join must compose entity-besu; calls=%+v", fake.Calls)
	}
	syncCalled := false
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	cfg.WaitSync = func(context.Context) error { syncCalled = true; return nil }
	if err := findStep(JoinSteps(cfg), "wait-sync").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !syncCalled {
		t.Fatal("wait-sync must call WaitSync")
	}
}

// US2/SC-004: wire-addresses writes spoke addresses idempotently.
func TestWireAddressesJoin(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	step := findStep(JoinSteps(cfg), "wire-addresses")
	if err := step.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := step.Run(context.Background()); err != nil { // idempotent
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg.BankEnvFile)
	s := string(b)
	if !strings.Contains(s, "SPOKE_IDENTITY_REGISTRY_ADDRESS=0xs1") || strings.Count(s, "SPOKE_BRIDGE_ADDRESS=") != 1 {
		t.Fatalf("spoke addresses not wired idempotently:\n%s", s)
	}
}

// US2: provision-keycloak-bank writes back the client secret and is idempotent.
func TestProvisionKeycloakBank(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	step := findStep(JoinSteps(cfg), "provision-keycloak-bank")
	if err := step.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ok, _ := step.Check(context.Background()); !ok {
		t.Fatal("provision-keycloak-bank should be idempotent after write-back")
	}
	b, _ := os.ReadFile(cfg.BankEnvFile)
	if !strings.Contains(string(b), "KEYCLOAK_CLIENT_SECRET=s3cr3t") {
		t.Fatalf("client secret not written back:\n%s", b)
	}
}

// US3/SC-005: gen-csr creates key (0600) + csr (OU=ROLE_COMMERCIAL_BANK),
// idempotent, zero CA material, pki dir pre-created.
func TestGenCSR(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	step := findStep(JoinSteps(cfg), "gen-csr")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("gen-csr: %v", err)
	}
	pkiDir := filepath.Join(cfg.DataDir, "pki")
	keyPath := filepath.Join(pkiDir, "bank-a.key")
	csrPath := filepath.Join(pkiDir, "bank-a.csr")

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key perm = %v, want 0600", info.Mode().Perm())
	}
	csr, _ := os.ReadFile(csrPath)
	if !strings.Contains(string(csr), "BEGIN CERTIFICATE REQUEST") {
		t.Fatalf("csr not a PEM CSR:\n%s", csr)
	}
	// Zero CA material.
	entries, _ := os.ReadDir(pkiDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "-ca.") {
			t.Fatalf("gen-csr must not produce CA material, found %s", e.Name())
		}
	}
	// Idempotent (Check true once present).
	if ok, _ := step.Check(context.Background()); !ok {
		t.Fatal("gen-csr should skip when key+csr already exist")
	}
}

// Integration: JoinSteps assembles the canonical order/deps with no cycle and
// no relay/noc step (clarification: canonical flow).
func TestJoinStepsOrder(t *testing.T) {
	steps := JoinSteps(testJoinCfg(t, &exec.FakeRunner{}))
	if _, err := topoSort(steps); err != nil {
		t.Fatalf("topoSort: %v", err)
	}
	names := map[string]bool{}
	for _, s := range steps {
		names[s.Name] = true
	}
	for _, want := range []string{
		"consume-spoke-bundle", "write-genesis", "start-besu-join", "wait-sync",
		"wire-addresses", "provision-keycloak-bank", "render-bank-compose-env",
		"start-bank-infra", "start-bank-backend", "start-bank-frontend", "gen-csr",
	} {
		if !names[want] {
			t.Errorf("missing canonical step %q", want)
		}
	}
	for _, forbidden := range []string{"register-relay-bank", "register-relay-spoke", "add-noc-agent"} {
		if names[forbidden] {
			t.Errorf("join must not include %q (canonical flow)", forbidden)
		}
	}
}
