// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Logout must DELETE the auth cookies, and the only place that can be checked is the wire.
//
// clearAuthCookies used to expire them with MaxAge: -1. Fiber writes a Max-Age attribute only
// when the value is positive, so a negative one is dropped without warning and the browser
// receives a cookie with an empty value and no expiry at all:
//
//	Set-Cookie: access_token=; path=/; HttpOnly; SameSite=Strict
//
// Logout still worked — an empty session fails authentication — but three empty cookies stayed
// in the jar until the browser closed, on every portal, for every operator, and the function's
// own comment claimed it expired them.
//
// This test reads the Set-Cookie header rather than the fiber.Cookie struct, and that is the
// whole point: a test that reads MaxAge back sees the value the handler set, not what Fiber
// wrote. Such a test passes today AND would pass after a regression, which is why the defect
// survived a test suite in the first place. Measured against Fiber v2.52.9.
//
// The NOC backend already does it this way (noc-backend/internal/api/auth.go), in both
// scenarios, and already guards it: TestLogout_ExpiresEveryAuthCookie in that package's
// auth_test.go checks each cookie BY NAME, that the value is emptied, and that the expiry is in
// the past. This gateway was the half that never caught up — on the code and on the guard.
//
// The assertions below are that test's, restated for this handler's shape. An earlier draft of
// this branch also added a second guard to the NOC, on the false premise that it had none; the
// existing one is stronger, and a weaker duplicate beside it is what later gets mistaken for
// the coverage.

// logoutSetCookies runs clearAuthCookies through a real Fiber app and returns the Set-Cookie
// headers exactly as a browser would receive them.
func logoutSetCookies(t *testing.T) []string {
	t.Helper()
	app := fiber.New()
	app.Post("/logout", func(c *fiber.Ctx) error {
		clearAuthCookies(c, false)
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest("POST", "/logout", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	got := resp.Header.Values("Set-Cookie")
	if len(got) == 0 {
		t.Fatal("logout set no cookies at all; the guard would pass by checking nothing")
	}
	return got
}

// Every auth cookie must carry an expiry the browser will act on.
func TestClearAuthCookies_DeletesOnTheWire(t *testing.T) {
	cookies := logoutSetCookies(t)

	want := []string{"access_token", "refresh_token", "XSRF-TOKEN"}
	for _, name := range want {
		var found string
		for _, sc := range cookies {
			if strings.HasPrefix(sc, name+"=") {
				found = sc
				break
			}
		}
		if found == "" {
			t.Errorf("logout does not clear %s at all:\n\t%s", name, strings.Join(cookies, "\n\t"))
			continue
		}
		lower := strings.ToLower(found)
		// Either attribute deletes the cookie; neither leaves it in the jar as a session cookie.
		if !strings.Contains(lower, "expires=") && !strings.Contains(lower, "max-age=") {
			t.Errorf("%s carries neither Expires nor Max-Age, so the browser keeps it (with an "+
				"empty value) until it closes. Fiber drops a negative MaxAge silently — set "+
				"Expires in the past instead.\n\t%s", name, found)
		}
	}
}

// The value must also be emptied. Deleting without emptying would leave a live token in any
// client that ignores the expiry.
func TestClearAuthCookies_EmptiesTheValue(t *testing.T) {
	for _, sc := range logoutSetCookies(t) {
		name, rest, _ := strings.Cut(sc, "=")
		value, _, _ := strings.Cut(rest, ";")
		if strings.TrimSpace(value) != "" {
			t.Errorf("logout left a value in %s: %q", name, value)
		}
	}
}

// A deletion whose expiry is in the FUTURE is not a deletion. Guarded because "set Expires" is
// the fix, and setting it to time.Now() or to a future instant would satisfy the check above
// while leaving the cookie alive.
func TestClearAuthCookies_ExpiryIsInThePast(t *testing.T) {
	for _, sc := range logoutSetCookies(t) {
		lower := strings.ToLower(sc)
		i := strings.Index(lower, "expires=")
		if i < 0 {
			continue // covered by the Max-Age branch above
		}
		rest := sc[i+len("expires="):]
		if j := strings.Index(rest, ";"); j >= 0 {
			rest = rest[:j]
		}
		when, err := parseCookieExpiry(strings.TrimSpace(rest))
		if err != nil {
			t.Errorf("could not read the expiry of %q: %v", sc, err)
			continue
		}
		if !when.Before(nowForCookieTest()) {
			t.Errorf("the expiry of %q is not in the past (%s); the browser would keep the cookie",
				sc, when)
		}
	}
}

// parseCookieExpiry reads the Expires attribute. net/http writes RFC 1123 with GMT; the other
// forms are accepted because the attribute's grammar allows them and a stricter parser would
// fail the test for a reason that has nothing to do with the cookie being deleted.
func parseCookieExpiry(v string) (time.Time, error) {
	var lastErr error
	for _, layout := range []string{time.RFC1123, http.TimeFormat, time.RFC1123Z} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func nowForCookieTest() time.Time { return time.Now() }
