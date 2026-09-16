// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/middleware"
)

// Stable error codes for the auth routes.
//
// The portal keys on these rather than on prose, so a reworded message does not change
// behaviour and a 401 can be told apart from a provider outage.
const (
	// CodeInvalidCredentials is the realm rejecting what it was given.
	CodeInvalidCredentials = "INVALID_CREDENTIALS" //#nosec G101 -- a wire error code, no credential material
	// CodeAuthServiceUnavailable is the identity provider being unreachable. Deliberately
	// NOT an authentication failure: telling an operator their password is wrong when
	// Keycloak is down sends them to rotate a credential that was fine.
	CodeAuthServiceUnavailable = "AUTH_SERVICE_UNAVAILABLE"
	// CodeMissingSession means no refresh cookie was presented — usually a session-restore
	// probe on a page that has no session yet, which a portal can answer with silence
	// rather than rendering as a login failure.
	CodeMissingSession = "MISSING_SESSION"
)

// AuthHandler owns the login the NOC backend never had.
//
// It exists because the portals kept their tokens in localStorage: the login ran in the
// browser, against Keycloak's token endpoint, so any script on the page could read the
// session. A cookie the browser cannot read can only be set by a server — so the grant
// moved here, and this is the server that performs it.
type AuthHandler struct {
	kc keycloak.Client
	// cookieSecure mirrors COOKIE_SECURE. It must be false on a plain-HTTP local stack:
	// a browser silently DISCARDS a Secure cookie over http, which presents as a login
	// that succeeds and a session that never exists.
	cookieSecure bool
	// csrfSecret keys the HMAC binding a CSRF token to its session. Read from the
	// environment rather than generated, so replicas and restarts agree — a per-process
	// secret invalidates every token issued by another replica.
	csrfSecret []byte
}

// NewAuthHandler builds the handler.
func NewAuthHandler(kc keycloak.Client, cookieSecure bool, csrfSecret []byte) *AuthHandler {
	return &AuthHandler{kc: kc, cookieSecure: cookieSecure, csrfSecret: csrfSecret}
}

// Register mounts the routes that must be reachable WITHOUT a session — this is where a
// browser obtains one, so guarding them would make the cookie unobtainable.
func (h *AuthHandler) Register(g fiber.Router) {
	g.Post("/login", h.login)
	g.Post("/refresh", h.refresh)
	g.Post("/logout", h.logout)
}

// RegisterProtected mounts the routes that require a session. Separate from Register so the
// guard is applied explicitly here rather than inherited from a group prefix — the auth
// group deliberately sits outside the portal group's middleware.
func (h *AuthHandler) RegisterProtected(g fiber.Router, requireAuth fiber.Handler) {
	g.Get("/me", requireAuth, h.me)
}

// profile is what the portal renders from. It exists because the session moved into an
// HttpOnly cookie: the browser can no longer decode the token to learn who it is, which is
// how the old portal built this — and that only worked because the token was readable by
// any script on the page.
type profile struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

func profileFrom(claims keycloak.TokenClaims) profile {
	return profile{ID: claims.Subject, Name: claims.Actor(), Roles: claims.Roles}
}

// me answers the session-restore probe: reopening the portal has a cookie but no profile.
func (h *AuthHandler) me(c *fiber.Ctx) error {
	claims, ok := middleware.ClaimsFrom(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no session",
			"code":  CodeMissingSession,
		})
	}
	return c.JSON(fiber.Map{"user": profileFrom(claims)})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
			"code":  CodeInvalidCredentials,
		})
	}
	// Checked here rather than sent on: an empty field is not a credential, and forwarding
	// it spends a round trip to the realm to be told the obvious.
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username and password are required",
			"code":  CodeInvalidCredentials,
		})
	}

	tokens, err := h.kc.PasswordGrant(c.UserContext(), req.Username, req.Password)
	if err != nil {
		return h.grantFailure(c, err)
	}
	if err := h.setAuthCookies(c, tokens); err != nil {
		log.Printf("[noc-auth] login: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not establish session"})
	}
	// The profile is returned with the login rather than left to a second round trip. The
	// portal needs it immediately to decide what to render, and this server has just
	// obtained the token it comes from.
	claims, err := h.kc.ValidateToken(c.UserContext(), tokens.AccessToken)
	if err != nil {
		// The realm minted this token seconds ago; failing to validate it means the
		// service's own configuration disagrees with the realm (issuer or audience), which
		// an operator needs to see rather than a blank profile.
		log.Printf("[noc-auth] login: the realm's own token did not validate: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "session established but the token could not be read",
		})
	}
	return c.JSON(fiber.Map{"expiresIn": tokens.ExpiresIn, "user": profileFrom(claims)})
}

