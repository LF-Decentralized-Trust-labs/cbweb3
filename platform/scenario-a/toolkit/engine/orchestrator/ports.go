// SPDX-License-Identifier: Apache-2.0

package orchestrator

// Per-entity host port derivation (feature 034). All operational ports for an
// entity (backend, dedicated infra, frontend) are derived from the entity's Besu
// RPC host port, which the operator already sets uniquely per entity in the
// manifest. This keeps ports deterministic and collision-free across entities and
// spokes on one host, with no hashing and no extra manifest fields.
//
// Each service gets its own +1000 band so entities whose Besu RPC ports differ by
// as little as 1 (e.g. 8645/8646/8647 within a spoke) never collide: within a band
// the entity's Besu port keeps them distinct, and bands are 1000 apart. This holds
// as long as the spread of Besu RPC ports across all entities on one host is < 1000
// (true for the samples: spoke ports differ by ~1, spokes by ~100). Bands also
// avoid the Besu ports and the commercial-bank Paladin ports (Besu RPC +
// 19000/+20000/+21000; see step_start_paladin_join.go).
const (
	portOffsetAPIGateway     = 10000
	portOffsetAuthGRPC       = 11000
	portOffsetComplianceGRPC = 12000
	portOffsetPaymentGRPC    = 13000

	portOffsetPostgres = 14000
	portOffsetRedis    = 15000
	portOffsetKeycloak = 16000

	// Frontend portals, each in its own band (CB serves several; a bank serves one).
	portOffsetFrontendPrimary    = 17000 // governance (CB) / bank (commercial)
	portOffsetFrontendSecondary  = 18000 // treasury (CB)
	portOffsetFrontendSupervisor = 22000 // supervisor (CB)
	portOffsetFrontendNOC        = 24000 // NOC (CB)
)

// EntityPorts holds the derived host ports for one entity's operational stack.
type EntityPorts struct {
	APIGateway         int
	AuthGRPC           int
	ComplianceGRPC     int
	PaymentGRPC        int
	Postgres           int
	Redis              int
	Keycloak           int
	FrontendPrimary    int
	FrontendSecondary  int
	FrontendSupervisor int
	FrontendNOC        int
}

// entityPorts derives the operational stack ports for an entity from its Besu RPC
// host port. besuRPCPort must be > 0 (the manifest's spec.node.rpc.port).
func entityPorts(besuRPCPort int) EntityPorts {
	return EntityPorts{
		APIGateway:         besuRPCPort + portOffsetAPIGateway,
		AuthGRPC:           besuRPCPort + portOffsetAuthGRPC,
		ComplianceGRPC:     besuRPCPort + portOffsetComplianceGRPC,
		PaymentGRPC:        besuRPCPort + portOffsetPaymentGRPC,
		Postgres:           besuRPCPort + portOffsetPostgres,
		Redis:              besuRPCPort + portOffsetRedis,
		Keycloak:           besuRPCPort + portOffsetKeycloak,
		FrontendPrimary:    besuRPCPort + portOffsetFrontendPrimary,
		FrontendSecondary:  besuRPCPort + portOffsetFrontendSecondary,
		FrontendSupervisor: besuRPCPort + portOffsetFrontendSupervisor,
		FrontendNOC:        besuRPCPort + portOffsetFrontendNOC,
	}
}

// ephemeralPortFloor is the lowest port the Linux kernel hands out for OUTBOUND
// connections: net.ipv4.ip_local_port_range has defaulted to 32768-60999 for many
// years. A published host port at or above this floor races every outbound
// connection on the machine — the kernel may already have handed that number to
// some other socket by the time Docker binds it, and the bind fails with
// "address already in use" against a port that nothing appears to hold.
//
// A bring-up is the worst case: it starts dozens of containers, each opening
// connections of its own, so the deploy is itself the machine's largest consumer
// of ephemeral ports and every container it starts is a fresh chance to lose the
// race. The failure lands on a random step of a random entity and reads as a
// product defect until someone looks at the port number.
//
// The floor is a fixed constant rather than a read of /proc on purpose: the
// guard has to give the same answer on every host, including one whose range has
// been narrowed. Narrowing the host's range does avoid the collision, but it is
// unversioned machine state — the port choice is what belongs in the repository.
const ephemeralPortFloor = 32768

// besuRPCPortCeiling is the highest Besu RPC host port a manifest may declare and
// still keep every port derived from it below the ephemeral floor. The NOC portal
// carries the largest offset (portOffsetFrontendNOC), so it sets the ceiling.
const besuRPCPortCeiling = ephemeralPortFloor - portOffsetFrontendNOC - 1

// cbPaladinFallbackRPCPort is the CB Paladin RPC host port used when
// CBWEB3_PALADIN_CB_URL carries no port. Unlike everything in EntityPorts it is
// not derived from the Besu RPC port — a second spoke on one host overrides the
// URL (31748, 31848, …) — so it sits in its own band above the p2p ports and
// below the ephemeral floor, with WS at +1 and gRPC at +2.
const cbPaladinFallbackRPCPort = 31648

// derivedHostPorts returns every host port this toolkit publishes for an entity
// whose Besu RPC host port is besuRPCPort, keyed by a name an operator would
// recognise. Used by the ephemeral-range guard: checking only the ports a
// manifest declares would miss the ~11 that are derived from them, which is
// where the largest numbers live.
//
// The CB Paladin ports are absent because they are not derived from the manifest:
// they come from CBWEB3_PALADIN_CB_URL (fallback cbPaladinFallbackRPCPort), and
// the guard checks that fallback separately.
func derivedHostPorts(besuRPCPort int) map[string]int {
	p := entityPorts(besuRPCPort)
	return map[string]int{
		"api-gateway":         p.APIGateway,
		"auth-grpc":           p.AuthGRPC,
		"compliance-grpc":     p.ComplianceGRPC,
		"payment-grpc":        p.PaymentGRPC,
		"postgres":            p.Postgres,
		"redis":               p.Redis,
		"keycloak":            p.Keycloak,
		"frontend-primary":    p.FrontendPrimary,
		"frontend-secondary":  p.FrontendSecondary,
		"frontend-supervisor": p.FrontendSupervisor,
		"frontend-noc":        p.FrontendNOC,
		"paladin-bank-rpc":    besuRPCPort + bankPaladinRPCPortOffset,
		"paladin-bank-ws":     besuRPCPort + bankPaladinWSPortOffset,
		"paladin-bank-grpc":   besuRPCPort + bankPaladinGRPCPortOffset,
	}
}
