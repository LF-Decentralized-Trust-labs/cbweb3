// SPDX-License-Identifier: Apache-2.0

// Package handlers tests that a replayed bridge-in notification does not charge the payer
// bank twice (R2-CR-6 follow-up).
//
// The other bridge-in handler tests drive a stub enqueuer, which is right for what they
// assert (which address the mint targets). Replay protection lives one layer down, in the
// service and its unique index, so a stub would prove nothing here: this test wires the real
// BridgeLockMintService over SQLite and replays the HTTP call the relay actually makes.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/glebarez/sqlite" // pure-Go (no CGO) sqlite driver, test-only
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// realLockMintEnqueuer adapts the concrete service to the handler's interface: the handler
// takes extras variadically, the service signature is the same shape.
type realLockMintEnqueuer struct {
	svc *services.BridgeLockMintService
}

func (r realLockMintEnqueuer) LockAndEnqueue(
	ctx context.Context,
	ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
	extras ...string,
) (*services.BridgePositionResult, error) {
	return r.svc.LockAndEnqueue(ctx, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID, extras...)
}

func newBridgeInApp(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.BridgedAssetPosition{}, &domain.RelayerQueueItem{}))

	h := handlers.NewCrossCurrencyBridgeInHandler(
		realLockMintEnqueuer{svc: services.NewBridgeLockMintService(db)},
		stubBridgeStateActive{},
		"0xWTOKEN", "0xFIAT", "spoke-brl",
	).WithHubSignerAddress("0xCBHUB")

	app := fiber.New()
	app.Post("/internal/amm/cross-currency-bridge-in", asVerifiedCaller("bank-a"), h.HandleBridgeIn)
	return app, db
}

func postBridgeInReplay(t *testing.T, app *fiber.App, correlationID string) map[string]any {
	t.Helper()
	body := `{"correlation_id":"` + correlationID + `","payer_bank_id":"bank-a","source_currency":"BRL",` +
		`"amount":"1000","spoke_in":"spoke-brl","swap_sender_address":"0xSWAPSENDER"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/amm/cross-currency-bridge-in", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// The relay retries a notification whose first attempt was slow. Before the fix each retry
// created its own position, and every position is one spoke-side burn of the bank's tCeBM
// plus one Hub mint — so the bank paid twice for one payment.
func TestBridgeInHandler_ReplayedNotificationDoesNotChargeTwice(t *testing.T) {
	app, db := newBridgeInApp(t)

	first := postBridgeInReplay(t, app, "corr-http-replay")
	second := postBridgeInReplay(t, app, "corr-http-replay")

	require.Equal(t, first["position_id"], second["position_id"],
		"the replay must return the existing position, not open a second one")

	var positions, queued int64
	db.Model(&domain.BridgedAssetPosition{}).Where("correlation_id = ?", "corr-http-replay").Count(&positions)
	db.Model(&domain.RelayerQueueItem{}).Where("event_type = ?", "LOCK_MINT").Count(&queued)
	require.EqualValues(t, 1, positions, "a replay created a second bridged position")
	require.EqualValues(t, 1, queued, "a replay enqueued a second lock-mint — the relayer would burn twice")
}

// Two genuinely different payments from the same bank must both proceed.
func TestBridgeInHandler_DistinctCorrelationsAreIndependent(t *testing.T) {
	app, db := newBridgeInApp(t)

	first := postBridgeInReplay(t, app, "corr-payment-1")
	second := postBridgeInReplay(t, app, "corr-payment-2")

	require.NotEqual(t, first["position_id"], second["position_id"])

	var positions int64
	db.Model(&domain.BridgedAssetPosition{}).Count(&positions)
	require.EqualValues(t, 2, positions)
}
