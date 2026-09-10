// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// This handler already deletes its cookies correctly — Expires in the past, not MaxAge: -1 —
// and its comment explains why. What it did not have was anything stopping that from being
// undone.
//
// The api-gateway had the same code written the wrong way for months. The defect is invisible
// in normal use (an empty session cookie still fails authentication) and invisible to a Go test
// that reads the fiber.Cookie struct back, because the struct holds what the handler set, not
// what Fiber wrote. It survived precisely because nothing asserted the wire.
//
// So this file guards the reference implementation. Leaving it unguarded while the gateway's
// fix cites it as the pattern to copy would be citing something nothing holds in place.
//
// Deliberate copy of the gateway's logout_cookie_wire_test.go, adapted to this handler's shape.

func TestNOCClearAuthCookies_DeletesOnTheWire(t *testing.T) {
	h := &AuthHandler{cookieSecure: false}

	app := fiber.New()
	app.Post("/logout", func(c *fiber.Ctx) error {
		h.clearAuthCookies(c)
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest("POST", "/logout", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("logout set no cookies at all; the guard would pass by checking nothing")
	}

	for _, sc := range cookies {
		name, rest, _ := strings.Cut(sc, "=")
		value, _, _ := strings.Cut(rest, ";")

		if strings.TrimSpace(value) != "" {
			t.Errorf("logout left a value in %s: %q", name, value)
		}
		lower := strings.ToLower(sc)
		if !strings.Contains(lower, "expires=") && !strings.Contains(lower, "max-age=") {
			t.Errorf("%s carries neither Expires nor Max-Age, so the browser keeps it (empty) "+
				"until it closes. Fiber drops a negative MaxAge silently — set Expires in the "+
				"past.\n\t%s", name, sc)
		}
	}
}
