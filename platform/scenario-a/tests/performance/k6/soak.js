/**
 * soak.js — 12-hour soak harness for Scenario A (R1-12.3).
 *
 * Drives a MODERATE, continuous mixed load (HTLC lock + escrow + read operations) over a
 * long window to surface memory leaks, connection-pool exhaustion, and state drift across
 * Besu nodes and the Paladin/Zeto layer.
 *
 * Thresholds during soak (same gates as short-run tests):
 *   - <1% error rate sustained for the full window
 *   - Read p95 < 500ms   (no degradation over time)
 *   - Write p95 < 1500ms (no degradation over time)
 *   - No memory growth trend / node crash (observed out-of-band via metrics)
 *
 * Out-of-band monitoring required during this run:
 *   - `docker stats` or Prometheus: RSS and goroutines for each backend service
 *   - Postgres `pg_stat_activity`: watch for unbounded connection growth
 *   - Besu logs: confirm no QBFT consensus failures or node restarts on either spoke
 *   - Cacti relay logs: confirm no relay crashes or event backlog growth
 *
 * Usage:
 *   API_GW_URL=http://localhost:3001 AUTH_TOKEN=<jwt> \
 *   RECEIVER=funded_operator@spoke-a-bank-c DURATION=12h \
 *   k6 run tests/performance/k6/soak.js
 *
 * IMPORTANT: This is a 12-hour run. Do NOT execute as part of CI or a local smoke test.
 * Run it on dedicated devnet infra with metrics scraping enabled.
 *
 * Environment variables:
 *   API_GW_URL    — bank-a API Gateway base URL (default: http://localhost:3001)
 *   AUTH_TOKEN    — access_token JWT value (REQUIRED)
 *   RECEIVER      — Paladin identity of the HTLC receiver (default: funded_operator@spoke-a-bank-c)
 *   AMOUNT        — token amount (default: 1000000)
 *   DURATION      — soak window (default: 12h)
 *   READ_TPS      — HTLC search req/s (default: 10 — moderate)
 *   LOCK_TPS      — HTLC lock req/s  (default: 5  — moderate)
 *   ESCROW_TPS    — escrow req/s     (default: 3  — moderate)
 */

import http from "k6/http";
import { check } from "k6";
import { Trend, Rate } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3001";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const RECEIVER = __ENV.RECEIVER || "funded_operator@spoke-a-bank-c";
const AMOUNT = __ENV.AMOUNT || "1000000";
const DURATION = __ENV.DURATION || "12h";

const READ_TPS = Number(__ENV.READ_TPS || 10);
const LOCK_TPS = Number(__ENV.LOCK_TPS || 5);
const ESCROW_TPS = Number(__ENV.ESCROW_TPS || 3);

const readLatency = new Trend("soak_read_latency_ms", true);
const lockLatency = new Trend("soak_lock_latency_ms", true);
const escrowLatency = new Trend("soak_escrow_latency_ms", true);
const okRate = new Rate("soak_ok_rate");

function arrival(exec, rate) {
  return {
    executor: "constant-arrival-rate",
    exec,
    rate,
    timeUnit: "1s",
    duration: DURATION,
    preAllocatedVUs: Math.max(5, rate * 3),
    maxVUs: Math.max(20, rate * 8),
  };
}

export const options = {
  scenarios: {
    read: arrival("readScenario", READ_TPS),
    lock: arrival("lockScenario", LOCK_TPS),
    escrow: arrival("escrowScenario", ESCROW_TPS),
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    soak_ok_rate: ["rate>0.99"],
    soak_read_latency_ms: ["p(95)<500"],
    soak_lock_latency_ms: ["p(95)<1500"],
    soak_escrow_latency_ms: ["p(95)<1500"],
  },
};

function cookieJar() {
  const jar = http.cookieJar();
  if (AUTH_TOKEN) jar.set(API_GW_URL, "access_token", AUTH_TOKEN);
  return jar;
}

function headers() {
  return { "Content-Type": "application/json", Accept: "application/json" };
}

export function readScenario() {
  const r = http.get(`${API_GW_URL}/api/v1/htlc/search?state=LOCKED`, {
    headers: headers(),
    jar: cookieJar(),
    tags: { endpoint: "htlc-search" },
  });
  readLatency.add(r.timings.duration);
  okRate.add(check(r, { "read 200": (res) => res.status === 200 }));
}

export function lockScenario() {
  if (!AUTH_TOKEN) return;
  const timeLock = Math.floor(Date.now() / 1000) + 3600;
  const r = http.post(
    `${API_GW_URL}/api/v1/htlc/lock`,
    JSON.stringify({ receiver: RECEIVER, amount: AMOUNT, time_lock: timeLock }),
    { headers: headers(), jar: cookieJar(), tags: { endpoint: "htlc-lock" } },
  );
  lockLatency.add(r.timings.duration);
  okRate.add(check(r, { "lock 201": (res) => res.status === 201 }));
}

export function escrowScenario() {
  if (!AUTH_TOKEN) return;
  const r = http.post(
    `${API_GW_URL}/api/v1/payments/escrows`,
    JSON.stringify({ amount: AMOUNT }),
    { headers: headers(), jar: cookieJar(), tags: { endpoint: "escrow-request" } },
  );
  escrowLatency.add(r.timings.duration);
  okRate.add(check(r, { "escrow 201": (res) => res.status === 201 }));
}
