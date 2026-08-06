// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSupplyReader struct {
	supply *handlers.SovereignSupply
	err    error
}

func (f *fakeSupplyReader) SovereignSupply(_ context.Context) (*handlers.SovereignSupply, error) {
	return f.supply, f.err
}

func supplyApp(r handlers.SovereignSupplyReader) *fiber.App {
	app := fiber.New()
	app.Get("/supply", handlers.NewTokenSupplyHandler(r).GetSovereignSupply)
	return app
}

func TestTokenSupplyHandler_ReturnsSovereignSupply(t *testing.T) {
	app := supplyApp(&fakeSupplyReader{supply: &handlers.SovereignSupply{
		Symbol:       "W-tCeBM_BRL",
		TokenAddress: "0x686afd6e502a81d2e77f2e038a23c0def4949a20",
		TotalSupply:  "1005025125628140703518",
		Decimals:     18,
	}})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/supply", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got handlers.SovereignSupply
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "W-tCeBM_BRL", got.Symbol)
	assert.Equal(t, "1005025125628140703518", got.TotalSupply)
	assert.Equal(t, uint8(18), got.Decimals)
}

// A hub RPC failure must surface as 502. Reporting 200 with a zero supply would read
// as "nothing is bridged" to a supervisor — the exact defect this endpoint replaces.
func TestTokenSupplyHandler_ReaderErrorIsBadGateway(t *testing.T) {
	app := supplyApp(&fakeSupplyReader{err: errors.New("dial hub token: connection refused")})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/supply", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "read sovereign token supply")
	assert.NotContains(t, string(body), "total_supply")
}
