// SPDX-License-Identifier: Apache-2.0

// This is a per-scenario copy of scenario-a's api-gateway CSRF guard (Constitution
// Principle I: scenarios do not share code). Copied rather than reimplemented on purpose —
// the three properties commented below were each found in review on the gateway side, and a
// fresh implementation would have to rediscover them.
package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CSRF token cookie and header names. XSRF-TOKEN / X-XSRF-TOKEN is the pair axios
// reads and writes by default, so the browser side needs no bespoke code — only
// withXSRFToken, because axios refuses to attach the header cross-origin without it.
const (
	CSRFCookieName = "XSRF-TOKEN"
	CSRFHeaderName = "X-XSRF-TOKEN"
)

// CodeCSRFTokenInvalid is returned when the double-submit check fails.
//
// It exists so a caller — and the router-level test — can tell a CSRF refusal from
// an authorization refusal. Both are 403, and without a distinguishing code a test
// asserting "mutating routes reject a missing CSRF header" passes on any route that
// happens to reject the caller's ROLE instead, proving nothing about CSRF.
const CodeCSRFTokenInvalid = "CSRF_TOKEN_INVALID" //#nosec G101 -- not a secret; a wire error code, no credential material

// safeMethods never mutate state, so they carry no CSRF requirement. HEAD and
// OPTIONS matter as much as GET: OPTIONS is the CORS preflight, and rejecting it
// would break every cross-origin request before the real one is sent.
var safeMethods = map[string]bool{
	fiber.MethodGet:     true,
	fiber.MethodHead:    true,
	fiber.MethodOptions: true,
}

// NewCSRFToken mints a token bound to the session it is issued with.
//
// The token is `<random>.<hmac(secret, random|session)>`. Binding matters: an
// unauthenticated random value compared by equality is defeated by any cookie-write
// primitive — a sibling subdomain, or a MITM on plain HTTP — because the attacker
// can then supply BOTH halves of the double submit. With the HMAC, a token minted
// for one session does not validate for another, and a token the server never
// issued does not validate at all.
//
// sessionID should be a stable, non-secret identifier for the session; the access
// token itself is used by the caller, which is acceptable because the HMAC is
// one-way and the result is what the browser sees, never the input.
func NewCSRFToken(secret []byte, sessionID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		// Propagated, never swallowed. Emitting an empty XSRF-TOKEN cookie would
		// lock the user out of every mutating request with no way to tell why.
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(raw)
	return nonce + "." + csrfMAC(secret, nonce, sessionID), nil
}

// csrfMAC computes the binding half of the token.
func csrfMAC(secret []byte, nonce, sessionID string) string {
	m := hmac.New(sha256.New, secret)
	// The separator keeps (nonce, session) unambiguous: without it "ab"+"c" and
	// "a"+"bc" would MAC identically.
	m.Write([]byte(nonce))
	m.Write([]byte{0})
	m.Write([]byte(sessionID))
	return hex.EncodeToString(m.Sum(nil))
}

// ValidCSRFToken reports whether token was minted by NewCSRFToken for this session.
func ValidCSRFToken(secret []byte, token, sessionID string) bool {
	nonce, mac, found := strings.Cut(token, ".")
	if !found || nonce == "" || mac == "" {
		return false
	}
	return hmac.Equal([]byte(mac), []byte(csrfMAC(secret, nonce, sessionID)))
}

// CSRFConfig configures the guard.
type CSRFConfig struct {
	// Secret keys the HMAC that binds a token to its session.
	Secret []byte
	// SessionID extracts the session identifier a token must be bound to. It
	// returns "" when the request carries no session cookie, and the guard then lets
	// the request through — see the reasoning in CSRF below: with no ambient
	// credential there is nothing for this control to defend.
	SessionID func(*fiber.Ctx) string
	// Exempt reports whether a request is outside the guard's scope.
	//
	// The guard is mounted app-wide and exemptions are named here, rather than
	// being attached group by group. That ordering is deliberate: per-group wiring
	// is how the v2 tree came to exist with 27 mutating routes and no CSRF at all —
	// the guard was attached to the groups that existed when it was written, and
	// nothing failed when new ones appeared. Mounted app-wide, a new route is
	// protected by default and an exemption is a visible edit here.
	Exempt func(*fiber.Ctx) bool
}

// CSRF enforces the double-submit cookie check on state-changing requests.
//
// Three properties are deliberate, and each closes a hole found in review:
//
//   - The header must be present AND match the cookie AND carry a valid binding for
//     THIS session. An empty header is an explicit refusal, never treated as a match.
//   - There is no Bearer exemption. The rejected design skipped the check whenever
//     an Authorization header was present WHILE the session cookie was still
//     attached and still usable, so a caller could waive the control and keep the
//     credential. Here a session cookie always triggers the check, whatever else
//     the caller sends.
//   - Comparison is constant-time via hmac.Equal.
func CSRF(cfg CSRFConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if safeMethods[c.Method()] {
			return c.Next()
		}
		if cfg.Exempt != nil && cfg.Exempt(c) {
			return c.Next()
		}

		sessionID := ""
		if cfg.SessionID != nil {
			sessionID = cfg.SessionID(c)
		}

		// No session cookie means no ambient credential, and CSRF exists to protect
		// exactly that: a credential the BROWSER attaches on its own. A cross-site
		// attack always carries the victim's cookie, because the browser sends it
		// without being asked — so the guard still covers every real attack. A request
		// without one is either unauthenticated (authentication will refuse it) or
		// authenticated by something a cross-site page cannot set, which CORS already
		// prevents.
		//
		// This is NOT the self-selected Bearer exemption the review rejected. That one
		// skipped the check whenever an Authorization header was present, while the
		// session cookie was still attached and still usable — the caller chose the
		// exemption and kept the credential. Here the exemption applies only when there
		// is no ambient credential at all, which an attacker in a browser cannot arrange.
		//
		// It is also load-bearing for the server-to-server hops inside this product: the
		// onboarding proxy on a commercial-bank gateway builds a FRESH request to the
		// central bank carrying neither cookie nor CSRF header. Refusing it broke bank
		// onboarding end to end, which is how this was found — in a browser, after every
		// unit test passed.
		if sessionID == "" {
			return c.Next()
		}

		header := c.Get(CSRFHeaderName)
		cookie := c.Cookies(CSRFCookieName)

		// Every branch answers with the same code and message. Distinguishing
		// "missing" from "mismatched" tells an attacker which half to work on and
		// helps a legitimate client not at all — the remedy is identical.
		if header == "" || cookie == "" ||
			!hmac.Equal([]byte(header), []byte(cookie)) ||
			!ValidCSRFToken(cfg.Secret, header, sessionID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF token missing or invalid",
				"code":  CodeCSRFTokenInvalid,
			})
		}

		return c.Next()
	}
}
