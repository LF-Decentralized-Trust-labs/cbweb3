package orchestrator

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

var uuidCanonical = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestDeterministicUUID(t *testing.T) {
	a := deterministicUUID("spoke-a")
	if !uuidCanonical.MatchString(a) {
		t.Fatalf("not a canonical UUID: %q", a)
	}
	if a != deterministicUUID("spoke-a") {
		t.Fatal("not stable across calls")
	}
	if a == deterministicUUID("spoke-b") {
		t.Fatal("distinct spoke ids must yield distinct UUIDs")
	}
	// version 5 nibble + RFC-4122 variant bits.
	if a[14] != '5' {
		t.Errorf("expected version nibble 5, got %c in %q", a[14], a)
	}
	if v := a[19]; v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Errorf("expected RFC-4122 variant nibble, got %c in %q", v, a)
	}
}

func TestDeterministicAgentKey(t *testing.T) {
	cb := deterministicAgentKey("spoke-a", "central-bank")
	if cb != deterministicAgentKey("spoke-a", "central-bank") {
		t.Fatal("not stable")
	}
	// Per-entity: the CB and a bank on the same spoke get DISTINCT keys so the
	// backend tracks their liveness independently.
	if cb == deterministicAgentKey("spoke-a", "bank-a") {
		t.Fatal("distinct entities on a spoke must yield distinct keys")
	}
	// Per-spoke: same entity name on a different spoke also differs.
	if cb == deterministicAgentKey("spoke-b", "central-bank") {
		t.Fatal("distinct spokes must yield distinct keys")
	}
}

func TestSpokeNOCBundle(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.RelayEndpoint = "http://hub:7000"
	cfg.WithDefaults()

	b := cfg.nocBundle()
	if err := bundle.ValidateNOC(b); err != nil {
		t.Fatalf("nocBundle invalid: %v", err)
	}
	if b.SpokeUUID != deterministicUUID(cfg.SpokeID) {
		t.Errorf("SpokeUUID = %q, want deterministic", b.SpokeUUID)
	}
	if b.CurrencyCode == "" || b.Name == "" || b.Jurisdiction == "" {
		t.Errorf("required backend fields empty: %+v", b)
	}
	// BESU (always) + CACTI_RELAY (RelayEndpoint set).
	types := map[string]bool{}
	for _, c := range b.Components {
		types[c.Type] = true
	}
	if !types["BESU"] || !types["CACTI_RELAY"] {
		t.Errorf("expected BESU + CACTI_RELAY, got %+v", b.Components)
	}

	// No relay configured → BESU only.
	cfg.RelayEndpoint = ""
	if got := cfg.nocBundle(); len(got.Components) != 1 || got.Components[0].Type != "BESU" {
		t.Errorf("expected BESU-only without relay, got %+v", got.Components)
	}
}

// The relay component only carries a container name when the manifest names one; the
// agent needs it to collect the relay's logs (without it the NOC shows no relay logs).
func TestSpokeNOCBundleRelayContainerName(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.RelayEndpoint = "http://hub:7000"
	cfg.WithDefaults()

	if got := relayComponent(t, cfg.nocBundle()); got.ContainerName != "" {
		t.Errorf("ContainerName = %q, want empty when the manifest names no container", got.ContainerName)
	}

	cfg.RelayContainerName = "cbweb3-cacti-liquidity-relay"
	got := relayComponent(t, cfg.nocBundle())
	if got.ContainerName != "cbweb3-cacti-liquidity-relay" {
		t.Errorf("ContainerName = %q, want the configured relay container", got.ContainerName)
	}
	// The Besu component keeps its own container name — the relay must not overwrite it.
	for _, c := range cfg.nocBundle().Components {
		if c.Type == "BESU" && c.ContainerName == "" {
			t.Error("BESU component lost its container name")
		}
	}
}

func TestJoinNOCBundleRelayContainerName(t *testing.T) {
	cfg := testJoinCfg(t, &exec.FakeRunner{})
	cfg.RelayEndpoint = "http://hub:7000"
	cfg.RelayContainerName = "cbweb3-cacti-liquidity-relay"
	cfg.WithDefaults()

	if got := relayComponent(t, cfg.nocBundle()); got.ContainerName != "cbweb3-cacti-liquidity-relay" {
		t.Errorf("ContainerName = %q, want the configured relay container", got.ContainerName)
	}
}

// relayComponent returns the bundle's CACTI_RELAY component, failing when absent.
func relayComponent(t *testing.T, b bundle.NOCBundle) bundle.NOCComponent {
	t.Helper()
	for _, c := range b.Components {
		if c.Type == "CACTI_RELAY" {
			return c
		}
	}
	t.Fatalf("no CACTI_RELAY component in %+v", b.Components)
	return bundle.NOCComponent{}
}

