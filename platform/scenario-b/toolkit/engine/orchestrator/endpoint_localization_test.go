// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// containerReachable maps loopback hosts to host.docker.internal (single-host)
// but leaves routable hosts untouched (multi-host) — the core of the endpoint
// localization fix.
func TestContainerReachable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:8845", "http://host.docker.internal:8845"},
		{"http://127.0.0.1:7000", "http://host.docker.internal:7000"},
		{"http://13.59.73.155:8845", "http://13.59.73.155:8845"}, // routable → unchanged
		{"http://host.docker.internal:8845", "http://host.docker.internal:8845"},
		{"ws://10.0.0.5:8846", "ws://10.0.0.5:8846"},
		{"", ""},
	}
	for _, c := range cases {
		if got := containerReachable(c.in); got != c.want {
			t.Errorf("containerReachable(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// HostReachable is the opposite of containerReachable: the host-run toolkit maps the
// host.docker.internal container sentinel to localhost (single-host register-cb /
// register-currency posting to the hub gateway) but leaves a routable multi-VM hub
// unchanged.
func TestHostReachable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://host.docker.internal:41845", "http://localhost:41845"},
		{"http://host.docker.internal:33845", "http://localhost:33845"},
		{"http://13.59.73.155:41845", "http://13.59.73.155:41845"},                           // routable → unchanged
		{"http://localhost:41845", "http://localhost:41845"},                                 // already host-reachable
		{"http://cb-brazil.cbweb3.l-net.io:41845", "http://cb-brazil.cbweb3.l-net.io:41845"}, // routable DNS → unchanged
		{"", ""},
	}
	for _, c := range cases {
		if got := HostReachable(c.in); got != c.want {
			t.Errorf("HostReachable(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// relayCactiURL uses the relay's own endpoint when set (container-reachable),
// else the single-host default.
func TestRelayCactiURL(t *testing.T) {
	if got := relayCactiURL("http://13.59.73.155:7000"); got != "http://13.59.73.155:7000" {
		t.Errorf("routable relay endpoint = %q", got)
	}
	if got := relayCactiURL("http://localhost:7000"); got != "http://host.docker.internal:7000" {
		t.Errorf("loopback relay endpoint = %q", got)
	}
	if got := relayCactiURL(""); got != "http://host.docker.internal:7000" {
		t.Errorf("empty relay endpoint fallback = %q", got)
	}
}

// found-spoke ComposeEnv must publish the routable hub RPC (HUB_BESU_RPC_URL) and
// the relay's own endpoint (CACTI_API_URL) so a cross-VM deploy reaches them.
func TestFoundSpokeComposeEnvRoutableEndpoints(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.HubRPC = "http://13.59.73.155:8845"
	cfg.RelayEndpoint = "http://13.59.73.155:7000"
	env := cfg.ComposeEnv()

	assertEnv(t, env, "HUB_BESU_RPC_URL", "http://13.59.73.155:8845")
	assertEnv(t, env, "CACTI_API_URL", "http://13.59.73.155:7000")
}

// Single-host defaults: a loopback hub RPC maps to host.docker.internal and an
// empty relay endpoint falls back to the local relay port.
func TestFoundSpokeComposeEnvSingleHostFallback(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.HubRPC = "http://localhost:8845"
	cfg.RelayEndpoint = ""
	env := cfg.ComposeEnv()

	assertEnv(t, env, "HUB_BESU_RPC_URL", "http://host.docker.internal:8845")
	assertEnv(t, env, "CACTI_API_URL", "http://host.docker.internal:7000")
}

func assertEnv(t *testing.T, env []string, key, want string) {
	t.Helper()
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			if got := strings.TrimPrefix(e, prefix); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Errorf("%s not found in ComposeEnv", key)
}
