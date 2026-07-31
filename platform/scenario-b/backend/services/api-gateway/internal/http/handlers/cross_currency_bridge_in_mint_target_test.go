// SPDX-License-Identifier: Apache-2.0

// Package handlers tests where the issuing CB mints the W-<source> on a delegated bridge-in.
//
// Once a bank delegates the AMM swap too, it holds no Hub key and names no mint address. The
// CB must then mint to itself, because the CB is what spends the tokens in Step 2. Leaving the
// target empty would fall through to the executor's HUB_MINT_RECIPIENT default, which in some
// stacks is a different address than the swap signer — the swap would revert for insufficient
// balance with nothing pointing at the cause.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLockMintEnqueuer struct {
	mintTo string
	calls  int
}

func (s *stubLockMintEnqueuer) LockAndEnqueue(
	_ context.Context, _, _, _, _, _, _ string, extras ...string,
) (*services.BridgePositionResult, error) {
	s.calls++
	if len(extras) > 0 {
		s.mintTo = extras[0]
	}
	return &services.BridgePositionResult{PositionID: "pos-1"}, nil
}

type stubBridgeStateActive struct{}

func (stubBridgeStateActive) GetBridgeState(context.Context, string) (domain.BridgeState, error) {
	return domain.BridgeStateActive, nil
}

func postBridgeIn(t *testing.T, cbHubAddr, swapSender string) *stubLockMintEnqueuer {
	t.Helper()
	enq := &stubLockMintEnqueuer{}
	h := handlers.NewCrossCurrencyBridgeInHandler(
		enq, stubBridgeStateActive{}, "0xWTOKEN", "0xFIAT", "spoke-brl",
	).WithHubSignerAddress(cbHubAddr)

	app := fiber.New()
	app.Post("/internal/amm/cross-currency-bridge-in", h.HandleBridgeIn)

	body := `{"correlation_id":"corr-1","payer_bank_id":"bank-a","source_currency":"BRL",` +
		`"amount":"1000","spoke_in":"spoke-brl","swap_sender_address":"` + swapSender + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/amm/cross-currency-bridge-in", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, enq.calls)
	return enq
}

func TestBridgeIn_MintsToTheCBWhenTheBankNamesNoAddress(t *testing.T) {
	enq := postBridgeIn(t, "0xCBHUB", "")
	assert.Equal(t, "0xCBHUB", enq.mintTo,
		"a keyless delegating bank must have the W-<source> minted to the CB that will spend it")
}

func TestBridgeIn_HonoursAnExplicitMintTarget(t *testing.T) {
	// A gateway that still signs on the Hub itself names its own address; that must win, so
	// the pre-delegation behaviour is unchanged.
	enq := postBridgeIn(t, "0xCBHUB", "0xLEGACYSIGNER")
	assert.Equal(t, "0xLEGACYSIGNER", enq.mintTo)
}
