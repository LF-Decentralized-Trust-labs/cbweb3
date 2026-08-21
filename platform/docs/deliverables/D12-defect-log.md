# D12 — Defect Log

**Finding:** R1-12.9 (P2) — deliverables review: "No defect log, UAT records, or timeline actuals."
**Scope:** Deliverable 12 test execution, both scenarios
**Companion artifacts:** [timeline actuals](D12-timeline-actuals.md) · [UAT records](D12-uat-records.md) ·
[measured results](../../D12_results.md)
**Last updated:** 2026-08-21

## 1. What this is

Two things at once: the **register format** the LNet-run validation phases (Phase 5 UAT and
Phase 6 regression/retest) will use, and the **register itself**, already populated with the
defects the executed phases produced.

It is deliberately not a fresh empty template. Phases 1–4 have run and found real defects;
starting the log at Phase 5 would present the validation phases with a platform that appears
never to have failed a test. §6 therefore records what was found before UAT, and §7 is the
UAT-phase register, which is empty because UAT has not run.

Defects found before this file existed were tracked as cards in the project tracker rather
than here. §6 is the consolidated view of the material ones, each traceable to a commit or a
recorded run. From this file's date forward, the register is the primary record and the
tracker holds the workflow.

## 2. Register format

One row per defect. Fields, in order:

| Field | Rule |
| --- | --- |
| **ID** | `DEF-nnn` for defects found by LNET-run phases (0–4, 6); `DEF-UAT-nnn` for defects raised by a bank during Phase 5. Never reused, never renumbered. |
| **Scenario** | `A`, `B`, or `both`. A defect present in both trees gets one row and names both fixes — the scenarios are separate products, so a fix in one is not a fix in the other. |
| **Phase** | The phase that *found* it (0–7), not the phase that caused it. |
| **Severity** | Critical / Major / Minor / Cosmetic — defined in §3. |
| **Component** | Contract, service, portal, relay, or toolkit path — precise enough to route the fix. |
| **Raised** | ISO-8601 date the defect was recorded. |
| **Reported by** | Entity and role for UAT defects (`bank-a / treasury operator`); `LNET` plus the source for internal ones (`LNET / perf run 20260619T160230Z`). |
| **Summary** | What is wrong, stated as an observable failure. Not a proposed fix. |
| **Status** | Open / In progress / Fixed — awaiting retest / Closed / Deferred / Not a defect — defined in §4. |
| **Evidence** | Commit, run id, evidence-bundle run id, or document. A defect with no evidence pointer is not admissible. |
| **Resolution** | What changed, and where. Empty while open. |
| **Retested** | Date and by whom. For a UAT defect this must be the reporting entity, not LNET. |

Reproduction detail, log excerpts and root-cause analysis do not belong in the table. Put
them in the commit message that fixes the defect, or in the linked document, and reference
them from the row.

## 3. Severity and response times

Severity is about consequence, not effort. Both plans commit LNET to a 4-hour
acknowledgement and 24-hour resolution SLA for defects raised during Phase 5; the table
below is that commitment made specific.

| Severity | Definition | Acknowledge | Resolve | Escalation |
| --- | --- | --- | --- | --- |
| **Critical** | Value is lost, stranded, or created; atomicity breaks (one leg settles, the other does not); a compliance or authorization gate can be bypassed; private data is exposed. Also: the phase cannot proceed at all. | 4 h | 24 h | Immediate — project lead and the affected entity, in writing. |
| **Major** | A required flow cannot be completed, or an acceptance threshold is missed, but no value or privacy is at risk and a workaround exists. | 4 h | 24 h | Named in the daily phase report. |
| **Minor** | The flow completes; the result is wrong or misleading in a way that does not change the ledger outcome. Includes measurement defects — a number the harness cannot produce. | 1 business day | Before phase exit | None. |
| **Cosmetic** | Wording, layout, or non-functional presentation. | 1 business day | Best effort; may exit the phase open | None. |

Two rules that follow from the constitution rather than from severity: a defect that
weakens a compliance or security check is **Critical by definition**, and a defect involving
plaintext amounts or PII is **Critical by definition** regardless of how narrow the exposure
is.

## 4. Lifecycle

