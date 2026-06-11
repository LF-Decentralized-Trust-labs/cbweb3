// Package services — CentralBankPoolClient proxies pool status reads to the spoke's
// Central Bank API gateway (CENTRAL_BANK_API_URL). Commercial banks do not read the
// Hub sovereign AMM directly; the CB gateway holds SOVEREIGN_AMM_ADDRESS and DB enrichment.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CentralBankPoolClient fetches pool status from the Central Bank of the same spoke.
type CentralBankPoolClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCentralBankPoolClient creates a client targeting the CB api-gateway base URL
// (e.g. http://api-gateway-central-bank-a:8080).
func NewCentralBankPoolClient(baseURL string, timeout time.Duration) *CentralBankPoolClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &CentralBankPoolClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// normalizeSovereignPoolPair maps quote-style pair IDs to the Hub sovereign pair id.
// e.g. W-BRL-W-ARS (cross-currency quote) → W-BRL-ARS (SeedNewSovereignPair / CB liquidity).
func normalizeSovereignPoolPair(pair string) string {
	parts := strings.Split(pair, "-")
	if len(parts) == 4 && parts[0] == "W" && parts[2] == "W" {
		return "W-" + parts[1] + "-" + parts[3]
	}
	return pair
}

// GetPoolStatus implements handlers.PoolStatusServiceIface via GET /api/v2/amm/pool/{pair}/status.
func (c *CentralBankPoolClient) GetPoolStatus(ctx context.Context, pair string) (*PoolStatusResponse, error) {
	pair = normalizeSovereignPoolPair(pair)
	url := fmt.Sprintf("%s/api/v2/amm/pool/%s/status", c.baseURL, pair)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("central bank pool status: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("central bank pool status: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("central bank pool status: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("central bank pool status: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out PoolStatusResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("central bank pool status: decode response: %w", err)
	}
	return &out, nil
}

// IsActive implements PoolStatusGate for cross-currency pre-checks (FR-002).
func (c *CentralBankPoolClient) IsActive(ctx context.Context, pair string) (bool, error) {
	st, err := c.GetPoolStatus(ctx, pair)
	if err != nil {
		return false, err
	}
	return st.PoolStatus == "ACTIVE", nil
}

// GetPoolReserves implements AMMReserveReader for cross-currency quote generation.
func (c *CentralBankPoolClient) GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error) {
	st, err := c.GetPoolStatus(ctx, pair)
	if err != nil {
		return "", "", 0, err
	}
	return st.ReserveA, st.ReserveB, st.CurrentRatio, nil
}

// GetFeeBps implements AMMReserveReader for cross-currency quote generation.
func (c *CentralBankPoolClient) GetFeeBps(ctx context.Context, pair string) (uint16, error) {
	st, err := c.GetPoolStatus(ctx, pair)
	if err != nil {
		return 0, err
	}
	return uint16(st.FeeRateBps), nil // #nosec G115 -- FeeRateBps is a BPS value (0–10000), always fits in uint16
}

// GetHubLiquidityConfig fetches sovereign AMM deployment info from the spoke CB (T054 option B).
func (c *CentralBankPoolClient) GetHubLiquidityConfig(ctx context.Context) (*HubLiquidityConfig, error) {
	url := c.baseURL + "/api/v2/amm/hub-liquidity-config"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("central bank hub liquidity config: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("central bank hub liquidity config: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("central bank hub liquidity config: read body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("central bank hub liquidity config: not configured on CB (HTTP 404)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("central bank hub liquidity config: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out HubLiquidityConfig
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("central bank hub liquidity config: decode: %w", err)
	}
	if strings.TrimSpace(out.SovereignAMMAddress) == "" {
		return nil, fmt.Errorf("central bank hub liquidity config: empty sovereign_amm_address")
	}
	return &out, nil
}
