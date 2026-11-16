// SPDX-License-Identifier: Apache-2.0

// Every Besu entrypoint must pass an explicit, parameterised WebSocket connection
// ceiling, and every compose file that runs it must forward the parameter.
//
// SUP-152: the Chile central bank's Besu ran on the implicit default of
// --rpc-ws-max-active-connections=80, set nowhere in these templates. Leaked
// connections filled all 80 slots and every new connection was refused with
// "unexpected EOF". The local Paladin node could not open its block listener, never
// finished starting, and the spoke settled nothing for fifteen days.
//
// The entrypoint is EXECUTED here against a stub besu binary, not grepped. What
// matters is the argument vector Besu receives; a flag that is present in the
// script text but lost to quoting or word splitting would pass a grep and still
// leave the node on the default.
package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	wsLimitVar     = "BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS"
	wsLimitFlag    = "--rpc-ws-max-active-connections"
	wsLimitDefault = "256"
	besuBinaryPath = "/opt/besu/bin/besu"
)

// besuTemplateDirs names, BY NAME, every template whose entrypoint starts Besu.
// A discovered list would let a template the glob misses go silently exempt.
var besuTemplateDirs = []string{"central-bank", "commercial-bank"}

// runEntrypoint copies entry.sh into a temp dir with the hardcoded Besu path pointed
// at a stub that prints one argument per line, runs it with env, and returns the
// argument vector Besu would have received.
func runEntrypoint(t *testing.T, tmplDir string, env map[string]string) ([]string, string, error) {
	t.Helper()
	src := filepath.Join(templatesDir, tmplDir, "scripts", "entry.sh")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if !strings.Contains(string(raw), besuBinaryPath) {
		t.Fatalf("%s no longer execs %s; update this test's stub substitution", src, besuBinaryPath)
	}

	dir := t.TempDir()
	stub := filepath.Join(dir, "besu-stub")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	script := filepath.Join(dir, "entry.sh")
	patched := strings.ReplaceAll(string(raw), besuBinaryPath, stub)
	if err := os.WriteFile(script, []byte(patched), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command("/bin/sh", script)
	// A minimal environment: found mode (no bootnode), DOCKER NAT profile. Nothing
	// inherited from the developer's shell can leak a value into the run.
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	var args []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "--") {
			args = append(args, line)
		}
	}
	return args, string(out), err
}

func wsLimitArgs(args []string) []string {
	var found []string
	for _, a := range args {
		if strings.HasPrefix(a, wsLimitFlag) {
			found = append(found, a)
		}
	}
	return found
}

func TestBesuEntrypoint_WebSocketLimitDefaultsExplicitly(t *testing.T) {
	for _, tmpl := range besuTemplateDirs {
		t.Run(tmpl, func(t *testing.T) {
			args, out, err := runEntrypoint(t, tmpl, nil)
			if err != nil {
				t.Fatalf("entry.sh failed with no override: %v\n%s", err, out)
			}
			got := wsLimitArgs(args)
			want := wsLimitFlag + "=" + wsLimitDefault
			if len(got) != 1 || got[0] != want {
				t.Fatalf("Besu received %v, want exactly [%s]: without it the node runs on the "+
					"implicit default of 80", got, want)
			}
		})
	}
}

func TestBesuEntrypoint_WebSocketLimitIsOverridable(t *testing.T) {
	for _, tmpl := range besuTemplateDirs {
		t.Run(tmpl, func(t *testing.T) {
			args, out, err := runEntrypoint(t, tmpl, map[string]string{wsLimitVar: "512"})
			if err != nil {
				t.Fatalf("entry.sh failed with %s=512: %v\n%s", wsLimitVar, err, out)
			}
			got := wsLimitArgs(args)
			if len(got) != 1 || got[0] != wsLimitFlag+"=512" {
				t.Fatalf("Besu received %v, want exactly [%s=512]", got, wsLimitFlag)
			}
		})
	}
}

// An empty value is what compose hands over when the operator's env file does not
// set the variable (the compose files forward it as ${VAR:-}), so it must fall
// back to the default rather than reach Besu as an empty flag.
func TestBesuEntrypoint_WebSocketLimitEmptyFallsBackToDefault(t *testing.T) {
	for _, tmpl := range besuTemplateDirs {
		t.Run(tmpl, func(t *testing.T) {
			args, out, err := runEntrypoint(t, tmpl, map[string]string{wsLimitVar: ""})
			if err != nil {
				t.Fatalf("entry.sh failed with %s empty: %v\n%s", wsLimitVar, err, out)
			}
			got := wsLimitArgs(args)
			if len(got) != 1 || got[0] != wsLimitFlag+"="+wsLimitDefault {
				t.Fatalf("Besu received %v, want exactly [%s=%s]", got, wsLimitFlag, wsLimitDefault)
			}
		})
	}
}

// A value that is not a positive integer must stop the container with a message
// naming the variable. Passing it through would make Besu fail with a generic
// option-parsing error and a restart loop, or, for 0, start a node that refuses
// every WebSocket client.
func TestBesuEntrypoint_WebSocketLimitRejectsInvalidValues(t *testing.T) {
	for _, tmpl := range besuTemplateDirs {
		for _, bad := range []string{"0", "-5", "abc", "12x", "1 2", "08"} {
			t.Run(tmpl+"/"+bad, func(t *testing.T) {
				args, out, err := runEntrypoint(t, tmpl, map[string]string{wsLimitVar: bad})
				if err == nil {
					t.Fatalf("entry.sh accepted %s=%q and started Besu with %v", wsLimitVar, bad, args)
				}
				if !strings.Contains(out, wsLimitVar) {
					t.Fatalf("entry.sh rejected %q but the message does not name %s:\n%s", bad, wsLimitVar, out)
				}
				if len(args) != 0 {
					t.Fatalf("entry.sh rejected %q but Besu was still started with %v", bad, args)
				}
			})
		}
	}
}

type besuComposeService struct {
	Environment map[string]string `yaml:"environment"`
	Entrypoint  []string          `yaml:"entrypoint"`
}

type besuComposeFile struct {
	Services map[string]besuComposeService `yaml:"services"`
}

// The entrypoint only sees what compose forwards. Without this, setting the variable
// in an env file would have no effect and the node would silently keep the default.
func TestBesuCompose_ForwardsWebSocketLimit(t *testing.T) {
	for _, tmpl := range besuTemplateDirs {
		t.Run(tmpl, func(t *testing.T) {
			path := filepath.Join(templatesDir, tmpl, "docker-compose.yaml")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var cf besuComposeFile
			if err := yaml.Unmarshal(raw, &cf); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			checked := 0
			for name, svc := range cf.Services {
				if strings.Join(svc.Entrypoint, " ") != "/bin/sh /entry.sh" {
					continue
				}
				checked++
				want := "${" + wsLimitVar + ":-}"
				if got, ok := svc.Environment[wsLimitVar]; !ok || got != want {
					t.Errorf("service %q in %s forwards %s=%q, want %q", name, path, wsLimitVar, got, want)
				}
			}
			if checked == 0 {
				t.Fatalf("no service in %s runs /entry.sh; this test would vacuously pass", path)
			}
		})
	}
}
