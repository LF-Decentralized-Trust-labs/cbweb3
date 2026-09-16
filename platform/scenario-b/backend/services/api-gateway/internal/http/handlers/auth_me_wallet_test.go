// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
)

// The issuance screen renders `profile.wallet` and falls back to "Not available in
// session". Operators saw that fallback minutes after onboarding had displayed the very
// address on its own success screen, on the one screen where confirming which wallet
// gets credited is the point.
//
// The address was never missing: this gateway is configured with it, and every deposit
// it creates carries it as requester_besu_address. /auth/me simply did not publish it,
// because the identity provider issues no wallet claim.

func meBody(t *testing.T, handler *AuthHandler, provider authProviderStub) map[string]any {
	t.Helper()
	app := fiber.New()
	app.Get("/auth/me", middleware.RequireCookieAuth(provider), handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "valid-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return body
}

func TestMe_PublishesTheGatewaysOwnWalletWhenTheTokenCarriesNone(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{
		token: domain.AuthToken{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600},
	}
	const entityWallet = "0x409e0f7a712B8f90e3a597e3258A2E2f04652e4d"
	handler := NewAuthHandler(provider, kycCheckerStub{}, false).WithEntityWallet(entityWallet)

	body := meBody(t, handler, provider)

	if body["wallet"] != entityWallet {
		t.Errorf("wallet = %v, want %q — the issuance screen renders this field and says "+
			"the wallet is unavailable when it is absent", body["wallet"], entityWallet)
	}
}

// TestMe_OmitsTheWalletWhenTheGatewayHasNone keeps the fallback honest. A gateway with no
// configured address must not answer with an empty string, which the portal would render
// as a wallet of "" rather than as the absence it is.
func TestMe_OmitsTheWalletWhenTheGatewayHasNone(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{
		token: domain.AuthToken{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600},
	}
	handler := NewAuthHandler(provider, kycCheckerStub{}, false)

	body := meBody(t, handler, provider)

	if _, present := body["wallet"]; present {
		t.Errorf("wallet was published as %v with nothing configured; the field must be absent",
			body["wallet"])
	}
}

// TestMe_TrimsAConfiguredWallet guards the env-var path: the address arrives from
// ENTITY_BESU_ADDRESS, which is rendered into a compose file, so stray whitespace is a
// realistic input and an untrimmed value would not match the address the deposits carry.
func TestMe_TrimsAConfiguredWallet(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{
		token: domain.AuthToken{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600},
	}
	const entityWallet = "0x409e0f7a712B8f90e3a597e3258A2E2f04652e4d"
	handler := NewAuthHandler(provider, kycCheckerStub{}, false).WithEntityWallet("  " + entityWallet + "\n")

	if got := meBody(t, handler, provider)["wallet"]; got != entityWallet {
		t.Errorf("wallet = %v, want the trimmed %q", got, entityWallet)
	}
}
