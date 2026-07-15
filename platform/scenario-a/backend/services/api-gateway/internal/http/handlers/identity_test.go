// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type fakeRosterProvider struct {
	identities []string
	err        error
}

func (f *fakeRosterProvider) ListParticipantIdentities(context.Context) ([]string, error) {
	return f.identities, f.err
}

func doIdentities(t *testing.T, h *IdentityHandler) (int, struct {
	Identities []string `json:"identities"`
	Configured bool     `json:"configured"`
}) {
	t.Helper()
	app := fiber.New()
	app.Get("/api/v1/identities", h.ListIdentities)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/identities", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Identities []string `json:"identities"`
		Configured bool     `json:"configured"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.StatusCode, body
}

func TestIdentityHandler_ServesStaticOverride(t *testing.T) {
	want := []string{"funded_operator@spoke-a-cb", "funded_operator@spoke-b-bank-b"}
	h := NewIdentityHandler(NewIdentityRoster(want, nil))

	status, body := doIdentities(t, h)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !body.Configured {
		t.Error("configured = false, want true for a non-empty roster")
	}
	if len(body.Identities) != len(want) {
		t.Fatalf("identities = %v, want %v", body.Identities, want)
	}
	for i, id := range want {
		if body.Identities[i] != id {
			t.Errorf("identities[%d] = %q, want %q", i, body.Identities[i], id)
		}
	}
}

func TestIdentityHandler_SourcesLiveMembership(t *testing.T) {
	live := &fakeRosterProvider{identities: []string{"cb@brl", "itau@brl"}}
	// Empty override → the roster falls through to live Pente membership.
	h := NewIdentityHandler(NewIdentityRoster(nil, live))

	status, body := doIdentities(t, h)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !body.Configured || len(body.Identities) != 2 {
		t.Errorf("expected 2 live identities and configured=true, got %+v", body)
	}
}

func TestIdentityRoster_UnionOfConfigAndLive(t *testing.T) {
	// Config carries the cross-spoke consortium roster; live adds a newly-joined
	// local bank. The union is de-duplicated with config entries first.
	override := []string{"funded_operator@spoke-brl-cb", "funded_operator@spoke-cop-cb"}
	live := &fakeRosterProvider{identities: []string{
		"funded_operator@spoke-brl-cb",         // dup of config → dropped
		"funded_operator@spoke-brl-bank-newco", // new local member
	}}
	r := NewIdentityRoster(override, live)

	ids, err := r.Identities(context.Background())
	if err != nil {
		t.Fatalf("Identities: %v", err)
	}
	want := []string{
		"funded_operator@spoke-brl-cb",
		"funded_operator@spoke-cop-cb",
		"funded_operator@spoke-brl-bank-newco",
	}
	if len(ids) != len(want) {
		t.Fatalf("union = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("union[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestIdentityRoster_ConfigSurvivesLiveError(t *testing.T) {
	// A configured roster is authoritative: a live-membership fetch error must not
	// wipe it out (best-effort augmentation).
	override := []string{"funded_operator@spoke-brl-cb"}
	r := NewIdentityRoster(override, &fakeRosterProvider{err: errors.New("orchestrator down")})

	ids, err := r.Identities(context.Background())
	if err != nil {
		t.Fatalf("Identities: %v", err)
	}
	if len(ids) != 1 || ids[0] != "funded_operator@spoke-brl-cb" {
		t.Errorf("roster = %v, want the configured entry despite live error", ids)
	}
}

func TestIdentityHandler_EmptyRosterNotConfigured(t *testing.T) {
	h := NewIdentityHandler(NewIdentityRoster(nil, nil))

	status, body := doIdentities(t, h)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body.Configured {
		t.Error("configured = true, want false when no roster is configured")
	}
	if len(body.Identities) != 0 {
		t.Errorf("identities = %v, want empty", body.Identities)
	}
}

func TestIdentityHandler_LiveErrorReturns502(t *testing.T) {
	live := &fakeRosterProvider{err: errors.New("orchestrator down")}
	h := NewIdentityHandler(NewIdentityRoster(nil, live))

	status, _ := doIdentities(t, h)
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", status)
	}
}

func TestNewIdentityRoster_CopiesOverride(t *testing.T) {
	src := []string{"funded_operator@spoke-a-cb"}
	r := NewIdentityRoster(src, nil)
	src[0] = "mutated" // mutating the caller's slice must not affect the roster

	ids, err := r.Identities(context.Background())
	if err != nil {
		t.Fatalf("Identities: %v", err)
	}
	if len(ids) != 1 || ids[0] != "funded_operator@spoke-a-cb" {
		t.Errorf("roster leaked caller slice: got %v", ids)
	}
}

func TestFederatedIdentityHandler_ScopeLocalUsesLocalRoster(t *testing.T) {
	// The federated (default) roster carries the network-wide view; the local
	// roster carries only this spoke. ?scope=local must return the local roster —
	// the recursion guard that keeps a peer lookup from re-federating.
	federated := NewIdentityRoster([]string{"funded_operator@spoke-brl-cb", "funded_operator@spoke-cop-cb"}, nil)
	local := NewIdentityRoster([]string{"funded_operator@spoke-brl-cb"}, nil)
	h := NewFederatedIdentityHandler(federated, local)

	app := fiber.New()
	app.Get("/api/v1/identities", h.ListIdentities)

	// Default: network-wide (both spokes).
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/identities", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	var netBody struct {
		Identities []string `json:"identities"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&netBody)
	resp.Body.Close()
	if len(netBody.Identities) != 2 {
		t.Errorf("default roster = %v, want 2 (network-wide)", netBody.Identities)
	}

	// scope=local: only the local spoke.
	resp2, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/identities?scope=local", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	var locBody struct {
		Identities []string `json:"identities"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&locBody)
	if len(locBody.Identities) != 1 || locBody.Identities[0] != "funded_operator@spoke-brl-cb" {
		t.Errorf("scope=local roster = %v, want only the local entry", locBody.Identities)
	}
}
