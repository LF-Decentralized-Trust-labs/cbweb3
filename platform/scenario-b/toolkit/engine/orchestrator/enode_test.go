// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminNodeInfoEnode(t *testing.T) {
	want := "enode://abcd@127.0.0.1:30303"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"enode":"` + want + `"}}`))
	}))
	defer srv.Close()

	got, err := adminNodeInfoEnode(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("enode = %q, want %q", got, want)
	}
}

func TestAdminNodeInfoEnodeEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()
	if _, err := adminNodeInfoEnode(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for empty enode")
	}
}
