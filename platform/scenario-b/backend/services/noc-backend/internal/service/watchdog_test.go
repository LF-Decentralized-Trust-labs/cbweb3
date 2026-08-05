// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/domain"
)

// fakeMarker stands in for the components repository.
type fakeMarker struct {
	returns []domain.NocComponent
	err     error
	calls   int
}

func (f *fakeMarker) MarkComponentsUnknownForAgent(uuid.UUID) ([]domain.NocComponent, error) {
	f.calls++
	return f.returns, f.err
}

// recordedEvaluation is one call the watchdog made into the alert service.
type recordedEvaluation struct {
	component  string
	status     string
	prevStatus string
}

type fakeEvaluator struct {
	seen []recordedEvaluation
}

func (f *fakeEvaluator) Evaluate(_ context.Context, comp *domain.NocComponent, newStatus, prevStatus string) {
	f.seen = append(f.seen, recordedEvaluation{component: comp.Name, status: newStatus, prevStatus: prevStatus})
}

func componentsOf(statuses map[string]string) []domain.NocComponent {
	out := make([]domain.NocComponent, 0, len(statuses))
	for name, status := range statuses {
		out = append(out, domain.NocComponent{ID: uuid.New(), Name: name, Type: "BESU", HealthStatus: status})
	}
	return out
}

// An agent that stops pushing is a blind spot: its components go UNKNOWN and each
// transition must be evaluated, or the portal shows UNKNOWN with no alert and the NOC
// goes blind silently.
func TestWatchdogAlertsOnTheBlindSpot(t *testing.T) {
	marker := &fakeMarker{returns: componentsOf(map[string]string{"besu-central-bank": "HEALTHY"})}
	alerts := &fakeEvaluator{}
	w := NewWatchdogService(nil, marker, alerts, nil, 3)

	w.markBlindSpot(context.Background(), uuid.New())

	if len(alerts.seen) != 1 {
		t.Fatalf("evaluations = %+v, want exactly one", alerts.seen)
	}
	got := alerts.seen[0]
	if got.status != "UNKNOWN" {
		t.Errorf("newStatus = %q, want UNKNOWN", got.status)
	}
	if got.prevStatus != "HEALTHY" {
		t.Errorf("prevStatus = %q, want the status stored before the flip (HEALTHY)", got.prevStatus)
	}
	// UNKNOWN is a HIGH-severity condition in resolveSeverity; guard that mapping here so
	// the blind spot cannot silently become an unalertable status.
	if sev := resolveSeverity("BESU", "UNKNOWN"); sev != "HIGH" {
		t.Errorf("resolveSeverity(UNKNOWN) = %q, want HIGH", sev)
	}
}

func TestWatchdogEvaluatesEveryComponentOfTheAgent(t *testing.T) {
	marker := &fakeMarker{returns: componentsOf(map[string]string{
		"besu-central-bank": "HEALTHY",
		"cacti-relay":       "OFFLINE",
	})}
	alerts := &fakeEvaluator{}
	w := NewWatchdogService(nil, marker, alerts, nil, 3)

	w.markBlindSpot(context.Background(), uuid.New())

	if len(alerts.seen) != 2 {
		t.Fatalf("evaluations = %+v, want one per component", alerts.seen)
	}
	for _, ev := range alerts.seen {
		if ev.status != "UNKNOWN" {
			t.Errorf("%s: newStatus = %q, want UNKNOWN", ev.component, ev.status)
		}
	}
	// The previous status is carried per component, not overwritten by the last one read.
	prev := map[string]string{}
	for _, ev := range alerts.seen {
		prev[ev.component] = ev.prevStatus
	}
	if prev["besu-central-bank"] != "HEALTHY" || prev["cacti-relay"] != "OFFLINE" {
		t.Errorf("previous statuses mixed up: %+v", prev)
	}
}

// Already-UNKNOWN components keep being evaluated while the agent stays stale, so a
// dismissed blind-spot alert comes back exactly like a still-OFFLINE component's does.
func TestWatchdogKeepsEvaluatingWhileTheAgentStaysStale(t *testing.T) {
	marker := &fakeMarker{returns: componentsOf(map[string]string{"besu-central-bank": "UNKNOWN"})}
	alerts := &fakeEvaluator{}
	w := NewWatchdogService(nil, marker, alerts, nil, 3)

	w.markBlindSpot(context.Background(), uuid.New())
	w.markBlindSpot(context.Background(), uuid.New())

	if len(alerts.seen) != 2 {
		t.Fatalf("evaluations = %d, want one per tick (dedup belongs to the alert service)", len(alerts.seen))
	}
	if alerts.seen[0].prevStatus != "UNKNOWN" {
		t.Errorf("prevStatus = %q, want UNKNOWN for an already-blind component", alerts.seen[0].prevStatus)
	}
}

func TestWatchdogDoesNotEvaluateWhenTheMarkFails(t *testing.T) {
	marker := &fakeMarker{err: errors.New("db down")}
	alerts := &fakeEvaluator{}
	w := NewWatchdogService(nil, marker, alerts, nil, 3)

	w.markBlindSpot(context.Background(), uuid.New())

	if len(alerts.seen) != 0 {
		t.Errorf("evaluated %+v after a failed update; state is unknown, alerts must not be invented", alerts.seen)
	}
}

func TestWatchdogHandlesAnAgentWithNoComponents(t *testing.T) {
	marker := &fakeMarker{}
	alerts := &fakeEvaluator{}
	w := NewWatchdogService(nil, marker, alerts, nil, 3)

	w.markBlindSpot(context.Background(), uuid.New())

	if marker.calls != 1 {
		t.Errorf("marker calls = %d, want 1", marker.calls)
	}
	if len(alerts.seen) != 0 {
		t.Errorf("evaluations = %+v, want none", alerts.seen)
	}
}