```
Open ──▶ In progress ──▶ Fixed — awaiting retest ──▶ Closed
  │                                  │
  └──▶ Deferred                      └──▶ Open  (retest failed)
  └──▶ Not a defect
```

- **Open** — recorded, evidence attached, not yet being worked.
- **In progress** — an owner and a branch exist.
- **Fixed — awaiting retest** — the fix is merged **and carries a test that fails without
  it**. A fix with no regression test does not reach this state.
- **Closed** — retest passed. A `DEF-UAT-nnn` may be closed only by the entity that raised
  it; LNET can move it to "awaiting retest" and no further.
- **Deferred** — real, accepted for now, with a written justification and an owner. Critical
  and Major defects cannot be deferred past Phase 7 without project-lead approval.
- **Not a defect** — the behaviour is correct or is a recorded design decision. The row
  stays, with the decision linked. This state exists so that a finding can be answered
  rather than quietly dropped; DEF-007's underlying auth decision is an example of the
  distinction it protects.

Phase exit criteria (from both plans): all Critical and Major defects closed, regression
suite re-run after each fix batch, and the reporting entity's confirmation on record.

## 5. Raising a defect during UAT

Minimum for a row to be admissible:

1. Entity, portal, role, and the account used.
2. The catalog case id being exercised (`E2E-A-03`, `INT-API-B-08`, …) or a description of
   the manual path.
3. Ordered reproduction steps, including inputs and the amounts involved.
4. Expected result versus observed result.
5. The `X-Correlation-Id` from the failing response, and the transaction hash if one was
   returned. This is what makes a report correlatable against the relay and the ledger;
   without it a stuck settlement cannot be told from a rejected one.
6. Timestamp (with timezone) and the stack/run id.

Amounts and counterparties belong in the record. They must not be pasted into a public
channel — route the report through the agreed UAT channel, and treat a defect report as
containing production-shaped data even on a devnet.

## 6. Register — defects found before Phase 5

Status as of 2026-08-21. Two sources feed this table, and both belong in it: the Phase 4
threshold findings, and defects found by internal code review between June and August.

- `DEF-001` … `DEF-006` are the open Phase 4 threshold findings, analysed in full in
  [`D12_results.md`](../../D12_results.md) §A.4 and §B.4 — the rows here do not restate
  that analysis.
- `DEF-007` onward were raised by review of the branches merged in that window. They are
  listed whatever their state: closing a defect is not a reason to drop it from the
  register, and neither is failing to close one. A register holding only the fixed ones
  measures effort rather than quality.

