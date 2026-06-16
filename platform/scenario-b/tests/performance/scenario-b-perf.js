/**
 * scenario-b-perf.js — k6 performance baseline for Scenario B Liquidity Pool (T105 / FR-037).
 *
 * Latency gates (from SC-021/SC-022/SC-023):
 *   - GET  /api/v2/amm/quote/exact-output     p95 <= 300ms
 *   - POST /api/v2/amm/swap/exact-output      p95 <= 6000ms
 *   - GET  /api/v2/amm/pool/{pair}/status     p95 <= 15000ms (liquidity-monitor cadence)
 *
 * This script supports two load models, selected via LOAD_MODEL:
 *   - "vus"  (default) — constant-VU latency probe (original baseline behaviour).
 *   - "rate"           — constant-arrival-rate throughput probe used to validate the
 *                        AMM 30 TPS DRAFT target (R1-12.3). Set SWAP_TPS / QUOTE_TPS.
 *
 * Usage (latency baseline):
 *   API_GW_URL=http://localhost:3000 AUTH_TOKEN=<jwt> \
 *   k6 run tests/performance/scenario-b-perf.js
 *
 * Usage (AMM 30 TPS throughput validation):
 *   API_GW_URL=http://localhost:3000 AUTH_TOKEN=<jwt> \
 *   LOAD_MODEL=rate SWAP_TPS=30 QUOTE_TPS=60 DURATION=10m \
 *   k6 run tests/performance/scenario-b-perf.js
 *
 * Environment variables:
 *   API_GW_URL   — API Gateway base URL (default: http://localhost:3000)
 *   AUTH_TOKEN   — Bearer token with commercial_bank role (required for swap)
 *   PAIR         — Pool pair to probe (default: W-BRL-ARS)
 *   DURATION     — Test duration (default: 1m; use 12h for the soak)
 *   LOAD_MODEL   — "vus" | "rate" (default: vus)
 *   VUS          — quote VUs in vus-mode (default: 20)
 *   VUS_SWAP     — swap VUs in vus-mode (default: 5)
 *   VUS_POOL     — pool VUs in vus-mode (default: 5)
 *   QUOTE_TPS    — quote target req/s in rate-mode (default: 60)
 *   SWAP_TPS     — swap target req/s in rate-mode  (default: 30 — the DRAFT gate)
 *   PAYER_ID     — swap payer bank id     (default: bank-a)
 *   BENEFICIARY_ID — swap beneficiary bank id (default: bank-b)
 *   AMOUNT_OUT   — exact-output target per swap (default: 1000)
 *   MAX_AMOUNT_IN— max input cap per swap (default: 999999999)
 *
 * API CONTRACT (verified against backend/.../handlers/swap_handler.go):
 *   POST /api/v2/amm/swap/exact-output requires {pair, amount_out, max_amount_in,
 *   payer_id, beneficiary_id}. There is NO `recipient` field — the earlier
 *   "recipient: 0xPerfRecipient" body 400'd on every request.
 *
 * A delta >20% above the gates MUST block merge per Decision 13.
 * NOTE: measured numbers belong in docs/performance/RESULTS-TEMPLATE.md after a real run.
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Counter } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3000";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const PAIR = __ENV.PAIR || "W-BRL-ARS";
const DURATION = __ENV.DURATION || "1m";
const LOAD_MODEL = (__ENV.LOAD_MODEL || "vus").toLowerCase();

const QUOTE_TPS = Number(__ENV.QUOTE_TPS || 60);
const SWAP_TPS = Number(__ENV.SWAP_TPS || 30); // DRAFT AMM throughput target (R1-12.3)
const PAYER_ID = __ENV.PAYER_ID || "bank-a";
const BENEFICIARY_ID = __ENV.BENEFICIARY_ID || "bank-b";
const AMOUNT_OUT = __ENV.AMOUNT_OUT || "1000";
const MAX_AMOUNT_IN = __ENV.MAX_AMOUNT_IN || "999999999";

const quoteLatency = new Trend("quote_latency_ms", true);
const swapLatency = new Trend("swap_latency_ms", true);
const poolLatency = new Trend("pool_latency_ms", true);
const swapOk = new Counter("swap_success_total");

function vusScenarios() {
  return {
    quote: {
      executor: "constant-vus",
      exec: "quoteScenario",
      vus: Number(__ENV.VUS || 20),
      duration: DURATION,
    },
    swap: {
      executor: "constant-vus",
      exec: "swapScenario",
      vus: Number(__ENV.VUS_SWAP || 5),
      duration: DURATION,
      startTime: "1s",
    },
    pool: {
      executor: "constant-vus",
      exec: "poolScenario",
      vus: Number(__ENV.VUS_POOL || 5),
      duration: DURATION,
      startTime: "2s",
    },
  };
}

function rateScenarios() {
  return {
    quote: {
      executor: "constant-arrival-rate",
      exec: "quoteScenario",
      rate: QUOTE_TPS,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(20, QUOTE_TPS),
      maxVUs: Math.max(50, QUOTE_TPS * 3),
    },
    swap: {
      executor: "constant-arrival-rate",
      exec: "swapScenario",
      rate: SWAP_TPS,
      timeUnit: "1s",
      duration: DURATION,
      startTime: "1s",
      preAllocatedVUs: Math.max(20, SWAP_TPS * 2),
      maxVUs: Math.max(60, SWAP_TPS * 6),
    },
  };
}

export const options = {
  scenarios: LOAD_MODEL === "rate" ? rateScenarios() : vusScenarios(),
  thresholds: {
    // Gate enforcement: abort test if p95 exceeds targets.
    quote_latency_ms: ["p(95)<300"],
    swap_latency_ms: ["p(95)<6000"],
    pool_latency_ms: ["p(95)<15000"],
    http_req_failed: ["rate<0.01"], // <1% error rate
  },
};

function headers(authRequired = false) {
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
  check(r, { "quote 200": (res) => res.status === 200 });
  if (LOAD_MODEL !== "rate") sleep(0.2);
}

export function swapScenario() {
  if (!AUTH_TOKEN) return;
  const body = JSON.stringify({
    pair: PAIR,
    amount_out: AMOUNT_OUT,
    max_amount_in: MAX_AMOUNT_IN,
    payer_id: PAYER_ID,
    beneficiary_id: BENEFICIARY_ID,
  });
  const r = http.post(`${API_GW_URL}/api/v2/amm/swap/exact-output`, body, {
    headers: headers(true),
    tags: { endpoint: "swap" },
  });
  swapLatency.add(r.timings.duration);
  const ok = check(r, { "swap 2xx": (res) => res.status >= 200 && res.status < 300 });
  if (ok) swapOk.add(1);
  if (LOAD_MODEL !== "rate") sleep(1);
}

export function poolScenario() {
  const r = http.get(`${API_GW_URL}/api/v2/amm/pool/${PAIR}/status`, {
    headers: headers(false),
    tags: { endpoint: "pool" },
  });
  poolLatency.add(r.timings.duration);
  check(r, { "pool 200": (res) => res.status === 200 });
  if (LOAD_MODEL !== "rate") sleep(2);
}
