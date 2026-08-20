// SPDX-License-Identifier: Apache-2.0

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

// nocPortalOrigins returns the browser origin(s) the co-located NOC portal is served
// from: RPC+12000, plus the routable-host form when frontendHost is not localhost, or the
// single proxy origin when the entity runs behind the reverse proxy.
//
// Used for the PUBLIC noc-portal Keycloak client's webOrigins. That client did a browser
// password grant with webOrigins=["*"], which let any origin read its token response —
// the worst shape for a public client, since there is no client secret between an
// attacker's page and a token (finding R1-10.7). Scoping it to the portal's own origin
// keeps the grant working and closes that.
func nocPortalOrigins(rpcPort int, frontendHost string, proxy bool) []string {
	if proxy && frontendHost != "" {
		return []string{proxyOrigin(frontendHost)}
	}
	origins := []string{fmt.Sprintf("http://localhost:%d", rpcPort+12000)}
	if frontendHost != "" && frontendHost != "localhost" {
		origins = append(origins, fmt.Sprintf("http://%s:%d", frontendHost, rpcPort+12000))
	}
	return origins
}
