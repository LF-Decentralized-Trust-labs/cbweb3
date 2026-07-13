package orchestrator

import "fmt"

// CORS origins for the entity api-gateways. The operator portals are served on
// distinct host ports (different origin from the api-gateway), and the SPAs call
// the gateway with credentials (cookie auth), so `*` is not allowed — every portal
// origin must be whitelisted explicitly or that portal fails CORS at login. Mirrors
// scenario-a's cbCORSOrigins/bankCORSOrigins. Ports follow the fixed offsets:
// governance/bank portal +9000, noc-portal +12000, treasury +13000, supervisor +14000.

// corsOriginsCB returns the comma-separated browser origins a Central Bank
// api-gateway must allow: its four operator portals (governance, treasury,
// supervisor, noc).
func corsOriginsCB(rpcPort int) string {
	return fmt.Sprintf("http://localhost:%d,http://localhost:%d,http://localhost:%d,http://localhost:%d",
		rpcPort+9000,  // governance
		rpcPort+13000, // treasury
		rpcPort+14000, // supervisor
		rpcPort+12000, // noc-portal
	)
}

// corsOriginSingle returns the single browser origin for an entity that serves one
// portal at RPC+9000 — a commercial bank's bank app or the hub's governance portal.
func corsOriginSingle(rpcPort int) string {
	return fmt.Sprintf("http://localhost:%d", rpcPort+9000)
}
