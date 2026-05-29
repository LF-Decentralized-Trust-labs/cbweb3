/**
 * scenario-b-perf.js — k6 performance baseline for Scenario B Liquidity Pool (T105 / FR-037).
 *
 * Gates (from SC-021/SC-022/SC-023):
 *   - GET  /api/v2/amm/quote/exact-output     p95 <= 300ms
 *   - POST /api/v2/amm/swap/exact-output      p95 <= 6000ms
 *   - GET  /api/v2/amm/pool/{pair}/status     p95 <= 15000ms (liquidity-monitor cadence)
 *
 * Usage:
 *   API_GW_URL=http://localhost:3000 \
 *   AUTH_TOKEN=<jwt>                  \
 *   k6 run tests/performance/scenario-b-perf.js
 *
 * Environment variables:
 *   API_GW_URL   — API Gateway base URL (default: http://localhost:3000)
 *   AUTH_TOKEN   — Bearer token with commercial_bank role (required for swap)
 *   PAIR         — Pool pair to probe (default: BRL-USD)
 *   DURATION     — Test duration (default: 1m)
 *   VUS          — Virtual users  (default: 20)
 *
 * A delta >20% above the gates MUST block merge per Decision 13.
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Trend } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3000";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const PAIR = __ENV.PAIR || "BRL-USD";

const quoteLatency  = new Trend("quote_latency_ms", true);
const swapLatency   = new Trend("swap_latency_ms", true);
const poolLatency   = new Trend("pool_latency_ms", true);

export const options = {
  scenarios: {
    quote: {
      executor: "constant-vus",
      exec: "quoteScenario",
      vus: Number(__ENV.VUS || 20),
      duration: __ENV.DURATION || "1m",
    },
    swap: {
      executor: "constant-vus",
      exec: "swapScenario",
      vus: Number(__ENV.VUS_SWAP || 5),
      duration: __ENV.DURATION || "1m",
      startTime: "1s",
    },
    pool: {
      executor: "constant-vus",
      exec: "poolScenario",
      vus: Number(__ENV.VUS_POOL || 5),
      duration: __ENV.DURATION || "1m",
      startTime: "2s",
    },
  },
  thresholds: {
    // Gate enforcement: abort test if p95 exceeds targets.
    "quote_latency_ms":  ["p(95)<300"],
    "swap_latency_ms":   ["p(95)<6000"],
    "pool_latency_ms":   ["p(95)<15000"],
    "http_req_failed":   ["rate<0.01"], // <1% error rate
  },
};

function headers(authRequired = false) {
  const h = { "Content-Type": "application/json", "Accept": "application/json" };
  if (authRequired && AUTH_TOKEN) h["Authorization"] = `Bearer ${AUTH_TOKEN}`;
  return h;
}

export function quoteScenario() {
  const r = http.get(
    `${API_GW_URL}/api/v2/amm/quote/exact-output?pair=${PAIR}&amount_out=1000`,
    { headers: headers(false), tags: { endpoint: "quote" } },
  );
  quoteLatency.add(r.timings.duration);
  check(r, { "quote 200": (res) => res.status === 200 });
  sleep(0.2);
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
  check(r, { "swap 2xx": (res) => res.status >= 200 && res.status < 300 });
  sleep(1);
}

export function poolScenario() {
  const r = http.get(
    `${API_GW_URL}/api/v2/amm/pool/${PAIR}/status`,
    { headers: headers(false), tags: { endpoint: "pool" } },
  );
  poolLatency.add(r.timings.duration);
  check(r, { "pool 200": (res) => res.status === 200 });
  sleep(2);
}