func TestHubNOCBundle(t *testing.T) {
	cfg := testHubConfig(t, &exec.FakeRunner{})
	cfg.WithDefaults()
	b := cfg.nocBundle()
	if err := bundle.ValidateNOC(b); err != nil {
		t.Fatalf("hub nocBundle invalid: %v", err)
	}
	if b.SpokeID != "hub" || b.SpokeUUID != deterministicUUID("hub") {
		t.Errorf("hub bundle id/uuid mismatch: %+v", b)
	}
	if len(b.Components) != 1 || b.Components[0].Type != "BESU" {
		t.Errorf("expected single BESU component, got %+v", b.Components)
	}
}

func TestEmitNOCBundleStepSpoke(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	steps := FoundSpokeSteps(cfg)
	step := findStep(steps, "emit-noc-bundle")
	if step.Name == "" {
		t.Fatal("emit-noc-bundle step not found")
	}
	if !step.Soft {
		t.Error("emit-noc-bundle must be Soft (observability never blocks)")
	}

	// Not emitted yet → Check reports not-done.
	if ok, _ := step.Check(context.Background()); ok {
		t.Fatal("Check should be false before emission")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	p := filepath.Join(cfg.OutDir, "bundles", "spoke-a.noc.bundle.yaml")
	b, err := bundle.LoadNOC(p)
	if err != nil {
		t.Fatalf("LoadNOC: %v", err)
	}
	if b.SpokeUUID != deterministicUUID("spoke-a") {
		t.Errorf("emitted SpokeUUID mismatch: %q", b.SpokeUUID)
	}
	// Idempotent: a second Check (bundle present, matching UUID) reports done.
	if ok, err := step.Check(context.Background()); err != nil || !ok {
		t.Errorf("Check after emit = (%v,%v), want (true,nil)", ok, err)
	}
}

func TestRenderAgentYAML(t *testing.T) {
	b := bundle.NOCBundle{
		SpokeID: "spoke-brl", SpokeUUID: deterministicUUID("spoke-brl"),
		Components: []bundle.NOCComponent{
			{Name: "besu-cb", Type: "BESU", Endpoint: "http://cb-besu:8545", ContainerName: "cb-besu"},
			{Name: "cacti-relay", Type: "CACTI_RELAY", Endpoint: "http://host.docker.internal:7000"},
		},
	}
	raw, err := renderAgentYAML(b, "http://host.docker.internal:8090", "noc-agent-abc", 0)
	if err != nil {
		t.Fatalf("renderAgentYAML: %v", err)
	}
	var out struct {
		SpokeID             string `yaml:"spoke_id"`
		NocBackendURL       string `yaml:"noc_backend_url"`
		APIKey              string `yaml:"api_key"`
		PushIntervalSeconds int    `yaml:"push_interval_seconds"`
		Components          []struct {
			Name, Type, Endpoint, ContainerName string
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The pushed spoke_id MUST be the UUID (matches the provisioned key binding).
	if out.SpokeID != deterministicUUID("spoke-brl") {
		t.Errorf("spoke_id = %q, want UUID", out.SpokeID)
	}
	if out.NocBackendURL != "http://host.docker.internal:8090" || out.APIKey != "noc-agent-abc" {
		t.Errorf("backend/key mismatch: %+v", out)
	}
	if out.PushIntervalSeconds != 15 { // 0 → default 15
		t.Errorf("pushInterval = %d, want default 15", out.PushIntervalSeconds)
	}
	if len(out.Components) != 2 || out.Components[1].Type != "CACTI_RELAY" {
		t.Errorf("expected 2 components incl CACTI_RELAY, got %+v", out.Components)
	}
}

func TestAddNOCAgentStep(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testSpokeCfg(t, fake)
	cfg.NOCBackendURL = "http://host.docker.internal:8090"
	step := findStep(FoundSpokeSteps(cfg), "add-noc-agent")
	if step.Name == "" || !step.Soft {
		t.Fatal("add-noc-agent must exist and be Soft")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Assert: agent.yaml seeded into the config volume (writeVolumeFile docker run
	// with the volume mount) and the noc-agent compose brought up. (The image
	// build is skipped here because FakeRunner reports the image already exists.)
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
		t.Fatalf("missing step actions: seed=%v compose=%v", sawSeed, sawCompose)
	}
}

func TestEmitNOCBundleStepHub(t *testing.T) {
	cfg := testHubConfig(t, &exec.FakeRunner{})
	step := findStep(FoundHubSteps(cfg), "emit-noc-bundle")
	if step.Name == "" {
		t.Fatal("emit-noc-bundle step not found in hub")
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := bundle.LoadNOC(filepath.Join(cfg.OutDir, "bundles", "hub.noc.bundle.yaml")); err != nil {
		t.Fatalf("LoadNOC hub: %v", err)
	}
}