| ID | Scen. | Ph. | Severity | Component | Raised | Summary | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DEF-001 | A | 4 | Major | `payment-orchestrator` `/htlc/lock` + Paladin/Zeto | 2026-06-19 | HTLC lock admission sustains ≈ 2.9–4.4 TPS against the 50 TPS gate; dominant error is `wait lock receipt: context deadline exceeded`. Root cause is the synchronous blocking ZKP lock, confirmed hardware-independent on a c6a.8xlarge. | Open — fix is architectural (asynchronous admission) | [`RESULTS-2026-06-19T160230Z.md`](../../scenario-a/docs/performance/RESULTS-2026-06-19T160230Z.md), [`ec2-run-20260619/`](../../scenario-a/docs/performance/ec2-run-20260619/) |
| DEF-002 | A | 4 | Major | same as DEF-001 | 2026-06-19 | API write (admission) latency p95 pins at the 60 s request ceiling against a 1500 ms gate; median ≈ 7 s is already over the gate. Same root cause as DEF-001. | Open — resolves with DEF-001 | as DEF-001 |
| DEF-003 | A | 4 | Minor (measurement) | perf harness TTF correlation | 2026-06-19 | Time-to-finality reported `n = 0`: the id-capture source is the failing lock benchmark, so nothing was correlated against `HTLCLocked`. Unmeasured, not failed. | Open — needs a low-concurrency TTF run | `D12_results.md` §A.4 #3 |
| DEF-004 | B | 4 | Major | `AutomatedMarketMaker` swap path signer | 2026-06-18 | AMM swap error rate 33.4 % when driven to ≈ 88.9 TPS, against a < 1 % gate; latency gates stay green. Root cause is single-signer EVM nonce serialisation at the 2 s QBFT cadence. Requires a revised sustainable-rate target plus multi-key signing. | Open | [`scenario-b/docs/performance/RESULTS.md`](../../scenario-b/docs/performance/RESULTS.md) |
| DEF-005 | B | 4 | Major | as DEF-004 | 2026-06-18 | Steady-state error rate 2.369 % against a < 1 % gate — the over-driven swap path bleeding into the aggregate. | Open — resolves with DEF-004 | as DEF-004 |
| DEF-006 | B | 4 | Minor (measurement) | perf harness TTF correlation | 2026-06-18 | Cross-chain bridge finality reported `n = 0`; same capture failure as DEF-003. | Open | `D12_results.md` §B.4 #3b |
| DEF-007 | A | 3 | Major | `tests/integration` defaults, `backend/config/.env.infra.*` | 2026-08-21 | `TestFullHappyPath` cannot be run on demand. **Restated 2026-08-21 after the legacy path was retired** (`3b14ecaa`): the original diagnosis — client-credentials login refused by the password-grant-only auth service, with a local Keycloak that provisioned no users — described `deploy/local`, which no longer exists. Portal login is no longer the blocker: the toolkit provisions per-role users from `spec.adminUsers` and the samples walkthrough authenticates every entity successfully. What blocks the test now is that it still targets the deleted topology — legacy entities and ports (bank-a/bank-b/bank-d) and eight secrets read from `backend/config/.env.infra.*`, files the toolkit deliberately does not write — and `scenario-a.test-integration` no longer brings any stack up. Phase 3's evidence therefore has no current reproduction path. | Open — migration tracked as the "migrar o E2E e os tryouts para o toolkit" card; the auth behaviour itself remains *not a defect* | [`scenario-a/make/20-tests.mk:37-49`](../../scenario-a/make/20-tests.mk#L37-L49); [`scenario-a/tests/e2e/README.md`](../../scenario-a/tests/e2e/README.md) (still describes the removed path — see DEF-021) |
| DEF-008 | A | 2 | Critical | `payment-orchestrator` Paladin adapter | 2026-08-20 | `transferLocked` fails permanently (`PD210134`, wanted 1 found 0) for a locked-state id whose first byte is `0x00`; five retries over 50 s all returned 500. Central-bank money is locked on-chain and unrecoverable, and the only trace was one log line that did not name the amount. | Closed — the unsettleable id is refused before the relay can loop on it, and the refusal record now names amount and delegate so reconciliation can act. Underlying domain behaviour is upstream and unfixed; the guard is the containment. | `adf76ffc`, `ef78a528` |
| DEF-009 | both | 4 | Critical | `payment-orchestrator` gRPC server | 2026-08-20 | Zeto operation amounts were logged in plaintext to stdout, re-publishing the exact figure the privacy layer exists to hide. (Reviewed alongside `hashLock` and ERC20 amounts, which are already public on-chain and were deliberately left in place.) | **Partially closed** — the five Zeto call sites no longer carry the amount, and the regression guard is installed in both scenarios. The figure is still reachable: in `ApproveEscrow` the same `record.Amount` is logged seven lines above the redacted line (`escrow.go:211`, "burning fCeBM for escrow"), and the redacted line now carries `escrow_id`, which joins it to `escrow.go:185` where that escrow's amount is printed in full. Left in place on the argument that fCeBM is a public ERC20 — true against a chain observer, but the exposure this row names is the log stream, which the noc-agent ships to the NOC portal. Tracked as DEF-018. | `e8500ceb` (A), `ac744f26` (B) |
| DEF-010 | both | 0 | Critical | provisioning toolkit + compose templates | 2026-08-20 | Postgres and Keycloak credentials were constants in the toolkit, so every entity of every deployment shared them (`admin/admin` for Keycloak); Redis had no password while holding the relay-auth replay guard and the PKI login nonces, and was published on a host port nothing needed. Holding the repository meant holding every database in every network. | Closed — per-entity credentials generated from `crypto/rand` on first provisioning, persisted `0600`, required by the templates with no fallback | `85c888fe` |
| DEF-011 | both | 4 | Critical | Keycloak realm generation (toolkit) | 2026-08-19 | Realms were emitted with `redirectUris: ["*"]` and `webOrigins: ["*"]` — an open redirector for the authorization code, and any page able to read a token response — with `sslRequired` hardcoded to `none`. | Closed — origins now derive from the same list the entity gateway receives as CORS origins, generation fails closed with no origins, and `sslRequired` follows `spec.environment` | `d534fe74`, `323b5a8d` |
| DEF-012 | B | 4 | Critical | Cacti relay routes, swap handler | 2026-08-19 | Three authorization defects: `POST /api/v1/spokes` accepted an anonymous registration (which redirects what the relay watches and where it forwards settlement); the bridge-out route compared its secret with `!==`, leaking match length through timing; and `GetSwapStatus` resolved a swap by id alone, so any authenticated bank could read another bank's amounts, rate, counterparty and position ids. | Closed — both relay routes authenticate through one fail-closed helper, and the swap read is scoped to the caller's bank, answering 404 | `864082ca` |
| DEF-013 | B | 3 | Minor | supervisor portal / hub token supply | 2026-08-11 | The supervisor portal resolved a hub token by token-name prefix instead of by currency, so supply figures could be attributed to the wrong token. | Closed | `8566c511` |
| DEF-014 | both | 1 | Minor | HTLC invariant suite | 2026-08-20 | The terminal-state invariant could not observe a repeated terminal transition: mutating the contract to accept a second `settle` on a SETTLED lock left all six invariants green. A second settle re-emits the claim event that the relay reads to release private value, which is exactly the failure the invariant is named for. | Closed — the violation is recorded at the point of success rather than in the sweep, under its own flag, with a comment on why the sweep cannot catch it | `f646cf01` (B), `14bb8eb6` (ported to A) |
| DEF-015 | A | 4 | Major | `api-gateway` supervisor handler | 2026-08-20 | The `DECRYPT_TRANSACTION` audit entry builds its `Details` JSON with `fmt.Sprintf` and `%q` (`supervisor_handler.go:220`). `%q` is Go quoting, not JSON quoting: a control byte, a DEL byte or invalid UTF-8 in the operator-typed reason renders as `\x7f`, which `encoding/json` rejects. The record created for accountability is then the one nobody can parse, on a field a human types by hand. | Open — the sibling defect in the mint/burn path was fixed by encoding with `json.Marshal`; this call site was left | `7ad6b837` (the fix applied to the sibling), `supervisor_handler.go:220` |
| DEF-016 | both | 0 | Major | provisioning toolkit `.gitignore` | 2026-08-20 | The per-entity credentials DEF-010 introduced are written to `<dataDir>/.infra-secrets.env`, which is git-ignored only when the data dir happens to sit under `**/cbweb3-data/`. With an operator-chosen `--data-dir` the file is committable, while `.provisioning-state.yaml` beside it is ignored by name. The remedy for shipping shared credentials can therefore put generated ones back in the repository. | Open — one rule (`**/.infra-secrets.env`) closes it | `.gitignore`, verified with `GIT_CONFIG_GLOBAL=/dev/null git check-ignore -v` |
| DEF-017 | B | 1 | Minor | contract deploy-script tests | 2026-08-20 | `DeployFiatCBMTest.test_DeployScript_ConfiguresCorrectly` and `DeployCBWeb3SpokeTest.test_ScriptRun_Success` are flaky. Four consecutive full-suite runs on one unchanged tree gave 2, 1, 0 and 1 failures, at identical gas, and a failure passes on `--rerun`. A suite that is red at random trains readers to ignore red. | Open — cause not diagnosed; suspected order or shared-state dependence between suites | `scenario-b/contracts/test/FiatCentralBankMoney.t.sol`, `scenario-b/contracts/test/CBWeb3Spoke.t.sol` |
| DEF-018 | A | 4 | Major | `payment-orchestrator` escrow flow | 2026-08-20 | The residue of DEF-009: `ApproveEscrow` logs the escrow amount at `escrow.go:211` and `:185`, and the Zeto line redacted by DEF-009 now carries `escrow_id`, which joins them. For a reader with NOC-portal access and no chain access — the audience DEF-009 identifies as newly exposed — the amount is unchanged. | Open — needs a decision: redact the sibling lines too, or record that the escrow amount is public by construction (`s.fiat.Burn` and `s.zeto.Mint` take the same value) and narrow DEF-009's scope | `escrow.go:185`, `:211`, `:218` |
| DEF-019 | A | 2 | Minor | scenario A toolkit test suite | 2026-08-21 | `TestEncodeDeployData_Fiat` and `_HTLC` read Foundry build artifacts under `contracts/out/`, which is git-ignored, so both fail from a clean clone until `forge build` has run. Compounding it, no CI workflow runs either toolkit suite — the backend workflows filter on `scenario-*/backend/**` — so neither the failure nor any future toolkit regression is caught. | Open | `scenario-a/toolkit/engine/orchestrator/deploy_contract_test.go:14-15`, `.github/workflows/backend-scenario-a.yml` |
| DEF-020 | B | 3 | Minor | scenario B deployment runbook | 2026-08-20 | The relay diagnosis step tells the operator to run `curl -s http://localhost:7000/api/v1/spokes` (runbook line 1446). The relay registers that path for **POST** only, so the command returns 404 — and the runbook instructs the reader to interpret an empty result as "the registration step did not run", i.e. to read a 404 as a diagnosis. | Open — drop the check, point it at the provisioning-time log line, or add a `GET` to the relay | `scenario-b/docs/runbooks/deployment-runbook.md:1424`, `scenario-b/interop/hub-and-spoke/cacti/src/index.ts` |
| DEF-021 | A | 0 | Minor | `docs/test-execution-plan.md`, `tests/e2e/README.md` | 2026-08-21 | The retirement of the legacy `deploy/local` path (`3b14ecaa`, 2026-08-21) removed `make spoke-all`, and the documents that instruct it were not updated. Deliverable 12's own plan of record names it five times — the Phase 0 checklist, the E2E level definition, the devnet persistence and teardown rows, and the reproduction snippet — so the deliverable tells a reader to bring the stack up with a target that does not exist. The Scenario A E2E pointer still attributes the login failure to `deploy/local/keycloak/init.sh`, a file that is gone (see DEF-007). | Open — one pass over both files; the toolkit/samples path is the replacement | `scenario-a/docs/test-execution-plan.md:129,254,360,362,532`; `scenario-a/tests/e2e/README.md:42`; commit `3b14ecaa` |

