// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPRelayRegistrar_Register_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/spokes" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	reg := NewHTTPRelayRegistrar(srv.URL)
	err := reg.Register(context.Background(), SpokeInfo{SpokeID: "spoke-test"})
	if err != nil {
		t.Errorf("Register returned unexpected error: %v", err)
	}
}

func TestHTTPRelayRegistrar_Register_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	reg := NewHTTPRelayRegistrar(srv.URL)
	err := reg.Register(context.Background(), SpokeInfo{SpokeID: "spoke-test"})
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if !errors.Is(err, ErrRelayUnavailable) {
		t.Errorf("expected ErrRelayUnavailable, got: %v", err)
	}
}

func TestHTTPRelayRegistrar_Register_ConnectionRefused(t *testing.T) {
	reg := NewHTTPRelayRegistrar("http://127.0.0.1:1") // nothing listening there
	err := reg.Register(context.Background(), SpokeInfo{SpokeID: "spoke-test"})
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
	if !errors.Is(err, ErrRelayUnavailable) {
		t.Errorf("expected ErrRelayUnavailable, got: %v", err)
	}
}

func TestHTTPRelayRegistrar_IsRegistered_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/spokes/spoke-test" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"spoke_id": "spoke-test"})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	reg := NewHTTPRelayRegistrar(srv.URL)
	ok, err := reg.IsRegistered(context.Background(), "spoke-test")
	if err != nil {
		t.Fatalf("IsRegistered returned unexpected error: %v", err)
	}
	if !ok {
		t.Error("IsRegistered returned false; want true for HTTP 200")
	}
}

func TestHTTPRelayRegistrar_IsRegistered_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	reg := NewHTTPRelayRegistrar(srv.URL)
	ok, err := reg.IsRegistered(context.Background(), "spoke-test")
	if err != nil {
		t.Fatalf("IsRegistered returned unexpected error: %v", err)
	}
	if ok {
		t.Error("IsRegistered returned true; want false for HTTP 404")
	}
}

func TestHTTPRelayRegistrar_IsRegistered_ConnectionRefused(t *testing.T) {
	reg := NewHTTPRelayRegistrar("http://127.0.0.1:1")
	ok, err := reg.IsRegistered(context.Background(), "spoke-test")
	if err != nil {
		t.Fatalf("IsRegistered should swallow connection errors, got: %v", err)
	}
	if ok {
		t.Error("IsRegistered should return false when relay is unreachable")
	}
}
