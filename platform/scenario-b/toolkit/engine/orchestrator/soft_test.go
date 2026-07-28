package orchestrator

import (
	"context"
	"errors"
	"testing"
)

// A Soft step that fails is soft-failed and does NOT interrupt the run.
func TestSoftStepDoesNotInterrupt(t *testing.T) {
	ran := false
	steps := []Step{
		{Name: "soft", Soft: true, Run: func(context.Context) error { return errors.New("noc down") }},
		{Name: "after", Deps: []string{"soft"}, Run: func(context.Context) error { ran = true; return nil }},
	}
	rep, err := New("t", steps, loadState(t), false).Run(context.Background())
	if err != nil {
		t.Fatalf("soft failure must not abort the run: %v", err)
	}
	if !ran {
		t.Fatal("step after a soft-failed step should still run")
	}
	if rep.Steps[0].Status != StatusSoftFailed {
		t.Fatalf("soft step status = %s, want soft-failed", rep.Steps[0].Status)
	}
}

// A non-soft step that fails still interrupts (unchanged behavior).
func TestNonSoftStepInterrupts(t *testing.T) {
	after := false
	steps := []Step{
		{Name: "hard", Run: func(context.Context) error { return errors.New("boom") }},
		{Name: "after", Deps: []string{"hard"}, Run: func(context.Context) error { after = true; return nil }},
	}
	_, err := New("t", steps, loadState(t), false).Run(context.Background())
	if err == nil {
		t.Fatal("non-soft failure must abort")
	}
	if after {
		t.Fatal("step after a hard failure must not run")
	}
}