### What is deliberately not in this register

Three categories are tracked elsewhere, and mixing them in would make the log useless as a
measure of quality:

- **Known coverage gaps** — areas with no test yet, listed in
  [`scenario-b/docs/test-execution-plan.md`](../../scenario-b/docs/test-execution-plan.md)
  ("Known Test Coverage Gaps") and in the per-scenario catalogs. A missing test is not a
  failed test.
- **Design decisions that read as defects** — for example the password-grant-only login
  behind DEF-007. Recorded where the decision lives, and referenced from the row it
  explains.
- **Deliberate scenario asymmetries** — [`docs/scenario-drift.md`](../scenario-drift.md).
  Read it before filing an A-versus-B difference as a defect.

## 7. Register — Phase 5 UAT defects

**Zero rows. Phase 5 has not been executed** — no bank testers have been named and no UAT
session has taken place (see [timeline actuals](D12-timeline-actuals.md) §2). The table below
is the format those defects will use; the columns match §2, with the reporting entity
mandatory.

| ID | Scen. | Entity / role | Portal | Case id | Severity | Raised | Summary | Status | Ack'd | Resolved | Retested by / date | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| _(none)_ | | | | | | | | | | | | |

Phase 6 exits when every Critical and Major row here is **Closed**, closed by the entity
that raised it, with the regression suite re-run after the fix batch.

## 8. Traceability

| This log | Links to |
| --- | --- |
| Threshold findings DEF-001 … DEF-006 | [`D12_results.md`](../../D12_results.md) §A.4 / §B.4 — root cause and justification per finding |
| Phase evidence and dates | [timeline actuals](D12-timeline-actuals.md) |
| UAT records and sign-off | [UAT records](D12-uat-records.md) |
| Coverage and gate status | [`docs/TEST-REPORTS.md`](../TEST-REPORTS.md) |
| Case ids | [`scenario-a/tests/TEST-CATALOG.md`](../../scenario-a/tests/TEST-CATALOG.md) · [`scenario-b/tests/TEST-CATALOG.md`](../../scenario-b/tests/TEST-CATALOG.md) |
| Machine-readable run evidence | `tools/gen_evidence_bundles.py` → `evidence-bundles/` (build output, regenerate with `make evidence.e2e-a` / `make evidence.e2e-b`) |
