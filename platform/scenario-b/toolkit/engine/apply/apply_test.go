package apply

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
)

const fixtures = "../manifest/testdata"

// SC-004: dry-run produces a report of planned steps without external effects.
func TestApplyFoundHubDryRun(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "found-hub.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply dry-run: %v", err)
	}
	if rep.Mode != "found-hub" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s has status %s (want planned/skipped)", s.Name, s.Status)
		}
	}
}

// TK-B7: found-spoke dry-run plans the steps (hub bundle validated, no effects).
func TestApplyFoundSpokeDryRun(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "found-spoke.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply found-spoke dry-run: %v", err)
	}
	if rep.Mode != "found-spoke" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s status %s", s.Name, s.Status)
		}
	}
}

// The sovereign FX corridor is opened at runtime via the CB governance portal,
// NOT at provisioning time: found-spoke never plans sovereign-tail steps, and
// the manifest has no way to ask for one.
func TestApplyFoundSpokeHasNoSovereignTail(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "found-spoke.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply found-spoke dry-run: %v", err)
	}
	names := map[string]bool{}
	for _, s := range rep.Steps {
		names[s.Name] = true
	}
	for _, forbidden := range []string{"open-sovereign-pair", "commit-liquidity", "seed-oracle"} {
		if names[forbidden] {
			t.Errorf("provisioning must not plan the sovereign step %q (opened via the CB portal); steps=%v", forbidden, names)
		}
	}
	// emit-spoke-bundle still closes the flow.
	if !names["emit-spoke-bundle"] {
		t.Errorf("expected emit-spoke-bundle in the plan; steps=%v", names)
	}
}

// TK-B8/SC-001: join dry-run plans the canonical steps (spoke bundle validated,
// no effects).
func TestApplyJoinDryRun(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "join.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply join dry-run: %v", err)
	}
	if rep.Mode != "join" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s status %s", s.Name, s.Status)
		}
	}
}

// SC-001: an invalid/missing spoke bundle is rejected before any effect.
func TestApplyJoinInvalidBundle(t *testing.T) {
	dir := t.TempDir()
	// A join manifest whose joinBundleRef points nowhere.
	m := filepath.Join(dir, "join-bad.yaml")
	if err := os.WriteFile(m, []byte(`apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata: { name: bank-x }
spec:
  scenario: "b"
  environment: local
  mode: join
  topology: { role: commercial-bank }
  bankId: bank-x
  spoke: { id: spoke-a, chainId: 1338, currency: BRL }
  joinBundleRef: ./bundles/does-not-exist.bundle.yaml
  node: { advertisedHost: host.docker.internal, dataDir: `+dir+` }
  image: build
  keyProvider: kms://local-emulator
  certSource: ca://central-bank-a
  frontendHost: localhost
  adminUsers: [ { role: BANK, username: x, password: y } ]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), Options{ManifestPath: m, DataDir: dir, DryRun: true}); err == nil {
		t.Fatal("expected error for invalid spoke bundle")
	}
}

// TK-B11: observe dry-run plans the NOC control-plane steps (NOC bundle
// validated, no effects). No node/relay is required for observe.
func TestApplyObserveDryRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := bundle.EmitNOC(bundle.NOCBundle{
		SpokeID: "spoke-a", SpokeUUID: "b1000000-0000-0000-0000-000000000001",
		Name: "spoke-a", CurrencyCode: "BRL", Jurisdiction: "spoke-a",
		Components: []bundle.NOCComponent{
			{Name: "besu-cb", Type: "BESU", Endpoint: "http://cb-besu:8545", ContainerName: "cb-besu"},
		},
	}, dir); err != nil {
		t.Fatal(err)
	}
	m := filepath.Join(dir, "noc.yaml")
	if err := os.WriteFile(m, []byte(`apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata: { name: noc-brazil }
spec:
  scenario: "b"
  environment: local
  mode: observe
  topology: { role: noc }
  nocBundleRef: ./bundles/spoke-a.noc.bundle.yaml
  frontendHost: localhost
  adminUsers: [ { role: ROLE_NOC_ADMIN, username: a, password: b } ]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Apply(context.Background(), Options{ManifestPath: m, DataDir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("apply observe dry-run: %v", err)
	}
	if rep.Mode != "observe" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	names := map[string]bool{}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s status %s", s.Name, s.Status)
		}
		names[s.Name] = true
	}
	for _, want := range []string{"start-noc-stack", "register-noc-spoke", "provision-noc-key"} {
		if !names[want] {
			t.Errorf("expected step %q in observe plan; steps=%v", want, names)
		}
	}
}

// observe with a missing NOC bundle is rejected before any effect.
func TestApplyObserveMissingBundle(t *testing.T) {
	dir := t.TempDir()
	m := filepath.Join(dir, "noc-bad.yaml")
	if err := os.WriteFile(m, []byte(`apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata: { name: noc-x }
spec:
  scenario: "b"
  environment: local
  mode: observe
  topology: { role: noc }
  nocBundleRef: ./bundles/does-not-exist.noc.bundle.yaml
  frontendHost: localhost
  adminUsers: [ { role: ROLE_NOC_ADMIN, username: a, password: b } ]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), Options{ManifestPath: m, DataDir: dir, DryRun: true}); err == nil {
		t.Fatal("expected error for missing NOC bundle")
	}
}

// SC-008: invalid/missing manifest is rejected before any effect.
func TestApplyMissingManifest(t *testing.T) {
	_, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(t.TempDir(), "nope.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestRenderFormats(t *testing.T) {
	r := orchestrator.Report{Mode: "found-hub", Steps: []orchestrator.StepResult{{Name: "s", Status: "planned"}}}
	j, err := Render(r, "json")
	if err != nil || !strings.Contains(string(j), "\"mode\": \"found-hub\"") {
		t.Fatalf("json render: %v\n%s", err, j)
	}
	y, err := Render(r, "yaml")
	if err != nil || !strings.Contains(string(y), "mode: found-hub") {
		t.Fatalf("yaml render: %v\n%s", err, y)
	}
	if _, err := Render(r, "xml"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
