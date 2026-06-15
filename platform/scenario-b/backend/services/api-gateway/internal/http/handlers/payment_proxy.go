// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PaymentProxyHandler forwards escrow-related requests from a commercial bank
// gateway to the Central Bank's payment endpoints.
type PaymentProxyHandler struct {
	client            *http.Client
	baseURL           string // Central Bank API base URL
	entityBesuAddress string // this entity's Besu address
	relayAuthSecret   string // shared secret for X-Relay-Auth header on internal endpoints
}

// NewPaymentProxyHandler creates a proxy handler targeting the given Central Bank URL.
func NewPaymentProxyHandler(
	centralBankURL string,
	entityBesuAddress, relayAuthSecret string,
) *PaymentProxyHandler {
	return &PaymentProxyHandler{
		client:            &http.Client{Timeout: 30 * time.Second},
		baseURL:           strings.TrimRight(centralBankURL, "/"),
		entityBesuAddress: entityBesuAddress,
		relayAuthSecret:   relayAuthSecret,
	}
}

// RegisterDeposit proxies POST /payments/deposits to the Central Bank.
func (h *PaymentProxyHandler) RegisterDeposit(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/deposits")
}

// ListDeposits proxies GET /payments/deposits to the Central Bank.
func (h *PaymentProxyHandler) ListDeposits(c *fiber.Ctx) error {
	path := "/internal/v1/payments/deposits"
	if q := c.Request().URI().QueryString(); len(q) > 0 {
		path += "?" + string(q)
	}
	return h.proxy(c, http.MethodGet, path, nil)
}

// RequestFiatExchange proxies POST /payments/deposits/exchange to the Central Bank.
func (h *PaymentProxyHandler) RequestFiatExchange(c *fiber.Ctx) error {
	return h.proxy(c, http.MethodPost, "/internal/v1/payments/deposits/exchange", c.Body())
}

// RequestEscrow proxies the escrow (tokenization) request enriched with the entity Besu address.
func (h *PaymentProxyHandler) RequestEscrow(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/escrows")
}

// ListEscrows proxies GET /payments/escrows to the Central Bank.
func (h *PaymentProxyHandler) ListEscrows(c *fiber.Ctx) error {
	path := "/internal/v1/payments/escrows"
	if q := c.Request().URI().QueryString(); len(q) > 0 {
		path += "?" + string(q)
	}
	return h.proxy(c, http.MethodGet, path, nil)
}

// RequestRedeem proxies the redeem request enriched with the entity Besu address.
func (h *PaymentProxyHandler) RequestRedeem(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/redeems")
}

// ListRedeems proxies GET /payments/redeems to the Central Bank.
func (h *PaymentProxyHandler) ListRedeems(c *fiber.Ctx) error {
	path := "/internal/v1/payments/redeems"
	if q := c.Request().URI().QueryString(); len(q) > 0 {
		path += "?" + string(q)
	}
	return h.proxy(c, http.MethodGet, path, nil)
}

// proxyWithEntityEnrichment injects entity Besu address into the request body
// before proxying to the Central Bank.
func (h *PaymentProxyHandler) proxyWithEntityEnrichment(c *fiber.Ctx, method, path string) error {
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	body["requester_besu_address"] = h.entityBesuAddress

	enriched, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "marshal enriched payload"})
	}

	return h.proxy(c, method, path, enriched)
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
