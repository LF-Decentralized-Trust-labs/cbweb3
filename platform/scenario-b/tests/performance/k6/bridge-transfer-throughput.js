/**
 * bridge-transfer-throughput.js — k6 throughput harness for Scenario B value transfers (R1-12.3).
 *
 * Measures sustained throughput + latency for the cross-network transfer path that backs the
 * Report 1 Deliverable 12 / Finding 12.3 thresholds:
 *
 *   - 50 TPS  general value transfers   (TRANSFER_TPS, default 50)
 *   - 15 TPS  privacy-token (Zeto) transfers  (ZETO_TPS, default 15)
 *   - <1% error rate
 *   - TTF (time-to-finality) < 5s  — see note below
 *
 * In Scenario B a cross-network value transfer is initiated through the bridge endpoints:
 *   - POST /api/v2/bridge/lock-mint    (lock on Spoke -> mint mirrored asset on Hub)
 *   - POST /api/v2/bridge/burn-unlock  (burn on Hub -> unlock native on Spoke)
 * Mirrored-asset privacy is provided by Zeto/Noto, so the lock-mint path is also the
 * harness for the 15 TPS Zeto threshold when TOKEN_KIND=zeto.
 *
 * TTF NOTE: k6 measures *API admission* latency only (the endpoints return 202 ACCEPTED and
 * the relayer finalises asynchronously). End-to-end Time-To-Finality (<5s) is NOT something a
 * black-box HTTP probe can observe. To measure TTF, correlate the returned position id with
 * the on-chain `Locked`/`Minted` (or `Burned`/`Released`) events via the relayer logs /
 * indexer. See docs/performance/README.md ("Measuring Time-To-Finality"). This script emits
 * the position ids it created to stdout so a TTF post-processor can consume them.
 *
 * Usage (50 TPS transfer throughput):
 *   API_GW_URL=http://localhost:3000 AUTH_TOKEN=<jwt> \
 *   TRANSFER_TPS=50 DURATION=10m \
 *   k6 run tests/performance/k6/bridge-transfer-throughput.js
 *
 * Usage (15 TPS Zeto/privacy throughput):
 *   API_GW_URL=http://localhost:3000 AUTH_TOKEN=<jwt> \
 *   TOKEN_KIND=zeto TRANSFER_TPS=15 DURATION=10m \
 *   k6 run tests/performance/k6/bridge-transfer-throughput.js
 *
 * Environment variables:
 *   API_GW_URL    — API Gateway base URL (default: http://localhost:3000)
 *   AUTH_TOKEN    — Bearer token with commercial_bank role (REQUIRED)
 *   TOKEN_KIND    — "noto" | "zeto" (default: noto) — selects privacy domain for the mirrored asset
 *   TRANSFER_TPS  — target transfers/sec (default: 50; use 15 for the Zeto threshold)
 *   DURATION      — test duration (default: 5m; use 12h for the soak)
 *   SPOKE         — source spoke network id (default: spoke-a)
 *   ASSET         — native asset symbol (default: BRL)
 *   AMOUNT        — per-transfer amount (default: 1000)
 *   PRINT_IDS     — "1" to print created position ids for TTF post-processing (default: 0)
 */

import http from "k6/http";
import { check } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const API_GW_URL = __ENV.API_GW_URL || "http://localhost:3000";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";
const TOKEN_KIND = (__ENV.TOKEN_KIND || "noto").toLowerCase();
const TRANSFER_TPS = Number(__ENV.TRANSFER_TPS || 50);
const DURATION = __ENV.DURATION || "5m";
const SPOKE = __ENV.SPOKE || "spoke-a";
const ASSET = __ENV.ASSET || "BRL";
const AMOUNT = Number(__ENV.AMOUNT || 1000);
const PRINT_IDS = __ENV.PRINT_IDS === "1";

const admitLatency = new Trend("transfer_admit_latency_ms", true);
const accepted = new Counter("transfer_accepted_total");
const acceptRate = new Rate("transfer_accept_rate");

export const options = {
  scenarios: {
    transfer: {
      executor: "constant-arrival-rate",
      exec: "transferScenario",
      rate: TRANSFER_TPS,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(20, TRANSFER_TPS * 2),
      maxVUs: Math.max(80, TRANSFER_TPS * 6),
    },
  },
  thresholds: {
    // <1% error rate (R1-12.3). The API admits transfers with 202; anything else is an error.
    http_req_failed: ["rate<0.01"],
    transfer_accept_rate: ["rate>0.99"],
    // API admission latency budget — NOT end-to-end TTF (see header note).
    transfer_admit_latency_ms: ["p(95)<2000"],
  },
};

function headers() {
  const h = { "Content-Type": "application/json", Accept: "application/json" };
  if (AUTH_TOKEN) h["Authorization"] = `Bearer ${AUTH_TOKEN}`;
  return h;
}

export function transferScenario() {
  const idem = `perf-${__VU}-${__ITER}-${Date.now()}`;
  const body = JSON.stringify({
    spoke: SPOKE,
    asset: ASSET,
    amount: AMOUNT,
    token_kind: TOKEN_KIND, // noto | zeto — privacy domain for mirrored asset
    recipient: "0xPerfRecipient",
    idempotency_key: idem,
  });
  const r = http.post(`${API_GW_URL}/api/v2/bridge/lock-mint`, body, {
    headers: headers(),
    tags: { endpoint: "lock-mint", token_kind: TOKEN_KIND },
  });
  admitLatency.add(r.timings.duration);
  const ok = check(r, { "lock-mint 202": (res) => res.status === 202 });
  acceptRate.add(ok);
  if (ok) {
    accepted.add(1);
    if (PRINT_IDS) {
      try {
        const id = r.json("id") || r.json("positionId");
        if (id) console.log(`POSITION_ID ${id}`);
      } catch (_) {
        /* body not JSON — ignore for id capture */
      }
    }
  }
}
