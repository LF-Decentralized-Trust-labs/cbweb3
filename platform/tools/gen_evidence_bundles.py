#!/usr/bin/env python3
"""Generate machine-readable test-evidence bundles from the measured results.

Produces, under evidence-bundles/, one bundle per test run:

    evidence-bundle-<run-id>/
      execution_summary.json     # pass/fail per step (the schema the consumer expects)
      aggregated_traces.log      # correlation artifacts (e2e bundles)
      performance_report.html    # rendered metric summary (perf bundles)

Data source: the runs captured on branch test/r1-12.3-perf-harness (2026-06-19), recorded
in D12_results.md / docs/TEST-REPORTS.md / docs/performance/. Fields the current harness
does not emit per call (tx_hash, block_number, X-Correlation-Id) are null — not fabricated.
Re-run this script to regenerate (run ids are deterministic per bundle key so diffs are stable).
"""
import json, os, uuid, html, datetime

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "evidence-bundles")
TS = "2026-06-19"
# Deterministic run ids (uuid5) so regeneration is stable.
NS = uuid.UUID("00000000-0000-0000-0000-00000000d12d")
def run_id(key): return str(uuid.uuid5(NS, key))

def step(scenario_id, name, latency_ms, passed, http_status=None,
         tx_hash=None, block_number=None, **extra):
    r = {"scenario_id": scenario_id, "step": name, "http_status": http_status,
         "tx_hash": tx_hash, "block_number": block_number,
         "latency_ms": latency_ms, "passed": passed}
    r.update(extra)
    return r

NOTE_TRACE = ("Per-call tx_hash/block_number/X-Correlation-Id are null: the Go integration "
              "harness reports phase-level pass/fail + duration, not per-HTTP-call on-chain "
              "traces. contract_id is included where the test captured it. Add OTel/correlation "
              "emission to the harness for fully machine-graded traces.")

bundles = {}

# ── E2E Scenario A — TestFullHappyPath (live stack), 2026-06-19, PASS 56.11s ──
bundles["e2e-scenario-a"] = {
  "dir": "e2e-scenario-a",
  "summary": {
    "bundle": "e2e-scenario-a",
    "suite": "scenario-a TestFullHappyPath (go test -tags integration)",
    "scenario": "A",
    "passed": True,
    "total_duration_ms": 56110,
    "run_id": run_id("e2e-scenario-a"),
    "timestamp": TS,
    "notes": NOTE_TRACE,
    "results": [
      step("E2E-A-03", "phase_0_readiness", 50, True, http_status=200),
      step("E2E-A-03", "phase_1_login", 1640, True, http_status=200),
      step("E2E-A-03", "phase_2_onboard", 0, None, http_status=None, skipped=True),
      step("E2E-A-03", "phase_3_mint", 18730, True, http_status=201),
      step("E2E-A-03", "phase_4_fx_propose", 150, True, http_status=201),
      step("E2E-A-03", "phase_5_cross_spoke_sync", 6100, True, http_status=200),
      step("E2E-A-03", "phase_6_htlc_lock", 17120, True, http_status=201,
           contract_id_a="1c10748db4974fb073a4eb7d452fc4b8f0f9cf3a0f6764d6bc4a5ee9e64ef5ec",
           contract_id_b="59b53a13c55133ec7d939f9d01863e1da1d8904aa1f6cf1dfdd52e2027b40b7c"),
      step("E2E-A-03", "phase_7_settle", 12300, True, http_status=200),
      step("E2E-A-03", "phase_8_verify", 20, True, http_status=200),
    ],
  },
  "traces": [
    "# Scenario A — TestFullHappyPath correlation artifacts (2026-06-19)",
    "# format: contract_id -> tx_hash -> block_number (tx/block require trace harness; see note)",
    "phase_6_htlc_lock origin_leg   contract_id=1c10748db4974fb073a4eb7d452fc4b8f0f9cf3a0f6764d6bc4a5ee9e64ef5ec tx_hash=<not-captured> block=<not-captured>",
    "phase_6_htlc_lock counter_leg  contract_id=59b53a13c55133ec7d939f9d01863e1da1d8904aa1f6cf1dfdd52e2027b40b7c tx_hash=<not-captured> block=<not-captured>",
    "# X-Correlation-Id mapping not available: the harness does not yet propagate/emit it per request.",
  ],
}

