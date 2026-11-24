// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The Besu compose templates read BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS. These tests pin that the
// toolkit validates it and writes the resolved value into the env it renders, so the ceiling
// in force is one the toolkit checked, and the documented tuning reaches Besu through apply
// rather than only through the operator's shell.

func TestParseBesuWSMaxActiveConnections(t *testing.T) {
	for raw, want := range map[string]string{"": "256", "1": "1", "512": "512", "2147483647": "2147483647"} {
		got, err := parseBesuWSMaxActiveConnections(raw)
		if err != nil || got != want {
			t.Errorf("parse(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	// 0 starts a node that refuses every WebSocket client; the others make Besu fail option
	// parsing inside a restart loop. 2147483648 is one above the Java int Besu parses.
	for _, bad := range []string{"0", "-5", "abc", "25O", "12x", "1 2", " 256", "08", "2147483648", "99999999999"} {
		if got, err := parseBesuWSMaxActiveConnections(bad); err == nil {
			t.Errorf("parse(%q) accepted %q", bad, got)
		} else if !strings.Contains(err.Error(), BesuWSMaxActiveConnectionsVar) {
			t.Errorf("parse(%q) error does not name %s: %v", bad, BesuWSMaxActiveConnectionsVar, err)
		}
	}
}

func TestHubComposeEnvWritesTheWebSocketLimit(t *testing.T) {
	for _, tc := range []struct{ env, want string }{{"", "256"}, {"512", "512"}} {
		t.Setenv(BesuWSMaxActiveConnectionsVar, tc.env)
		cfg := testHubConfig(t, &exec.FakeRunner{})
		cfg.WithDefaults()
		if err := cfg.renderHubComposeEnv(); err != nil {
			t.Fatalf("render with %q: %v", tc.env, err)
		}
		if got := addrs.ReadAddr(cfg.HubEnvFile, BesuWSMaxActiveConnectionsVar); got != tc.want {
			t.Errorf("hub env file has %s=%q with operator value %q, want %q", BesuWSMaxActiveConnectionsVar, got, tc.env, tc.want)
		}
	}
}

func TestHubComposeEnvRejectsAnInvalidWebSocketLimit(t *testing.T) {
	t.Setenv(BesuWSMaxActiveConnectionsVar, "0")
	cfg := testHubConfig(t, &exec.FakeRunner{})
	cfg.WithDefaults()
	err := cfg.renderHubComposeEnv()
	if err == nil || !strings.Contains(err.Error(), BesuWSMaxActiveConnectionsVar) {
		t.Fatalf("render accepted 0 or did not name the variable: %v", err)
	}
}

func TestSpokeAndJoinComposeEnvCarryTheWebSocketLimit(t *testing.T) {
	for _, tc := range []struct{ env, want string }{{"", "256"}, {"512", "512"}} {
		t.Setenv(BesuWSMaxActiveConnectionsVar, tc.env)
		if got := envMap(testSpokeCfg(t, &exec.FakeRunner{}).ComposeEnv())[BesuWSMaxActiveConnectionsVar]; got != tc.want {
			t.Errorf("found-spoke ComposeEnv %s=%q with operator value %q, want %q", BesuWSMaxActiveConnectionsVar, got, tc.env, tc.want)
		}
		if got := envMap(testJoinCfg(t, &exec.FakeRunner{}).ComposeEnv())[BesuWSMaxActiveConnectionsVar]; got != tc.want {
			t.Errorf("join ComposeEnv %s=%q with operator value %q, want %q", BesuWSMaxActiveConnectionsVar, got, tc.env, tc.want)
		}
	}
}

// An invalid value must stop the start-besu step by name, before compose starts Besu.
func TestStartBesuStepsRejectAnInvalidWebSocketLimit(t *testing.T) {
	t.Setenv(BesuWSMaxActiveConnectionsVar, "25O")

	spokeFake := &exec.FakeRunner{}
	joinFake := &exec.FakeRunner{}
	for name, step := range map[string]Step{
		"start-besu-spoke": findStep(FoundSpokeSteps(testSpokeCfg(t, spokeFake)), "start-besu-spoke"),
		"start-besu-join":  findStep(JoinSteps(testJoinCfg(t, joinFake)), "start-besu-join"),
	} {
		if step.Run == nil {
			t.Fatalf("step %s not found", name)
		}
		err := step.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), BesuWSMaxActiveConnectionsVar) {
			t.Errorf("%s accepted 25O or did not name the variable: %v", name, err)
		}
	}
	if len(spokeFake.Calls)+len(joinFake.Calls) != 0 {
		t.Errorf("compose ran despite the invalid value: %v %v", spokeFake.Calls, joinFake.Calls)
	}
}
