// SPDX-License-Identifier: Apache-2.0

// Server-timeout tests for finding R2-LOW (backend service robustness).
//
// The Fiber app was built with no ReadTimeout, so a connection that opened and then sent
// its request headers one byte at a time held a server slot for as long as it liked — the
// slowloris shape. Nothing in the handler chain bounds that, because it happens before any
// handler runs.
//
// The tests also pin the other side of the trade: the timeouts must NOT cut a legitimately
// slow handler. A cross-currency swap is documented as taking up to 180s, and the gateway's
// own internal deadlines already reach 150s. fasthttp applies WriteTimeout to writing the
// response rather than to the handler's duration — measured, not assumed — so a tight value
// is safe here, and this test is what keeps it that way if anyone changes the config.
package app

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// listen starts app on an ephemeral port and returns its address.
func listen(t *testing.T, app *fiber.App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	time.Sleep(200 * time.Millisecond) // let the listener come up
	return ln.Addr().String()
}

// A connection that dribbles its request must be closed by the server rather than held
// open indefinitely. Without ReadTimeout this test hangs until its own deadline.
func TestServerTimeouts_SlowRequestIsDropped(t *testing.T) {
	app := fiber.New(testTimeoutConfig())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })
	addr := listen(t, app)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a request line and then stall, never completing the headers.
	if _, err := fmt.Fprintf(conn, "GET /x HTTP/1.1\r\nHost: x\r\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server must close the connection on its own. Give it the configured read
	// timeout plus slack; a server with no timeout leaves us blocked until this deadline.
	cfg := testTimeoutConfig()
	_ = conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout + 5*time.Second))
	_, err = bufio.NewReader(conn).ReadString('\n')
	if err == nil {
		return // the server answered (e.g. 408) — also a bounded outcome
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("the server never closed a stalled connection: no ReadTimeout is in effect")
	}
	// io.EOF / reset: the server dropped it, which is what we want.
	if err != io.EOF {
		t.Logf("connection closed with %v (acceptable: the server dropped it)", err)
	}
}

// The timeouts must not truncate a handler that legitimately runs longer than them.
func TestServerTimeouts_SlowHandlerStillCompletes(t *testing.T) {
	cfg := testTimeoutConfig()
	app := fiber.New(cfg)
	// Comfortably longer than WriteTimeout, far shorter than the 180s real ceiling so the
	// suite stays quick.
	handlerDelay := cfg.WriteTimeout + 2*time.Second
	app.Get("/slow", func(c *fiber.Ctx) error {
		time.Sleep(handlerDelay)
		return c.SendString("done")
	})
	addr := listen(t, app)

	cl := &http.Client{Timeout: handlerDelay + 20*time.Second}
	resp, err := cl.Get("http://" + addr + "/slow")
	if err != nil {
		t.Fatalf("a handler of %v was cut by the server timeouts: %v", handlerDelay, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "done" {
		t.Errorf("body = %q, want \"done\" — the response was truncated", body)
	}
}

// The configuration must actually carry bounds; a zero value means "no limit".
func TestServerTimeouts_AreConfigured(t *testing.T) {
	cfg := serverConfig() // the PRODUCTION values, not the short test ones
	if cfg.ReadTimeout <= 0 {
		t.Error("ReadTimeout is unset — a stalled request holds a server slot indefinitely")
	}
	if cfg.WriteTimeout <= 0 {
		t.Error("WriteTimeout is unset")
	}
	if cfg.IdleTimeout <= 0 {
		t.Error("IdleTimeout is unset — idle keep-alive connections are never reclaimed")
	}
	// An hour-long read window is a bound in name only; keep the value meaningful.
	if cfg.ReadTimeout > time.Minute {
		t.Errorf("ReadTimeout = %v: too long to bound a stalled request usefully", cfg.ReadTimeout)
	}
}

// testTimeoutConfig is the production configuration on one-second bounds. The behaviour
// under test is identical; waiting on the real 30s ReadTimeout cost 62s per scenario.
func testTimeoutConfig() fiber.Config {
	return serverConfigWith(time.Second, time.Second, 2*time.Second)
}
