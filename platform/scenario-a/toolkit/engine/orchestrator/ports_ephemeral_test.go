// SPDX-License-Identifier: Apache-2.0

// Guard for the finding that a published host port inside the kernel's ephemeral
// range makes a bring-up race the whole machine: the kernel may hand that number
// to an outbound connection before Docker binds it, and the bind fails with
// "address already in use" against a port nothing appears to hold. See
// ephemeralPortFloor and docs/scenario-drift.md.
//
// This is a scan of every sample manifest rather than an assertion about one port
// on purpose. The defect was never a single wrong number — it was that a whole
// family of sample ports sat in the range, and a new sample copied from an
// existing one would inherit the mistake. A test that reads the directory catches
// the next copy.
//
// It checks DERIVED ports, not only declared ones. Reading the manifests alone
// would miss the eleven ports the toolkit computes from each declared Besu RPC
// port, which is exactly where the largest numbers live: the NOC portal at
// +24000 is what put Scenario A in the range at all.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// sampleManifestDirsSkipped are directories under samples/ that hold generated
// state, not checked-in manifests. They are gitignored and carry the ports of
// whatever was last deployed on the developer's machine, so reading them would
// make this guard fail on a stale local run instead of on a repository defect.
var sampleManifestDirsSkipped = map[string]bool{
	"cbweb3-data": true,
	"bundles":     true,
}

// sampleManifests loads every checked-in sample manifest, keyed by its path.
//
// An unreachable directory or an empty result is a failure, not a skip: a guard
// that quietly reports "nothing to check" after a layout change is the drift it
// exists to catch.
func sampleManifests(t *testing.T) map[string]*manifest.Manifest {
	t.Helper()
	root := filepath.Join("..", "..", "..", "samples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("samples/ not reachable from this module (the guard cannot run): %v", err)
	}
	out := map[string]*manifest.Manifest{}
	for _, e := range entries {
		if !e.IsDir() || sampleManifestDirsSkipped[e.Name()] {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, e.Name(), "*.yaml"))
		if err != nil {
			t.Fatalf("glob %s: %v", e.Name(), err)
		}
		for _, f := range files {
			m, err := manifest.Load(f)
			if err != nil {
				t.Fatalf("load sample manifest %s: %v", f, err)
			}
			out[filepath.Join(e.Name(), filepath.Base(f))] = m
		}
	}
	if len(out) == 0 {
		t.Fatalf("no sample manifests found under %s — the guard would pass vacuously", root)
	}
	return out
}

func TestNoSampleHostPortReachesTheEphemeralRange(t *testing.T) {
	manifests := sampleManifests(t)
	// A manifest that declares a Besu node is what produces derived ports. The
	// observe (NOC) manifests declare none and are still checked for whatever
	// they do declare, below.
	withNode := 0
	for name, m := range manifests {
		n := m.Spec.Node
		declared := map[string]int{}
		if n.RPC != nil {
			declared["besu-rpc"] = n.RPC.Port
		}
		if n.WS != nil {
			declared["besu-ws"] = n.WS.Port
		}
		if n.P2P != nil {
			declared["besu-p2p"] = n.P2P.Port
		}
		for svc, port := range declared {
			if port >= ephemeralPortFloor {
				t.Errorf("%s declares %s on %d, inside the kernel's ephemeral range (>= %d): every bring-up races the machine for it",
					name, svc, port, ephemeralPortFloor)
			}
		}
		if n.RPC == nil {
			continue
		}
		withNode++
		if n.RPC.Port > besuRPCPortCeiling {
			t.Errorf("%s declares Besu RPC %d, above the ceiling %d: ports derived from it reach the ephemeral range even though the declared port does not",
				name, n.RPC.Port, besuRPCPortCeiling)
		}
		for svc, port := range derivedHostPorts(n.RPC.Port) {
			if port >= ephemeralPortFloor {
				t.Errorf("%s: %s is derived as %d (Besu RPC %d), inside the kernel's ephemeral range (>= %d)",
					name, svc, port, n.RPC.Port, ephemeralPortFloor)
			}
		}
	}
	if withNode == 0 {
		t.Fatal("no sample manifest declares spec.node.rpc — the derived-port half of the guard checked nothing")
	}
}

// The NOC backend and the CB Paladin fallback are fixed ports rather than derived
// ones, so the scan above cannot see them.
func TestFixedHostPortsStayBelowTheEphemeralFloor(t *testing.T) {
	fixed := map[string]int{
		"noc-backend":       NOCBackendPort,
		"paladin-cb-rpc":    cbPaladinFallbackRPCPort,
		"paladin-cb-ws":     cbPaladinFallbackRPCPort + 1,
		"paladin-cb-grpc":   cbPaladinFallbackRPCPort + 2,
		"paladin-peer-grpc": paladinPeerGRPCPort,
	}
	for svc, port := range fixed {
		if port >= ephemeralPortFloor {
			t.Errorf("fixed host port %s = %d is inside the kernel's ephemeral range (>= %d)", svc, port, ephemeralPortFloor)
		}
	}
}

// The ceiling is only as good as the offset it is computed from. If a service is
// added with a larger offset than the NOC portal's, besuRPCPortCeiling silently
// stops protecting it — this fails when that happens.
func TestNOCPortalStillCarriesTheLargestOffset(t *testing.T) {
	offsets := map[string]int{
		"api-gateway":         portOffsetAPIGateway,
		"auth-grpc":           portOffsetAuthGRPC,
		"compliance-grpc":     portOffsetComplianceGRPC,
		"payment-grpc":        portOffsetPaymentGRPC,
		"postgres":            portOffsetPostgres,
		"redis":               portOffsetRedis,
		"keycloak":            portOffsetKeycloak,
		"frontend-primary":    portOffsetFrontendPrimary,
		"frontend-secondary":  portOffsetFrontendSecondary,
		"frontend-supervisor": portOffsetFrontendSupervisor,
		"frontend-noc":        portOffsetFrontendNOC,
		"paladin-bank-rpc":    bankPaladinRPCPortOffset,
		"paladin-bank-ws":     bankPaladinWSPortOffset,
		"paladin-bank-grpc":   bankPaladinGRPCPortOffset,
	}
	for svc, off := range offsets {
		if off > portOffsetFrontendNOC {
			t.Errorf("%s has offset +%d, larger than the NOC portal's +%d that besuRPCPortCeiling is computed from: raise the ceiling or lower the offset",
				svc, off, portOffsetFrontendNOC)
		}
	}
	// derivedHostPorts must cover every offset above, or the scan has a blind spot.
	covered := derivedHostPorts(besuRPCPortCeiling)
	for svc := range offsets {
		if _, ok := covered[svc]; !ok {
			t.Errorf("offset %s is not in derivedHostPorts: the ephemeral-range scan cannot see it", svc)
		}
	}
}

// The two scenarios must run side by side on one host: the launcher pairs them per
// entity, so a collision between an A port and a B port is not hypothetical. A
// declares its bases in the x645 lane and B in x145 (see docs/scenario-drift.md);
// this pins A's half of that split so a renumbering cannot quietly break it.
func TestScenarioABasesStayInTheirLane(t *testing.T) {
	for name, m := range sampleManifests(t) {
		if m.Spec.Node.RPC == nil {
			continue
		}
		base := m.Spec.Node.RPC.Port
		if within := base % 1000; within < 645 || within > 767 {
			t.Errorf("%s declares Besu RPC %d: Scenario A bases live in the x645-x767 lane (Scenario B owns x145-x557), and %s uses x%d",
				name, base, strings.TrimSuffix(name, ".yaml"), within)
		}
	}
}
