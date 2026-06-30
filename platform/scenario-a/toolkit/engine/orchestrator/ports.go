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
