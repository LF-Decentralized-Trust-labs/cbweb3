package relayregistrar

import (
	"net/http"
	"strings"
)

// New selects the implementation by URI:
//
//	local                 -> in-memory (idempotent registry)
//	relay://<host>[:port] -> production stub (POST /api/v1/spokes)
//	anything else         -> ErrUnsupportedURI
func New(uri string) (RelayRegistrar, error) {
	if uri == "local" {
		return newLocal(), nil
	}
	if strings.HasPrefix(uri, "relay://") {
		endpoint := "http://" + strings.TrimPrefix(uri, "relay://")
		return &prodRegistrar{endpoint: endpoint, client: &http.Client{}}, nil
	}
	return nil, ErrUnsupportedURI
}
