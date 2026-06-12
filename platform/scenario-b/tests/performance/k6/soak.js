/**
 * soak.js — 12-hour soak harness for Scenario B (R1-12.3).
 *
 * Drives a MODERATE, continuous mixed load (AMM quote/swap + bridge transfer) over a long
 * window to surface memory leaks, connection-pool exhaustion, and AMM pool-state drift.
 * This is the load generator; the leak/drift evidence comes from host/container metrics
 * (RSS, goroutines, DB connections) and the AMM constant-product invariant check — see
 * docs/performance/README.md ("12-hour soak").
 *
 * Thresholds during soak:
 *   - <1% error rate sustained for the full window
 *   - p95 latencies stay within the standard gates (no degradation over time)
 *   - No memory growth trend / node crash (observed out-of-band via metrics)
 *
 * Usage:
 *   API_GW_URL=http://localhost:3000 AUTH_TOKEN=<jwt> \
 *   DURATION=12h \
 *   k6 run tests/performance/k6/soak.js
 *
 * Environment variables:
 *   API_GW_URL    — API Gateway base URL (default: http://localhost:3000)
 *   AUTH_TOKEN    — Bearer token with commercial_bank role (REQUIRED for swap/transfer)
 *   PAIR          — AMM pair to probe (default: BRL-USD)
 *   SPOKE/ASSET   — bridge transfer source/asset (defaults: spoke-a / BRL)
 *   DURATION      — soak window (default: 12h)
 *   QUOTE_TPS     — quote req/s (default: 10 — moderate)
 *   SWAP_TPS      — swap req/s  (default: 3  — moderate)
 *   TRANSFER_TPS  — transfer req/s (default: 5 — moderate)
 *
 * IMPORTANT: This is a 12-hour run. Do NOT execute as part of CI or a local smoke test.
 * Run it on dedicated devnet infra with metrics scraping enabled.
 */

import http from "k6/http";
import { check } from "k6";
import { Trend, Rate } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3000";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const PAIR = __ENV.PAIR || "BRL-USD";
const SPOKE = __ENV.SPOKE || "spoke-a";
const ASSET = __ENV.ASSET || "BRL";
const DURATION = __ENV.DURATION || "12h";

const QUOTE_TPS = Number(__ENV.QUOTE_TPS || 10);
const SWAP_TPS = Number(__ENV.SWAP_TPS || 3);
const TRANSFER_TPS = Number(__ENV.TRANSFER_TPS || 5);

const quoteLatency = new Trend("quote_latency_ms", true);
const swapLatency = new Trend("swap_latency_ms", true);
const transferLatency = new Trend("transfer_admit_latency_ms", true);
const okRate = new Rate("soak_ok_rate");

function arrival(exec, rate) {
  return {
    executor: "constant-arrival-rate",
    exec,
    rate,
    timeUnit: "1s",
    duration: DURATION,
    preAllocatedVUs: Math.max(10, rate * 3),
    maxVUs: Math.max(40, rate * 8),
  };
}

export const options = {
  scenarios: {
    quote: arrival("quoteScenario", QUOTE_TPS),
    swap: arrival("swapScenario", SWAP_TPS),
    transfer: arrival("transferScenario", TRANSFER_TPS),
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    soak_ok_rate: ["rate>0.99"],
    quote_latency_ms: ["p(95)<300"],
    swap_latency_ms: ["p(95)<6000"],
    transfer_admit_latency_ms: ["p(95)<2000"],
  },
};

function headers(authRequired) {
  const h = { "Content-Type": "application/json", Accept: "application/json" };
  if (authRequired && AUTH_TOKEN) h["Authorization"] = `Bearer ${AUTH_TOKEN}`;
  return h;
}

export function quoteScenario() {
  const r = http.get(
    `${API_GW_URL}/api/v2/amm/quote/exact-output?pair=${PAIR}&amount_out=1000`,
    { headers: headers(false), tags: { endpoint: "quote" } },
  );
  quoteLatency.add(r.timings.duration);
  okRate.add(check(r, { "quote 200": (res) => res.status === 200 }));
}

export function swapScenario() {
  if (!AUTH_TOKEN) return;
  const body = JSON.stringify({
    pair: PAIR,
    amount_out: 1000,
    max_amount_in: "999999999",
    recipient: "0xPerfRecipient",
  });
  const r = http.post(`${API_GW_URL}/api/v2/amm/swap/exact-output`, body, {
    headers: headers(true),
    tags: { endpoint: "swap" },
  });
  swapLatency.add(r.timings.duration);
  okRate.add(check(r, { "swap 2xx": (res) => res.status >= 200 && res.status < 300 }));
}

export function transferScenario() {
  if (!AUTH_TOKEN) return;
  const idem = `soak-${__VU}-${__ITER}-${Date.now()}`;
  const body = JSON.stringify({
    spoke: SPOKE,
    asset: ASSET,
    amount: 1000,
    token_kind: "noto",
    recipient: "0xPerfRecipient",
    idempotency_key: idem,
  });
  const r = http.post(`${API_GW_URL}/api/v2/bridge/lock-mint`, body, {
    headers: headers(true),
    tags: { endpoint: "lock-mint" },
  });
  transferLatency.add(r.timings.duration);
  okRate.add(check(r, { "lock-mint 202": (res) => res.status === 202 }));
}
