// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"testing"
)

func loadState(t *testing.T) *State {
	t.Helper()
	s, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// SC-005-like: steps run in dependency order.
func TestTopologicalOrder(t *testing.T) {
	var order []string
	mk := func(name string, deps ...string) Step {
		return Step{Name: name, Deps: deps, Run: func(context.Context) error {
			order = append(order, name)
			return nil
		}}
	}
	steps := []Step{mk("c", "b"), mk("b", "a"), mk("a")}
	_, err := New("t", steps, loadState(t), false).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("order = %v", order)
	}
}

// SC-001: Check satisfied → skipped, Run not called.
func TestCheckSkipsRun(t *testing.T) {
	ran := false
	steps := []Step{{
		Name:  "s",
		Check: func(context.Context) (bool, error) { return true, nil },
		Run:   func(context.Context) error { ran = true; return nil },
	}}
	rep, err := New("t", steps, loadState(t), false).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Fatal("Run should be skipped when Check is true")
	}
	if rep.Steps[0].Status != StatusSkipped {
		t.Fatalf("status = %s", rep.Steps[0].Status)
	}
}

// SC-001: second run is idempotent (state-based) for a step without Check.
func TestIdempotentAcrossRuns(t *testing.T) {
	st := loadState(t)
	count := 0
	steps := []Step{{Name: "s", Run: func(context.Context) error { count++; return nil }}}
	if _, err := New("t", steps, st, false).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// second run reuses the same state → skipped
	if _, err := New("t", steps, st, false).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("Run executed %d times, want 1 (idempotent)", count)
	}
}

// SC-003: failure stops and persists; resume skips done, runs the failed one.
func TestResumeAfterFailure(t *testing.T) {
	st := loadState(t)
	aRuns, bRuns := 0, 0
	fail := true
	steps := []Step{
		{Name: "a", Run: func(context.Context) error { aRuns++; return nil }},
		{Name: "b", Deps: []string{"a"}, Run: func(context.Context) error {
			bRuns++
			if fail {
				return errors.New("boom")
			}
			return nil
		}},
	}
	if _, err := New("t", steps, st, false).Run(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	if st.Get("a") != StatusDone || st.Get("b") != StatusFailed {
		t.Fatalf("state a=%s b=%s", st.Get("a"), st.Get("b"))
	}
	fail = false
	if _, err := New("t", steps, st, false).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if aRuns != 1 {
		t.Fatalf("a ran %d times, want 1 (not re-run)", aRuns)
	}
	if bRuns != 2 {
		t.Fatalf("b ran %d times, want 2 (retried)", bRuns)
	}
}

// SC-004: dry-run plans without executing Run.
func TestDryRunPlansWithoutRunning(t *testing.T) {
	ran := false
	steps := []Step{{Name: "s", Run: func(context.Context) error { ran = true; return nil }}}
	rep, err := New("t", steps, loadState(t), true).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Fatal("dry-run must not execute Run")
	}
	if rep.Steps[0].Status != StatusPlanned {
		t.Fatalf("status = %s", rep.Steps[0].Status)
	}
}

// SC-002: a second concurrent run is refused (lock held).
func TestLockRefusesConcurrent(t *testing.T) {
	dir := t.TempDir()
	l1, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Release()
	if _, err := AcquireLock(dir); err == nil {
		t.Fatal("second AcquireLock should be refused while held")
	}
	// after release, a new lock succeeds
	_ = l1.Release()
	l2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	_ = l2.Release()
}

func TestUnknownDepAndCycle(t *testing.T) {
	if _, err := topoSort([]Step{{Name: "a", Deps: []string{"missing"}}}); err == nil {
		t.Fatal("expected unknown dependency error")
	}
	if _, err := topoSort([]Step{{Name: "a", Deps: []string{"b"}}, {Name: "b", Deps: []string{"a"}}}); err == nil {
		t.Fatal("expected cycle error")
	}
}
