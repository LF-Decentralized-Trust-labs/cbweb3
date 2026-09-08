// SPDX-License-Identifier: Apache-2.0

// Guard for the finding that a published host port inside the kernel's ephemeral
// range makes a bring-up race the whole machine: the kernel may hand that number
// to an outbound connection before Docker binds it, and the bind fails with
// "address already in use" against a port nothing appears to hold. See
// ephemeralPortFloor and docs/scenario-drift.md.
//
// This is a scan of every sample manifest rather than an assertion about one port
// on purpose. The defect was never a single wrong number — every host port the
// Scenario B samples pinned sat in the range, and a new sample copied from an
// existing one would inherit that. A test that reads the directory catches the
// next copy.
//
// It checks DERIVED ports, not only declared ones. The manifests declare three
// ports per entity and the toolkit computes nine more from them, up to +14000 —
// so a guard that read only the manifests would have reported a third of the
// exposure and missed the largest numbers entirely.
package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
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
func sampleManifests(t *testing.T) map[string]*manifest.ParticipantDeployment {
	t.Helper()
	root := filepath.Join("..", "..", "..", "samples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("samples/ not reachable from this module (the guard cannot run): %v", err)
	}
	out := map[string]*manifest.ParticipantDeployment{}
	for _, e := range entries {
		if !e.IsDir() || sampleManifestDirsSkipped[e.Name()] {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, e.Name(), "*.yaml"))
		if err != nil {
			t.Fatalf("glob %s: %v", e.Name(), err)
		}
		for _, f := range files {
			pd, err := manifest.Load(f)
			if err != nil {
				t.Fatalf("load sample manifest %s: %v", f, err)
			}
			out[filepath.Join(e.Name(), filepath.Base(f))] = pd
		}
	}
	if len(out) == 0 {
		t.Fatalf("no sample manifests found under %s — the guard would pass vacuously", root)
	}
	return out
}

func TestNoSampleHostPortReachesTheEphemeralRange(t *testing.T) {
	manifests := sampleManifests(t)
	withNode := 0
	for name, pd := range manifests {
		// spec.node is absent on an observe (NOC) manifest: it runs no on-chain node.
		n := pd.Spec.Node
		declared := map[string]int{}
		if n != nil && n.RPC != nil {
			declared["besu-rpc"] = n.RPC.Port
		}
		if n != nil && n.WS != nil {
			declared["besu-ws"] = n.WS.Port
		}
		if n != nil && n.P2P != nil {
			declared["besu-p2p"] = n.P2P.Port
		}
		if pd.Spec.LauncherPort != 0 {
			declared["launcher"] = pd.Spec.LauncherPort
		}
		for svc, port := range declared {
			if port >= ephemeralPortFloor {
				t.Errorf("%s declares %s on %d, inside the kernel's ephemeral range (>= %d): every bring-up races the machine for it",
					name, svc, port, ephemeralPortFloor)
			}
		}
		if n == nil || n.RPC == nil {
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

// The NOC control plane publishes fixed ports (one NOC per host), so the scan
// above cannot see them.
func TestFixedHostPortsStayBelowTheEphemeralFloor(t *testing.T) {
	fixed := map[string]int{
		"noc-backend": defaultNOCBackendPort,
		"noc-portal":  defaultNOCPortalPort,
	}
	for svc, port := range fixed {
		if port >= ephemeralPortFloor {
			t.Errorf("fixed host port %s = %d is inside the kernel's ephemeral range (>= %d)", svc, port, ephemeralPortFloor)
		}
	}
}

// rpcOffsetLiteral matches the port arithmetic the engine renders inline, e.g.
// `c.RPCPort + 8000` or `rpcPort+13000`.
var rpcOffsetLiteral = regexp.MustCompile(`(?:RPCPort|rpcPort)\s*\+\s*(\d+)`)

// The ceiling is only as good as the offset set it is computed from, and this
// package derives ports with inline literals rather than these constants. If a
// service is added with an offset ports.go does not know about, the
// ephemeral-range scan silently stops covering it — this fails when that happens.
func TestDerivedOffsetsMatchTheEngineSources(t *testing.T) {
	known := map[int]bool{}
	for _, off := range []int{
		portOffsetWSDefault,
		portOffsetPostgres,
		portOffsetRedis,
		portOffsetKeycloak,
		portOffsetGateway,
		portOffsetFrontendPrimary,
		portOffsetNOCBackend,
		portOffsetNOCPortal,
		portOffsetFrontendTreasury,
		portOffsetFrontendSupervisor,
	} {
		known[off] = true
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package sources: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no package sources found — the guard would pass vacuously")
	}
	found := map[int][]string{}
	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		for _, m := range rpcOffsetLiteral.FindAllStringSubmatch(string(body), -1) {
			off, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			found[off] = append(found[off], src)
		}
	}
	if len(found) == 0 {
		t.Fatal("no `RPCPort + <n>` arithmetic found in the package — the guard cannot see the engine's offsets any more")
	}
	for off, srcs := range found {
		if !known[off] {
			t.Errorf("the engine derives a host port at +%d (%s) but ports.go does not list that offset: the ephemeral-range guard cannot see it, and besuRPCPortCeiling may be wrong",
				off, strings.Join(srcs, ", "))
		}
		if off > portOffsetFrontendSupervisor {
			t.Errorf("offset +%d (%s) is larger than the +%d that besuRPCPortCeiling is computed from: raise the ceiling or lower the offset",
				off, strings.Join(srcs, ", "), portOffsetFrontendSupervisor)
		}
	}
}

// The two scenarios must run side by side on one host: the launcher pairs them per
// entity, so a collision between an A port and a B port is not hypothetical. B
// declares its bases in the x145 lane and A in x645 (see docs/scenario-drift.md);
// this pins B's half of that split so a renumbering cannot quietly break it.
func TestScenarioBBasesStayInTheirLane(t *testing.T) {
	for name, pd := range sampleManifests(t) {
		if pd.Spec.Node == nil || pd.Spec.Node.RPC == nil {
			continue
		}
		base := pd.Spec.Node.RPC.Port
		if within := base % 1000; within < 145 || within > 557 {
			t.Errorf("%s declares Besu RPC %d: Scenario B bases live in the x145-x557 lane (Scenario A owns x645-x767), and this uses x%d",
				name, base, within)
		}
	}
}
