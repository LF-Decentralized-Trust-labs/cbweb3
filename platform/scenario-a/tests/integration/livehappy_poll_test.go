// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"strings"
	"testing"
	"time"
)

// pollUntil calls fn repeatedly until it returns (true, nil) or the deadline
// expires. A non-nil error from fn fails the test immediately; (false, nil)
// schedules a retry after interval.
func pollUntil(t *testing.T, interval, timeout time.Duration, fn func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	attempts := 0
	for time.Now().Before(deadline) {
		ok, err := fn()
		if err != nil {
			t.Fatalf("pollUntil: fn returned error after %d attempts: %v", attempts, err)
		}
		if ok {
			return
		}
		attempts++
		time.Sleep(interval)
	}
	t.Fatalf("pollUntil: condition not met after %s (%d attempts)", timeout, attempts)
}

// waitForHTTP polls {baseURL}/healthz until it returns 2xx or the deadline expires.
func waitForHTTP(t *testing.T, name, baseURL string, timeout time.Duration) {
	t.Helper()
	c := newHTTPClient(baseURL, "")
	pollUntil(t, 3*time.Second, timeout, func() (bool, error) {
		return c.get("/healthz", nil) == nil, nil
	})
	t.Logf("  [%s] healthy", name)
}

// fxState fetches the current state of an FX agreement via the given client.
// Returns "" (no error) when the agreement is not yet visible, so it composes
// inside pollUntil while a cross-spoke mirror is still propagating.
func fxState(c *httpClient, tradeID string) (string, error) {
	var resp struct {
		Agreement struct {
			State string `json:"state"`
		} `json:"agreement"`
	}
	if err := c.get("/api/v1/payments/fx/agreements/"+tradeID, &resp); err != nil {
		return "", nil //nolint:nilerr // not-yet-mirrored is an expected transient
	}
	return resp.Agreement.State, nil
}

// isFXState reports whether state matches want, tolerating the FX_STATE_ prefix
// the orchestrator may or may not include (e.g. "ACCEPTED" vs "FX_STATE_ACCEPTED").
func isFXState(state, want string) bool {
	return state == want || state == "FX_STATE_"+want
}

// htlcStatus is the subset of GET /htlc/status/:id we assert on.
type htlcStatus struct {
	State    string `json:"state"`
	HashLock string `json:"hash_lock"`
	Secret   string `json:"secret"`
	TimeLock int64  `json:"time_lock"`
}

// isHTLCState reports whether state matches want, tolerating the HTLC_STATE_ prefix.
func isHTLCState(state, want string) bool {
	return state == want || state == "HTLC_STATE_"+want
}

// pollHTLCSettled polls the responder leg until the relay drives it to SETTLED.
func pollHTLCSettled(t *testing.T, c *httpClient, contractID, label string) {
	t.Helper()
	pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
		var s htlcStatus
		if err := c.get("/api/v1/htlc/status/"+contractID, &s); err != nil {
			return false, nil //nolint:nilerr // transient until relay catches up
		}
		state := strings.TrimSpace(s.State)
		t.Logf("  [%s] htlc state: %s", label, state)
		return isHTLCState(state, "SETTLED"), nil
	})
}
