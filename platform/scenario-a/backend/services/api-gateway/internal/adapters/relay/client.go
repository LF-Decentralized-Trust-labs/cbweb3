// SPDX-License-Identifier: Apache-2.0

// Package relay is the api-gateway's client for the Cacti relay. It is used to
// discover the consortium's spokes (the relay's dynamic spoke registry) so the
// FX-party identity roster can be federated across every spoke in the network
// instead of pinned to a hand-maintained static list.
package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Spoke is a single entry from the relay's spoke registry (GET /api/v1/spokes).
// Only the fields the roster federation needs are decoded; the registry carries
// more (besuRpc, htlcAddress, …) that are irrelevant here.
type Spoke struct {
	ID             string `json:"id"`
	InternalApiURL string `json:"internalApiUrl"`
}

// Client talks to the Cacti relay (spoke discovery) and to peer gateways (roster
// federation) over HTTP.
type Client struct {
	baseURL     string
	relaySecret string
	http        *http.Client
}

// NewClient builds a relay client for the given base URL (e.g.
// http://host.docker.internal:4000). relaySecret is the shared X-Relay-Auth
// secret sent on peer gateway-to-gateway calls (INTERNAL_RELAY_AUTH_SECRET). The
// per-request timeout bounds both the spoke-registry lookup and each peer
// fan-out call.
func NewClient(baseURL, relaySecret string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		relaySecret: relaySecret,
		http:        &http.Client{Timeout: timeout},
	}
}

// ListSpokes returns every spoke registered with the relay.
func (c *Client) ListSpokes(ctx context.Context) ([]Spoke, error) {
	var spokes []Spoke
	if err := c.getJSON(ctx, c.baseURL+"/api/v1/spokes", &spokes, false); err != nil {
		return nil, fmt.Errorf("list spokes: %w", err)
	}
	return spokes, nil
}

// identitiesResponse mirrors the api-gateway's GET /api/v1/identities payload.
type identitiesResponse struct {
	Identities []string `json:"identities"`
	Configured bool     `json:"configured"`
}

// FetchPeerIdentities fetches a peer gateway's LOCAL roster from its internal
// gateway-to-gateway endpoint (/internal/v1/identities), authenticated with the
// shared X-Relay-Auth secret. That endpoint returns only the peer's own Pente
// membership (plus any static override) and does NOT itself federate — the
// recursion guard that keeps a network-wide lookup from fanning out repeatedly.
func (c *Client) FetchPeerIdentities(ctx context.Context, baseURL string) ([]string, error) {
	base := strings.TrimRight(baseURL, "/")
	endpoint := base + "/internal/v1/identities"
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("invalid peer url %q: %w", baseURL, err)
	}
	var resp identitiesResponse
	if err := c.getJSON(ctx, endpoint, &resp, true); err != nil {
		return nil, fmt.Errorf("fetch peer identities from %s: %w", base, err)
	}
	return resp.Identities, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any, relayAuth bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if relayAuth && c.relaySecret != "" {
		req.Header.Set("X-Relay-Auth", c.relaySecret)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d from %s", res.StatusCode, endpoint)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response from %s: %w", endpoint, err)
	}
	return nil
}
