package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// OnboardingProxyHandler forwards onboarding requests from a commercial bank
// gateway to the Central Bank's public onboarding endpoints.
type OnboardingProxyHandler struct {
	client  *http.Client
	baseURL string // Central Bank API base URL (e.g. "http://api-gateway-central-bank-a:8080")
}

// NewOnboardingProxyHandler creates a proxy handler targeting the given Central Bank URL.
func NewOnboardingProxyHandler(centralBankURL string) *OnboardingProxyHandler {
	return &OnboardingProxyHandler{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: strings.TrimRight(centralBankURL, "/"),
	}
}

// InitiateCredentialRequest proxies POST /onboarding/initiate to the Central
// Bank's POST /api/v1/onboarding/credential-request.
func (h *OnboardingProxyHandler) InitiateCredentialRequest(c *fiber.Ctx) error {
	return h.proxy(c, http.MethodPost, "/api/v1/onboarding/credential-request", c.Body())
}

// GetOnboardingStatus proxies GET /onboarding/status/:requestId to the Central
// Bank's GET /api/v1/onboarding/status/:requestId.
func (h *OnboardingProxyHandler) GetOnboardingStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestId")
	if requestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "requestId is required"})
	}
	return h.proxy(c, http.MethodGet, "/api/v1/onboarding/status/"+requestID, nil)
}

// CompleteOnboarding proxies POST /onboarding/complete to the Central Bank's
// POST /api/v1/onboarding/complete.
func (h *OnboardingProxyHandler) CompleteOnboarding(c *fiber.Ctx) error {
	return h.proxy(c, http.MethodPost, "/api/v1/onboarding/complete", c.Body())
}

func (h *OnboardingProxyHandler) proxy(c *fiber.Ctx, method, path string, body []byte) error {
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
