// SPDX-License-Identifier: Apache-2.0

package relayregistrar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// prodRegistrar forwards registrations to the relay's POST /api/v1/spokes.
type prodRegistrar struct {
	endpoint string // base URL, e.g. http://relay-host:4000
	secret   string // X-Relay-Auth credential the relay's inbound guard expects
	client   *http.Client
}

func (p *prodRegistrar) Register(ctx context.Context, s Spoke) error {
	if err := validate(s); err != nil {
		return err
	}
	if p.endpoint == "" {
		return ErrNotImplemented
	}
	// Fail before the request, not after: the relay's guard would reject an unauthenticated
	// registration with a 401 that says nothing about which side is misconfigured.
	if p.secret == "" {
		return ErrMissingSecret
	}
	body, err := json.Marshal(map[string]string{
		"spokeId":    s.ID,
		"besuRpc":    s.BesuRPC,
		"besuWs":     s.BesuWS,
		"gatewayUrl": s.GatewayURL,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/api/v1/spokes", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// The relay authenticates every inbound route with this header (finding R2-M-10).
	req.Header.Set("X-Relay-Auth", p.secret)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("relayregistrar: POST /api/v1/spokes returned %d", resp.StatusCode)
	}
	return nil
}
