// SPDX-License-Identifier: Apache-2.0

// Package exec is the injectable boundary for external side effects of the
// toolkit's steps (docker compose, forge, Keycloak). A real runner shells out
// via os/exec; a fake runner records invocations for tests; a dry runner
// records the plan without executing anything (used by `apply --dry-run`).
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner runs an external command and returns its combined output.
//
// RunWithEnv is the same, plus environment ("NAME=value") for that one call. It exists because a
// secret must be able to reach a child process WITHOUT passing through an argument: an argument
// vector is world-readable through `ps`, an environment is not. The Keycloak steps use it to set
// operator passwords; see step_found_spoke.go.
//
// It is on the interface rather than a separate optional one on purpose. A type assertion with a
// fallback would mean a runner that forgot to implement it silently took the argv path — which is
// the defect, arriving quietly.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	RunWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error)
}

// Call records one command invocation (fake/dry runners).
//
// Env is what RunWithEnv was given, so a test can assert both halves: that the value is in the
// environment and that it is NOT in Args.
type Call struct {
	Name string
	Args []string
	Env  []string
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
	return r.RunWithEnv(ctx, nil, name, args...)
}

func (r *realRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.dir != "" {
		cmd.Dir = r.dir
	}
	if len(r.env) > 0 || len(env) > 0 {
		cmd.Env = append(append(os.Environ(), r.env...), env...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		// Surface the command output in the error (Principle VI: no silent
		// swallowing) so a failed step's report explains WHY, not just "exit 1".
		trimmed := strings.TrimSpace(out.String())
		if trimmed != "" {
			return out.Bytes(), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, trimmed)
		}
		return out.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out.Bytes(), nil
}

// FakeRunner records calls and returns programmed outputs/errors keyed by
// command name. Used in unit tests — never executes anything.
type FakeRunner struct {
	Calls   []Call
	Outputs map[string][]byte
	Errs    map[string]error
}

func (f *FakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f.RunWithEnv(ctx, nil, name, args...)
}

func (f *FakeRunner) RunWithEnv(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args, Env: env})
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

func (d *DryRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return d.RunWithEnv(ctx, nil, name, args...)
}

// RunWithEnv records the environment alongside the call. A dry run prints its plan, so a secret
// recorded here would be printed — callers must keep passing values through env and names through
// args, which is what the exposure guards assert.
func (d *DryRunner) RunWithEnv(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
	d.Planned = append(d.Planned, Call{Name: name, Args: args, Env: env})
	return nil, nil
}
