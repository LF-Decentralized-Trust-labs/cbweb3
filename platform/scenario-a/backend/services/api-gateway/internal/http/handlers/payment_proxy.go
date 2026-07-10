// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/gofiber/fiber/v2"
)

// PaymentProxyHandler forwards escrow-related requests from a commercial bank
// gateway to the Central Bank's payment endpoints. For request-redeem the proxy
// first initiates a local Zeto transfer and enriches the payload with the
// resulting transaction hash before forwarding.
type PaymentProxyHandler struct {
	client            *http.Client
	baseURL           string // Central Bank API base URL
	payment           *paymentadapter.GRPCAdapter
	entityBesuAddress string // this entity's Besu address
	paladinIdentity   string // this entity's Paladin identity
	cbPaladinIdentity string // Central Bank's Paladin identity (Zeto transfer receiver)
	relayAuthSecret   string // shared secret for X-Relay-Auth header on internal endpoints
}

// NewPaymentProxyHandler creates a proxy handler targeting the given Central
// Bank URL. The payment adapter is used for local Zeto operations.
func NewPaymentProxyHandler(
	centralBankURL string,
	payment *paymentadapter.GRPCAdapter,
	entityBesuAddress, paladinIdentity, cbPaladinIdentity, relayAuthSecret string,
) *PaymentProxyHandler {
	return &PaymentProxyHandler{
		client:            &http.Client{Timeout: 30 * time.Second},
		baseURL:           strings.TrimRight(centralBankURL, "/"),
		payment:           payment,
		entityBesuAddress: entityBesuAddress,
		paladinIdentity:   paladinIdentity,
		cbPaladinIdentity: cbPaladinIdentity,
		relayAuthSecret:   relayAuthSecret,
	}
}

// RegisterDeposit proxies POST /payments/deposits to the Central Bank.
func (h *PaymentProxyHandler) RegisterDeposit(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/deposits")
}

// ListDeposits proxies GET /payments/deposits to the Central Bank,
// always scoped to this entity's Besu address so commercial banks only
// see their own issuance requests.
func (h *PaymentProxyHandler) ListDeposits(c *fiber.Ctx) error {
	path := "/internal/v1/payments/deposits?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	return h.proxy(c, http.MethodGet, path, nil)
}

// RequestEscrow proxies POST /payments/escrows to the Central Bank.
func (h *PaymentProxyHandler) RequestEscrow(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/escrows")
}

// ListEscrows proxies GET /payments/escrows to the Central Bank,
// always scoped to this entity's Besu address so commercial banks only
// see their own reserve tokenisation requests.
func (h *PaymentProxyHandler) ListEscrows(c *fiber.Ctx) error {
	path := "/internal/v1/payments/escrows?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	return h.proxy(c, http.MethodGet, path, nil)
}

// RequestRedeem first initiates a local Zeto transfer to the Central Bank,
// then proxies the redeem request enriched with the transfer tx hash.
func (h *PaymentProxyHandler) RequestRedeem(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	amountStr, _ := body["amount"].(string)
	if amountStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount is required"})
	}

	// Initiate Zeto transfer from this entity to the Central Bank.
	zetoResult, err := h.payment.InitiateZetoTransfer(
		c.Context(),
		h.cbPaladinIdentity,
		amountStr,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("initiate Zeto transfer: %v", err),
		})
	}

	// Enrich payload with entity addresses and Zeto tx hash.
	body["requester_besu_address"] = h.entityBesuAddress
	body["requester_paladin_identity"] = h.paladinIdentity
	body["zeto_transfer_tx_hash"] = zetoResult.TxHash

	enriched, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "marshal enriched payload"})
	}

	return h.proxy(c, http.MethodPost, "/internal/v1/payments/redeems", enriched)
}

// ListRedeems proxies GET /payments/redeems to the Central Bank,
// always scoped to this entity's Besu address so commercial banks only
// see their own redeem requests.
func (h *PaymentProxyHandler) ListRedeems(c *fiber.Ctx) error {
	path := "/internal/v1/payments/redeems?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	return h.proxy(c, http.MethodGet, path, nil)
}

// FetchDeposits retrieves this entity's deposit records from the Central Bank
// (decoded, unlike the passthrough ListDeposits handler), scoped to the entity's
// own Besu address. Used by the statement handler to consolidate movements.
func (h *PaymentProxyHandler) FetchDeposits(ctx context.Context) ([]paymentadapter.DepositRecord, error) {
	var env struct {
		Deposits []paymentadapter.DepositRecord `json:"deposits"`
	}
	path := "/internal/v1/payments/deposits?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	if err := h.getInternalJSON(ctx, path, &env); err != nil {
		return nil, err
	}
	return env.Deposits, nil
}

// FetchEscrows retrieves this entity's reserve-tokenisation records from the
// Central Bank, scoped to the entity's own Besu address.
func (h *PaymentProxyHandler) FetchEscrows(ctx context.Context) ([]paymentadapter.EscrowRecord, error) {
	var env struct {
		Escrows []paymentadapter.EscrowRecord `json:"escrows"`
	}
	path := "/internal/v1/payments/escrows?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	if err := h.getInternalJSON(ctx, path, &env); err != nil {
		return nil, err
	}
	return env.Escrows, nil
}

// FetchRedeems retrieves this entity's redeem records from the Central Bank,
// scoped to the entity's own Besu address.
func (h *PaymentProxyHandler) FetchRedeems(ctx context.Context) ([]paymentadapter.RedeemRecord, error) {
	var env struct {
		Redeems []paymentadapter.RedeemRecord `json:"redeems"`
	}
	path := "/internal/v1/payments/redeems?requester_id=" + url.QueryEscape(h.entityBesuAddress)
	if err := h.getInternalJSON(ctx, path, &env); err != nil {
		return nil, err
	}
	return env.Redeems, nil
}

// getInternalJSON performs a relay-authenticated GET against the Central Bank's
// internal API and decodes the JSON response into out.
func (h *PaymentProxyHandler) getInternalJSON(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if h.relayAuthSecret != "" {
		req.Header.Set("X-Relay-Auth", h.relayAuthSecret)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("central bank unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("central bank returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// proxyWithEntityEnrichment injects entity Besu address and Paladin identity
// into the request body before proxying to the Central Bank.
func (h *PaymentProxyHandler) proxyWithEntityEnrichment(c *fiber.Ctx, method, path string) error {
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	body["requester_besu_address"] = h.entityBesuAddress
	body["requester_paladin_identity"] = h.paladinIdentity

	enriched, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "marshal enriched payload"})
	}

	return h.proxy(c, http.MethodPost, path, enriched)
}

// proxy forwards an HTTP request to the Central Bank and relays the response.
func (h *PaymentProxyHandler) proxy(c *fiber.Ctx, method, path string, body []byte) error {
	url := h.baseURL + path

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(c.UserContext(), method, url, bodyReader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("proxy: build request: %v", err),
		})
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if corrID := c.Get("X-Correlation-Id"); corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}
	if h.relayAuthSecret != "" {
		req.Header.Set("X-Relay-Auth", h.relayAuthSecret)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": fmt.Sprintf("proxy: central bank unreachable: %v", err),
		})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "proxy: failed to read central bank response",
		})
	}

	c.Set("Content-Type", resp.Header.Get("Content-Type"))
	return c.Status(resp.StatusCode).Send(respBody)
}