# ── E2E Scenario B — TestFullHappyPath (live stack), 2026-06-19, PASS 63.06s ──
bundles["e2e-scenario-b"] = {
  "dir": "e2e-scenario-b",
  "summary": {
    "bundle": "e2e-scenario-b",
    "suite": "scenario-b TestFullHappyPath (go test -tags integration) — composite covering "
             "E2E-B-01 (liquidity), E2E-B-02 (MLP), E2E-B-03 (AMM swap), E2E-B-06 (bridge lock&mint)",
    "scenario": "B",
    "passed": True,
    "total_duration_ms": 63060,
    "run_id": run_id("e2e-scenario-b"),
    "timestamp": TS,
    "notes": NOTE_TRACE,
    "results": [
      step("E2E-B-01", "phase_0_readiness", 30, True, http_status=200),
      step("E2E-B-01", "phase_1_liquidity_provision", 32130, True, http_status=201),
      step("E2E-B-01", "phase_2_onboarding", 4370, True, http_status=201),
      step("E2E-B-06", "phase_3_fiat_issuance", 2640, True, http_status=201),
      step("E2E-B-06", "phase_3b_reserve_tokenisation", 4370, True, http_status=201),
      step("E2E-B-03", "phase_4_cross_currency_transfer", 10380, True, http_status=200),
      step("E2E-B-03", "phase_5_bank_b_receipt", 6130, True, http_status=200),
      step("E2E-B-02", "phase_6_lp_withdrawal", 2160, True, http_status=200),
    ],
  },
  "traces": [
    "# Scenario B — TestFullHappyPath correlation artifacts (2026-06-19)",
    "# X-Correlation-Id -> txHash -> blockNumber mapping not emitted by the current harness.",
    "# Phases passed; per-call on-chain traces require OTel/correlation instrumentation.",
  ],
}

# ── Coverage (backend + contracts), both scenarios ──
def cov(sid, name, pct, gate=80.0):
    return step(sid, name, None, pct >= gate, coverage_pct=pct, gate_pct=gate)
bundles["coverage"] = {
  "dir": "coverage",
  "summary": {
    "bundle": "coverage", "suite": "backend unit (D6 80% gate) + Foundry contract coverage",
    "scenario": "A+B", "passed": True, "total_duration_ms": None,
    "run_id": run_id("coverage"), "timestamp": TS,
    "notes": "Coverage results: latency/http/tx fields are null (not applicable). passed = coverage_pct >= gate_pct.",
    "results": [
      cov("COV-A-backend", "scenario-a/payment-orchestrator", 82.0),
      cov("COV-A-backend", "scenario-a/compliance", 90.9),
      cov("COV-A-backend", "scenario-a/auth", 85.6),
      cov("COV-A-backend", "scenario-a/shared-identity", 80.5),
      step("COV-A-contracts", "scenario-a/contracts (183 tests)", None, True,
           coverage_pct_lines=97.95, coverage_pct_branches=98.46, coverage_pct_funcs=100.0, gate_pct=80.0),
      cov("COV-B-backend", "scenario-b/api-gateway", 80.0),
      cov("COV-B-backend", "scenario-b/auth", 93.2),
      cov("COV-B-backend", "scenario-b/compliance", 88.8),
      cov("COV-B-backend", "scenario-b/payment-orchestrator", 83.5),
      cov("COV-B-backend", "scenario-b/shared-identity", 88.2),
      step("COV-B-contracts", "scenario-b/contracts (286 tests)", None, True,
           coverage_pct_lines=98.12, coverage_pct_branches=92.36, coverage_pct_funcs=100.0, gate_pct=80.0),
    ],
  },
}

# ── Performance Scenario A (R1-12.3), local 160230Z + EC2 confirmation ──
def perf(sid, name, target, measured, passed, p95_ms=None):
    return step(sid, name, p95_ms, passed, target=target, measured=measured)
