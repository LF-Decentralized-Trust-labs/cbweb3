/**
 * zeto-escrow-throughput.js — k6 Zeto privacy-token throughput harness for Scenario A (R1-12.3).
 *
 * Measures sustained throughput + admission latency for the Zeto shielded-transfer path:
 *   - 15 TPS  Zeto privacy transactions (ZETO_TPS, default 15)
 *   - <1% error rate
 *   - Write p95 < 1500ms  (API admission)
 *
 * In Scenario A the Zeto privacy operation is the tokenization escrow:
 *   POST /api/v1/payments/escrows  — fCeBM (ERC-20) → tCeBM (Zeto/Paladin ZKP token)
 * The Central Bank burns fCeBM and mints tCeBM via Paladin. This is the operation that
 * exercises the Paladin ZKP proof generation path, making it the correct proxy for the
 * "15 TPS Zeto shielded transaction" threshold.
 *
 * IMPORTANT: Each escrow request is submitted as PENDING and requires a separate CB approval
 * step to complete. This script only drives the REQUEST side (the Zeto-heavy step is the
 * approval/mint, but the REQUEST already invokes Paladin for balance validation). The TPS
 * here represents the rate of escrow initiations entering the Paladin pipeline.
 *
 * Auth note: Scenario A uses cookie-based auth. Pass the JWT in AUTH_TOKEN.
 *
 * Usage (15 TPS Zeto):
 *   API_GW_URL=http://localhost:3001 AUTH_TOKEN=<jwt> \
 *   REQUESTER_PALADIN_ID=funded_operator@spoke-a-bank-a ZETO_TPS=15 DURATION=10m \
 *   k6 run tests/performance/k6/zeto-escrow-throughput.js
 *
 * Environment variables:
 *   API_GW_URL               — bank-a API Gateway base URL (default: http://localhost:3001)
 *   AUTH_TOKEN               — access_token JWT value (REQUIRED)
 *   AMOUNT                   — fCeBM amount to escrow (default: 5000000)
 *   REQUESTER_PALADIN_ID     — Paladin identity of the requester (optional; derived from auth)
 *   ZETO_TPS                 — target escrow requests/sec (default: 15)
 *   DURATION                 — test duration (default: 10m)
 */

import http from "k6/http";
import { check } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3001";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const AMOUNT = __ENV.AMOUNT || "5000000";
const REQUESTER_PALADIN_ID = __ENV.REQUESTER_PALADIN_ID || "";
const ZETO_TPS = Number(__ENV.ZETO_TPS || 15);
const DURATION = __ENV.DURATION || "10m";

const admitLatency = new Trend("zeto_escrow_admit_latency_ms", true);
const submitted = new Counter("zeto_escrow_submitted_total");
const submitRate = new Rate("zeto_escrow_submit_rate");

export const options = {
  scenarios: {
    zetoEscrow: {
      executor: "constant-arrival-rate",
      exec: "escrowScenario",
      rate: ZETO_TPS,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(10, ZETO_TPS * 2),
      maxVUs: Math.max(50, ZETO_TPS * 6),
    },
  },
  thresholds: {
    // <1% error rate (R1-12.3).
    http_req_failed: ["rate<0.01"],
    zeto_escrow_submit_rate: ["rate>0.99"],
    // Write admission latency gate (p95 < 1500ms — test-execution-plan.md).
    zeto_escrow_admit_latency_ms: ["p(95)<1500"],
  },
};

function cookieJar() {
  const jar = http.cookieJar();
  if (AUTH_TOKEN) jar.set(API_GW_URL, "access_token", AUTH_TOKEN);
  return jar;
}

export function escrowScenario() {
  const payload = { amount: AMOUNT };
  if (REQUESTER_PALADIN_ID) payload.requester_paladin_identity = REQUESTER_PALADIN_ID;

  const r = http.post(
    `${API_GW_URL}/api/v1/payments/escrows`,
    JSON.stringify(payload),
    {
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      jar: cookieJar(),
      tags: { endpoint: "escrow-request" },
    },
  );
  admitLatency.add(r.timings.duration);
  const ok = check(r, { "escrow 201": (res) => res.status === 201 });
  submitRate.add(ok);
  if (ok) submitted.add(1);
}
