/**
 * scenario-a-perf.js — k6 performance baseline for Scenario A (R1-12.3).
 *
 * Latency gates (from docs/test-execution-plan.md "Acceptance Thresholds"):
 *   - GET  /api/v1/htlc/search                p95 <= 500ms  (read operations)
 *   - GET  /api/v1/payments/fx/agreements     p95 <= 500ms  (read operations)
 *   - POST /api/v1/htlc/lock                  p95 <= 1500ms (write — admission only, not finality)
 *   - POST /api/v1/payments/fx/agreements     p95 <= 1500ms (write — admission)
 *
 * Two load models (LOAD_MODEL env var):
 *   - "vus"  (default) — constant-VU latency baseline; measures p50/p95 under moderate concurrency
 *   - "rate"           — constant-arrival-rate probe; validates TPS targets with strict thresholds
 *
 * Usage (latency baseline):
 *   API_GW_URL=http://localhost:3001 AUTH_TOKEN=<jwt> \
 *   RECEIVER=funded_operator@spoke-a-bank-c \
 *   k6 run tests/performance/scenario-a-perf.js
 *
 * Usage (throughput probe, e.g. 20 TPS writes):
 *   API_GW_URL=http://localhost:3001 AUTH_TOKEN=<jwt> \
 *   LOAD_MODEL=rate LOCK_TPS=20 READ_TPS=40 DURATION=10m \
 *   RECEIVER=funded_operator@spoke-a-bank-c \
 *   k6 run tests/performance/scenario-a-perf.js
 *
 * Environment variables:
 *   API_GW_URL        — bank-a API Gateway base URL (default: http://localhost:3001)
 *   AUTH_TOKEN        — access_token JWT value (REQUIRED)
 *   RECEIVER          — Paladin identity of the HTLC receiver (for write scenarios)
 *   COUNTERPARTY_B    — Paladin identity for FX agreement counterparty (for FX write scenario)
 *   DURATION          — test duration (default: 5m)
 *   LOAD_MODEL        — "vus" | "rate" (default: vus)
 *   VUS_READ          — read VUs in vus-mode (default: 20)
 *   VUS_WRITE         — write VUs in vus-mode (default: 5)
 *   READ_TPS          — read target req/s in rate-mode (default: 40)
 *   LOCK_TPS          — HTLC lock target req/s in rate-mode (default: 20)
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Trend } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3001";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const RECEIVER = __ENV.RECEIVER || "funded_operator@spoke-a-bank-c";
const COUNTERPARTY_B = __ENV.COUNTERPARTY_B || "funded_operator@spoke-b-bank-d";
const DURATION = __ENV.DURATION || "5m";
const LOAD_MODEL = (__ENV.LOAD_MODEL || "vus").toLowerCase();

const READ_TPS = Number(__ENV.READ_TPS || 40);
const LOCK_TPS = Number(__ENV.LOCK_TPS || 20);

const htlcSearchLatency = new Trend("htlc_search_latency_ms", true);
const fxListLatency = new Trend("fx_list_latency_ms", true);
const htlcLockLatency = new Trend("htlc_lock_latency_ms", true);

function vusScenarios() {
  return {
    htlcRead: {
      executor: "constant-vus",
      exec: "htlcSearchScenario",
      vus: Number(__ENV.VUS_READ || 20),
      duration: DURATION,
    },
    fxRead: {
      executor: "constant-vus",
      exec: "fxListScenario",
      vus: Math.max(5, Number(__ENV.VUS_READ || 20) / 4),
      duration: DURATION,
      startTime: "1s",
    },
    htlcWrite: {
      executor: "constant-vus",
      exec: "htlcLockScenario",
      vus: Number(__ENV.VUS_WRITE || 5),
      duration: DURATION,
      startTime: "2s",
    },
  };
}

function rateScenarios() {
  return {
    htlcRead: {
      executor: "constant-arrival-rate",
      exec: "htlcSearchScenario",
      rate: READ_TPS,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(20, READ_TPS),
      maxVUs: Math.max(60, READ_TPS * 3),
    },
    htlcWrite: {
      executor: "constant-arrival-rate",
      exec: "htlcLockScenario",
      rate: LOCK_TPS,
      timeUnit: "1s",
      duration: DURATION,
      startTime: "1s",
      preAllocatedVUs: Math.max(10, LOCK_TPS * 2),
      maxVUs: Math.max(40, LOCK_TPS * 6),
    },
  };
}

export const options = {
  scenarios: LOAD_MODEL === "rate" ? rateScenarios() : vusScenarios(),
  thresholds: {
    // D6 read gates: p50 < 200ms, p95 < 500ms.
    htlc_search_latency_ms: ["p(50)<200", "p(95)<500"],
    fx_list_latency_ms: ["p(50)<200", "p(95)<500"],
    // D6 write (admission) gates: p50 < 800ms, p95 < 1500ms.
    htlc_lock_latency_ms: ["p(50)<800", "p(95)<1500"],
    http_req_failed: ["rate<0.01"],
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

export function htlcSearchScenario() {
  const r = http.get(`${API_GW_URL}/api/v1/htlc/search?state=LOCKED`, {
    headers: headers(),
    jar: cookieJar(),
    tags: { endpoint: "htlc-search" },
  });
  htlcSearchLatency.add(r.timings.duration);
  check(r, { "htlc-search 200": (res) => res.status === 200 });
  if (LOAD_MODEL !== "rate") sleep(0.1);
}

export function fxListScenario() {
  const r = http.get(`${API_GW_URL}/api/v1/payments/fx/agreements`, {
    headers: headers(),
    jar: cookieJar(),
    tags: { endpoint: "fx-list" },
  });
  fxListLatency.add(r.timings.duration);
  check(r, { "fx-list 200": (res) => res.status === 200 });
  if (LOAD_MODEL !== "rate") sleep(0.5);
}

export function htlcLockScenario() {
  if (!AUTH_TOKEN) return;
  const timeLock = Math.floor(Date.now() / 1000) + 3600;
  const r = http.post(
    `${API_GW_URL}/api/v1/htlc/lock`,
    JSON.stringify({ receiver: RECEIVER, amount: "1000000", time_lock: timeLock }),
    { headers: headers(), jar: cookieJar(), tags: { endpoint: "htlc-lock" } },
  );
  htlcLockLatency.add(r.timings.duration);
  check(r, { "htlc-lock 201": (res) => res.status === 201 });
  if (LOAD_MODEL !== "rate") sleep(1);
}