bundles["perf-scenario-a"] = {
  "dir": "perf-scenario-a",
  "summary": {
    "bundle": "perf-scenario-a", "suite": "scenario-a.perf-all (R1-12.3) — local DURATION=2m + EC2 c6a.8xlarge",
    "scenario": "A", "passed": False, "total_duration_ms": None,
    "run_id": run_id("perf-scenario-a"), "timestamp": TS,
    "notes": "Functional + latency + D6 lifecycle gates pass; HTLC lock throughput + write latency fail "
             "(Paladin/Zeto synchronous blocking lock path; hardware-independent per EC2). See D12_results.md A.4.",
    "results": [
      perf("PERF-A-1", "htlc_transfer_throughput_50tps", "50 TPS, <1% err", "2.9-4.4 TPS", False),
      perf("PERF-A-2", "zeto_escrow_throughput_15tps", "15 TPS, <1% err", "15.0 TPS, 0% err", True),
      perf("PERF-A-3", "time_to_finality_per_spoke", "p95 < 5s", "no samples (n=0)", None),
      perf("PERF-A-4", "api_read_latency_p95", "p95 < 500ms", "36-81 ms", True, p95_ms=81),
      perf("PERF-A-5", "api_write_latency_p95", "p50<800 / p95<1500 ms", "p50 7200 / p95 60000 ms", False, p95_ms=60000),
      perf("PERF-A-6", "error_rate_steady_state", "< 1%", "0.03%", True),
      perf("PERF-A-D6-lifecycle", "htlc_full_lifecycle", "<= 60s", "p95 30.6s (local) / 32.9s (EC2)", True, p95_ms=32910),
      perf("PERF-A-D6-apisync", "synchronous_api_response", "<= 30s", "p95 7.1s", True, p95_ms=7148),
      perf("PERF-A-D6-relay", "relayer_cross_chain_propagation", "<= 15s", "p95 3.8s", True, p95_ms=3773),
      perf("PERF-A-D6-completion", "settlement_completion_rate", "> 90%", "100%", True),
    ],
  },
  "perf_metrics": [
    ("HTLC lock throughput (50 TPS)", "2.9–4.4 TPS", "50 TPS, <1% err", "FAIL"),
    ("Zeto escrow throughput (15 TPS)", "15.0 TPS, 0% err", "15 TPS, <1% err", "PASS"),
    ("Time-to-finality per spoke", "no samples", "p95 < 5s", "n/a"),
    ("API read latency p95", "36–81 ms", "p95 < 500ms", "PASS"),
    ("API write latency", "p50 7.2s / p95 60s", "p50<800 / p95<1500 ms", "FAIL"),
    ("Error rate (steady state)", "0.03%", "< 1%", "PASS"),
    ("D6 full HTLC lifecycle", "p95 30.6s / 32.9s (EC2)", "<= 60s", "PASS"),
    ("D6 synchronous API response", "p95 7.1s", "<= 30s", "PASS"),
    ("D6 relayer cross-chain", "p95 3.8s", "<= 15s", "PASS"),
    ("D6 settlement completion", "100%", "> 90%", "PASS"),
  ],
}

# ── Performance Scenario B (R1-12.3), run 20260618T162301Z ──
bundles["perf-scenario-b"] = {
  "dir": "perf-scenario-b",
  "summary": {
    "bundle": "perf-scenario-b", "suite": "scenario-b.perf-all (R1-12.3) — run 20260618T162301Z",
    "scenario": "B", "passed": False, "total_duration_ms": None,
    "run_id": run_id("perf-scenario-b"), "timestamp": "2026-06-18",
    "notes": "transfer/Zeto/latency gates pass; AMM 30 TPS over-driven (REVISE) + steady-state error rate fail "
             "(single-signer EVM nonce serialisation). TTF unmeasured. See D12_results.md B.4.",
    "results": [
      perf("PERF-B-1", "value_transfer_throughput_50tps", "50 TPS, <1% err", "9001 accepted, 0% err, admit p95 6ms", True, p95_ms=6),
      perf("PERF-B-2", "zeto_privacy_transfer_15tps", "15 TPS, <1% err", "2701 accepted, 0% err", True),
      perf("PERF-B-3a", "amm_swap_throughput_hub_only_30tps", "30 TPS, <1% err, p95<=6s", "88.9 TPS, 33.4% err, p95 2023ms", False, p95_ms=2023),
      perf("PERF-B-3b", "cross_chain_bridge_finality_ttf", "p95 < 5s", "no samples (n=0)", None),
      perf("PERF-B-3c", "full_cross_currency_payment", "ungated SLA", "9.4 TPS, 7.3% err, p95 60s", None, p95_ms=60001),
      perf("PERF-B-5", "amm_quote_latency_p95", "<= 300ms", "8 ms", True, p95_ms=8),
      perf("PERF-B-6", "amm_swap_latency_p95_hub_only", "<= 6000ms", "1027 ms", True, p95_ms=1027),
      perf("PERF-B-7", "pool_status_latency_p95", "<= 15000ms", "10 ms", True, p95_ms=10),
      perf("PERF-B-8", "error_rate_steady_state", "< 1%", "2.369%", False),
    ],
  },
  "perf_metrics": [
    ("Value-transfer throughput (50 TPS)", "9001 accepted, 0% err", "50 TPS, <1% err", "PASS"),
    ("Zeto privacy-transfer (15 TPS)", "2701 accepted, 0% err", "15 TPS, <1% err", "PASS"),
    ("AMM swap throughput hub-only (30 TPS)", "88.9 TPS, 33.4% err", "30 TPS, <1%, p95<=6s", "REVISE"),
    ("Cross-chain bridge finality (TTF)", "no samples", "p95 < 5s", "UNKNOWN"),
    ("Full cross-currency payment", "9.4 TPS, 7.3% err, p95 60s", "ungated SLA", "CHARACTERIZED"),
    ("AMM quote latency p95", "8 ms", "<= 300ms", "PASS"),
    ("AMM swap latency p95 (hub-only)", "1027 ms", "<= 6000ms", "PASS"),
    ("Pool-status latency p95", "10 ms", "<= 15000ms", "PASS"),
    ("Error rate (steady state)", "2.369%", "< 1%", "FAIL"),
  ],
}

