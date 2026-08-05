// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

func observeBundle() bundle.NOCBundle {
	return bundle.NOCBundle{
		Version:      bundle.NOCBundleVersion,
		SpokeID:      "spoke-brl",
		SpokeUUID:    deterministicUUID("spoke-brl"),
		Name:         "spoke-brl",
		CurrencyCode: "BRL",
		Jurisdiction: "spoke-brl",
		Components: []bundle.NOCComponent{
			{Name: "besu-central-bank", Type: "BESU", Endpoint: "http://cb-besu:8545", ContainerName: "cb-besu"},
		},
	}
}

// fakeNOCBackend records admin calls and simulates the real handlers' status codes.
type fakeNOCBackend struct {
	registered map[string]bool // spoke uuid → exists
	spokeBody  map[string]any
	keyBody    map[string]string
}

func newFakeNOCBackend() (*fakeNOCBackend, *httptest.Server) {
	b := &fakeNOCBackend{registered: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/admin/spokes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &b.spokeBody)
		id, _ := b.spokeBody["id"].(string)
		if b.registered[id] {
			w.WriteHeader(http.StatusConflict)
			return
		}
		b.registered[id] = true
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/api/v1/admin/spokes/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/spokes/")
		if b.registered[id] {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v1/admin/agents/provision-key", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &b.keyBody)
		w.WriteHeader(http.StatusCreated)
	})
	return b, httptest.NewServer(mux)
}

func testObserveCfg(t *testing.T, backendURL string) ObserveConfig {
	t.Helper()
	cfg := ObserveConfig{
		Runner:       &exec.FakeRunner{},
		ScenarioBDir: t.TempDir(),
		TemplatesDir: t.TempDir(),
		Bundle:       observeBundle(),
		BackendURL:   backendURL,
	}
	cfg.WithDefaults()
	return cfg
}

func TestObserveStepOrder(t *testing.T) {
	steps := ObserveSteps(testObserveCfg(t, "http://localhost:9"))
	ordered, err := topoSort(steps)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, s := range ordered {
		pos[s.Name] = i
	}
	for _, name := range []string{"build-noc-images", "start-noc-stack", "wait-noc-backend", "register-noc-spoke", "provision-noc-key"} {
		if _, ok := pos[name]; !ok {
			t.Fatalf("missing step %q", name)
		}
	}
	if pos["start-noc-stack"] < pos["build-noc-images"] {
		t.Error("build-noc-images must precede start-noc-stack")
	}
	if pos["register-noc-spoke"] < pos["wait-noc-backend"] {
		t.Error("wait-noc-backend must precede register-noc-spoke")
	}
	if pos["provision-noc-key"] < pos["register-noc-spoke"] {
		t.Error("register-noc-spoke must precede provision-noc-key")
	}
}

func TestObserveStartStackCompose(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testObserveCfg(t, "http://localhost:9")
	cfg.Runner = fake
	cfg.ContainerPrefix = "sc-b-cbweb3-noc-brazil"
	if err := findStep(ObserveSteps(cfg), "start-noc-stack").Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var joined string
	for _, c := range fake.Calls {
		joined = c.Name + " " + strings.Join(c.Args, " ")
	}
	if !strings.Contains(joined, "noc-stack.compose.yaml") ||
		!strings.Contains(joined, "-p sc-b-cbweb3-noc-brazil") ||
		!strings.Contains(joined, "up -d") {
		t.Fatalf("unexpected compose invocation: %q", joined)
	}
}

func TestObserveRegisterAndProvision(t *testing.T) {
	back, srv := newFakeNOCBackend()
	defer srv.Close()
	cfg := testObserveCfg(t, srv.URL)
	cfg.HTTPClient = srv.Client()
	steps := ObserveSteps(cfg)

	// Not yet registered → Check false.
	if ok, _ := findStep(steps, "register-noc-spoke").Check(context.Background()); ok {
		t.Fatal("register Check should be false before registration")
	}
	if err := findStep(steps, "register-noc-spoke").Run(context.Background()); err != nil {
		t.Fatalf("register Run: %v", err)
	}
	if id, _ := back.spokeBody["id"].(string); id != deterministicUUID("spoke-brl") {
		t.Errorf("registered id = %v, want deterministic UUID", back.spokeBody["id"])
	}
	if back.spokeBody["currency_code"] != "BRL" || back.spokeBody["jurisdiction"] != "spoke-brl" {
		t.Errorf("register payload missing fields: %+v", back.spokeBody)
	}
	// Now registered → Check true (idempotent).
	if ok, err := findStep(steps, "register-noc-spoke").Check(context.Background()); err != nil || !ok {
		t.Errorf("register Check after = (%v,%v), want (true,nil)", ok, err)
	}

	if err := findStep(steps, "provision-noc-key").Run(context.Background()); err != nil {
		t.Fatalf("provision Run: %v", err)
	}
	if back.keyBody["spoke_id"] != deterministicUUID("spoke-brl") {
		t.Errorf("provision spoke_id = %q, want deterministic UUID", back.keyBody["spoke_id"])
	}
	if back.keyBody["raw_key"] != deterministicAgentKey("spoke-brl", nocFoundingAgentLabel) {
		t.Errorf("provision raw_key = %q, want deterministic agent key", back.keyBody["raw_key"])
	}
}

func TestObserveRegisterSendsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	cfg := testObserveCfg(t, srv.URL)
	cfg.HTTPClient = srv.Client()
	if err := findStep(ObserveSteps(cfg), "register-noc-spoke").Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotAuth != "Bearer "+nocLocalAdminBearer {
		t.Errorf("Authorization = %q, want Bearer %s", gotAuth, nocLocalAdminBearer)
	}
}
