// SPDX-License-Identifier: Apache-2.0

// Package relayregistrar is the toolkit's pluggable boundary for registering
// spokes with the generalized relay (TK-B5). A local in-memory implementation
// (idempotent registry) serves dev/tests; a production stub forwards to the
// relay's POST /api/v1/spokes. The implementation is selected by URI factory,
// mirroring keyprovider/certsource. No secrets are persisted.
package relayregistrar

import (
	"context"
	"errors"
)

// Spoke is the minimal record registered with the relay.
type Spoke struct {
	ID         string
	BesuRPC    string
	BesuWS     string
	GatewayURL string
}

// RelayRegistrar registers spokes with the relay (idempotent upsert).
type RelayRegistrar interface {
	Register(ctx context.Context, s Spoke) error
}

// LocalRegistry is implemented by the local provider; List supports test
// assertions on the in-memory set.
type LocalRegistry interface {
	RelayRegistrar
	List() []Spoke
}

// Typed, distinguishable errors (no silent failures — Constitution VI).
var (
	ErrUnsupportedURI = errors.New("relayregistrar: unsupported factory URI")
	ErrInvalidSpoke   = errors.New("relayregistrar: invalid spoke (id/rpc/ws/gateway required)")
	ErrNotImplemented = errors.New("relayregistrar: production registrar requires a relay endpoint")
	// ErrMissingSecret is returned when a live registrar has no X-Relay-Auth credential.
	// The relay guards POST /api/v1/spokes (finding R2-M-10), so posting without the header
	// would be refused there; failing here names the missing configuration instead of
	// surfacing as an opaque 401 in the middle of provisioning.
	ErrMissingSecret = errors.New("relayregistrar: live registrar requires INTERNAL_RELAY_AUTH_SECRET")
)

func validate(s Spoke) error {
	if s.ID == "" || s.BesuRPC == "" || s.BesuWS == "" || s.GatewayURL == "" {
		return ErrInvalidSpoke
	}
	return nil
}
