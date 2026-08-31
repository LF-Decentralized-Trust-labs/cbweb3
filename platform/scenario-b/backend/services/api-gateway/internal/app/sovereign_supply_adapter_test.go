// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// fakeCurrencies is a CurrencyServiceIface that only serves the read path; the write
// methods exist to satisfy the interface and must never be reached from a view endpoint.
type fakeCurrencies struct {
	entries []domain.CurrencyEntry
	err     error
}

func (f *fakeCurrencies) ListCurrencies(context.Context) ([]domain.CurrencyEntry, error) {
	return f.entries, f.err
}

func (f *fakeCurrencies) RegisterCurrency(context.Context, services.CurrencyRegisterRequest) (*services.CurrencyRegisterResult, error) {
	return nil, errors.New("unexpected write from the supply reader")
}

func (f *fakeCurrencies) RemoveCurrency(context.Context, services.CurrencyRemoveRequest) (*services.CurrencyRemoveResult, error) {
	return nil, errors.New("unexpected write from the supply reader")
}

const hubTokenBRL = "0x686afd6e502a81d2e77f2e038a23c0def4949a20"

func brlRegistry() *fakeCurrencies {
	return &fakeCurrencies{entries: []domain.CurrencyEntry{
		{Symbol: "W-tCeBM_ARS", TokenAddress: "0x4e72770760c011647d4873f60a3cf6cdea896cd8"},
		{Symbol: "W-tCeBM_BRL", TokenAddress: hubTokenBRL},
	}}
}

// The hub names a wrapped token after the currency alone ("W-tCeBM_"+currency): the
// spoke's own token symbol never reaches it. A spoke that deploys its tCeBM under a
// different prefix must therefore still resolve to the same hub token — otherwise the
// Dashboard card is a permanent dash on a perfectly healthy hub.
func TestSovereignSupplyAdapter_LookupIgnoresSpokeTokenPrefix(t *testing.T) {
	for _, native := range []string{"tCeBM_BRL", "tRD_BRL", "tcebm_brl"} {
		a := newSovereignSupplyAdapter(brlRegistry(), "http://hub:8545", native, 0)
		if a == nil {
			t.Fatalf("adapter must be built for NATIVE_ASSET_SYMBOL=%q", native)
		}
		symbol, addr, err := a.lookup(context.Background())
		if err != nil {
			t.Fatalf("lookup(%q): %v", native, err)
		}
		if symbol != "W-tCeBM_BRL" || addr != hubTokenBRL {
			t.Fatalf("lookup(%q) = %q/%q, want W-tCeBM_BRL/%s", native, symbol, addr, hubTokenBRL)
		}
	}
}

// The reported symbol is the one actually on-chain, so the card labels itself with what
// the hub registered rather than with what this gateway expected.
func TestSovereignSupplyAdapter_LookupFallsBackToRegisteredSymbol(t *testing.T) {
	reg := &fakeCurrencies{entries: []domain.CurrencyEntry{
		{Symbol: "W-tCeBMv2_BRL", TokenAddress: hubTokenBRL},
	}}
	symbol, addr, err := newSovereignSupplyAdapter(reg, "http://hub:8545", "tCeBM_BRL", 0).lookup(context.Background())
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if symbol != "W-tCeBMv2_BRL" || addr != hubTokenBRL {
		t.Fatalf("lookup = %q/%q, want W-tCeBMv2_BRL/%s", symbol, addr, hubTokenBRL)
	}
}

// An unregistered currency is an error, never a zero: a fabricated zero reads as
// "nothing is bridged". The message names the currency, not a symbol the operator
// would then hunt for in the registry.
func TestSovereignSupplyAdapter_LookupUnregisteredCurrency(t *testing.T) {
	reg := &fakeCurrencies{entries: []domain.CurrencyEntry{
		{Symbol: "W-tCeBM_ARS", TokenAddress: "0x4e72770760c011647d4873f60a3cf6cdea896cd8"},
	}}
	_, _, err := newSovereignSupplyAdapter(reg, "http://hub:8545", "tCeBM_BRL", 0).lookup(context.Background())
	if err == nil {
		t.Fatal("lookup must fail when this CB's currency is not registered")
	}
	if !strings.Contains(err.Error(), "BRL") {
		t.Fatalf("error must name the currency: %v", err)
	}
}

// A registry read failure propagates: it is not the same as "no such currency", and the
// caller turns it into a 502 rather than caching a wrong answer.
func TestSovereignSupplyAdapter_LookupPropagatesRegistryError(t *testing.T) {
	reg := &fakeCurrencies{err: errors.New("connection refused")}
	if _, _, err := newSovereignSupplyAdapter(reg, "http://hub:8545", "tCeBM_BRL", 0).lookup(context.Background()); err == nil {
		t.Fatal("a CurrencyRegistry failure must propagate")
	}
}

// Missing configuration leaves the route unregistered instead of serving errors.
func TestNewSovereignSupplyAdapter_DisabledWithoutConfig(t *testing.T) {
	if a := newSovereignSupplyAdapter(nil, "http://hub:8545", "tCeBM_BRL", 0); a != nil {
		t.Fatal("no CurrencyService → adapter must be nil")
	}
	if a := newSovereignSupplyAdapter(brlRegistry(), "", "tCeBM_BRL", 0); a != nil {
		t.Fatal("no hub RPC URL → adapter must be nil")
	}
	if a := newSovereignSupplyAdapter(brlRegistry(), "http://hub:8545", "  ", 0); a != nil {
		t.Fatal("no NATIVE_ASSET_SYMBOL → adapter must be nil")
	}
}
