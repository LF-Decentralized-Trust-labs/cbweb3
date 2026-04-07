package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

// NewPaymentProxyHandler creates a proxy handler targeting the given Central
// Bank URL. The payment adapter is used for local Zeto operations.
func NewPaymentProxyHandler(
	centralBankURL string,
	payment *paymentadapter.GRPCAdapter,
	entityBesuAddress, paladinIdentity, cbPaladinIdentity string,
) *PaymentProxyHandler {
	return &PaymentProxyHandler{
		client:            &http.Client{Timeout: 30 * time.Second},
		baseURL:           strings.TrimRight(centralBankURL, "/"),
		payment:           payment,
		entityBesuAddress: entityBesuAddress,
		paladinIdentity:   paladinIdentity,
		cbPaladinIdentity: cbPaladinIdentity,
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

// RequestEscrow proxies POST /payments/escrows to the Central Bank.
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

// ListRedeems proxies GET /payments/redeems to the Central Bank.
func (h *PaymentProxyHandler) ListRedeems(c *fiber.Ctx) error {
	path := "/internal/v1/payments/redeems"
	if q := c.Request().URI().QueryString(); len(q) > 0 {
		path += "?" + string(q)
	}
	return h.proxy(c, http.MethodGet, path, nil)
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
