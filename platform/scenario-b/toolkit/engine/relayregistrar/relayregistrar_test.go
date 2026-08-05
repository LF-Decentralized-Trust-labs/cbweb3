// SPDX-License-Identifier: Apache-2.0

package relayregistrar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

var validSpoke = Spoke{
	ID: "spoke-br", BesuRPC: "http://h:8545", BesuWS: "ws://h:8546", GatewayURL: "http://gw",
}

func TestLocalRegisterIdempotentAndList(t *testing.T) {
	rr, err := New("local")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := rr.Register(ctx, validSpoke); err != nil {
		t.Fatal(err)
	}
	// Re-register same id → idempotent (no duplicate).
	if err := rr.Register(ctx, validSpoke); err != nil {
		t.Fatal(err)
	}
	lr, ok := rr.(LocalRegistry)
	if !ok {
		t.Fatal("local must implement LocalRegistry")
	}
	if got := len(lr.List()); got != 1 {
		t.Fatalf("List len = %d, want 1", got)
	}
}

func TestRegisterRejectsInvalidSpoke(t *testing.T) {
	rr, _ := New("local")
	if err := rr.Register(context.Background(), Spoke{ID: "x"}); err != ErrInvalidSpoke {
		t.Fatalf("got %v, want ErrInvalidSpoke", err)
	}
}

func TestFactoryURIs(t *testing.T) {
	if _, err := New("local"); err != nil {
		t.Fatalf("local: %v", err)
	}
	if _, err := New("relay://relay-host:4000"); err != nil {
		t.Fatalf("relay://: %v", err)
	}
	if _, err := New("vault://x"); err != ErrUnsupportedURI {
		t.Fatalf("got %v, want ErrUnsupportedURI", err)
	}
}

// SC-008 / prod stub: Register POSTs to /api/v1/spokes.
func TestProdRegistrarPostsToRelay(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rr, err := New("relay://" + srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rr.Register(context.Background(), validSpoke); err != nil {
		t.Fatalf("prod Register: %v", err)
	}
	if gotPath != "/api/v1/spokes" {
		t.Fatalf("posted to %q, want /api/v1/spokes", gotPath)
	}
	// prod must not expose the local List surface.
	if _, ok := rr.(LocalRegistry); ok {
		t.Fatal("prod stub must not implement LocalRegistry")
	}
}
