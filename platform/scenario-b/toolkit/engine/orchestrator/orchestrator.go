package orchestrator

import (
	"context"
	"fmt"
)

// Orchestrator runs steps in dependency order with Check→skip / Run→persist.
type Orchestrator struct {
	mode   string
	steps  []Step
	state  *State
	dryRun bool
}

func New(mode string, steps []Step, state *State, dryRun bool) *Orchestrator {
	return &Orchestrator{mode: mode, steps: steps, state: state, dryRun: dryRun}
}

// Run executes the steps in topological order. Returns a Report always; on a
// step failure or context cancellation returns the partial Report + error.
func (o *Orchestrator) Run(ctx context.Context) (Report, error) {
	ordered, err := topoSort(o.steps)
	if err != nil {
		return Report{Mode: o.mode}, err
	}
	rep := Report{Mode: o.mode}
	for _, st := range ordered {
		if err := ctx.Err(); err != nil {
			return rep, err // partial report on interruption
		}
		res := StepResult{Name: st.Name}

		// Idempotency: prefer the live Check; fall back to persisted state.
		skip := false
		if st.Check != nil {
			ok, cerr := st.Check(ctx)
			if cerr != nil {
				res.Status, res.Detail = StatusFailed, cerr.Error()
				rep.Steps = append(rep.Steps, res)
				_ = o.state.Set(st.Name, StatusFailed)
				return rep, fmt.Errorf("step %q check failed: %w", st.Name, cerr)
			}
			skip = ok
		} else if o.state.Get(st.Name) == StatusDone {
			skip = true
		}
		if skip {
			res.Status = StatusSkipped
			rep.Steps = append(rep.Steps, res)
			_ = o.state.Set(st.Name, StatusSkipped)
			continue
		}

		if o.dryRun {
			res.Status = StatusPlanned
			rep.Steps = append(rep.Steps, res)
			continue
		}

		if rerr := st.Run(ctx); rerr != nil {
			res.Status, res.Detail = StatusFailed, rerr.Error()
			rep.Steps = append(rep.Steps, res)
			_ = o.state.Set(st.Name, StatusFailed)
			return rep, fmt.Errorf("step %q failed: %w", st.Name, rerr)
		}
		res.Status = StatusDone
		rep.Steps = append(rep.Steps, res)
		_ = o.state.Set(st.Name, StatusDone)
	}
	return rep, nil
}

// topoSort returns the steps in dependency order (stable: insertion order among
// independents). Errors on unknown dependency or cycle.
func topoSort(steps []Step) ([]Step, error) {
	byName := make(map[string]Step, len(steps))
	for _, s := range steps {
		byName[s.Name] = s
	}
	const (
		unseen = iota
		visiting
		done
	)
	mark := make(map[string]int, len(steps))
	var order []Step
	var visit func(name string) error
	visit = func(name string) error {
		switch mark[name] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("dependency cycle at %q", name)
		}
		s, ok := byName[name]
		if !ok {
			return fmt.Errorf("unknown dependency %q", name)
		}
		mark[name] = visiting
		for _, d := range s.Deps {
			if err := visit(d); err != nil {
				return err
			}
		}
		mark[name] = done
		order = append(order, s)
		return nil
	}
	for _, s := range steps {
		if err := visit(s.Name); err != nil {
			return nil, err
		}
	}
	return order, nil
}
