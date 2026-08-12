// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// stepByName returns a step from the observe DAG, failing when it is absent.
func stepByName(t *testing.T, steps []Step, name string) Step {
	t.Helper()
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("step %q not found", name)
	return Step{}
}

// The steps that bring the NOC stack up must decide from the live containers, not from
// the persisted state: state lives on the host and the containers live in docker, so a
// --clean redeploy (or any docker rm) leaves state claiming "done" for a stack that no
// longer exists. Skipping them then starts nothing and the registration that follows
// fails against a backend that was never launched.
func TestStartNocStackChecksTheContainersNotTheState(t *testing.T) {
	c := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle()}
	c.WithDefaults()

	step := stepByName(t, ObserveSteps(c), "start-noc-stack")
	if step.Check == nil {
		t.Fatal("start-noc-stack has no Check, so persisted state alone decides whether it runs")
	}

	// Containers absent (inspect returns nothing) → must not skip.
	skip, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if skip {
		t.Error("stack reported as up while no container is running")
	}

	// All containers running → skip.
	c.Runner = &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("true\n")}}
	skip, err = stepByName(t, ObserveSteps(c), "start-noc-stack").Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !skip {
		t.Error("running stack should be skipped, not recreated on every apply")
	}
}

func TestStackRunningRequiresEveryContainer(t *testing.T) {
	c := ObserveConfig{Runner: &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("false")}}, Bundle: observeBundle()}
	c.WithDefaults()

	if c.stackRunning(context.Background()) {
		t.Error("a stopped container must report the stack as not running")
	}
	if got, want := len(c.stackContainerNames()), 3; got != want {
		t.Errorf("stack has %d containers, want %d (db + backend + portal)", got, want)
	}
}

// Readiness is a live property too: answering /health now means no wait is needed, and
// silence means wait regardless of what a previous run recorded.
func TestWaitNocBackendChecksHealthNow(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()

	c := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle(), BackendURL: healthy.URL}
	c.WithDefaults()
	skip, err := stepByName(t, ObserveSteps(c), "wait-noc-backend").Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !skip {
		t.Error("a backend answering /health should not be waited for again")
	}

	down := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle(), BackendURL: "http://127.0.0.1:1"}
	down.WithDefaults()
	skip, err = stepByName(t, ObserveSteps(down), "wait-noc-backend").Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if skip {
		t.Error("an unreachable backend must still be waited for")
	}
}

// A fresh NOC database with a leftover state file must still get the agent key: the key
// lives in the database, the state file lives on the host, and when they disagree every
// agent push is rejected with 401 and the portal shows no components.
func TestProvisionKeyStepNeverSkips(t *testing.T) {
	c := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle()}
	c.WithDefaults()

	for _, st := range ObserveSteps(c) {
		if st.Name != "provision-noc-key" {
			continue
		}
		if st.Check == nil {
			t.Fatal("provision-noc-key has no Check, so persisted state alone decides whether it runs")
		}
		skip, err := st.Check(context.Background())
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if skip {
			t.Error("provision-noc-key reported itself satisfied; it must run on every apply")
		}
		return
	}
	t.Fatal("provision-noc-key step not found")
}
