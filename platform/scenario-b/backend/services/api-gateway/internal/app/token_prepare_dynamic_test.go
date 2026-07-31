// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"
	"testing"
)

// TestNewDynamicTokenPrepareAdapter verifies the sovereign dynamic preparer is
// built with only the on-chain resolver (no static TOKEN_A/B clients) so the
// sovereign-seed + token routes register in the per-pair deploy without pinning
// a fixed pair via env.
func TestNewDynamicTokenPrepareAdapter(t *testing.T) {
	resolver := &pairAMMResolver{}
	tp := NewDynamicTokenPrepareAdapter(resolver)

	if tp == nil {
		t.Fatal("expected non-nil adapter")
	}
	if tp.resolver != resolver {
		t.Error("expected resolver to be wired for dynamic per-pair resolution")
	}
	if tp.tokenA != nil || tp.tokenB != nil {
		t.Error("dynamic adapter must hold no static TOKEN_A/B clients")
	}
	if !tp.isCB {
		t.Error("dynamic adapter must report isCB=true (side derived per-pair via SideForSigner)")
	}
}

// TestDynamicTokenPrepare_NoPoolPair_NoPanic guards the nil static-token clients:
// a mint/approve without a pool_pair must fail cleanly (pool_pair required), never
// dereference the absent TOKEN_A/B clients.
func TestDynamicTokenPrepare_NoPoolPair_NoPanic(t *testing.T) {
	tp := NewDynamicTokenPrepareAdapter(&pairAMMResolver{})
	ctx := context.Background()

	// Zero amount short-circuits before any token resolution.
	if err := tp.MintAndApproveForAMM(ctx, "", "", "0"); err != nil {
		t.Errorf("zero amount should be a no-op, got %v", err)
	}

	// Positive amount with no pool_pair must return the "pool_pair required" error
	// (no default AMM) rather than panicking on the nil static token clients.
	err := tp.MintAndApproveForAMM(ctx, "", "", "100")
	if err == nil {
		t.Fatal("expected an error when no pool_pair is supplied")
	}
	if !strings.Contains(err.Error(), "pool_pair") {
		t.Errorf("expected a pool_pair-required error, got %v", err)
	}
}
