// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// The decision this implements, from the sovereign-hub-delegation security review (the residual of
// finding NEW-2): when the shared replay store is unreachable, refuse on the ONE route where a
// replay is a compliance failure, and keep serving everywhere else.
//
// POST /internal/v2/transfer-limits/restore credits a bank's daily allowance back. Replayed, it
// lets a bank transact past the limit its central bank configured. Failing closed globally was
// rejected and stays rejected: it would stop bridge-in, the delegated hub swap and the residue
// return — a far larger outage than the window it closes.
//
// Both halves are load-bearing and both are pinned here. A change that only refused on restore
// would be indistinguishable, in a green suite, from the global fail-closed that was ruled out.

type unreachableStore struct{}

func (unreachableStore) Admit(context.Context, string, time.Duration) (bool, error) {
	return false, errors.New("dial tcp 127.0.0.1:6379: connect: connection refused")
}

type healthyStore struct{}

func (healthyStore) Admit(context.Context, string, time.Duration) (bool, error) { return true, nil }

// callSigned mounts the middleware on a catch-all POST route and issues one correctly signed
// request to path. It reports the status and whether the handler behind the middleware ran.
func callSigned(t *testing.T, path string, guard *relayauth.ReplayGuard) (int, bool) {
	t.Helper()
	signer, reg := keyAndReg(t, "cb-peer")

	ran := false
	app := fiber.New()
	app.Post("/+", middleware.RequireRelayAuthMigrating(middleware.RelayAuthConfig{
		Registry: relayauth.NewStore(reg),
		Replay:   guard,
	}), func(c *fiber.Ctx) error {
		ran = true
		return c.SendStatus(fiber.StatusOK)
	})

	const body = `{"payer_bank_id":"bank-a"}`
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h, err := signer.HeadersFor(http.MethodPost, path, []byte(body), time.Now())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	return do(t, app, req), ran
}

// The half the decision asked for.
func TestRestoreRefusesWhileTheReplayStoreIsDown(t *testing.T) {
	status, ran := callSigned(t, "/internal/v2/transfer-limits/restore",
		relayauth.NewReplayGuard(5*time.Minute).WithShared(unreachableStore{}))

	if ran {
		t.Error("the restore handler ran with replay protection degraded; a replayed restore would " +
			"credit a bank's daily allowance back a second time")
	}
	if status != fiber.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503. The caller's credentials are fine — what failed is our "+
			"ability to check the request safely, and 503 says retry where 401 says wrong key", status)
	}
}

// The half that keeps the refusal from becoming the global fail-closed the review rejected. If
// this goes red, the blast radius is the bridge, the hub swap and the residue return.
func TestOtherInternalRoutesStillServeWhileTheReplayStoreIsDown(t *testing.T) {
	for _, path := range []string{
		"/internal/v2/transfer-limits/check-and-deduct",
		"/internal/amm/cross-currency-bridge-in",
		"/internal/v2/swaps/claim",
		"/internal/v2/residues/return",
	} {
		status, ran := callSigned(t, path,
			relayauth.NewReplayGuard(5*time.Minute).WithShared(unreachableStore{}))
		if !ran || status != fiber.StatusOK {
			t.Errorf("%s was refused while the replay store was down (status %d, handler ran=%v). "+
				"Failing closed everywhere is exactly the option the review rejected: it stops "+
				"settlement to close a replay window on one route.", path, status, ran)
		}
	}
}

// With the store healthy, restore behaves as before: the refusal is about the store, not the route.
func TestRestoreServesNormallyWhenTheStoreIsHealthy(t *testing.T) {
	status, ran := callSigned(t, "/internal/v2/transfer-limits/restore",
		relayauth.NewReplayGuard(5*time.Minute).WithShared(healthyStore{}))
	if !ran || status != fiber.StatusOK {
		t.Errorf("restore was refused with a healthy store (status %d, ran=%v)", status, ran)
	}
}

// A deployment with no shared store is the single-process one the guard was born in, not a fault.
// Refusing there would turn hardening into an outage wherever Redis was never wired.
func TestRestoreServesWhenNoSharedStoreIsConfigured(t *testing.T) {
	status, ran := callSigned(t, "/internal/v2/transfer-limits/restore",
		relayauth.NewReplayGuard(5*time.Minute))
	if !ran || status != fiber.StatusOK {
		t.Errorf("restore was refused with no shared store configured (status %d, ran=%v); that is "+
			"a deployment choice, not a degraded state", status, ran)
	}
}

// A real replay is still refused AS a replay. The new path must not swallow the case the guard
// already handled, nor report it as a store outage — the portal classifies on the code.
func TestRestoreStillRefusesARealReplayAsAReplay(t *testing.T) {
	signer, reg := keyAndReg(t, "cb-peer")
	guard := relayauth.NewReplayGuard(5 * time.Minute).WithShared(healthyStore{})

	app := fiber.New()
	app.Post("/+", middleware.RequireRelayAuthMigrating(middleware.RelayAuthConfig{
		Registry: relayauth.NewStore(reg), Replay: guard,
	}), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	const path = "/internal/v2/transfer-limits/restore"
	const body = `{"payer_bank_id":"bank-a"}`
	h, err := signer.HeadersFor(http.MethodPost, path, []byte(body), time.Now())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	send := func() int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range h {
			req.Header.Set(k, v)
		}
		return do(t, app, req)
	}

	if got := send(); got != fiber.StatusOK {
		t.Fatalf("first use = %d, want 200", got)
	}
	if got := send(); got != fiber.StatusUnauthorized {
		t.Errorf("a replayed restore returned %d, want 401 — it must read as a replay, not as a "+
			"store outage", got)
	}
}
