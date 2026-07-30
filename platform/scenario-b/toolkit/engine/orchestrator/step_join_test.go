package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
		BankRPC: "http://bank:8646", SpokeBundlePath: bp,
		DataDir: dir, BankEnvFile: env, KeycloakEnv: []string{env},
		WaitRPC:          func(context.Context) error { return nil },
		WaitSync:         func(context.Context) error { return nil },
		WaitKeycloak:     func(context.Context) error { return nil },
		ReadClientSecret: func(context.Context) (string, error) { return "s3cr3t", nil },
	}
}

// ComposeEnv carries plumbing (images/prefixes/ports/bootnode/static infra
// creds) for the runner's process env, and NEVER the runtime-discovered values
// (contract addresses, Keycloak client secret) that live in the slim .env.bank.
func TestJoinComposeEnvCarriesPlumbingNotRuntimeState(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	cfg.WithDefaults()
	env := strings.Join(cfg.ComposeEnv(), "\n")
	for _, want := range []string{"CONTAINER_PREFIX=", "ENTITY_RPC_PORT=", "BESU_IMAGE=", "BOOTNODE_ENODE=", "POSTGRES_PASSWORD="} {
		if !strings.Contains(env, want) {
			t.Errorf("ComposeEnv missing plumbing %q:\n%s", want, env)
		}
	}
	for _, forbidden := range []string{"SPOKE_TCEBM_ADDRESS=", "KEYCLOAK_CLIENT_SECRET="} {
		if strings.Contains(env, forbidden) {
			t.Errorf("ComposeEnv must not carry runtime-discovered value %q", forbidden)
		}
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

// R1-10.3: the currency is the routing key ("spoke-<currency>", hub currency
// registration), so a bank declaring a currency other than its spoke's is
// rejected. A bundle without a currency (emitted before the field existed) and
// a matching declaration (case-insensitive) both pass.
func TestConsumeSpokeBundleCurrencyCrossCheck(t *testing.T) {
	emit := func(t *testing.T, currency string) string {
		t.Helper()
		sb := bundle.SpokeBundle{
			SpokeID: "spoke-a", ChainID: 1338, Enode: "enode://cb@spoke:31303",
			SpokeRPC: "http://spoke:8645", SpokeWS: "ws://spoke:8655", Genesis: testGenesis,
			Contracts: map[string]string{
				"identityRegistry": "0xs1", "tCeBM": "0xs2", "spokeBridge": "0xs3", "fCeBM": "0xs4",
			},
			Currency: currency,
		}
		p, err := bundle.EmitSpoke(sb, filepath.Join(t.TempDir(), "bundleout"))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("mismatch is rejected", func(t *testing.T) {
		cfg := testJoinCfg(t, &exec.FakeRunner{})
		cfg.SpokeBundlePath = emit(t, "BRL")
		cfg.Currency = "COP"
		err := findStep(JoinSteps(cfg), "consume-spoke-bundle").Run(context.Background())
		if err == nil {
			t.Fatal("consume-spoke-bundle must reject a currency that differs from the spoke bundle")
		}
		if !strings.Contains(err.Error(), "COP") || !strings.Contains(err.Error(), "BRL") {
			t.Errorf("error should name both currencies, got: %v", err)
		}
	})

	t.Run("match passes", func(t *testing.T) {
		cfg := testJoinCfg(t, &exec.FakeRunner{})
		cfg.SpokeBundlePath = emit(t, "BRL")
		cfg.Currency = "brl" // case-insensitive
		if err := findStep(JoinSteps(cfg), "consume-spoke-bundle").Run(context.Background()); err != nil {
			t.Fatalf("consume-spoke-bundle: %v", err)
		}
	})

	t.Run("legacy bundle without currency passes", func(t *testing.T) {
		cfg := testJoinCfg(t, &exec.FakeRunner{}) // helper emits no currency
		cfg.Currency = "COP"
		if err := findStep(JoinSteps(cfg), "consume-spoke-bundle").Run(context.Background()); err != nil {
			t.Fatalf("consume-spoke-bundle: %v", err)
		}
	})
}

// The bank portal bakes VITE_FIAT_SYMBOL so balances fall back to the spoke's
// currency code instead of the generic "fiat units" label — in both the
// host-port and the proxy build.
func TestJoinPortalBakesFiatSymbol(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	cfg.Currency = "COP"
	cfg.WithDefaults()
	if got := cfg.portalViteArgs(18645)["VITE_FIAT_SYMBOL"]; got != "COP" {
		t.Errorf("VITE_FIAT_SYMBOL = %q, want COP", got)
	}
	cfg.ProxyEnabled = true
	cfg.FrontendHost = "bank-a.example"
	if got := cfg.portalViteArgs(18645)["VITE_FIAT_SYMBOL"]; got != "COP" {
		t.Errorf("proxy build: VITE_FIAT_SYMBOL = %q, want COP", got)
	}
}

// US1/SC-002: write-genesis seeds the bundle genesis STRAIGHT INTO the named
// volume (no host file); re-run with matching content skips (Check true); a
// divergent volume genesis is a hard error.
func TestWriteGenesisGuard(t *testing.T) {
	// Node state lives only in the named volume: Run seeds it from memory via the
	// runner (docker run … base64 -d); Check reads it back (docker cat → fake
	// Outputs["docker"]). No host genesis file is written.
	fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(testGenesis)}}
	cfg := testJoinCfg(t, fake)
	step := findStep(JoinSteps(cfg), "write-genesis")

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("write-genesis run: %v", err)
	}
	// The genesis must NOT be staged on the host — only in the volume.
	if fileExists(filepath.Join(cfg.DataDir, "genesis", "genesis.json")) {
		t.Fatal("write-genesis must not write a host genesis file (volume-only)")
	}
	// Run must seed the volume from memory: a docker call carrying the genesis
	// volume + base64 decode into genesis.json.
	seeded := false
	for _, c := range fake.Calls {
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, cfg.genesisVolume()) && strings.Contains(joined, "base64 -d") && strings.Contains(joined, "genesis.json") {
			seeded = true
		}
	}
	if !seeded {
		t.Fatalf("write-genesis must seed the volume from memory; calls=%+v", fake.Calls)
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
		"wire-addresses", "provision-keycloak-bank",
		"start-bank-infra", "start-bank-backend", "start-bank-frontend", "gen-csr",
		"add-noc-agent", // node-level monitoring of the bank's own node
	} {
		if !names[want] {
			t.Errorf("missing canonical step %q", want)
		}
	}
	for _, forbidden := range []string{"register-relay-bank", "register-relay-spoke"} {
		if names[forbidden] {
			t.Errorf("join must not include %q (canonical flow)", forbidden)
		}
	}
}

