// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"io"
	"log"
	"net/http"
	"net/url"
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
	// signer, when set, signs each proxied request with this entity's own key. Without it the only
	// credential is relayAuthSecret, which is identical in every entity — so any entity could forge
	// these calls as any other bank, registering a deposit or driving a redemption in its name.
	signer *relayauth.Signer
}

// WithSigner attaches the per-entity signer. The shared secret is still sent alongside, so a central
// bank that has not pinned this bank yet keeps authenticating it instead of failing the payment.
func (h *PaymentProxyHandler) WithSigner(s *relayauth.Signer) *PaymentProxyHandler {
	h.signer = s
	return h
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

// ListDeposits proxies GET /payments/deposits to the Central Bank, scoped to this bank.
func (h *PaymentProxyHandler) ListDeposits(c *fiber.Ctx) error {
	return h.proxyScopedList(c, "/internal/v1/payments/deposits")
}

// RequestFiatExchange proxies POST /payments/deposits/exchange to the Central Bank.
func (h *PaymentProxyHandler) RequestFiatExchange(c *fiber.Ctx) error {
	return h.proxy(c, http.MethodPost, "/internal/v1/payments/deposits/exchange", c.Body())
}

// RequestEscrow proxies the escrow (tokenization) request enriched with the entity Besu address.
func (h *PaymentProxyHandler) RequestEscrow(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/escrows")
}

// ListEscrows proxies GET /payments/escrows to the Central Bank, scoped to this bank.
func (h *PaymentProxyHandler) ListEscrows(c *fiber.Ctx) error {
	return h.proxyScopedList(c, "/internal/v1/payments/escrows")
}

// RequestRedeem proxies the redeem request enriched with the entity Besu address.
func (h *PaymentProxyHandler) RequestRedeem(c *fiber.Ctx) error {
	return h.proxyWithEntityEnrichment(c, http.MethodPost, "/internal/v1/payments/redeems")
}

// ListRedeems proxies GET /payments/redeems to the Central Bank, scoped to this bank.
func (h *PaymentProxyHandler) ListRedeems(c *fiber.Ctx) error {
	return h.proxyScopedList(c, "/internal/v1/payments/redeems")
}

// proxyScopedList forwards a listing to the Central Bank with requester_id set to this entity.
//
// The Central Bank filters by requester_id and returns EVERY record when it is absent, so the
// parameter is not an optional refinement — it is the tenant boundary. It is therefore taken from this
// gateway's own identity and any value the caller sent is discarded, mirroring how the write paths
// inject requester_besu_address. Url.Values.Set (not Add) matters: a duplicated parameter would leave
// the receiver free to read the caller's copy.
func (h *PaymentProxyHandler) proxyScopedList(c *fiber.Ctx, path string) error {
	if h.entityBesuAddress == "" {
		// Falling through would ask the Central Bank for an unscoped listing, i.e. every bank's
		// records. Refuse instead, and say why.
		log.Printf("[payment-proxy] refusing to list %s: this entity has no Besu address configured, so the request cannot be scoped", path)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "this gateway has no entity address configured, so listings cannot be scoped to this institution",
		})
	}

	query, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query string"})
	}
	query.Set("requester_id", h.entityBesuAddress)

	return h.proxy(c, http.MethodGet, path+"?"+query.Encode(), nil)
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
	// Signed HERE, at the single point every proxied call funnels through, so the signature always
	// covers the body actually sent. That matters because proxyWithEntityEnrichment rewrites the body
	// before calling this: signing at the caller would sign the pre-enrichment bytes and the
	// receiver's canonical string would never match.
	//
	// The receiver builds its canonical string from the request PATH only, so the query string must be
	// stripped before signing: signing "/…/deposits?requester_id=0x…" against a receiver that verifies
	// "/…/deposits" produces a well-formed signature that never matches.
	if h.signer != nil {
		signedPath, _, _ := strings.Cut(path, "?")
		if headers, sErr := h.signer.HeadersFor(method, signedPath, body, time.Now()); sErr == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		} else {
			// Not fatal: the shared secret still authenticates during the migration. Blocking a
			// payment over a signing failure the receiver can tolerate would be the worse outcome.
			log.Printf("[payment-proxy] could not sign %s %s: %v (falling back to the shared secret)", method, path, sErr)
		}
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
