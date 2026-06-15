// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type fakePreparer struct {
	mintApproveErr error
	mintToErr      error
	approveErr     error
}

func (f *fakePreparer) MintAndApproveForAMM(_ context.Context, _ string) error { return f.mintApproveErr }
func (f *fakePreparer) MintToForAMM(_ context.Context, _, _ string) error      { return f.mintToErr }
func (f *fakePreparer) ApproveAMM(_ context.Context, _, _ string) error        { return f.approveErr }

type fakeCBChecker struct {
	isCB bool
	err  error
}

func (f *fakeCBChecker) IsCentralBankAddress(_ context.Context, _ string) (bool, error) {
	return f.isCB, f.err
}

func tokenApp(h *handlers.TokenHandler) *fiber.App {
	app := fiber.New()
	app.Post("/mint", h.MintAndApprove)
	app.Post("/approve", h.ApproveAMM)
	return app
}

func TestTokenHandler_MintAndApprove_Success(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenHandler_MintAndApprove_DeprecatedField(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount_a":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_MintAndApprove_MissingAmount(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_MintTo_Recipient(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandlerWithCBChecker(&fakePreparer{}, &fakeCBChecker{isCB: false}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount":"100","recipient":"0xabc"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenHandler_MintTo_CrossCBProhibited(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandlerWithCBChecker(&fakePreparer{}, &fakeCBChecker{isCB: true}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount":"100","recipient":"0xCB"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTokenHandler_MintTo_CheckerError(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandlerWithCBChecker(&fakePreparer{}, &fakeCBChecker{err: errors.New("rpc")}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount":"100","recipient":"0xCB"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestTokenHandler_MintAndApprove_PreparerError(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{mintApproveErr: errors.New("revert")}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/mint", `{"amount":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_Success(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{"amount":"100","side":"A"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_InvalidSide(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{"amount":"100","side":"X"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_ConfiguredSideOverrides(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandlerWithConfig(&fakePreparer{}, nil, "B"))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{"amount":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_SideRequiredError(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{approveErr: errors.New("token_prepare: side is required for non-central-bank callers")}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{"amount":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_DeprecatedField(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{"amount_b":"100"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_ApproveAMM_MissingAmount(t *testing.T) {
	app := tokenApp(handlers.NewTokenHandler(&fakePreparer{}))
	resp, _ := app.Test(reqJSON(http.MethodPost, "/approve", `{}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
