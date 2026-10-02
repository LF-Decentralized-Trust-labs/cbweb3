// SPDX-License-Identifier: Apache-2.0

// Per-entity host port derivation, named. Every operational host port of a
// Scenario B entity is computed from the entity's Besu RPC host port, which the
// operator already sets uniquely per entity in the manifest. The arithmetic
// itself lives inline in step_found_hub.go, step_found_spoke.go, step_join.go,
// cors.go and launcher.go; this file gives those offsets names and a single place
// that knows the whole set, so the ephemeral-range guard can compute every port a
// manifest produces rather than only the three it declares.
//
// The inline literals are the ones the engine actually renders. They and this set
// are kept honest by TestDerivedOffsetsMatchTheEngineSources, which reads those
// sources and fails on an offset this file does not know about. Collapsing the
// literals onto these constants is a follow-up (three open PRs own those files).
package orchestrator

// Host-port offsets over the entity's Besu RPC port. Each service gets its own
// +1000 band so entities whose Besu RPC ports differ by as little as 1 (e.g.
// 9145/9146/9147 within a spoke) never collide: within a band the entity's Besu
// port keeps them distinct, and bands are 1000 apart. This holds as long as the
// spread of Besu RPC ports across all entities on one host is < 1000.
const (
	portOffsetWSDefault = 1 // only when the manifest declares no ws port

	portOffsetPostgres = 5000
	portOffsetRedis    = 6000
	portOffsetKeycloak = 7000
	portOffsetGateway  = 8000

	// Frontends. The primary is governance on a central bank and the bank portal
	// on a commercial bank; treasury and supervisor are central-bank only.
	portOffsetFrontendPrimary    = 9000
	portOffsetNOCBackend         = 11000
	portOffsetNOCPortal          = 12000
	portOffsetFrontendTreasury   = 13000
	portOffsetFrontendSupervisor = 14000
)

// ephemeralPortFloor is the lowest port the Linux kernel hands out for OUTBOUND
// connections: net.ipv4.ip_local_port_range has defaulted to 32768-60999 for many
// years. A published host port at or above this floor races every outbound
// connection on the machine — the kernel may already have handed that number to
// some other socket by the time Docker binds it, and the bind fails with
// "address already in use" against a port that nothing appears to hold.
//
// A bring-up is the worst case: deploy-all.sh starts dozens of containers, each
// opening connections of its own, so the deploy is itself the machine's largest
// consumer of ephemeral ports and every container it starts is a fresh chance to
// lose the race. The failure lands on a random step of a random entity and reads
// as a product defect until someone looks at the port number.
//
// The floor is a fixed constant rather than a read of /proc on purpose: the guard
// has to give the same answer on every host, including one whose range has been
// narrowed. Narrowing the host's range does avoid the collision, but it is
// unversioned machine state — the port choice is what belongs in the repository.
const ephemeralPortFloor = 32768

// besuRPCPortCeiling is the highest Besu RPC host port a manifest may declare and
// still keep every port derived from it below the ephemeral floor. The supervisor
// portal carries the largest offset, so it sets the ceiling.
const besuRPCPortCeiling = ephemeralPortFloor - portOffsetFrontendSupervisor - 1

// derivedHostPorts returns every host port this toolkit publishes for an entity
// whose Besu RPC host port is rpcPort, keyed by a name an operator would
// recognise. Used by the ephemeral-range guard: checking only the ports a
// manifest declares would miss the nine that are derived from them, which is
// where the largest numbers live.
func derivedHostPorts(rpcPort int) map[string]int {
	return map[string]int{
		"besu-ws-default":     rpcPort + portOffsetWSDefault,
		"postgres":            rpcPort + portOffsetPostgres,
		"redis":               rpcPort + portOffsetRedis,
		"keycloak":            rpcPort + portOffsetKeycloak,
		"api-gateway":         rpcPort + portOffsetGateway,
		"frontend-primary":    rpcPort + portOffsetFrontendPrimary,
		"noc-backend":         rpcPort + portOffsetNOCBackend,
		"noc-portal":          rpcPort + portOffsetNOCPortal,
		"frontend-treasury":   rpcPort + portOffsetFrontendTreasury,
		"frontend-supervisor": rpcPort + portOffsetFrontendSupervisor,
	}
}
