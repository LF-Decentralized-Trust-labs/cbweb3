// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"crypto/x509"
	"testing"
)

// paladinDialHost / addTransportSAN implement Option B of the cross-VM Paladin
// transport fix: a node advertises its ROUTABLE host as the on-chain gRPC
// endpoint (so peers on other VMs can dial it without a per-peer extra_hosts
// entry), and carries that host in the transport cert SAN so the dns:/// mutual
// TLS handshake still validates. Both are gated on isRoutableHost so single-host
// deployments keep the container-name behavior unchanged.

func TestPaladinDialHost_RoutablePrefersAdvertisedHost(t *testing.T) {
	cases := []struct {
		name           string
		advertisedHost string
		containerHost  string
		want           string
	}{
		{"routable ip", "10.10.0.22", "paladin-spoke-brazil-cb1", "10.10.0.22"},
		{"routable dns", "cb1-brazil.cbweb3.lnet.io", "paladin-spoke-brazil-cb1", "cb1-brazil.cbweb3.lnet.io"},
		{"routable ip padded", "  10.10.0.21 ", "paladin-spoke-brazil-cb", "10.10.0.21"},
		{"empty falls back to container", "", "paladin-spoke-brazil-cb", "paladin-spoke-brazil-cb"},
		{"localhost falls back to container", "localhost", "paladin-spoke-brazil-cb", "paladin-spoke-brazil-cb"},
		{"docker-internal falls back to container", "host.docker.internal", "paladin-spoke-brazil-cb", "paladin-spoke-brazil-cb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := paladinDialHost(tc.advertisedHost, tc.containerHost); got != tc.want {
				t.Errorf("paladinDialHost(%q, %q) = %q; want %q", tc.advertisedHost, tc.containerHost, got, tc.want)
			}
		})
	}
}

func TestAddTransportSAN_IPGoesToIPAddresses(t *testing.T) {
	tmpl := &x509.Certificate{DNSNames: []string{"spoke-brazil-cb1", "paladin-spoke-brazil-cb1", "localhost"}}
	addTransportSAN(tmpl, "10.10.0.22")
	if len(tmpl.IPAddresses) != 1 || tmpl.IPAddresses[0].String() != "10.10.0.22" {
		t.Fatalf("IPAddresses = %v; want [10.10.0.22]", tmpl.IPAddresses)
	}
	if len(tmpl.DNSNames) != 3 {
		t.Errorf("DNSNames should be unchanged for an IP SAN; got %v", tmpl.DNSNames)
	}
}

func TestAddTransportSAN_DNSGoesToDNSNames(t *testing.T) {
	tmpl := &x509.Certificate{DNSNames: []string{"spoke-brazil-cb1"}}
	addTransportSAN(tmpl, "cb1-brazil.cbweb3.lnet.io")
	if len(tmpl.IPAddresses) != 0 {
		t.Errorf("IPAddresses should be empty for a DNS SAN; got %v", tmpl.IPAddresses)
	}
	found := false
	for _, d := range tmpl.DNSNames {
		if d == "cb1-brazil.cbweb3.lnet.io" {
			found = true
		}
	}
	if !found {
		t.Errorf("DNSNames = %v; want it to include cb1-brazil.cbweb3.lnet.io", tmpl.DNSNames)
	}
}

func TestAddTransportSAN_EmptyAndDuplicateAreNoOps(t *testing.T) {
	tmpl := &x509.Certificate{DNSNames: []string{"host-a"}, IPAddresses: nil}
	addTransportSAN(tmpl, "") // empty: no-op
	if len(tmpl.DNSNames) != 1 || len(tmpl.IPAddresses) != 0 {
		t.Fatalf("empty host must be a no-op; got DNS=%v IP=%v", tmpl.DNSNames, tmpl.IPAddresses)
	}
	addTransportSAN(tmpl, "host-a") // duplicate DNS: no-op
	if len(tmpl.DNSNames) != 1 {
		t.Errorf("duplicate DNS SAN must not be appended; got %v", tmpl.DNSNames)
	}
	addTransportSAN(tmpl, "10.0.0.1")
	addTransportSAN(tmpl, "10.0.0.1") // duplicate IP: no-op
	if len(tmpl.IPAddresses) != 1 {
		t.Errorf("duplicate IP SAN must not be appended; got %v", tmpl.IPAddresses)
	}
}
