package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- mock service ---

type mockTransferLimitService struct {
	created *domain.TransferLimit
	listed  []domain.TransferLimit
	createErr error
	listErr   error
	deleteErr error
}

func (m *mockTransferLimitService) Create(_ context.Context, _, _, _, _ string) (*domain.TransferLimit, error) {
	return m.created, m.createErr
}
func (m *mockTransferLimitService) List(_ context.Context, _ string) ([]domain.TransferLimit, error) {
	return m.listed, m.listErr
}
func (m *mockTransferLimitService) Delete(_ context.Context, _, _ string) error {
	return m.deleteErr
}

// --- helpers ---

func newLimitFiber(svc handlers.TransferLimitServiceIface, cbBankID string) *fiber.App {
	app := fiber.New()
	h := handlers.NewTransferLimitHandler(svc)
	// inject claims via middleware stub
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{BankID: cbBankID})
		return c.Next()
	})
	app.Post("/transfer-limits", h.CreateTransferLimit)
	app.Get("/transfer-limits", h.ListTransferLimits)
	app.Delete("/transfer-limits/:id", h.DeleteTransferLimit)
	return app
}

func postJSON(app *fiber.App, path string, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return app.Test(req, -1)
}

// --- handler tests ---

func TestTransferLimitHandler_Create_Success(t *testing.T) {
	svc := &mockTransferLimitService{
		created: &domain.TransferLimit{
			LimitID:       "limit-1",
			CentralBankID: "central-bank-a",
			ParticipantID: "bank-a",
			Currency:      "BRL",
			MaxAmount:     "1000000000000000000000000",
			IsActive:      true,
		},
	}
	app := newLimitFiber(svc, "central-bank-a")

	resp, err := postJSON(app, "/transfer-limits", map[string]string{
		"participant_id": "bank-a",
		"currency":       "BRL",
		"max_amount":     "1000000",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestTransferLimitHandler_Create_MissingMaxAmount(t *testing.T) {
	app := newLimitFiber(&mockTransferLimitService{}, "central-bank-a")

	resp, err := postJSON(app, "/transfer-limits", map[string]string{
		"participant_id": "bank-a",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTransferLimitHandler_Create_SpokeMismatch(t *testing.T) {
	app := newLimitFiber(&mockTransferLimitService{}, "central-bank-a")

	resp, err := postJSON(app, "/transfer-limits", map[string]string{
		"participant_id": "bank-b", // -b spoke, but CB is central-bank-a (-a spoke)
		"max_amount":     "1000000",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTransferLimitHandler_Create_MissingCBIdentity(t *testing.T) {
	// No claims injected — BankID is empty
	app := fiber.New()
	h := handlers.NewTransferLimitHandler(&mockTransferLimitService{})
	app.Post("/transfer-limits", h.CreateTransferLimit)

	resp, err := postJSON(app, "/transfer-limits", map[string]string{"max_amount": "1000000"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTransferLimitHandler_List_Success(t *testing.T) {
	svc := &mockTransferLimitService{
		listed: []domain.TransferLimit{
			{LimitID: "limit-1", CentralBankID: "central-bank-a", MaxAmount: "1000"},
		},
	}
	app := newLimitFiber(svc, "central-bank-a")

	req := httptest.NewRequest(http.MethodGet, "/transfer-limits", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTransferLimitHandler_Delete_Success(t *testing.T) {
	app := newLimitFiber(&mockTransferLimitService{}, "central-bank-a")

	req := httptest.NewRequest(http.MethodDelete, "/transfer-limits/limit-1", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestTransferLimitHandler_Delete_NotFound(t *testing.T) {
	svc := &mockTransferLimitService{deleteErr: gorm.ErrRecordNotFound}
	app := newLimitFiber(svc, "central-bank-a")

	req := httptest.NewRequest(http.MethodDelete, "/transfer-limits/missing-id", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTransferLimitHandler_Delete_ServiceError(t *testing.T) {
	svc := &mockTransferLimitService{deleteErr: errors.New("db error")}
	app := newLimitFiber(svc, "central-bank-a")

	req := httptest.NewRequest(http.MethodDelete, "/transfer-limits/limit-1", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}
