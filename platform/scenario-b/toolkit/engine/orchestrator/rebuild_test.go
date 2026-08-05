package orchestrator

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// imageExists must report "missing" under --rebuild so the build gates run against the
// current source instead of trusting a tag that only encodes build args.
func TestForceImageRebuildMakesImagesLookMissing(t *testing.T) {
	t.Cleanup(func() { SetForceImageRebuild(false) })
	fake := &exec.FakeRunner{}

	if !imageExists(context.Background(), fake, "cbweb3b/noc-portal:abc") {
		t.Fatal("baseline: FakeRunner succeeds, so the image must look present")
	}

	SetForceImageRebuild(true)
	if imageExists(context.Background(), fake, "cbweb3b/noc-portal:abc") {
		t.Error("under --rebuild an existing tag must still report missing")
	}

	SetForceImageRebuild(false)
	if !imageExists(context.Background(), fake, "cbweb3b/noc-portal:abc") {
		t.Error("clearing the flag must restore normal detection")
	}
}

// A forced step runs even though its Check reports "already satisfied" — the case that
// made a code-only change deploy nothing: build gate satisfied by tag, container left
// untouched by a state-skipped compose up.
func TestForcedStepRunsDespiteCheckAndState(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if err := state.Set("start-noc-stack", StatusDone); err != nil {
		t.Fatalf("Set: %v", err)
	}

	built, started := 0, 0
	steps := []Step{
		{
			Name:  "build-noc-images",
			Check: func(context.Context) (bool, error) { return true, nil }, // "image already exists"
			Run:   func(context.Context) error { built++; return nil },
		},
		{
			Name: "start-noc-stack", // no Check: skipped from persisted state
			Deps: []string{"build-noc-images"},
			Run:  func(context.Context) error { started++; return nil },
		},
	}

	rep, err := New("observe", steps, state, false).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if built != 0 || started != 0 {
		t.Fatalf("baseline should skip both, got built=%d started=%d (%+v)", built, started, rep.Steps)
	}

	rep, err = New("observe", steps, state, false).
		Force("build-noc-images", "start-noc-stack").
		Run(context.Background())
	if err != nil {
		t.Fatalf("forced Run: %v", err)
	}
	if built != 1 || started != 1 {
		t.Errorf("forced run should execute both, got built=%d started=%d (%+v)", built, started, rep.Steps)
	}
	for _, s := range rep.Steps {
		if s.Status != StatusDone {
			t.Errorf("step %s = %s, want done", s.Name, s.Status)
		}
	}
}

// Forcing is opt-in per step: an unforced step keeps its normal skip behaviour.
func TestForceDoesNotLeakToOtherSteps(t *testing.T) {
	state, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	ran := map[string]int{}
	steps := []Step{
		{Name: "forced", Check: func(context.Context) (bool, error) { return true, nil },
			Run: func(context.Context) error { ran["forced"]++; return nil }},
		{Name: "untouched", Check: func(context.Context) (bool, error) { return true, nil },
			Run: func(context.Context) error { ran["untouched"]++; return nil }},
	}

	if _, err := New("observe", steps, state, false).Force("forced").Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ran["forced"] != 1 {
		t.Errorf("forced step ran %d times, want 1", ran["forced"])
	}
	if ran["untouched"] != 0 {
		t.Errorf("unforced step ran %d times, want 0", ran["untouched"])
	}
}
