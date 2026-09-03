// SPDX-License-Identifier: Apache-2.0

package workers_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/workers"
)

// fakeFXRepo is a hand-written FXAgreementRepository test double that records
// the calls relevant to the expiration sweep.
type fakeFXRepo struct {
	mu sync.Mutex

	expired   []*domain.FXAgreementRecord
	listErr   error
	updateErr error

	updated []*domain.FXAgreementRecord
	events  []*domain.FXAgreementEvent
	listN   int
}

func (f *fakeFXRepo) CreateAgreement(context.Context, *domain.FXAgreementRecord) error { return nil }
func (f *fakeFXRepo) GetAgreement(context.Context, string) (*domain.FXAgreementRecord, error) {
	return nil, nil
}
func (f *fakeFXRepo) UpdateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, r)
	return nil
}
func (f *fakeFXRepo) ListAgreements(context.Context, ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}
func (f *fakeFXRepo) CreateAuditEvent(_ context.Context, e *domain.FXAgreementEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}
func (f *fakeFXRepo) ListAuditEvents(context.Context, string) ([]*domain.FXAgreementEvent, error) {
	return nil, nil
}
func (f *fakeFXRepo) ListExpiredNonTerminal(_ context.Context, _ int64) ([]*domain.FXAgreementRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listN++
	if f.listErr != nil {
		return nil, f.listErr
	}
	// Return copies so the worker's mutation does not corrupt the source slice
	// across repeated ticks.
	out := make([]*domain.FXAgreementRecord, 0, len(f.expired))
	for _, r := range f.expired {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewFXExpirationWorker_DefaultsInterval(t *testing.T) {
	// A non-positive interval must be coerced to the 5-minute default; the
	// worker must still be usable. We only assert it constructs without panic.
	w := workers.NewFXExpirationWorker(&fakeFXRepo{}, 0, testLogger())
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	w2 := workers.NewFXExpirationWorker(&fakeFXRepo{}, -1*time.Second, testLogger())
	if w2 == nil {
		t.Fatal("expected non-nil worker for negative interval")
	}
}

func TestFXExpirationWorker_RunOnce(t *testing.T) {
	cases := []struct {
		name         string
		input        []*domain.FXAgreementRecord
		listErr      error
		updateErr    error
		wantUpdated  int
		wantEvents   int
		wantCancelID map[string]bool
	}{
		{
			name: "cancels proposed and accepted, skips terminal",
			input: []*domain.FXAgreementRecord{
				{TradeID: "p1", State: domain.FXStateProposed},
				{TradeID: "a1", State: domain.FXStateAccepted},
				{TradeID: "t1", State: domain.FXStateSettled},
				{TradeID: "t2", State: domain.FXStateRejected},
				{TradeID: "t3", State: domain.FXStateCancelled},
			},
			wantUpdated:  2,
			wantEvents:   2,
			wantCancelID: map[string]bool{"p1": true, "a1": true},
		},
		{
			name:        "empty list does nothing",
			input:       nil,
			wantUpdated: 0,
			wantEvents:  0,
		},
		{
			name:        "list error short-circuits with no updates",
			input:       []*domain.FXAgreementRecord{{TradeID: "p1", State: domain.FXStateProposed}},
			listErr:     errors.New("db down"),
			wantUpdated: 0,
			wantEvents:  0,
		},
		{
			name:        "update error skips audit event for that record",
			input:       []*domain.FXAgreementRecord{{TradeID: "p1", State: domain.FXStateProposed}},
			updateErr:   errors.New("update failed"),
			wantUpdated: 0,
			wantEvents:  0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeFXRepo{expired: c.input, listErr: c.listErr, updateErr: c.updateErr}
			w := workers.NewFXExpirationWorker(repo, time.Hour, testLogger())

			// runOnce is exercised via Start with an immediately-cancelled context:
			// Start calls runOnce once before entering the ticker loop.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			w.Start(ctx)

			if len(repo.updated) != c.wantUpdated {
				t.Errorf("updated = %d, want %d", len(repo.updated), c.wantUpdated)
			}
			if len(repo.events) != c.wantEvents {
				t.Errorf("events = %d, want %d", len(repo.events), c.wantEvents)
			}
			for _, u := range repo.updated {
				if u.State != domain.FXStateCancelled {
					t.Errorf("trade %s updated to %s, want CANCELLED", u.TradeID, u.State)
				}
				if c.wantCancelID != nil && !c.wantCancelID[u.TradeID] {
					t.Errorf("unexpected cancellation of trade %s", u.TradeID)
				}
			}
			for _, e := range repo.events {
				if e.ToState != domain.FXStateCancelled {
					t.Errorf("event ToState = %s, want CANCELLED", e.ToState)
				}
				if e.Source != domain.EventSourceSystemJob {
					t.Errorf("event Source = %s, want SYSTEM_JOB", e.Source)
				}
			}
		})
	}
}

func TestFXExpirationWorker_StartLoopTicksAndStops(t *testing.T) {
	// Use a very short interval so the ticker fires at least once before we
	// cancel; verifies the ticker path (not just the initial runOnce) and the
	// graceful ctx.Done() shutdown.
	repo := &fakeFXRepo{expired: []*domain.FXAgreementRecord{
		{TradeID: "p1", State: domain.FXStateProposed},
	}}
	w := workers.NewFXExpirationWorker(repo, 5*time.Millisecond, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	// Wait until the worker has run more than once (initial + at least one tick).
	deadline := time.After(2 * time.Second)
	for {
		repo.mu.Lock()
		n := repo.listN
		repo.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("worker did not tick more than once")
		case <-time.After(2 * time.Millisecond):
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancel")
	}
}
