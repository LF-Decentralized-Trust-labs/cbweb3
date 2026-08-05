// SPDX-License-Identifier: Apache-2.0

package relayregistrar

import (
	"net/http"
	"strings"
)

// New selects the implementation by URI:
//
//	local                    -> in-memory (idempotent registry)
//	relay://<host>[:port]    -> live relay (POST /api/v1/spokes)
//	http(s)://<host>[:port]  -> live relay (manifest spec.relay.endpoint form)
//	anything else            -> ErrUnsupportedURI
func New(uri string) (RelayRegistrar, error) {
	if uri == "local" {
		return newLocal(), nil
	}
	if strings.HasPrefix(uri, "relay://") {
		endpoint := "http://" + strings.TrimPrefix(uri, "relay://")
		return &prodRegistrar{endpoint: endpoint, client: &http.Client{}}, nil
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return &prodRegistrar{endpoint: uri, client: &http.Client{}}, nil
	}
	return nil, ErrUnsupportedURI
}
