// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"
)

// Ported from Scenario B (engine/orchestrator/proxy_dns_label_test.go), closing the DNS
// row of docs/guard-parity.md. The port is deliberately not line-for-line, and the reason
// is worth stating because it changes what the guard is for.
//
// B's version asserts the bound for entity names nobody has deployed — the longest ISO
// country name included. Running that shape against A fails, but it does not report
// anything new: A already states the same bound, from the other direction, in
// TestEntityNameHeadroomIsStated ("metadata.name may be up to N octets"). A second test
// failing on a hypothetical name that the first one already prices would be noise, and
// worse, a red test nobody can act on gets disabled.
//
// What A genuinely lacked is the link between the proxy and the containers. The proxy
// resolves every upstream over the entity network, and a single-label name longer than 63
// octets is refused outright by Docker's embedded resolver ("bad address") — in B three
// portals answered 502 while the api-gateway, whose name is shorter, kept working, which
// read as a routing fault rather than a naming one. Two things have to hold for that not
// to recur, and nothing in A checked either:
//
//  1. every upstream the proxy dials fits a DNS label, for the entities actually declared;
//  2. every upstream names a container the templates actually create — a renamed service
//     leaves the proxy dialling a name that resolves to nothing, with the same symptom.
//
// The second is the one that catches a rename, which is how this class of defect arrives.

// TestProxyUpstreamsFitDNSLabel checks the proxy's upstreams for every entity declared in
// a checked-in manifest, which is the set a deploy can actually produce.
func TestProxyUpstreamsFitDNSLabel(t *testing.T) {
	for entity, where := range declaredEntityNames(t) {
		prefix := entityContainerPrefix(entity)
		assertUpstreamsResolvable(t, where+"/"+entity+" (central-bank routes)", centralBankProxyRoutes(prefix))
		assertUpstreamsResolvable(t, where+"/"+entity+" (commercial-bank routes)", commercialBankProxyRoutes(prefix))
	}
}

// TestProxyUpstreamsNameContainersTheTemplatesCreate is the rename guard. A proxy route
// whose upstream is not a container_name the compose templates produce dials a name the
// entity network cannot resolve; the route answers 502 against a stack that is entirely
// healthy, and nothing in the proxy config says which half is wrong.
func TestProxyUpstreamsNameContainersTheTemplatesCreate(t *testing.T) {
	suffixes := templateServiceSuffixes(t) // suffix -> template it came from; fails if empty

	const prefix = "cbweb3-probe"
	routes := append(centralBankProxyRoutes(prefix), commercialBankProxyRoutes(prefix)...)
	for _, r := range routes {
		host, _, _ := strings.Cut(r.Upstream, ":")
		suffix := strings.TrimPrefix(host, prefix)
		if suffix == host {
			t.Errorf("upstream %q for segment %q is not derived from the entity prefix; the proxy would dial a fixed name on a per-entity network",
				r.Upstream, r.Segment)
			continue
		}
		if _, ok := suffixes[suffix]; !ok {
			t.Errorf("segment %q dials %q, but no compose template declares a container_name ending %q — the proxy is routing to a container nothing creates (a rename, or a service that was removed)",
				r.Segment, r.Upstream, suffix)
		}
	}
}

// TestProxyUpstreamsAreEntityUnique guards the other half: the proxy attaches to EVERY
// entity network on its host, so two entities sharing an upstream alias would make
// resolution ambiguous and route one entity's portal to the other's container.
func TestProxyUpstreamsAreEntityUnique(t *testing.T) {
	seen := map[string]string{}
	record := func(who string, routes []ProxyRoute) {
		for _, r := range routes {
			if prev, dup := seen[r.Upstream]; dup {
				t.Errorf("upstream %q is produced by both %s and %s; on a shared host the proxy cannot tell them apart",
					r.Upstream, prev, who)
			}
			seen[r.Upstream] = who
		}
	}
	for _, e := range []string{"costa-rica", "chile", "peru"} {
		record("central-bank-"+e, centralBankProxyRoutes(entityContainerPrefix("central-bank-"+e)))
		record("bank-of-"+e, commercialBankProxyRoutes(entityContainerPrefix("bank-of-"+e)))
	}
}

// assertUpstreamsResolvable checks the host half of every upstream against the label limit.
func assertUpstreamsResolvable(t *testing.T, who string, routes []ProxyRoute) {
	t.Helper()
	if len(routes) == 0 {
		t.Fatalf("%s: no proxy routes to check — the guard would pass vacuously", who)
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
