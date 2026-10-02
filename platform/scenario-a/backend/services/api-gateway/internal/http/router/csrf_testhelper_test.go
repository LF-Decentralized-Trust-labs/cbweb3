// SPDX-License-Identifier: Apache-2.0

package router

import (
	"net/http"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
)

// attachCSRF gives a test request a valid CSRF token bound to its session.
//
// The CSRF guard is mounted app-wide and answers before any role check or route
// match, so a test about AUTHORIZATION or about ROUTE REGISTRATION must carry a
// token — otherwise every one of its assertions reads 403 and proves nothing about
// the thing it was written to pin.
//
// The guard is deliberately NOT relaxed for tests. A control that switches itself
// off under test is the control this card exists to remove.
func attachCSRF(t *testing.T, req *http.Request, session string) {
	t.Helper()
	token, err := middleware.NewCSRFToken(nil, session)
	if err != nil {
		t.Fatalf("mint csrf token: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
	req.Header.Set(middleware.CSRFHeaderName, token)
}
