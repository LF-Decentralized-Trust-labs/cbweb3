// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// checkRelayStep proves the joining bank's relay endpoint actually answers, before the join
// invests in Besu, Paladin and a backend stack.
//
// It exists because of how a bank settles. An HTLC leg is settled by transferLocked on its
// owner's own Paladin node, over locked Zeto states that are local to that node — no other
// entity can do it, and the relay's inbound SettleHTLC push cannot reach a commercial bank
// at all (only the founding central bank registers a gRPC endpoint). What settles a bank's
// leg is the bank itself, reacting to the relay's settle journal, which its
// payment-orchestrator polls at CACTI_API_URL.
//
// So the relay being reachable is not an optimisation for a bank — it is the whole settlement
// path. And a wrong endpoint fails silently in the worst way: CACTI_API_URL falls back to the
// co-located http://host.docker.internal:4000, which is non-empty, so the orchestrator's
// startup check passes, and on a separate host points at nothing. The bank joins clean, polls
// into the void, and never learns of a counterpart lock or a revealed secret.
//
// Retries rather than probing once: on a fresh bring-up the relay may still be starting.
type checkRelayStep struct {
	endpoint   string
	timeout    time.Duration
	interval   time.Duration
	httpClient *http.Client
	logw       io.Writer
}

func newCheckRelayStep(endpoint string, timeout, interval time.Duration, logw io.Writer) Step {
	if interval <= 0 {
		interval = time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &checkRelayStep{
		endpoint:   strings.TrimRight(endpoint, "/"),
		timeout:    timeout,
		interval:   interval,
		httpClient: sharedJoinHTTPClient,
		logw:       logw,
	}
}

func (s *checkRelayStep) Name() string { return StepCheckRelay }

// Check is not idempotent by inspection; the orchestrator's persisted state short-circuits
// re-runs, as with wait-sync.
func (s *checkRelayStep) Check(_ context.Context) (bool, error) { return false, nil }

func (s *checkRelayStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	url := s.endpoint + "/api/v1/health"
	var last error
	for {
		status, err := probeRelayHealth(ctx, s.httpClient, url)
		switch {
		case err != nil:
			last = fmt.Errorf("cannot reach the relay at %s: %w", s.endpoint, err)
		case status >= 200 && status < 300:
			return nil
		default:
			last = fmt.Errorf("the relay at %s answered HTTP %d", s.endpoint, status)
		}
		fmt.Fprintf(s.logw, "check-relay: %v — retrying\n", last)

		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"check-relay: %w. This bank settles its own HTLC legs by polling the relay's "+
					"settle journal, so without the relay it would join successfully and then never "+
					"settle anything, silently. Fix spec.relay.endpoint (or the bundle's) and re-run",
				last,
			)
		case <-time.After(s.interval):
		}
	}
}

func probeRelayHealth(ctx context.Context, c *http.Client, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
