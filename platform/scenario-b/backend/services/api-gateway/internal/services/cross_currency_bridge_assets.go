// SPDX-License-Identifier: Apache-2.0

package services

import (
	"strings"
)

// CrossCurrencyBridgeAssets holds on-chain addresses for bridge-in/out in cross-currency swaps.
type CrossCurrencyBridgeAssets struct {
	NativeSourceToken  string // ERC-20 on spoke-in (e.g. BRL fiat token)
	WrappedSourceToken string // W-tCeBM on Hub (e.g. W-BRL)
	WrappedTargetToken string // W-tCeBM on Hub (e.g. W-ARS) — used for bridge-out when wired
	SpokeInNetwork     string // e.g. spoke-a
}

// CrossCurrencyBridgeAssetsFromHub resolves wrapped token addresses from CB hub liquidity config.
// nativeSourceToken is the spoke tCeBM token address (TOKEN_ADDRESS for the payer bank).
func CrossCurrencyBridgeAssetsFromHub(
	cfg *HubLiquidityConfig,
	sourceCurrency, targetCurrency, nativeSourceToken, spokeIn string,
) *CrossCurrencyBridgeAssets {
	if cfg == nil {
		return nil
	}
	wrappedIn := wrappedTokenForCurrency(cfg, sourceCurrency)
	wrappedOut := wrappedTokenForCurrency(cfg, targetCurrency)
	if wrappedIn == "" {
		return nil
	}
	if spokeIn == "" {
		spokeIn = "spoke-a"
	}
	return &CrossCurrencyBridgeAssets{
		NativeSourceToken:  strings.TrimSpace(nativeSourceToken),
		WrappedSourceToken: wrappedIn,
		WrappedTargetToken: wrappedOut,
		SpokeInNetwork:     spokeIn,
	}
}

// wrappedTokenForCurrency maps BRL/ARS (and W-* symbols) to sovereign Hub token A/B.
func wrappedTokenForCurrency(cfg *HubLiquidityConfig, currency string) string {
	if cfg == nil {
		return ""
	}
	c := strings.ToUpper(strings.TrimSpace(currency))
	c = strings.TrimPrefix(c, "W-")
	switch c {
	case "BRL":
		return cfg.SovereignHubTokenAAddress
	case "ARS":
		return cfg.SovereignHubTokenBAddress
	default:
		return ""
	}
}
