// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CurrencyHandler
// ---------------------------------------------------------------------------

type mockCurrencySvc struct {
	registerErr error
	removeErr   error
	list        []domain.CurrencyEntry
	listErr     error
}

func (m *mockCurrencySvc) RegisterCurrency(_ context.Context, req services.CurrencyRegisterRequest) (*services.CurrencyRegisterResult, error) {
	if m.registerErr != nil {
		return nil, m.registerErr
	}
	return &services.CurrencyRegisterResult{Symbol: req.Symbol, TxHash: "0xtx"}, nil
}
func (m *mockCurrencySvc) RemoveCurrency(_ context.Context, req services.CurrencyRemoveRequest) (*services.CurrencyRemoveResult, error) {
	if m.removeErr != nil {
		return nil, m.removeErr
	}
	return &services.CurrencyRemoveResult{Symbol: req.Symbol, TxHash: "0xrm"}, nil
}
func (m *mockCurrencySvc) ListCurrencies(_ context.Context) ([]domain.CurrencyEntry, error) {
	return m.list, m.listErr
}

func currencyApp(svc services.CurrencyServiceIface) *fiber.App {
	h := handlers.NewCurrencyHandler(svc)
	app := fiber.New()
	app.Post("/c", h.RegisterCurrency)
	app.Delete("/c/:symbol", h.RemoveCurrency)
	app.Get("/c", h.ListCurrencies)
	return app
}