// The bank runs its OWN noc-agent (node-level): self-provision its per-entity
// key against the observe backend, render a multi-component agent.yaml under the
// SAME spoke UUID as the CB, seed it, and start the agent.
func TestJoinAddNOCAgent(t *testing.T) {
	var provReq map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/admin/agents/provision-key" {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &provReq)
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	fake := &exec.FakeRunner{}
	cfg := testJoinCfg(t, fake)
	cfg.NOCBackendURL = srv.URL // host-reachable in the test (no host.docker.internal)
	step := findStep(JoinSteps(cfg), "add-noc-agent")
	if step.Name == "" || !step.Soft {
		t.Fatal("add-noc-agent must exist and be Soft")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Self-provisioned this bank's per-entity key, bound to the shared spoke UUID.
	if provReq["spoke_id"] != deterministicUUID("spoke-a") {
		t.Errorf("provision spoke_id = %q, want CB's spoke UUID", provReq["spoke_id"])
	}
	if provReq["raw_key"] != deterministicAgentKey("spoke-a", "bank-a") {
		t.Errorf("provision raw_key = %q, want bank per-entity key", provReq["raw_key"])
	}
	// Seeded the bank's agent.yaml and brought up the noc-agent compose.
	var sawSeed, sawCompose bool
	for _, c := range fake.Calls {
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, "run") && strings.Contains(joined, "_noc_agent_cfg:/t") {
			sawSeed = true
		}
		if strings.Contains(joined, "noc-agent.compose.yaml") && strings.Contains(joined, "up -d") {
			sawCompose = true
		}
	}
	if !sawSeed || !sawCompose {
		t.Fatalf("missing actions: seed=%v compose=%v", sawSeed, sawCompose)
	}
}
