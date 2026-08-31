// SPDX-License-Identifier: Apache-2.0

package relayregistrar

import (
	"net/http"
	"os"
	"strings"
)

// New selects the implementation by URI, taking the live registrar's X-Relay-Auth
// credential from INTERNAL_RELAY_AUTH_SECRET. Callers that hold the secret explicitly
// should prefer NewWithSecret.
//
//	local                    -> in-memory (idempotent registry)
//	relay://<host>[:port]    -> live relay (POST /api/v1/spokes)
//	http(s)://<host>[:port]  -> live relay (manifest spec.relay.endpoint form)
//	anything else            -> ErrUnsupportedURI
func New(uri string) (RelayRegistrar, error) {
	return NewWithSecret(uri, os.Getenv("INTERNAL_RELAY_AUTH_SECRET"))
}

// NewWithSecret is New with the relay credential passed in rather than read from the
// environment. The local registrar never leaves the process, so it ignores the secret.
func NewWithSecret(uri, secret string) (RelayRegistrar, error) {
	if uri == "local" {
		return newLocal(), nil
	}
	if strings.HasPrefix(uri, "relay://") {
		endpoint := "http://" + strings.TrimPrefix(uri, "relay://")
		return &prodRegistrar{endpoint: endpoint, secret: secret, client: &http.Client{}}, nil
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return &prodRegistrar{endpoint: uri, secret: secret, client: &http.Client{}}, nil
	}
	return nil, ErrUnsupportedURI
}
