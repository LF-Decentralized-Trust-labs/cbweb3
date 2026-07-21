package orchestrator

import "fmt"

// CORS origins for the entity api-gateways. The operator portals are served on
// distinct host ports (different origin from the api-gateway), and the SPAs call
// the gateway with credentials (cookie auth), so `*` is not allowed — every portal
// origin must be whitelisted explicitly or that portal fails CORS at login. Mirrors
// scenario-a's cbCORSOrigins/bankCORSOrigins. Ports follow the fixed offsets:
// governance/bank portal +9000, noc-portal +12000, treasury +13000, supervisor +14000.

// frontendHostOrLocal returns the browser-facing host baked into the frontends'
// VITE_API_URL and whitelisted in api-gateway CORS. It is spec.frontendHost when
// set to a routable IP/DNS name, otherwise "localhost" (which only works when the
// browser runs on the Docker host itself). Mirrors scenario-a's frontendAdvertisedHost.
func frontendHostOrLocal(frontendHost string) string {
	if frontendHost == "" {
		return "localhost"
	}
	return frontendHost
}

// corsOriginsCB returns the comma-separated browser origins a Central Bank
// api-gateway must allow: its four operator portals (governance, treasury,
// supervisor, noc). When frontendHost is a routable host (not localhost), both the
// localhost and the remote origins are whitelisted so the same stack works from the
// Docker host and from remote browsers (e.g. a cloud VM public IP). Mirrors
// scenario-a's cbCORSOrigins.
func corsOriginsCB(rpcPort int, frontendHost string) string {
	local := fmt.Sprintf("http://localhost:%d,http://localhost:%d,http://localhost:%d,http://localhost:%d",
		rpcPort+9000,  // governance
		rpcPort+13000, // treasury
		rpcPort+14000, // supervisor
		rpcPort+12000, // noc-portal
	)
	if frontendHost == "" || frontendHost == "localhost" {
		return local
	}
	remote := fmt.Sprintf("http://%s:%d,http://%s:%d,http://%s:%d,http://%s:%d",
		frontendHost, rpcPort+9000,
		frontendHost, rpcPort+13000,
		frontendHost, rpcPort+14000,
		frontendHost, rpcPort+12000,
	)
	return local + "," + remote
}

// corsOriginSingle returns the browser origin(s) for an entity that serves one
// portal at RPC+9000 — a commercial bank's bank app or the hub's governance portal.
// When frontendHost is a routable host (not localhost), both the localhost and the
// remote origin are whitelisted. Mirrors scenario-a's bankCORSOrigins.
func corsOriginSingle(rpcPort int, frontendHost string) string {
	local := fmt.Sprintf("http://localhost:%d", rpcPort+9000)
	if frontendHost == "" || frontendHost == "localhost" {
		return local
	}
	return fmt.Sprintf("%s,http://%s:%d", local, frontendHost, rpcPort+9000)
}
