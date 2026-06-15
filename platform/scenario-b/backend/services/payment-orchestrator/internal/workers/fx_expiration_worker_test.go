// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewFXExpirationWorker_DefaultsInterval(t *testing.T) {
	w := NewFXExpirationWorker(&fakeFXRepo{}, 0, quietLogger())
	if w.interval != 5*time.Minute {
		t.Errorf("expected default 5m interval, got %v", w.interval)
	}
	w2 := NewFXExpirationWorker(&fakeFXRepo{}, 30*time.Second, quietLogger())
	if w2.interval != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", w2.interval)
	}
}

// runOnce cancels expired non-terminal agreements and appends an audit event.
func TestFXExpirationWorker_CancelsExpired(t *testing.T) {
	repo := &fakeFXRepo{
		expired: []*domain.FXAgreementRecord{
			{TradeID: "t1", State: domain.FXStateProposed},
			{TradeID: "t2", State: domain.FXStateAccepted},
		},
	}
	w := NewFXExpirationWorker(repo, time.Minute, quietLogger())

	w.runOnce(context.Background())

	if len(repo.updated) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(repo.updated))
	}
	for _, rec := range repo.updated {
		if rec.State != domain.FXStateCancelled {
			t.Errorf("trade %s expected CANCELLED, got %s", rec.TradeID, rec.State)
		}
	}
	if len(repo.auditEvents) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(repo.auditEvents))
	}
	ev := repo.auditEvents[0]
	if ev.ToState != domain.FXStateCancelled || ev.Source != domain.EventSourceSystemJob {
		t.Errorf("unexpected audit event: %+v", ev)
	}
	if ev.Actor != "system:fx-expiration-worker" {
		t.Errorf("unexpected actor: %s", ev.Actor)
	}
}

// Terminal records returned by the query are skipped defensively (no update).
func TestFXExpirationWorker_SkipsTerminal(t *testing.T) {
	repo := &fakeFXRepo{
		expired: []*domain.FXAgreementRecord{
			{TradeID: "settled", State: domain.FXStateSettled},
			{TradeID: "live", State: domain.FXStateProposed},
		},
	}
	w := NewFXExpirationWorker(repo, time.Minute, quietLogger())

	w.runOnce(context.Background())

	if len(repo.updated) != 1 || repo.updated[0].TradeID != "live" {
		t.Errorf("expected only 'live' updated, got %+v", repo.updated)
	}
}

// A list error aborts the run without panicking or updating anything.
func TestFXExpirationWorker_ListErrorIsHandled(t *testing.T) {
	repo := &fakeFXRepo{listErr: errors.New("db down")}
	w := NewFXExpirationWorker(repo, time.Minute, quietLogger())

	w.runOnce(context.Background())

	if len(repo.updated) != 0 {
		t.Errorf("expected no updates on list error")
	}
}

// An update error for one record must not block processing the rest.
func TestFXExpirationWorker_UpdateErrorContinues(t *testing.T) {
	repo := &fakeFXRepo{
		expired: []*domain.FXAgreementRecord{
			{TradeID: "bad", State: domain.FXStateProposed},
			{TradeID: "good", State: domain.FXStateProposed},
		},
		updateErr: map[string]error{"bad": errors.New("write conflict")},
	}
	w := NewFXExpirationWorker(repo, time.Minute, quietLogger())

	w.runOnce(context.Background())

	// "bad" update failed (no audit event), "good" succeeded.
	if len(repo.updated) != 1 || repo.updated[0].TradeID != "good" {
		t.Errorf("expected only 'good' updated, got %+v", repo.updated)
	}
	if len(repo.auditEvents) != 1 || repo.auditEvents[0].TradeID != "good" {
		t.Errorf("expected audit only for 'good', got %+v", repo.auditEvents)
	}
}

// Start runs an immediate pass then stops on context cancellation.
func TestFXExpirationWorker_StartRunsOnceThenStops(t *testing.T) {
	repo := &fakeFXRepo{
		expired: []*domain.FXAgreementRecord{{TradeID: "t1", State: domain.FXStateProposed}},
	}
	w := NewFXExpirationWorker(repo, time.Hour, quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Start(ctx); close(done) }()

	deadline := time.After(2 * time.Second)
	for len(repo.updated) == 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("Start did not run initial pass")
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not stop after cancel")
	}
}
