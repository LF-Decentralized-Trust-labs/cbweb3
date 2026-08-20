// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// serveUnix starts an HTTP server on a Unix socket and returns its path.
func serveUnix(t *testing.T, h http.Handler) string {
	t.Helper()
	// Not t.TempDir(): a Unix socket path is capped near 104 bytes and the per-test
	// temp directory on macOS is long enough to blow through it.
	dir, err := os.MkdirTemp("", "noc")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	sock := filepath.Join(dir, "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen %s: %v", sock, err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return sock
}

// Ping is the agent's startup check that it can actually talk to the Docker socket. It
// carries the whole warning: a socket the agent may not read is otherwise invisible,
// because collectLogs discards its error. So it has to separate "reachable" from every
// way the socket can fail — including a daemon that answers with something other than OK.
func TestPing(t *testing.T) {
	ok := serveUnix(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_ping" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	if err := New(ok).Ping(context.Background()); err != nil {
		t.Errorf("Ping on a reachable socket: %v", err)
	}

	// A socket that is not there at all — the agent started without the mount.
	absent := filepath.Join(t.TempDir(), "absent.sock")
	if err := New(absent).Ping(context.Background()); err == nil {
		t.Error("Ping on an absent socket: want an error, got nil")
	}

	// Reachable but refusing — stands in for the denied/unhealthy daemon. A permission
	// error surfaces at dial time and is covered by the absent case above; this covers
	// the other half, a response that is not OK.
	refusing := serveUnix(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	if err := New(refusing).Ping(context.Background()); err == nil {
		t.Error("Ping against a non-200 response: want an error, got nil")
	}
}
