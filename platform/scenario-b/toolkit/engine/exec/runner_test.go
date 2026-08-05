// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFakeRunnerRecordsCalls(t *testing.T) {
	f := &FakeRunner{Outputs: map[string][]byte{"forge": []byte("ok")}}
	out, err := f.Run(context.Background(), "forge", "build")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok" {
		t.Fatalf("output = %q", out)
	}
	if len(f.Calls) != 1 || f.Calls[0].Name != "forge" || f.Calls[0].Args[0] != "build" {
		t.Fatalf("calls = %+v", f.Calls)
	}
}

func TestFakeRunnerProgrammedError(t *testing.T) {
	f := &FakeRunner{Errs: map[string]error{"docker": errors.New("boom")}}
	if _, err := f.Run(context.Background(), "docker", "compose", "up"); err == nil {
		t.Fatal("expected programmed error")
	}
}

func TestDryRunnerDoesNotExecute(t *testing.T) {
	d := &DryRunner{}
	// A command that would fail if actually executed; DryRunner must not run it.
	out, err := d.Run(context.Background(), "definitely-not-a-real-binary-xyz", "--do-harm")
	if err != nil || out != nil {
		t.Fatalf("dry runner should not execute: out=%v err=%v", out, err)
	}
	if len(d.Planned) != 1 || d.Planned[0].Name != "definitely-not-a-real-binary-xyz" {
		t.Fatalf("planned = %+v", d.Planned)
	}
}

func TestRealRunnerExecutes(t *testing.T) {
	out, err := NewReal("", nil).Run(context.Background(), "echo", "hello-tkb6")
	if err != nil {
		t.Fatalf("real runner echo: %v", err)
	}
	if !strings.Contains(string(out), "hello-tkb6") {
		t.Fatalf("output = %q", out)
	}
}
