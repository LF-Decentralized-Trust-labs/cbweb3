/**
 * htlc-transfer-throughput.js — k6 throughput harness for Scenario A HTLC transfers (R1-12.3).
 *
 * Measures sustained throughput + admission latency for the cross-spoke HTLC lock path that
 * backs the Report 1 Deliverable 12 / Finding 12.3 thresholds:
 *
 *   - 50 TPS  general token transfers  (TRANSFER_TPS, default 50)
 *   - <1% error rate
 *   - Write p95 < 1500ms  (API admission, NOT on-chain finality)
 *
 * In Scenario A a cross-spoke transfer is initiated by locking tokens via HTLC:
 *   - POST /api/v1/htlc/lock    (initiator locks on Spoke-A; Cacti relay propagates)
 *
 * The Zeto threshold (15 TPS) is benchmarked separately via zeto-escrow-throughput.js,
 * which exercises the fCeBM→tCeBM escrow path (the Zeto/Paladin privacy operation).
 *
 * TTF NOTE: this script measures *API admission* latency only. The HTLC lock endpoint
 * returns 201 once the transaction is submitted; on-chain finality (QBFT, ~1 block ≈ 2s)
 * and cross-spoke relay propagation happen asynchronously. To measure end-to-end TTF (<5s
 * per spoke), correlate the returned contract_id with the on-chain HTLCLocked event via
 * the Cacti relay logs. Set PRINT_IDS=1 to emit contract_ids to stdout for a TTF
 * post-processor. See docs/performance/README.md §4.
 *
 * Auth note: Scenario A uses cookie-based auth (access_token cookie from /api/v1/auth/login).
 * Pass the raw JWT value in AUTH_TOKEN; this script sets it as the access_token cookie.
 *
 * Usage (50 TPS):
 *   API_GW_URL=http://localhost:3001 AUTH_TOKEN=<jwt> \
 *   RECEIVER=funded_operator@spoke-a-bank-c TRANSFER_TPS=50 DURATION=10m \
 *   k6 run tests/performance/k6/htlc-transfer-throughput.js
 *
 * Environment variables:
 *   API_GW_URL      — bank-a API Gateway base URL (default: http://localhost:3001)
 *   AUTH_TOKEN      — access_token JWT value (REQUIRED)
 *   RECEIVER        — Paladin identity of the HTLC receiver (REQUIRED for real runs)
 *   AMOUNT          — token amount to lock as decimal string (default: 1000000)
 *   TRANSFER_TPS    — target locks/sec (default: 50)
 *   DURATION        — test duration (default: 10m; use 12h for the soak)
 *   PRINT_IDS       — "1" to print contract_ids for TTF post-processing (default: 0)
 */

import http from "k6/http";
import { check } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3001";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const RECEIVER = __ENV.RECEIVER || "funded_operator@spoke-a-bank-c";
const AMOUNT = __ENV.AMOUNT || "1000000";
const TRANSFER_TPS = Number(__ENV.TRANSFER_TPS || 50);
const DURATION = __ENV.DURATION || "10m";
const PRINT_IDS = __ENV.PRINT_IDS === "1";

const admitLatency = new Trend("htlc_admit_latency_ms", true);
const locked = new Counter("htlc_locked_total");
const lockRate = new Rate("htlc_lock_rate");

export const options = {
  scenarios: {
    htlcLock: {
      executor: "constant-arrival-rate",
      exec: "lockScenario",
      rate: TRANSFER_TPS,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(20, TRANSFER_TPS * 2),
      maxVUs: Math.max(80, TRANSFER_TPS * 6),
    },
  },
  thresholds: {
    // <1% error rate (R1-12.3).
    http_req_failed: ["rate<0.01"],
    htlc_lock_rate: ["rate>0.99"],
    // API admission write latency gate.
    htlc_admit_latency_ms: ["p(95)<1500"],
  },
};

function cookieJar() {
  const jar = http.cookieJar();
  if (AUTH_TOKEN) jar.set(API_GW_URL, "access_token", AUTH_TOKEN);
  return jar;
}

export function lockScenario() {
  // time_lock: 1 hour from now, as Unix timestamp (seconds)
  const timeLock = Math.floor(Date.now() / 1000) + 3600;
  const body = JSON.stringify({
    receiver: RECEIVER,
    amount: AMOUNT,
    time_lock: timeLock,
  });
  // t0 = client send time (epoch ms), captured for TTF post-processing (§4).
  const t0 = Date.now();
  const r = http.post(`${API_GW_URL}/api/v1/htlc/lock`, body, {
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    jar: cookieJar(),
    tags: { endpoint: "htlc-lock" },
  });
  admitLatency.add(r.timings.duration);
  const ok = check(r, { "htlc-lock 201": (res) => res.status === 201 });
  lockRate.add(ok);
  if (ok) {
    locked.add(1);
    if (PRINT_IDS) {
      try {
        const id = r.json("contract_id");
        // "CONTRACT_ID <id> <t0_epoch_ms>" — the trailing t0 is optional and
        // backward-compatible with parsers that read only the id. The TTF
        // post-processor (lib/ttf.sh) uses t0 as the finality-clock start.
        if (id) console.log(`CONTRACT_ID ${id} ${t0}`);
      } catch (_) {
        /* body not JSON — ignore for id capture */
      }
    }
  }
}
