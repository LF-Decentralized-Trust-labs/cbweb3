/**
 * fx-settlement-throughput.js — k6 end-to-end happy-path benchmark for Scenario A.
 *
 * This is the Scenario A analogue of Scenario B's full cross-currency measurement
 * (scenario-b-perf.js SWAP_MODE=xc): it drives the COMPLETE correspondent-banking
 * happy path per iteration and measures the D6 HTLC settlement timing constraints.
 * Unlike Scenario B — where one endpoint (/swap/cross-currency) performs the whole
 * flow server-side — Scenario A's happy path is a multi-step cross-spoke dance
 * across two banks, so each iteration orchestrates the same sequence the integration
 * test exercises (tests/integration/livehappy_path_test.go):
 *
 *   1. bank-a (originator) proposes the FX agreement                 [spoke-a]
 *   2. relay mirrors it; bank-d (custodian) accepts                  [spoke-b]
 *   3. acceptance mirrors back to bank-a (poll until ACCEPTED)
 *   4. bank-a locks the origin leg → returns secret + hashlock       [spoke-a]
 *   5. bank-d locks the counter leg under the SAME hashlock          [spoke-b]
 *   6. bank-a reveals the secret (retry until the relay has confirmed
 *      the counterparty lock)                                        [spoke-a]
 *   7. relay carries the secret; poll bank-d until its leg is SETTLED [spoke-b]
 *
 * D6 timing constraints measured (docs: Scenario A HTLC settlement budgets):
 *   - fx_settlement_latency_ms  full HTLC lifecycle, end to end        ≤ 60s  (HARD GATE)
 *   - api_sync_ms               each state-changing API response        ≤ 30s  (HARD GATE)
 *   - fx_relay_propagation_ms   relayer cross-chain (secret carry)      ≤ 15s  (reported)
 * Per D6, state is polled every 1 second (no sleep-based fixed waits): each poll
 * checks the API status, which reflects the QBFT single-block-finality on-chain state.
 *
 * This path is RELAY-BOUND and single-signer per spoke, so it is measured at LOW
 * concurrency (HAPPY_VUS, default 2) to characterise the per-lifecycle SLA — raise
 * HAPPY_VUS for the burst/stress profiles. Component throughput (isolated HTLC lock
 * @ 50 TPS, Zeto escrow @ 15 TPS) and the latency baseline are the other scripts.
 *
 * Auth: Scenario A uses cookie-based auth. This script needs TWO tokens — the
 * originator (bank-a) and the custodian (bank-d) — passed via env, minted once by
 * run-all.sh / lib/auth.sh. Both are sent as cookie + Bearer.
 *
 * PREREQ: the originator (bank-a) and custodian (bank-d) operators must hold tCeBM.
 * run-all.sh does this once via lib/provision.sh before this benchmark.
 *
 * Usage (standalone):
 *   API_GW_URL=http://localhost:18080 API_GW_BANK_D_URL=http://localhost:58080 \
 *   ORIGINATOR_TOKEN=<bank-a jwt> CUSTODIAN_TOKEN=<bank-d jwt> \
 *   HAPPY_VUS=2 DURATION=3m \
 *   k6 run tests/performance/k6/fx-settlement-throughput.js
 *
 * Environment variables:
 *   API_GW_URL          — bank-a (originator) gateway      (default http://localhost:18080)
 *   API_GW_BANK_D_URL   — bank-d (custodian) gateway        (default http://localhost:58080)
 *   ORIGINATOR_TOKEN    — bank-a access_token JWT (REQUIRED)
 *   CUSTODIAN_TOKEN     — bank-d access_token JWT (REQUIRED)
 *   ORIGIN_RECEIVER     — origin-leg receiver  (default funded_operator@spoke-a-bank-c)
 *   BENEFICIARY         — counter-leg receiver (default funded_operator@spoke-b-bank-b)
 *   ORIGIN_AMOUNT       — origin leg amount    (default 100000)
 *   COUNTER_AMOUNT      — counter leg amount   (default 520000)
 *   RATE                — FX rate              (default 5.2)
 *   HAPPY_VUS           — concurrent settlement flows (default 2)
 *   DURATION            — test duration        (default 3m)
 *   REQ_TIMEOUT         — per-request timeout  (default 120s — > the 60s lifecycle ceiling)
 *   SYNC_TIMEOUT_S      — cross-spoke sync poll budget seconds (default 60)
 *   SETTLE_TIMEOUT_S    — settle-retry / settled-poll budget seconds (default 90)
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const ORIGINATOR_GW = __ENV.API_GW_URL || "http://localhost:18080";
const CUSTODIAN_GW = __ENV.API_GW_BANK_D_URL || "http://localhost:58080";
const ORIGINATOR_TOKEN = __ENV.ORIGINATOR_TOKEN || __ENV.AUTH_TOKEN || "";
const CUSTODIAN_TOKEN = __ENV.CUSTODIAN_TOKEN || "";

const ORIGIN_RECEIVER = __ENV.ORIGIN_RECEIVER || "funded_operator@spoke-a-bank-c";
const BENEFICIARY = __ENV.BENEFICIARY || "funded_operator@spoke-b-bank-b";
const ORIGIN_AMOUNT = __ENV.ORIGIN_AMOUNT || "100000";
const COUNTER_AMOUNT = __ENV.COUNTER_AMOUNT || "520000";
const FX_RATE = __ENV.RATE || "5.2";

const HAPPY_VUS = Number(__ENV.HAPPY_VUS || 1);
const DURATION = __ENV.DURATION || "3m";
const REQ_TIMEOUT = __ENV.REQ_TIMEOUT || "120s";
const SYNC_TIMEOUT_S = Number(__ENV.SYNC_TIMEOUT_S || 60);
const SETTLE_TIMEOUT_S = Number(__ENV.SETTLE_TIMEOUT_S || 90);

// D6 timing constraints (milliseconds).
const D6_LIFECYCLE_MS = 60000; // full HTLC lifecycle, end to end
const D6_API_SYNC_MS = 30000; // each synchronous API response
const D6_RELAY_MS = 15000; // relayer cross-chain propagation (reported)

// End-to-end happy-path settlement latency (propose → both legs settled): the D6
// full-lifecycle measurement, gated at 60s.
const settlementLatency = new Trend("fx_settlement_latency_ms", true);
const settlementOk = new Counter("fx_settlement_success_total");
const settlementRate = new Rate("fx_settlement_rate");
// Per-phase decomposition.
const proposeLatency = new Trend("fx_propose_latency_ms", true);
const syncLatency = new Trend("fx_crossspoke_sync_latency_ms", true); // FX agreement relay mirror
const lockLatency = new Trend("htlc_lock_both_latency_ms", true);
const relayPropagation = new Trend("fx_relay_propagation_ms", true); // secret carry → custodian SETTLED (D6 ≤15s)
// api_sync_ms — duration of each STATE-CHANGING gateway call (propose/accept/lock/settle).
// D6 caps the synchronous API response at 30s.
const apiSync = new Trend("api_sync_ms", true);

export const options = {
  scenarios: {
    fxSettlement: {
      executor: "constant-vus",
      exec: "settlementScenario",
      vus: HAPPY_VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    // D6 hard gates for Scenario A:
    fx_settlement_latency_ms: [`p(95)<${D6_LIFECYCLE_MS}`], // full lifecycle ≤ 60s
    api_sync_ms: [`p(95)<${D6_API_SYNC_MS}`], // synchronous API response ≤ 30s
    // The happy path must actually complete (sanity; relay-bound flow).
    fx_settlement_rate: ["rate>0.9"],
    // NOTE: http_req_failed is intentionally NOT gated here — the settle step
    // deliberately retries on FailedPrecondition (relay-confirm), so expected 5xx
    // retries would pollute it. Completion + latency are the meaningful gates.
  },
};

function hdr(token) {
  return {
    "Content-Type": "application/json",
    Accept: "application/json",
    Authorization: `Bearer ${token}`,
    Cookie: `access_token=${token}`,
  };
}

// params for a state-changing call: long timeout (> the 60s lifecycle) so a slow
// on-chain write is measured, not severed at k6's 60s default.
function postOpts(token, tag) {
  return { headers: hdr(token), timeout: REQ_TIMEOUT, tags: { endpoint: tag } };
}

function isState(state, want) {
  return state === want || state === `FX_STATE_${want}` || state === `HTLC_STATE_${want}`;
}

function fxState(gw, token, tradeId) {
  const r = http.get(`${gw}/api/v1/payments/fx/agreements/${tradeId}`, {
    headers: hdr(token),
    timeout: REQ_TIMEOUT,
    tags: { endpoint: "fx-get" },
  });
  if (r.status !== 200) return "";
  try {
    return (r.json("agreement") && r.json("agreement").state) || "";
  } catch (_) {
    return "";
  }
}

// settlementScenario — one full happy-path settlement; emits the D6 timing metrics.
export function settlementScenario() {
  if (!ORIGINATOR_TOKEN || !CUSTODIAN_TOKEN) {
    check(null, { "tokens present": () => false });
    return;
  }
  const t0 = Date.now();
  const tradeId = `PERF-${Date.now()}-${__VU}-${__ITER}`;

  // 1. propose (originator / bank-a)
  const proposeBody = JSON.stringify({
    trade_id: tradeId,
    counterparty_b: "bank-d",
    originator: "bank-a",
    settlement_agent: "bank-a",
    custodian: "bank-d",
    beneficiary: "bank-b",
    origin_amount: ORIGIN_AMOUNT,
    counter_amount: COUNTER_AMOUNT,
    origin_currency: "USD",
    counter_currency: "BRL",
    rate: FX_RATE,
    expiry_date: Math.floor(Date.now() / 1000) + 3600,
    spoke_a_receiver: ORIGIN_RECEIVER,
    spoke_b_receiver: BENEFICIARY,
    on_behalf: false,
  });
  const pr = http.post(`${ORIGINATOR_GW}/api/v1/payments/fx/agreements`, proposeBody, postOpts(ORIGINATOR_TOKEN, "fx-propose"));
  proposeLatency.add(pr.timings.duration);
  apiSync.add(pr.timings.duration);
  if (!check(pr, { "fx propose 201": (res) => res.status === 201 })) {
    settlementRate.add(false);
    return;
  }

  // 2-3. cross-spoke sync: relay mirrors to custodian (bank-d) → accept → mirrors back.
  // Poll every 1s per D6 (no sleep-based fixed waits).
  const syncStart = Date.now();
  let visibleOnD = false;
  for (let i = 0; i < SYNC_TIMEOUT_S; i++) {
    const s = fxState(CUSTODIAN_GW, CUSTODIAN_TOKEN, tradeId);
    if (isState(s, "PROPOSED") || isState(s, "ACCEPTED")) {
      visibleOnD = true;
      break;
    }
    sleep(1);
  }
  if (!check(null, { "proposal mirrored to custodian": () => visibleOnD })) {
    settlementRate.add(false);
    return;
  }
  const ar = http.post(
    `${CUSTODIAN_GW}/api/v1/payments/fx/agreements/${tradeId}/accept`,
    JSON.stringify({ on_behalf: false }),
    postOpts(CUSTODIAN_TOKEN, "fx-accept"),
  );
  apiSync.add(ar.timings.duration);
  if (!check(ar, { "fx accept 200": (res) => res.status === 200 })) {
    settlementRate.add(false);
    return;
  }
  let acceptedOnA = false;
  for (let i = 0; i < SYNC_TIMEOUT_S; i++) {
    if (isState(fxState(ORIGINATOR_GW, ORIGINATOR_TOKEN, tradeId), "ACCEPTED")) {
      acceptedOnA = true;
      break;
    }
    sleep(1);
  }
  syncLatency.add(Date.now() - syncStart);
  if (!check(null, { "acceptance mirrored to originator": () => acceptedOnA })) {
    settlementRate.add(false);
    return;
  }

  // 4-5. dual-leg lock: originator (bank-a), then custodian (bank-d) under the same hashlock
  const lockStart = Date.now();
  const lockA = http.post(
    `${ORIGINATOR_GW}/api/v1/htlc/lock`,
    JSON.stringify({ agreement_id: tradeId, receiver: ORIGIN_RECEIVER, amount: ORIGIN_AMOUNT }),
    postOpts(ORIGINATOR_TOKEN, "htlc-lock"),
  );
  apiSync.add(lockA.timings.duration);
  if (!check(lockA, { "origin lock 201": (res) => res.status === 201 })) {
    settlementRate.add(false);
    return;
  }
  let contractIdA = "";
  let hashLock = "";
  let secret = "";
  try {
    contractIdA = lockA.json("contract_id") || "";
    hashLock = lockA.json("hash_lock") || "";
    secret = lockA.json("secret") || "";
  } catch (_) {
    /* non-JSON */
  }
  if (!check(null, { "lock response has secret + hashlock": () => hashLock && secret })) {
    settlementRate.add(false);
    return;
  }
  const lockB = http.post(
    `${CUSTODIAN_GW}/api/v1/htlc/lock-with-hash`,
    JSON.stringify({
      agreement_id: tradeId,
      receiver: BENEFICIARY,
      amount: COUNTER_AMOUNT,
      hash_lock: hashLock,
    }),
    postOpts(CUSTODIAN_TOKEN, "htlc-lock-with-hash"),
  );
  apiSync.add(lockB.timings.duration);
  lockLatency.add(Date.now() - lockStart);
  let contractIdB = "";
  try {
    contractIdB = lockB.json("contract_id") || "";
  } catch (_) {
    /* non-JSON */
  }
  if (!check(lockB, { "counter lock 201": (res) => res.status === 201 }) || !contractIdB) {
    settlementRate.add(false);
    return;
  }

  // 6. settle (originator reveals secret). The orchestrator gates the reveal on the
  //    relay having confirmed the counterparty lock — retry every 1s until it clears.
  let settledA = false;
  for (let i = 0; i < SETTLE_TIMEOUT_S; i++) {
    const sr = http.post(
      `${ORIGINATOR_GW}/api/v1/htlc/settle`,
      JSON.stringify({ contract_id: contractIdA, secret: secret }),
      postOpts(ORIGINATOR_TOKEN, "htlc-settle"),
    );
    if (sr.status === 200) {
      apiSync.add(sr.timings.duration); // the successful synchronous settle
      settledA = true;
      break;
    }
    // FailedPrecondition while the relay hasn't confirmed the counterparty lock — retry.
    if (sr.body && sr.body.indexOf("counterparty spoke has not yet locked") === -1) {
      break; // a different, hard error — stop retrying
    }
    sleep(1);
  }
  if (!check(null, { "initiator settle ok": () => settledA })) {
    settlementRate.add(false);
    return;
  }

  // 7. relay carries the secret to spoke-b → custodian leg SETTLED. This is the
  //    relayer cross-chain propagation leg (D6 ≤15s). Poll every 1s.
  const relayStart = Date.now();
  let bSettled = false;
  for (let i = 0; i < SETTLE_TIMEOUT_S; i++) {
    const st = http.get(`${CUSTODIAN_GW}/api/v1/htlc/status/${contractIdB}`, {
      headers: hdr(CUSTODIAN_TOKEN),
      timeout: REQ_TIMEOUT,
      tags: { endpoint: "htlc-status" },
    });
    let state = "";
    try {
      state = st.json("state") || "";
    } catch (_) {
      /* ignore */
    }
    if (isState(state, "SETTLED")) {
      bSettled = true;
      break;
    }
    sleep(1);
  }
  relayPropagation.add(Date.now() - relayStart);
  if (!check(null, { "custodian leg settled (relay)": () => bSettled })) {
    settlementRate.add(false);
    return;
  }

  // success — whole happy path completed on both spokes.
  settlementLatency.add(Date.now() - t0);
  settlementOk.add(1);
  settlementRate.add(true);
}