def perf_html(b):
    rows = "\n".join(
        f'<tr class="{v.lower()}"><td>{html.escape(n)}</td><td>{html.escape(m)}</td>'
        f'<td>{html.escape(t)}</td><td class="verdict">{v}</td></tr>'
        for n, m, t, v in b["perf_metrics"])
    return f"""<!doctype html><html><head><meta charset="utf-8">
<title>{html.escape(b['summary']['bundle'])} — performance report</title>
<style>
body{{font-family:system-ui,sans-serif;margin:2rem;color:#1a1a1a}}
h1{{font-size:1.3rem}} table{{border-collapse:collapse;width:100%;margin-top:1rem}}
th,td{{border:1px solid #ddd;padding:.5rem .7rem;text-align:left;font-size:.9rem}}
th{{background:#f4f4f5}} .verdict{{font-weight:600}}
tr.pass .verdict{{color:#15803d}} tr.fail .verdict{{color:#b91c1c}}
tr.revise .verdict,tr.unknown .verdict,tr.characterized .verdict,tr.n\\/a .verdict{{color:#a16207}}
.meta{{color:#666;font-size:.85rem}}
</style></head><body>
<h1>{html.escape(b['summary']['suite'])}</h1>
<p class="meta">run_id {b['summary']['run_id']} · {b['summary']['timestamp']} ·
overall: {'PASS' if b['summary']['passed'] else 'gates breached (see below)'}</p>
<p class="meta">Rendered summary of the k6 measurements. The native k6 dashboard requires
running with <code>k6 run --out web-dashboard</code> (nightly). Source: D12_results.md.</p>
<table><thead><tr><th>Measurement</th><th>Measured</th><th>Target</th><th>Verdict</th></tr></thead>
<tbody>
{rows}
</tbody></table></body></html>"""

# ── write everything ──
os.makedirs(OUT, exist_ok=True)
index = []
for key, b in bundles.items():
    rid = b["summary"]["run_id"]
    d = os.path.join(OUT, f"evidence-bundle-{rid}")
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "execution_summary.json"), "w") as f:
        json.dump(b["summary"], f, indent=2)
        f.write("\n")
    if "traces" in b:
        with open(os.path.join(d, "aggregated_traces.log"), "w") as f:
            f.write("\n".join(b["traces"]) + "\n")
    if "perf_metrics" in b:
        with open(os.path.join(d, "performance_report.html"), "w") as f:
            f.write(perf_html(b))
    n = len(b["summary"]["results"])
    npass = sum(1 for r in b["summary"]["results"] if r.get("passed") is True)
    index.append((b["dir"], rid, b["summary"]["passed"], npass, n))

with open(os.path.join(OUT, "README.md"), "w") as f:
    f.write("# Test evidence bundles\n\n")
    f.write("Machine-readable evidence bundles generated by `tools/gen_evidence_bundles.py` "
            "from the measured runs on this branch (2026-06-19). Each bundle holds an "
            "`execution_summary.json` (pass/fail per step), and where applicable an "
            "`aggregated_traces.log` (e2e) and `performance_report.html` (perf).\n\n")
    f.write("| Bundle | run_id | overall | steps pass/total |\n|---|---|:---:|:---:|\n")
    for name, rid, passed, npass, n in index:
        f.write(f"| {name} | `{rid}` | {'✅' if passed else '❌'} | {npass}/{n} |\n")
    f.write("\nFidelity note: the Go integration harness emits phase-level pass/fail + durations, "
            "not per-call `tx_hash`/`block_number`/`X-Correlation-Id`; those fields are `null` "
            "(never fabricated). `contract_id`s are included where captured.\n")

print(f"wrote {len(bundles)} bundles to {OUT}")
for name, rid, passed, npass, n in index:
    print(f"  {name:22} {rid}  {'PASS' if passed else 'FAIL'}  {npass}/{n}")