func TestCurrencyHandler_Register(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{})
	body := `{"symbol":"BRL","country_name":"Brazil","token_address":"0xabc","proposer_cb":"cb"}`
	resp, _ := app.Test(reqJSON(http.MethodPost, "/c", body), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCurrencyHandler_Register_BadBody(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/c", "not json"), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCurrencyHandler_Register_MissingFields(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/c", `{"symbol":"BRL"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCurrencyHandler_Register_Conflict(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{registerErr: services.ErrCurrencyAlreadyExists})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/c", `{"symbol":"BRL","country_name":"B","token_address":"0x","proposer_cb":"cb"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCurrencyHandler_Remove(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{})
	resp, _ := app.Test(httptest.NewRequest(http.MethodDelete, "/c/BRL", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCurrencyHandler_Remove_NotFound(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{removeErr: services.ErrCurrencyNotFound})
	resp, _ := app.Test(httptest.NewRequest(http.MethodDelete, "/c/BRL", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCurrencyHandler_List(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{list: []domain.CurrencyEntry{{Symbol: "BRL"}}})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/c", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCurrencyHandler_List_Error(t *testing.T) {
	app := currencyApp(&mockCurrencySvc{listErr: errors.New("rpc")})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/c", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// PairHandler
// ---------------------------------------------------------------------------

type mockPairSvc struct {
	proposeErr error
	confirmErr error
	list       []domain.PairProposal
	listErr    error
}

func (m *mockPairSvc) ProposePair(_ context.Context, req services.PairProposeRequest) (*services.PairProposeResult, error) {
	if m.proposeErr != nil {
		return nil, m.proposeErr
	}
	return &services.PairProposeResult{PairID: req.PairID, Status: domain.PairStatusProposed, TxHash: "0x"}, nil
}
func (m *mockPairSvc) ConfirmPair(_ context.Context, req services.PairConfirmRequest) (*services.PairConfirmResult, error) {
	if m.confirmErr != nil {
		return nil, m.confirmErr
	}
	return &services.PairConfirmResult{PairID: req.PairID, Status: domain.PairStatusActive, TxHash: "0x"}, nil
}
func (m *mockPairSvc) ListActivePairs(_ context.Context) ([]domain.PairProposal, error) {
	return m.list, m.listErr
}

func pairApp(svc services.PairServiceIface) *fiber.App {
	h := handlers.NewPairHandler(svc)
	app := fiber.New()
	app.Post("/p/propose", h.ProposePair)
	app.Post("/p/confirm", h.ConfirmPair)
	app.Get("/p", h.ListPairs)
	return app
}

func TestPairHandler_Propose(t *testing.T) {
	app := pairApp(&mockPairSvc{})
	body := `{"pair_id":"BRL-USD","token_a_address":"0xa","token_b_address":"0xb","amm_address":"0xamm","proposer_cb":"cb"}`
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/propose", body), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestPairHandler_Propose_MissingFields(t *testing.T) {
	app := pairApp(&mockPairSvc{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/propose", `{"pair_id":"X"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPairHandler_Propose_Conflict(t *testing.T) {
	app := pairApp(&mockPairSvc{proposeErr: services.ErrPairAlreadyExists})
	body := `{"pair_id":"BRL-USD","token_a_address":"0xa","token_b_address":"0xb","amm_address":"0xamm","proposer_cb":"cb"}`
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/propose", body), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestPairHandler_Confirm(t *testing.T) {
	app := pairApp(&mockPairSvc{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/confirm", `{"pair_id":"BRL-USD","confirmer_cb":"cb-b"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPairHandler_Confirm_MissingFields(t *testing.T) {
	app := pairApp(&mockPairSvc{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/confirm", `{}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPairHandler_Confirm_Forbidden(t *testing.T) {
	app := pairApp(&mockPairSvc{confirmErr: services.ErrNotCentralBankOfTokenB})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/p/confirm", `{"pair_id":"X","confirmer_cb":"cb"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPairHandler_List(t *testing.T) {
	app := pairApp(&mockPairSvc{list: []domain.PairProposal{{PairID: "BRL-USD", Status: domain.PairStatusActive}}})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/p", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPairHandler_List_Error(t *testing.T) {
	app := pairApp(&mockPairSvc{listErr: errors.New("db")})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/p", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// CrossCurrencySwapHandler
// ---------------------------------------------------------------------------

type mockOrchestrator struct {
	result    *services.CrossCurrencySwapResult
	execErr   error
	statusErr error
}

func (m *mockOrchestrator) Execute(_ context.Context, req services.CrossCurrencySwapRequest) (*services.CrossCurrencySwapResult, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	if m.result != nil {
		return m.result, nil
	}
	return &services.CrossCurrencySwapResult{SwapID: req.SwapID, CorrelationID: req.CorrelationID, Status: domain.SwapStatusCompleted, CreatedAt: time.Now()}, nil
}
func (m *mockOrchestrator) GetStatus(_ context.Context, swapID string) (*services.CrossCurrencySwapResult, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	return &services.CrossCurrencySwapResult{SwapID: swapID, Status: domain.SwapStatusCompleted, CreatedAt: time.Now()}, nil
}

// withClaims injects authenticated claims so the handler passes the auth gate.
func swapApp(orch handlers.CrossCurrencySwapOrchestratorIface, injectClaims bool) *fiber.App {
	h := handlers.NewCrossCurrencySwapHandler(orch, "fallback-bank")
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if injectClaims {
			c.Locals("claims", domain.TokenClaims{BankID: "bank-a"})
		}
		return c.Next()
	})
	app.Post("/swap", h.SwapCrossCurrency)
	app.Get("/swap/:id", h.GetSwapStatus)
	return app
}

func validCrossSwapBody() string {
	return `{"source_currency":"BRL","target_currency":"ARS","pool_pair":"W-BRL-ARS","amount_out":"100","max_amount_in":"1000","beneficiary_bank_id":"bank-b"}`
}

func TestCrossCurrencySwap_Success(t *testing.T) {
	app := swapApp(&mockOrchestrator{}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCrossCurrencySwap_BadBody(t *testing.T) {
	app := swapApp(&mockOrchestrator{}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", "not json"), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCrossCurrencySwap_MissingFields(t *testing.T) {
	app := swapApp(&mockOrchestrator{}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", `{"source_currency":"BRL"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCrossCurrencySwap_Unauthenticated(t *testing.T) {
	app := swapApp(&mockOrchestrator{}, false) // no claims injected
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCrossCurrencySwap_PoolNotActiveError(t *testing.T) {
	app := swapApp(&mockOrchestrator{execErr: &domain.SwapExecError{Code: domain.ErrCodePoolNotActive}}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestCrossCurrencySwap_SlippageError(t *testing.T) {
	app := swapApp(&mockOrchestrator{execErr: &domain.SwapExecError{Code: domain.ErrCodeSlippageLimitExceeded}}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestCrossCurrencySwap_TransferLimitError(t *testing.T) {
	app := swapApp(&mockOrchestrator{execErr: &services.ErrTransferLimitExceeded{PayerBankID: "bank-a", Currency: "BRL"}}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestCrossCurrencySwap_BridgeOutPartialFailure(t *testing.T) {
	app := swapApp(&mockOrchestrator{execErr: errors.New("bridge-out failed (swap succeeded)")}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCrossCurrencySwap_GenericError(t *testing.T) {
	app := swapApp(&mockOrchestrator{execErr: errors.New("something else")}, true)
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", validCrossSwapBody()), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCrossCurrencySwap_GetStatus(t *testing.T) {
	app := swapApp(&mockOrchestrator{}, true)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/swap/swap-1", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCrossCurrencySwap_GetStatus_NotFound(t *testing.T) {
	app := swapApp(&mockOrchestrator{statusErr: errors.New("swap not found")}, true)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/swap/missing", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCrossCurrencySwap_GetStatus_InternalError(t *testing.T) {
	app := swapApp(&mockOrchestrator{statusErr: errors.New("db exploded")}, true)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/swap/x", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// reqJSON builds a JSON request with the content-type header set.
func reqJSON(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

var _ = require.NoError
