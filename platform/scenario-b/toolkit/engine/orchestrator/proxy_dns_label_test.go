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
	for _, e := range []string{"costa-rica", "chile", "peru"} {
		for _, r := range newCB(e).ProxyRoutes() {
			if prev, dup := seen[r.Upstream]; dup {
				t.Errorf("upstream %q is produced by both %s and %s; on a shared host the proxy cannot tell them apart", r.Upstream, prev, e)
			}
			seen[r.Upstream] = e
		}
	}
}
