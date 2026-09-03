// SPDX-License-Identifier: Apache-2.0

// Package deadlines holds the per-client call deadlines applied as a backstop to
// this gateway's outbound gRPC connections.
//
// These numbers were MEASURED, not guessed. The card that asked for them said not
// to pick a blanket default without measuring first, and the measurement changed
// the answer twice over. Method: a temporary unary client interceptor logged
// method + elapsed for every call, against a live Scenario B stack (hub + central
// bank + a joined commercial bank), driven by read-only load through the gateway
// (n=60 per endpoint) plus the provisioning traffic of the bring-up itself.
//
//	client          sample                             p50      p95      p99
//	auth+identity   login (end-to-end HTTP, n=40)     55.9ms   64.1ms   75.3ms
//	auth+identity   /auth/me (end-to-end HTTP, n=40)   2.1ms    3.1ms    4.9ms
//	compliance      ListParticipants (n=60)              0ms      0ms      0ms
//	compliance      GetAuditLogs (n=60)                  0ms      1ms      1ms
//	compliance      GetCircuitBreakerStatus (n=60)       0ms      2ms      2ms
//	compliance      RegisterParticipantOnChain (n=1)  4034ms
//	compliance      RegisterCurrencyOnChain (n=1)    16195ms
//	payment         ListDeposits (n=60)                  0ms      0ms      1ms
//	payment         ListEscrows (n=60)                   0ms      0ms      0ms
//
// What the measurement changed:
//
//  1. The card proposed "tight for identity and compliance, generous for payment".
//     Compliance cannot be tight: its reads answer in under 2ms, but the SAME client
//     carries on-chain writes that legitimately took 4.0s and 16.2s. The variance
//     that matters is not between clients, it is between methods within a client.
//
//  2. Per-method overrides were the obvious response and are the wrong one. They
//     reintroduce exactly the gap that made an interceptor the right mechanism: a
//     chain-touching method added later would silently inherit the tight default and
//     be cut. So each client gets ONE value, sized to the slowest legitimate call it
//     can make — deliberately loose.
//
// A loose bound is not a weak one. The defect being closed is the UNBOUNDED call: a
// sibling service that accepts the connection and then stops answering pins a handler
// goroutine forever, because Fiber's WriteTimeout bounds writing the response, not the
// handler's duration. Converting "forever" into "at most four minutes" is the whole
// win. Tightening these towards the observed p99 would trade that win for the risk of
// cutting a legitimate settlement, which is a far worse failure.
package deadlines

import "time"

const (
	// Auth is the auth service connection, shared by the auth and identity adapters
	// (one dial, two adapters). Nothing on it touches the chain: the slowest observed
	// call is a login at 75ms end-to-end, Keycloak round trip included. 15s is ~200x
	// that, and still fails fast enough that a wedged auth service does not look like
	// a hung gateway.
	Auth = 15 * time.Second

	// Compliance covers participant/currency registration, which write on-chain. The
	// slowest measured legitimate call is RegisterCurrencyOnChain at 16.2s, and that
	// was an uncontended local chain — a nonce wait or a busy validator makes it
	// slower. 120s leaves room for that without leaving the call unbounded.
	Compliance = 120 * time.Second

	// Payment carries the cross-currency swap path. This is the one value that must
	// not be derived from the read-path measurement: the swap write path could not be
	// exercised here (the payments routes require the internal relay signature), so
	// the bound comes from what the repository already documents about it — the
	// bridge relay's own HTTP client allows 150s and the tryout documents a swap
	// taking up to 180s. 240s sits above both. A 30s default, which is what a
	// blanket choice would have produced, would cut a legitimate swap.
	Payment = 240 * time.Second
)
