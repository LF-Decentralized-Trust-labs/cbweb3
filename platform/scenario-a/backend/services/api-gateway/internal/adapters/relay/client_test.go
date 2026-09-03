// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestClient_ListSpokes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/spokes" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"spoke-brl","internalApiUrl":"http://host.docker.internal:18645","besuRpc":"http://x:8645"},
			{"id":"spoke-cop","internalApiUrl":"http://host.docker.internal:18745"}
		]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", time.Second)
	got, err := c.ListSpokes(context.Background())
	if err != nil {
		t.Fatalf("ListSpokes: %v", err)
	}
	want := []Spoke{
		{ID: "spoke-brl", InternalApiURL: "http://host.docker.internal:18645"},
		{ID: "spoke-cop", InternalApiURL: "http://host.docker.internal:18745"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spokes = %v, want %v", got, want)
	}
}

func TestClient_FetchPeerIdentities_UsesInternalEndpointWithRelayAuth(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("X-Relay-Auth")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identities":["funded_operator@spoke-cop-cb"],"configured":true}`))
	}))
	defer srv.Close()

	c := NewClient("http://relay:4000", "test-secret", time.Second)
	got, err := c.FetchPeerIdentities(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchPeerIdentities: %v", err)
	}
	if gotPath != "/internal/v1/identities" {
		t.Errorf("peer queried path %q, want /internal/v1/identities", gotPath)
	}
	if gotAuth != "test-secret" {
		t.Errorf("peer queried with X-Relay-Auth=%q, want the shared secret", gotAuth)
	}
	if !reflect.DeepEqual(got, []string{"funded_operator@spoke-cop-cb"}) {
		t.Errorf("identities = %v", got)
	}
}

func TestClient_ErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", time.Second)
	if _, err := c.ListSpokes(context.Background()); err == nil {
		t.Error("expected an error on 500, got nil")
	}
}
