// SPDX-License-Identifier: Apache-2.0

// Package api provides the pool stability proxy endpoint for the Scenario B NOC backend.
// GET /api/v1/pools proxies to the api-gateway AMM pool status endpoint for each
// configured pair (AMM_PAIRS env var) and returns a summary shaped for the NOC portal.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/config"
)

// PoolsHandler proxies AMM pool status from the api-gateway for the NOC portal.
type PoolsHandler struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewPoolsHandler creates a PoolsHandler.
func NewPoolsHandler(cfg *config.Config) *PoolsHandler {
	return &PoolsHandler{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register mounts the pools route.
func (h *PoolsHandler) Register(g fiber.Router) {
	g.Get("/pools", h.GetPools)
}

// ammPoolResponse is the JSON returned by the api-gateway GET /api/v2/amm/pool/:pair/status.
// Fields are based on services.PoolStatusResponse in the api-gateway service.
type ammPoolResponse struct {
	PoolPair      string  `json:"pool_pair"`
	PoolStatus    string  `json:"pool_status"`
	ReserveA      string  `json:"reserve_a"`
	ReserveB      string  `json:"reserve_b"`
	CurrentRatio  float64 `json:"current_ratio"`
	ImbalanceFlag bool    `json:"imbalance_flag"`
}

// NocPoolStatus is the response shape sent to the NOC portal.
type NocPoolStatus struct {
	Pair         string  `json:"pair"`
	ReserveA     string  `json:"reserveA"`
	ReserveB     string  `json:"reserveB"`
	RatioA       float64 `json:"ratioA"`
	RatioB       float64 `json:"ratioB"`
	Breached7030 bool    `json:"breached7030"`
	Severity     string  `json:"severity"`
	UpdatedAt    string  `json:"updatedAt"`
}

// GetPools fetches pool stability data for all configured AMM pairs.
func (h *PoolsHandler) GetPools(c *fiber.Ctx) error {
	results := make([]NocPoolStatus, 0, len(h.cfg.AMMPairs))

	for _, pair := range h.cfg.AMMPairs {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		pool, err := h.fetchPoolStatus(ctx, pair)
		cancel()
		if err != nil {
			// Log and skip; partial results are better than total failure.
			_ = fmt.Errorf("pools: fetch %s: %w", pair, err)
			continue
		}
		results = append(results, pool)
	}

	return c.JSON(fiber.Map{"data": results})
}

func (h *PoolsHandler) fetchPoolStatus(ctx context.Context, pair string) (NocPoolStatus, error) {
	gatewayURL := fmt.Sprintf("%s/api/v2/amm/pool/%s/status", h.cfg.AMMGatewayURL, pair)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL, nil)
	if err != nil {
		return NocPoolStatus{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return NocPoolStatus{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NocPoolStatus{}, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return NocPoolStatus{}, fmt.Errorf("gateway returned %d: %s", resp.StatusCode, string(body))
	}

	var amm ammPoolResponse
	if err := json.Unmarshal(body, &amm); err != nil {
		return NocPoolStatus{}, fmt.Errorf("decode: %w", err)
	}

	ratioA, ratioB := computeRatios(amm.ReserveA, amm.ReserveB)
	breached := ratioA > 70 || ratioB > 70
	severity := "INFO"
	if breached {
		severity = "CRITICAL"
	}

	return NocPoolStatus{
		Pair:         pair,
		ReserveA:     amm.ReserveA,
		ReserveB:     amm.ReserveB,
		RatioA:       ratioA,
		RatioB:       ratioB,
		Breached7030: breached,
		Severity:     severity,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// computeRatios derives percentage ratios from two reserve string values.
// Returns (0, 0) when both reserves are zero or unparseable.
func computeRatios(reserveA, reserveB string) (float64, float64) {
	a := parseBigInt(reserveA)
	b := parseBigInt(reserveB)

	total := new(big.Int).Add(a, b)
	if total.Sign() == 0 {
		return 0, 0
	}

	// Compute ratioA = a * 10000 / total (in basis points, then divide by 100 for pct)
	aScaled := new(big.Int).Mul(a, big.NewInt(10000))
	bpsA := new(big.Int).Div(aScaled, total)

	ratioA := float64(bpsA.Int64()) / 100.0
	ratioB := 100.0 - ratioA
	return ratioA, ratioB
}

func parseBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}
