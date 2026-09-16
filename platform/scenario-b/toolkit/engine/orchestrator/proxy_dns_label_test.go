// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"
)

// dnsLabelMax is the hard limit on a single DNS label (RFC 1035 §2.3.4). Docker's embedded
// resolver enforces it: a longer name is refused outright ("bad address"), so the caller
// never even gets a connection error to explain the failure.
const dnsLabelMax = 63

// TestProxyUpstreamsFitDNSLabel is the guard the Costa Rica outage needed.
//
// The proxy resolves every upstream over the entity network, so an upstream host longer
// than a DNS label is unreachable no matter how the routing is written. The CB container
// name repeats the entity role — "sc-b-cbweb3-central-bank-costa-rica-central-bank-
// governance-frontend" is 68 octets — and the three portals answered 502 while the
// api-gateway (60 octets) kept working, which read as a routing fault rather than a
// naming one. Chile, at exactly 63, was one character from the same failure.
//
// The entity names below are deliberately longer than any deployed one: the bound has to
// hold for the next country, not for the three tried so far.
func TestProxyUpstreamsFitDNSLabel(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal")

	entities := []string{
		"costa-rica",                       // the entity that broke
		"chile",                            // was at exactly 63 via the container name
		"dominican-republic",               // longer than any deployed spoke
		"saint-vincent-and-the-grenadines", // the longest ISO country name
	}

	for _, e := range entities {
		spoke := SpokeConfig{
			ContainerPrefix: "sc-b-cbweb3-central-bank-" + e,
			Entity:          "central-bank",
			NetPrefix:       "central-bank-" + e,
			RPCPort:         8845,
			FrontendHost:    "cb." + e + ".example",
			ProxyEnabled:    true,
		}
		assertUpstreamsResolvable(t, "found-spoke/"+e, spoke.ProxyRoutes())

		bank := JoinConfig{
			ContainerPrefix: "sc-b-cbweb3-commercial-bank-of-" + e,
			Entity:          "commercial-bank-of-" + e,
			NetPrefix:       "commercial-bank-of-" + e,
			RPCPort:         10545,
			FrontendHost:    "bank." + e + ".example",
			ProxyEnabled:    true,
		}
		assertUpstreamsResolvable(t, "join/"+e, bank.ProxyRoutes())
	}

	// The hub is checked here for the reason the CB and bank paths are: it was the last
	// route set still built from the container name, and it was also the one no guard
	// covered — the same shape as the original defect, where the audit reached exactly
	// as far as the author was already looking. Its deployed name is short, so the names
	// below are the ones that would break it.
	for _, h := range []string{
		"hub",
		"hub-lacnet",
		"hub-central-clearing-and-settlement-authority",
	} {
		hub := HubConfig{
			ContainerPrefix: "sc-b-cbweb3-" + h,
			NetPrefix:       h,
			RPCPort:         8545,
			FrontendHost:    h + ".example",
			ProxyEnabled:    true,
		}
		assertUpstreamsResolvable(t, "found-hub/"+h, hub.ProxyRoutes())
	}
}

// assertUpstreamsResolvable checks the host half of every upstream against the label limit.
func assertUpstreamsResolvable(t *testing.T, who string, routes []ProxyRoute) {
	t.Helper()
	if len(routes) == 0 {
		t.Fatalf("%s: no proxy routes to check", who)
	}
	for _, r := range routes {
		host, _, found := strings.Cut(r.Upstream, ":")
		if !found {
			t.Errorf("%s: upstream %q has no port; the proxy fragment needs host:port", who, r.Upstream)
			continue
		}
		// Only a single-label name is at risk; a dotted name is resolved label by label.
		if strings.Contains(host, ".") {
			continue
		}
		if len(host) > dnsLabelMax {
			t.Errorf("%s: upstream host for segment %q is %d octets, over the %d-octet DNS label limit — docker's resolver will refuse it and the route will answer 502:\n\t%s",
				who, r.Segment, len(host), dnsLabelMax, host)
		}
	}
}

// TestProxyUpstreamsAreEntityUnique guards the other half: the proxy attaches to EVERY
// entity network on its host, so two entities sharing an upstream alias would make
// resolution ambiguous and route one entity's portal to the other's container.
func TestProxyUpstreamsAreEntityUnique(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal")

	newCB := func(e string) SpokeConfig {
		return SpokeConfig{
			ContainerPrefix: "sc-b-cbweb3-central-bank-" + e,
			Entity:          "central-bank",
			NetPrefix:       "central-bank-" + e,
			RPCPort:         8845,
			FrontendHost:    "cb." + e + ".example",
			ProxyEnabled:    true,
		}
	}

	seen := map[string]string{}
	record := func(who string, routes []ProxyRoute) {
		for _, r := range routes {
			if prev, dup := seen[r.Upstream]; dup {
				t.Errorf("upstream %q is produced by both %s and %s; on a shared host the proxy cannot tell them apart", r.Upstream, prev, who)
			}
			seen[r.Upstream] = who
		}
	}

	for _, e := range []string{"costa-rica", "chile", "peru"} {
		record("central-bank-"+e, newCB(e).ProxyRoutes())
		record("bank-of-"+e, JoinConfig{NetPrefix: "bank-of-" + e}.ProxyRoutes())
	}

	// The hub shares a host with a spoke in the reference topology, so its upstreams have
	// to be distinct from every entity's too.
	record("hub", HubConfig{NetPrefix: "hub"}.ProxyRoutes())
}

// The observe stack is the fourth route set, added after PR #226 migrated the other three off
// container names. Its own budget is looser — the NOC prefix carries no entity role — but "looser"
// is not "checked", and being unchecked is how the CB path ended up 68 octets long.
func TestObserveProxyUpstreamsFitDNSLabel(t *testing.T) {
	// Built the way engine/apply builds it: the container prefix carries "sc-b-cbweb3-" and the
	// network prefix does not (apply.go:125-126). That 12-octet difference is what the alias
	// saves, and a test that set both to the same string would prove nothing.
	for _, prefix := range []string{
		"noc-brazil",
		"noc-dominican-republic",
		"noc-saint-vincent-and-the-grenadines",
	} {
		noc := ObserveConfig{ContainerPrefix: "sc-b-cbweb3-" + prefix, NetPrefix: prefix, ProxyEnabled: true}
		assertUpstreamsResolvable(t, "observe/"+prefix, noc.ProxyRoutes())
	}
}

// The upstreams must be the network ALIASES the template declares, not the container names.
// A container name here repeats the prefix and grows with it; that is the defect PR #226 fixed
// three times over and left standing here.
func TestObserveProxyUpstreamsAreAliasesNotContainerNames(t *testing.T) {
	const prefix = "noc-brazil"
	const containerPrefix = "sc-b-cbweb3-" + prefix
	noc := ObserveConfig{ContainerPrefix: containerPrefix, NetPrefix: prefix, ProxyEnabled: true}

	for _, r := range noc.ProxyRoutes() {
		host, _, _ := strings.Cut(r.Upstream, ":")
		for _, containerName := range []string{containerPrefix + "-noc-portal", containerPrefix + "-noc-backend"} {
			if host == containerName {
				t.Errorf("segment %q dials the container name %q; use the network alias the "+
					"template declares, so the name does not grow with the prefix", r.Segment, host)
			}
		}
	}
}
