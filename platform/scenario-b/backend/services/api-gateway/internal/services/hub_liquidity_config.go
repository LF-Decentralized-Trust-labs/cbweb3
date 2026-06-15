// SPDX-License-Identifier: Apache-2.0

package services

import (
	"encoding/json"
	"os"
	"strings"
)

// HubLiquidityConfig describes the Hub sovereign AMM deployment (T054 / FR-012 option B).
// Exposed by CB gateways via GET /api/v2/amm/hub-liquidity-config for commercial banks.
type HubLiquidityConfig struct {
	SovereignAMMAddress        string            `json:"sovereign_amm_address"`
	SovereignHubTokenAAddress  string            `json:"sovereign_hub_token_a_address,omitempty"`
	SovereignHubTokenBAddress  string            `json:"sovereign_hub_token_b_address,omitempty"`
	DefaultSovereignPoolPair   string            `json:"default_sovereign_pool_pair,omitempty"`
	SovereignPairAMMMap        map[string]string `json:"sovereign_pair_amm_map,omitempty"`
}

// HubLiquidityConfigFromEnv builds config when this gateway is a CB with sovereign liquidity wired.
// Returns nil if SOVEREIGN_AMM_ADDRESS is unset (commercial gateways).
func HubLiquidityConfigFromEnv() *HubLiquidityConfig {
	amm := strings.TrimSpace(os.Getenv("SOVEREIGN_AMM_ADDRESS"))
	if amm == "" {
		return nil
	}
	cfg := &HubLiquidityConfig{
		SovereignAMMAddress:       amm,
		SovereignHubTokenAAddress: strings.TrimSpace(os.Getenv("SOVEREIGN_HUB_TOKEN_A_ADDRESS")),
		SovereignHubTokenBAddress: strings.TrimSpace(os.Getenv("SOVEREIGN_HUB_TOKEN_B_ADDRESS")),
		DefaultSovereignPoolPair:  strings.TrimSpace(os.Getenv("SOVEREIGN_PAIR_ID")),
	}
	if cfg.DefaultSovereignPoolPair == "" {
		cfg.DefaultSovereignPoolPair = "W-BRL-ARS"
	}
	if raw := strings.TrimSpace(os.Getenv("SOVEREIGN_PAIR_AMM_MAP")); raw != "" && raw != "{}" {
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err == nil && len(m) > 0 {
			cfg.SovereignPairAMMMap = m
		}
	}
	return cfg
}

// AMMAddressForPair returns the sovereign AMM contract for a pool pair (map lookup, then default).
func (c *HubLiquidityConfig) AMMAddressForPair(pair string) string {
	if c == nil {
		return ""
	}
	key := normalizeSovereignPoolPair(pair)
	if c.SovereignPairAMMMap != nil {
		if addr := strings.TrimSpace(c.SovereignPairAMMMap[key]); addr != "" {
			return addr
		}
	}
	return c.SovereignAMMAddress
}