// refresh renews the session from the refresh COOKIE.
//
// Never from the body. The refresh token is the longer-lived credential and lives in an
// HttpOnly cookie precisely so that page scripts cannot reach it; accepting one from the
// request would hand that back to anything able to make a request.
func (h *AuthHandler) refresh(c *fiber.Ctx) error {
	refreshToken := strings.TrimSpace(c.Cookies("refresh_token"))
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no session to refresh",
			"code":  CodeMissingSession,
		})
	}

	tokens, err := h.kc.RefreshGrant(c.UserContext(), refreshToken)
	if err != nil {
		if keycloak.IsInvalidCredentials(err) {
			// The session really expired: clear the cookies so the portal stops retrying
			// with a token the realm has already rejected.
			h.clearAuthCookies(c)
		}
		return h.grantFailure(c, err)
	}
	if err := h.setAuthCookies(c, tokens); err != nil {
		log.Printf("[noc-auth] refresh: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not refresh session"})
	}
	return c.JSON(fiber.Map{"expiresIn": tokens.ExpiresIn})
}

func (h *AuthHandler) logout(c *fiber.Ctx) error {
	h.clearAuthCookies(c)
	return c.JSON(fiber.Map{"message": "logged out"})
}

// grantFailure maps a grant error onto the one distinction that matters to an operator.
func (h *AuthHandler) grantFailure(c *fiber.Ctx, err error) error {
	if keycloak.IsInvalidCredentials(err) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credentials",
			"code":  CodeInvalidCredentials,
		})
	}
	// Logged, because an unreachable realm is an operational event and the operator's
	// screen deliberately does not carry the detail.
	log.Printf("[noc-auth] identity provider unavailable: %v", err)
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"error": "authentication service unavailable",
		"code":  CodeAuthServiceUnavailable,
	})
}

// setAuthCookies writes the session, its refresh, and the CSRF token that pairs with them.
//
// The CSRF token is minted HERE rather than at login only, because it is bound to the
// access token: a refresh issues a new access token, so a token minted at login stops
// validating the moment the session is renewed. Both paths funnel through this function,
// which is why it is the right place.
func (h *AuthHandler) setAuthCookies(c *fiber.Ctx, tokens keycloak.Tokens) error {
	c.Cookie(&fiber.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    tokens.AccessToken,
		Path:     "/",
		MaxAge:   tokens.ExpiresIn,
		HTTPOnly: true,
		Secure:   h.cookieSecure,
		SameSite: "Strict",
	})
	if tokens.RefreshToken != "" {
		// Its own MaxAge, so it outlives the access token and silent refresh is possible.
		// Falls back to the access lifetime when the realm reports none, which is better
		// than a session cookie that expires at the same instant as the thing meant to
		// renew it.
		refreshMaxAge := tokens.RefreshExpiresIn
		if refreshMaxAge <= tokens.ExpiresIn {
			refreshMaxAge = tokens.ExpiresIn * 2
		}
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    tokens.RefreshToken,
			Path:     "/",
			MaxAge:   refreshMaxAge,
			HTTPOnly: true,
			Secure:   h.cookieSecure,
			SameSite: "Strict",
		})
	}

	csrfToken, err := middleware.NewCSRFToken(h.csrfSecret, tokens.AccessToken)
	if err != nil {
		// Propagated, never swallowed: an empty XSRF-TOKEN cookie locks the browser out of
		// every mutating request, and the only clue would be a 403 with no cause.
		return fmt.Errorf("mint csrf token: %w", err)
	}
	// Deliberately NOT HttpOnly — the browser has to read this one to echo it in the
	// X-XSRF-TOKEN header. That is the whole double-submit mechanism, and it is safe
	// precisely because the session cookie beside it stays HttpOnly.
	c.Cookie(&fiber.Cookie{
		Name:     middleware.CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   tokens.ExpiresIn,
		HTTPOnly: false,
		Secure:   h.cookieSecure,
		SameSite: "Strict",
	})
	return nil
}

// clearAuthCookies expires every auth cookie immediately.
//
// Expires-in-the-past, NOT MaxAge: -1. Fiber only writes a Max-Age attribute when the value
// is positive, so a negative MaxAge is silently dropped and the browser receives a cookie
// with an empty value and no expiry — cleared in effect, because an empty session fails
// authentication, but still sitting in the jar until the browser closes. Setting Expires is
// what actually deletes it.
func (h *AuthHandler) clearAuthCookies(c *fiber.Ctx) {
	past := time.Now().Add(-time.Hour)
	for _, name := range []string{middleware.SessionCookieName, "refresh_token", middleware.CSRFCookieName} {
		c.Cookie(&fiber.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  past,
			HTTPOnly: name != middleware.CSRFCookieName,
			Secure:   h.cookieSecure,
			SameSite: "Strict",
		})
	}
}
