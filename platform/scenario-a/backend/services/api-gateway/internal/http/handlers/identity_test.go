// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestIdentityHandler_ListIdentities(t *testing.T) {
	want := []string{"funded_operator@spoke-a-cb", "funded_operator@spoke-b-bank-b"}
	h := NewIdentityHandler(want)

	app := fiber.New()
	app.Get("/api/v1/identities", h.ListIdentities)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/identities", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Identities []string `json:"identities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
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

func TestNewIdentityHandler_CopiesInput(t *testing.T) {
	src := []string{"funded_operator@spoke-a-cb"}
	h := NewIdentityHandler(src)
	src[0] = "mutated" // mutating the caller's slice must not affect the handler

	if h.identities[0] != "funded_operator@spoke-a-cb" {
		t.Errorf("handler leaked caller slice: got %q", h.identities[0])
	}
}

func TestIdentityHandler_EmptyList(t *testing.T) {
	h := NewIdentityHandler(nil)

	app := fiber.New()
	app.Get("/api/v1/identities", h.ListIdentities)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/identities", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Identities []string `json:"identities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Identities) != 0 {
		t.Errorf("identities = %v, want empty", body.Identities)
	}
}
