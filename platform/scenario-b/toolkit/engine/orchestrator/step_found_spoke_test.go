package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
			"identityRegistry": "0xh1",
			"fxAgreement":      "0xh4", "pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
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
		AdvertisedHost: "10.0.0.5", // explicit → deterministic enode (no host-IP resolution)
		SpokeEnvFile:   env, KeycloakEnv: []string{env}, GatewayURL: "http://gw", Registrar: reg,
		WaitRPC:          func(context.Context) error { return nil },
		WaitKeycloak:     func(context.Context) error { return nil },
		ReadClientSecret: func(context.Context) (string, error) { return "s3cr3t", nil },
		EnodeReader:      func(context.Context, string) (string, error) { return "enode://cb@spoke:30303", nil },
	}
}

// register-cb self-registers the CB via the hub API (POST /internal/v1/spokes/register
// with X-Relay-Auth) — no direct cast/forge. The hub compliance does the on-chain work.
func TestRegisterCBSelfRegistersViaHub(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-Relay-Auth")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"already_registered":false,"tx_hash":"0xabc"}`))
	}))
	defer srv.Close()

	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	// Emit a hub bundle whose gateway points at the test server.
	hp, err := bundle.EmitHub(bundle.HubBundle{
		ChainID: 1337, HubRPC: "http://hub:8545", HubWS: "ws://hub:8546", HubGateway: srv.URL,
		Contracts: map[string]string{
			"identityRegistry": "0xh1",
			"fxAgreement":      "0xh4", "pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.HubBundlePath = hp

	if err := findStep(FoundSpokeSteps(cfg), "register-cb").Run(context.Background()); err != nil {
		t.Fatalf("register-cb: %v", err)
	}
	if gotPath != "/internal/v1/spokes/register" {
		t.Fatalf("wrong path: %s", gotPath)
	}
	if gotAuth != hubRelayAuthSecret {
		t.Fatalf("X-Relay-Auth = %q, want %q", gotAuth, hubRelayAuthSecret)
	}
	if !strings.Contains(gotBody, "ROLE_CENTRAL_BANK") || !strings.Contains(gotBody, "cb_address") {
		t.Fatalf("payload missing fields: %s", gotBody)
	}
}

// register-cb fails when the hub returns a non-2xx (registration is a hard step).
func TestRegisterCBFailsOnHubError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"compliance down"}`))
	}))
	defer srv.Close()
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	hp, _ := bundle.EmitHub(bundle.HubBundle{
		ChainID: 1337, HubRPC: "http://hub:8545", HubWS: "ws://hub:8546", HubGateway: srv.URL,
		Contracts: map[string]string{
			"identityRegistry": "0xh1",
			"fxAgreement":      "0xh4", "pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
		},
	}, t.TempDir())
	cfg.HubBundlePath = hp
	if err := findStep(FoundSpokeSteps(cfg), "register-cb").Run(context.Background()); err == nil {
		t.Fatal("register-cb must fail when the hub returns non-2xx")
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
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, "CBWeb3Spoke.s.sol:DeployCBWeb3Spoke") && strings.Contains(joined, "--legacy") {
			found = true
		}
	}
	if !found {
		t.Fatalf("deploy-spoke-contracts must invoke CBWeb3Spoke with --legacy; calls=%+v", fake.Calls)
	}
}

// R1-10.3: token metadata is derived per spoke from the manifest currency and
// keeps the "<prefix>_<ISO>" shape the portals/api-gateway parse the currency
// code from. Two spokes with different currencies never share a symbol.
func TestSpokeTokenDefaultsDeriveFromCurrency(t *testing.T) {
	for _, currency := range []string{"BRL", "COP"} {
		t.Run(currency, func(t *testing.T) {
			cfg := testSpokeCfg(t, &exec.FakeRunner{})
			cfg.Currency = currency
			cfg.WithDefaults()
			if want := "tCeBM_" + currency; cfg.TokenSymbol != want {
				t.Errorf("TokenSymbol = %q, want %q", cfg.TokenSymbol, want)
			}
			if want := "fCeBM_" + currency; cfg.FiatTokenSymbol != want {
				t.Errorf("FiatTokenSymbol = %q, want %q", cfg.FiatTokenSymbol, want)
			}
			if want := "Tokenized " + currency; cfg.TokenName != want {
				t.Errorf("TokenName = %q, want %q", cfg.TokenName, want)
			}
			if want := "Fiat " + currency; cfg.FiatTokenName != want {
				t.Errorf("FiatTokenName = %q, want %q", cfg.FiatTokenName, want)
			}
		})
	}
}

// Manifest overrides reach the Foundry deploy script verbatim; the defaults are
// only a fallback.
func TestDeploySpokePassesTokenMetadata(t *testing.T) {
	t.Run("derived from currency", func(t *testing.T) {
		fake := &exec.FakeRunner{}
		cfg := testSpokeCfg(t, fake)
		cfg.Currency = "COP"
		cfg.WithDefaults()
		_ = findStep(FoundSpokeSteps(cfg), "deploy-spoke-contracts").Run(context.Background())
		got := forgeSpokeCall(fake)
		for _, want := range []string{`TOKEN_SYMBOL="tCeBM_COP"`, `FIAT_TOKEN_SYMBOL="fCeBM_COP"`, `TOKEN_NAME="Tokenized COP"`, `FIAT_TOKEN_NAME="Fiat COP"`} {
			if !strings.Contains(got, want) {
				t.Errorf("deploy call missing %s:\n%s", want, got)
			}
		}
	})
	t.Run("manifest override wins", func(t *testing.T) {
		fake := &exec.FakeRunner{}
		cfg := testSpokeCfg(t, fake)
		cfg.Currency = "BRL"
		cfg.TokenName = "Real Digital"
		cfg.TokenSymbol = "tRD_BRL"
		cfg.FiatTokenName = "Real"
		cfg.FiatTokenSymbol = "fRD_BRL"
		cfg.WithDefaults()
		_ = findStep(FoundSpokeSteps(cfg), "deploy-spoke-contracts").Run(context.Background())
		got := forgeSpokeCall(fake)
		for _, want := range []string{`TOKEN_NAME="Real Digital"`, `TOKEN_SYMBOL="tRD_BRL"`, `FIAT_TOKEN_NAME="Real"`, `FIAT_TOKEN_SYMBOL="fRD_BRL"`} {
			if !strings.Contains(got, want) {
				t.Errorf("deploy call missing %s:\n%s", want, got)
			}
		}
	})
}

// forgeSpokeCall returns the CBWeb3Spoke deploy invocation recorded by the fake.
func forgeSpokeCall(fake *exec.FakeRunner) string {
	for _, c := range fake.Calls {
		joined := c.Name + " " + strings.Join(c.Args, " ")
		if strings.Contains(joined, "CBWeb3Spoke.s.sol:DeployCBWeb3Spoke") {
			return joined
		}
	}
	return ""
}

// NATIVE_ASSET_SYMBOL must be the symbol actually deployed on-chain, not a
// string recomposed from the currency — otherwise env and ERC-20 disagree
// whenever the manifest overrides the symbol.
func TestComposeEnvNativeAssetSymbolTracksDeployedSymbol(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.Currency = "BRL"
	cfg.TokenSymbol = "tRD_BRL"
	cfg.WithDefaults()
	if env := strings.Join(cfg.ComposeEnv(), "\n"); !strings.Contains(env, "NATIVE_ASSET_SYMBOL=tRD_BRL") {
		t.Fatalf("NATIVE_ASSET_SYMBOL must mirror the deployed TokenSymbol:\n%s", env)
	}
}

// The CB portals bake VITE_FIAT_SYMBOL so balances fall back to the sovereign
// currency code instead of the generic "fiat units" label — in both the host-port
// and the proxy build. The supervisor portal has no fiat label and is excluded.
func TestCBPortalsBakeFiatSymbol(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.Currency = "COP"
	cfg.WithDefaults()

	check := func(t *testing.T, label string) {
		t.Helper()
		gov, tre, sup := cfg.portalViteArgs("http://localhost:16845")
		if got := gov["VITE_FIAT_SYMBOL"]; got != "COP" {
			t.Errorf("%s governance VITE_FIAT_SYMBOL = %q, want COP", label, got)
		}
		if got := tre["VITE_FIAT_SYMBOL"]; got != "COP" {
			t.Errorf("%s treasury VITE_FIAT_SYMBOL = %q, want COP", label, got)
		}
		if _, ok := sup["VITE_FIAT_SYMBOL"]; ok {
			t.Errorf("%s supervisor portal has no fiat label; VITE_FIAT_SYMBOL must not be passed", label)
		}
	}
	check(t, "host-port")

	cfg.ProxyEnabled = true
	cfg.FrontendHost = "cb-colombia.example"
	check(t, "proxy")
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
	cfg.Currency = "BRL" // relay id follows the api-gateway's spoke-<currency> convention
	steps := FoundSpokeSteps(cfg)
	if err := findStep(steps, "register-relay-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	lr := cfg.Registrar.(relayregistrar.LocalRegistry)
	// The relay is keyed by "spoke-<currency>" (matches spoke_out/spoke_in derived
	// by the gateway), NOT the country-based spoke.id ("spoke-a").
	if len(lr.List()) != 1 || lr.List()[0].ID != "spoke-brl" {
		t.Fatalf("relay registry = %+v", lr.List())
	}
	if !findStep(steps, "add-noc-agent").Soft {
		t.Fatal("add-noc-agent must be Soft")
	}
}

// US5: emit-spoke-bundle produces a valid spoke bundle (genesis + enode + contracts).
// When AdvertisedHost is set, public RPC/WS/gateway URLs use that host (not localhost).
func TestEmitSpokeBundle(t *testing.T) {
	// The genesis is read from the named volume via the runner (docker cat), so
	// the fake returns it for the docker call in emit-spoke-bundle.
	fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(`{"config":{"chainId":1338}}`)}}
	cfg := testSpokeCfg(t, fake)
	cfg.RPCPort = 8845
	cfg.WSPort = 8846
	writeSpokeBroadcast(t, cfg.ContractsDir)

	steps := FoundSpokeSteps(cfg)
	// start-besu-spoke captures the enode (shared closure) before emit.
	if err := findStep(steps, "start-besu-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := findStep(steps, "emit-spoke-bundle").Run(context.Background()); err != nil {
		t.Fatalf("emit-spoke-bundle: %v", err)
	}
	b, err := bundle.LoadSpoke(filepath.Join(cfg.OutDir, "bundles", "spoke-a.bundle.yaml"))
	if err != nil {
		t.Fatalf("load spoke bundle: %v", err)
	}
	// emit rewrites the enode host to the advertised endpoint (the configured
	// AdvertisedHost + P2P port) so a joining bank can dial it as --bootnodes.
	if b.Enode != "enode://cb@10.0.0.5:30303" || b.ChainID != 1338 || b.Contracts["spokeBridge"] != "0xs3" {
		t.Fatalf("spoke bundle mismatch: %+v", b)
	}
	if b.SpokeRPC != "http://10.0.0.5:8845" {
		t.Fatalf("SpokeRPC want http://10.0.0.5:8845, got %q", b.SpokeRPC)
	}
	if b.SpokeWS != "ws://10.0.0.5:8846" {
		t.Fatalf("SpokeWS want ws://10.0.0.5:8846, got %q", b.SpokeWS)
	}
	if b.CBGateway != "http://10.0.0.5:16845" {
		t.Fatalf("CBGateway want http://10.0.0.5:16845, got %q", b.CBGateway)
	}
	// The routable hub RPC (from the hub bundle) is published so a joining bank
	// reaches the hub cross-VM (HUB_BESU_RPC_URL) instead of host.docker.internal.
	if b.HubRPC != "http://hub:8545" {
		t.Fatalf("bundle HubRPC want http://hub:8545, got %q", b.HubRPC)
	}
}

// R1-10.3: the bundle publishes the spoke's sovereign currency so the join can
// reject a bank manifest claiming another one.
func TestEmitSpokeBundlePublishesCurrency(t *testing.T) {
	fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(`{"config":{"chainId":1338}}`)}}
	cfg := testSpokeCfg(t, fake)
	cfg.Currency = "COP"
	cfg.WithDefaults()
	writeSpokeBroadcast(t, cfg.ContractsDir)

	steps := FoundSpokeSteps(cfg)
	if err := findStep(steps, "start-besu-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := findStep(steps, "emit-spoke-bundle").Run(context.Background()); err != nil {
		t.Fatalf("emit-spoke-bundle: %v", err)
	}
	b, err := bundle.LoadSpoke(filepath.Join(cfg.OutDir, "bundles", "spoke-a.bundle.yaml"))
	if err != nil {
		t.Fatalf("load spoke bundle: %v", err)
	}
	if b.Currency != "COP" {
		t.Fatalf("bundle currency = %q, want COP", b.Currency)
	}
}

// emit-spoke-bundle Check re-runs when an on-disk bundle still has localhost /
// host.docker.internal but AdvertisedHost is a routable IP (stale LNET artifact).
func TestEmitSpokeBundleCheckRewritesLocalhost(t *testing.T) {
	fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(`{"config":{"chainId":1338}}`)}}
	cfg := testSpokeCfg(t, fake)
	cfg.RPCPort = 8845
	cfg.WSPort = 8846
	writeSpokeBroadcast(t, cfg.ContractsDir)

	steps := FoundSpokeSteps(cfg)
	if err := findStep(steps, "start-besu-spoke").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := findStep(steps, "emit-spoke-bundle").Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Overwrite with a stale localhost / host.docker.internal bundle.
	stalePath := filepath.Join(cfg.OutDir, "bundles", "spoke-a.bundle.yaml")
	stale, err := bundle.LoadSpoke(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	stale.SpokeRPC = "http://localhost:8845"
	stale.SpokeWS = "ws://localhost:8846"
	stale.CBGateway = "http://host.docker.internal:16845"
	if _, err := bundle.EmitSpoke(stale, cfg.OutDir); err != nil {
		t.Fatal(err)
	}

	ok, err := findStep(FoundSpokeSteps(cfg), "emit-spoke-bundle").Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Check should be false for stale localhost bundle when AdvertisedHost is set")
	}
}

// REGRESSION LOCK: a bundle left over from a PREVIOUS founding of the same spoke
// keeps the same host and ports, so endpoint equality alone would skip re-emission
// — and a bank joining on that bundle writes the wrong genesis and dials a dead
// bootnode, never peers, and only fails 10 minutes later in wait-sync. The Check
// must re-emit whenever the genesis or the node identity diverges from the live node.
func TestEmitSpokeBundleCheckDetectsRefoundedSpoke(t *testing.T) {
	emitCurrent := func(t *testing.T) (SpokeConfig, *exec.FakeRunner) {
		t.Helper()
		fake := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte(`{"config":{"chainId":1338}}`)}}
		cfg := testSpokeCfg(t, fake)
		cfg.RPCPort = 8845
		cfg.WSPort = 8846
		writeSpokeBroadcast(t, cfg.ContractsDir)
		steps := FoundSpokeSteps(cfg)
		if err := findStep(steps, "start-besu-spoke").Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := findStep(steps, "emit-spoke-bundle").Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return cfg, fake
	}

	t.Run("matching bundle skips", func(t *testing.T) {
		cfg, _ := emitCurrent(t)
		ok, err := findStep(FoundSpokeSteps(cfg), "emit-spoke-bundle").Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("Check should be true when the bundle matches the live genesis + enode")
		}
	})

	t.Run("re-founded genesis re-emits", func(t *testing.T) {
		cfg, fake := emitCurrent(t)
		// The spoke was re-founded: same host/ports, fresh genesis in the volume.
		fake.Outputs["docker"] = []byte(`{"config":{"chainId":1338},"extraData":"0xfresh"}`)
		ok, err := findStep(FoundSpokeSteps(cfg), "emit-spoke-bundle").Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("Check must be false when the volume genesis diverges from the bundle")
		}
	})

	t.Run("fresh node key re-emits", func(t *testing.T) {
		cfg, _ := emitCurrent(t)
		// The spoke was re-founded: same genesis content, but a new node identity.
		cfg.EnodeReader = func(context.Context, string) (string, error) {
			return "enode://freshnodekey@spoke:30303", nil
		}
		ok, err := findStep(FoundSpokeSteps(cfg), "emit-spoke-bundle").Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("Check must be false when the live node id differs from the bundle enode")
		}
	})
}

// ComposeEnv carries plumbing (images/prefixes/ports/static infra creds) for the
// runner's process env, and NEVER the runtime-discovered values (contract
// addresses, Keycloak client secret) that live in the slim .env.spoke file.
func TestComposeEnvCarriesPlumbingNotRuntimeState(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.WithDefaults()
	env := strings.Join(cfg.ComposeEnv(), "\n")
	for _, want := range []string{"CONTAINER_PREFIX=", "ENTITY_RPC_PORT=", "BESU_IMAGE=", "POSTGRES_PASSWORD=", "HUB_RPC_PORT="} {
		if !strings.Contains(env, want) {
			t.Errorf("ComposeEnv missing plumbing %q:\n%s", want, env)
		}
	}
	for _, forbidden := range []string{"SPOKE_TCEBM_ADDRESS=", "HUB_IDENTITY_REGISTRY_ADDRESS=", "KEYCLOAK_CLIENT_SECRET="} {
		if strings.Contains(env, forbidden) {
			t.Errorf("ComposeEnv must not carry runtime-discovered value %q", forbidden)
		}
	}
}

// composeUpArgs adds --env-file only once the state file exists (founder besu
// starts before any AppendAddr writes it; the backend gets it afterwards).
func TestComposeUpArgsGuardsEnvFile(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.WithDefaults()
	if strings.Contains(strings.Join(cfg.composeUpArgs("entity-besu-founder"), " "), "--env-file") {
		t.Fatal("no --env-file expected before the state file exists")
	}
	if err := os.WriteFile(cfg.SpokeEnvFile, []byte("SPOKE_TCEBM_ADDRESS=0x1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(cfg.composeUpArgs("entity-backend"), " "), "--env-file") {
		t.Fatal("--env-file expected once the state file exists")
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
	must("register-cb", "register-currency")
	// The separation needs the W-token to exist and the handover to have made this CB its
	// administrator, both of which register-currency produces.
	must("register-currency", "separate-token-admin")
	must("gen-genesis-spoke", "start-besu-spoke")
	must("start-besu-spoke", "deploy-spoke-contracts")
	must("deploy-spoke-contracts", "emit-spoke-bundle")
	must("start-besu-spoke", "register-relay-spoke")
	// Infra creates the network/DB that keycloak + backend join → before them.
	must("start-spoke-infra", "provision-keycloak-spoke")
	must("provision-keycloak-spoke", "start-spoke-backend")
}
