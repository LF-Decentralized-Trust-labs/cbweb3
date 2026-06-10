package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// pollUntil calls fn repeatedly until it returns (true, nil) or the deadline expires.
// fn returning a non-nil error immediately fails the test.
// fn returning (false, nil) causes a retry after interval.
func pollUntil(t *testing.T, interval, timeout time.Duration, fn func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	attempts := 0
	for time.Now().Before(deadline) {
		ok, err := fn()
		require.NoErrorf(t, err, "pollUntil: fn returned error after %d attempts", attempts)
		if ok {
			return
		}
		attempts++
		time.Sleep(interval)
	}
	t.Fatalf("pollUntil: condition not met after %s (%d attempts)", timeout, attempts)
}

// pollBridgeActive polls GET /api/v2/bridge/positions until positionID shows bridge_state=ACTIVE.
func pollBridgeActive(t *testing.T, c *httpClient, positionID, label string) {
	t.Helper()
	pollUntil(t, 5*time.Second, 3*time.Minute, func() (bool, error) {
		var resp struct {
			Positions []struct {
				PositionID  string `json:"position_id"`
				BridgeState string `json:"bridge_state"`
			} `json:"positions"`
		}
		if err := c.get(t, "/api/v2/bridge/positions", &resp); err != nil {
			return false, nil
		}
		for _, p := range resp.Positions {
			if p.PositionID == positionID {
				t.Logf("  %s bridge state: %s", label, p.BridgeState)
				return p.BridgeState == "ACTIVE", nil
			}
		}
		return false, nil
	})
}

// waitForHTTP polls url/healthz until it returns HTTP 200 or the deadline expires.
func waitForHTTP(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()
	c := newHTTPClient(baseURL, "")
	pollUntil(t, 3*time.Second, timeout, func() (bool, error) {
		var raw map[string]interface{}
		err := c.get(t, "/healthz", &raw)
		return err == nil, nil
	})
}
