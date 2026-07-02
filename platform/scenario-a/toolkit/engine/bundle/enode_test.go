// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- parseAndRewriteEnode tests (T008) ---

func TestParseAndRewriteEnode_Standard(t *testing.T) {
	got, err := parseAndRewriteEnode("enode://abc@0.0.0.0:30303", "bank.local", 31303)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "enode://abc@bank.local:31303"
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestParseAndRewriteEnode_IPv6Host(t *testing.T) {
	got, err := parseAndRewriteEnode("enode://abc@[::]:30303", "bank.local", 31303)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "enode://abc@bank.local:31303"
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestParseAndRewriteEnode_NoAtSign(t *testing.T) {
	_, err := parseAndRewriteEnode("enode://abc30303", "bank.local", 31303)
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable, got %v", err)
	}
}

func TestParseAndRewriteEnode_NoPrefix(t *testing.T) {
	_, err := parseAndRewriteEnode("abc@0.0.0.0:30303", "bank.local", 31303)
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable, got %v", err)
	}
}

func TestParseAndRewriteEnode_ZeroPort(t *testing.T) {
	_, err := parseAndRewriteEnode("enode://abc@0.0.0.0:30303", "bank.local", 0)
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable for zero port, got %v", err)
	}
}

func TestParseAndRewriteEnode_LongEnodeID(t *testing.T) {
	longID := "enode://deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef@192.168.1.10:30303"
	got, err := parseAndRewriteEnode(longID, "spoke.central-bank.example", 31303)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsString(got, "spoke.central-bank.example:31303") {
		t.Errorf("got %q; expected advertisedHost and port", got)
	}
}

// --- BesuEnodeProvider tests (T026) ---

func TestBesuEnodeProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]string{"enode": "enode://abc@0.0.0.0:30303"},
		})
	}))
	defer srv.Close()

	ep := NewBesuEnodeProvider(srv.URL, nil)
	got, err := ep.NodeInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "enode://abc@0.0.0.0:30303" {
		t.Errorf("got %q; want enode://abc@0.0.0.0:30303", got)
	}
}

func TestBesuEnodeProvider_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ep := NewBesuEnodeProvider(srv.URL, nil)
	_, err := ep.NodeInfo(context.Background())
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable for HTTP 500, got %v", err)
	}
}

func TestBesuEnodeProvider_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // close before calling

	ep := NewBesuEnodeProvider(url, nil)
	_, err := ep.NodeInfo(context.Background())
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable for connection refused, got %v", err)
	}
}

func TestBesuEnodeProvider_NoEnodeField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]string{"name": "Besu"},
		})
	}))
	defer srv.Close()

	ep := NewBesuEnodeProvider(srv.URL, nil)
	_, err := ep.NodeInfo(context.Background())
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable for missing enode field, got %v", err)
	}
}

func TestBesuEnodeProvider_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pre-cancelled context means this handler should not be reached in a meaningful way
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling NodeInfo

	ep := NewBesuEnodeProvider(srv.URL, nil)
	_, err := ep.NodeInfo(ctx)
	if err == nil {
		t.Fatal("expected error for pre-cancelled context, got nil")
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
