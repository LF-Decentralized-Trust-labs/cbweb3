// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func TestRelayAdvertisedHost_DefaultsToDockerHostAlias(t *testing.T) {
	// No relay block → default.
	if got := relayAdvertisedHost(&manifest.Manifest{}); got != dockerHostAlias {
		t.Errorf("no relay block: host = %q; want %q", got, dockerHostAlias)
	}

	// Relay block present but advertisedHost empty → default.
	m := &manifest.Manifest{}
	m.Spec.Relay = &manifest.Relay{Endpoint: "http://localhost:4000"}
	if got := relayAdvertisedHost(m); got != dockerHostAlias {
		t.Errorf("empty advertisedHost: host = %q; want %q", got, dockerHostAlias)
	}
}

func TestRelayAdvertisedHost_OverrideFromManifest(t *testing.T) {
	m := &manifest.Manifest{}
	m.Spec.Relay = &manifest.Relay{Endpoint: "http://relay.example:4000", AdvertisedHost: "10.0.0.7"}
	if got := relayAdvertisedHost(m); got != "10.0.0.7" {
		t.Errorf("host = %q; want 10.0.0.7", got)
	}
}

func TestRewriteHost(t *testing.T) {
	cases := []struct {
		name, url, host, want string
	}{
		{"localhost replaced", "http://localhost:8645", "10.0.0.7", "http://10.0.0.7:8645"},
		{"docker host alias", "http://localhost:8645", dockerHostAlias, "http://host.docker.internal:8645"},
		{"only first occurrence", "http://localhost/localhost", "h", "http://h/localhost"},
		{"no localhost is unchanged", "http://besu-rpc:8645", "10.0.0.7", "http://besu-rpc:8645"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rewriteHost(c.url, c.host); got != c.want {
				t.Errorf("rewriteHost(%q, %q) = %q; want %q", c.url, c.host, got, c.want)
			}
		})
	}
}
