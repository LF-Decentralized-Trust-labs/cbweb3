// Package exec is the injectable boundary for external side effects of the
// toolkit's steps (docker compose, forge, Keycloak). A real runner shells out
// via os/exec; a fake runner records invocations for tests; a dry runner
// records the plan without executing anything (used by `apply --dry-run`).
package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// CommandRunner runs an external command and returns its combined output.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Call records one command invocation (fake/dry runners).
type Call struct {
	Name string
	Args []string
}

// realRunner shells out via os/exec.
type realRunner struct {
	dir string
	env []string
}

// NewReal returns a runner that executes commands in dir (empty = cwd) with the
// given extra env vars appended to os.Environ().
func NewReal(dir string, env []string) CommandRunner {
	return &realRunner{dir: dir, env: env}
}

func (r *realRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.dir != "" {
		cmd.Dir = r.dir
	}
	if len(r.env) > 0 {
		cmd.Env = append(os.Environ(), r.env...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

// FakeRunner records calls and returns programmed outputs/errors keyed by
// command name. Used in unit tests — never executes anything.
type FakeRunner struct {
	Calls   []Call
	Outputs map[string][]byte
	Errs    map[string]error
}

func (f *FakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args})
	if f.Errs != nil {
		if err := f.Errs[name]; err != nil {
			return nil, err
		}
	}
	if f.Outputs != nil {
		return f.Outputs[name], nil
	}
	return nil, nil
}

// DryRunner records the planned calls without executing them (`--dry-run`).
type DryRunner struct {
	Planned []Call
}

func (d *DryRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	d.Planned = append(d.Planned, Call{Name: name, Args: args})
	return nil, nil
}
