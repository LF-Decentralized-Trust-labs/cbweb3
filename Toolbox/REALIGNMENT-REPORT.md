<!-- SPDX-License-Identifier: Apache-2.0 -->
# Toolbox realignment — final report

**To:** Carolina Velásquez, LNet — project lead, CBWeb3
**Date:** 2026-08-21
**Branch:** `chore/dpg-readiness`
**Status:** work complete on disk and staged in git. **Nothing is committed.** See §6.1.

---

## 1. What changed, and why

The Toolbox published an interface contract for an API that does not exist. It described
seven flat endpoints (`POST /fx/agreement`, `POST /htlc/lock`, …) secured by an
`Authorization: Bearer` header, and shipped mocks, test vectors and conformance tests that
all validated happily against that fiction — because the CI mock server was generated from
the same fictional document. The delivered platform serves **CBWeb3 API Gateway v2.3.0**,
whose paths are prefixed (`/api/v1/payments/fx/agreements`, `/api/v1/htlc/lock`) and whose
credential is an HttpOnly cookie, not a bearer token. Not one path matched. Since the
Toolbox is the artifact the Digital Public Goods submission points to as evidence that
third parties can build on CBWeb3, an integrator following it would have failed on their
first request. **This work replaced the entire kit with one built from the delivered
platform's own specification and, where that specification is wrong about its own
software, from the delivered source code.** The Toolbox now publishes three contracts
covering 88 real endpoints across both scenarios, backed by 59 reference mocks, 140 test
vectors and 91 executable conformance tests, plus two new automated gates that make the
original class of failure — publishing an endpoint nobody serves — impossible to repeat.
The vendor was asked to change nothing; every correction is on our side.

---

## 2. Decisions taken, and the reasoning behind each

These are the decisions you will be asked to defend, to GoLedger/AguilaHub and to the DPGA
reviewers. Each is stated with its rationale.

### 2.1 The platform surface wins, without exception

**Decision.** Where the Toolbox and the delivered gateway disagreed, the Toolbox changed.
No renaming, no shims, no change requests were produced for the vendor.

**Rationale.** The gateway is delivered and in production use on the staging estate by
BCCR, BCRP, BCCh and SUGEVAL. Asking the vendor to rename endpoints in a system central
banks are already testing would be expensive, slow, and would put the DPG timeline at the
mercy of a contract negotiation. The Toolbox has **zero downstream consumers** — nobody has
built against it — so breaking it costs nothing. This is the cheapest possible resolution
and it carries the vendor burden you required: **zero**.

**Verified consequence.** Every path the Toolbox publishes exists in a delivered platform
specification. Machine-checked by set difference against both gateway specs: **0 invented
paths**.

### 2.2 Three contracts, not one — split by scenario, not by domain

**Decision.** `contracts/auth/` (shared), `contracts/pvp/` (Scenario A), `contracts/amm/`
(Scenario B).

**Rationale.** §4.1 item 4 of the migration guidelines asked us to clarify how one
published contract can cover two scenarios with materially different settlement surfaces.
It cannot, honestly. Scenario A settles via FX agreement plus dual-layer HTLC. Scenario B
has **no FX agreement and no HTLC at all** — it settles through escrow, bridge
lock-mint/burn-unlock and AMM swaps. A single document with optional sections would have
forced every reader to work out which half applies to them, and would have invited exactly
the "one contract, two realities" drift we just spent this work undoing. The authentication
surface, by contrast, was verified **byte-identical** between the two gateway specs, so it
is published once — removing the only drift risk on the thing every integration starts
with.

One naming axis (`auth` / `pvp` / `amm`) is reused across `contracts/`, `mocks/`,
`test-vectors/` and `conformance/tests/`. There is no second axis.

### 2.3 Versioning tracks the gateway: `v2.3.0`, not `v0.2.0`

**Decision.** All three contracts are versioned `2.3.0`, matching
`CBWeb3 API Gateway v2.3.0`. The migration guidelines proposed re-cutting to `v0.2.0`.

**Rationale.** A Toolbox version number that is independent of the gateway version is a
second thing to keep in sync, and this whole exercise is about eliminating things that can
silently drift. Naming the file after the gateway release it describes means the answer to
"which gateway does this contract match?" is visible in the filename, and a reader who sees
`openapi_pvp_v2.3.0.yaml` beside a gateway reporting `v2.4.0` knows immediately that the
Toolbox is stale. **This is a deviation from what the migration guidelines proposed and you
should ratify it explicitly** — see §6.2.

`Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml` is **deleted, not deprecated**. No alias,
no shim, no compatibility layer. It described an API that never existed on any CBWeb3
deployment, so there is nothing to be compatible with. `Toolbox/CONTRIBUTING.md` was
reworded accordingly: compatibility is owed to real consumers, not to published files.

### 2.4 Scope boundary: settlement, not administration

**Decision.** The Toolbox publishes the surface a third party needs to move value. It does
**not** publish compliance, governance, supervisor, oversight, onboarding, treasury,
identities, statement, PKI administration, or any `/internal/*` route.

**Rationale.** Those surfaces are operator-facing, not integrator-facing; several are
relay-internal and are guarded by a shared-secret header a third party will never hold.
Publishing them would inflate the apparent coverage without helping anyone build anything,
and would put endpoints in a DPG-facing artifact that no external party can legitimately
call.

Measured against the delivered specifications:

| | Platform paths | Published | Deferred |
|---|---:|---:|---:|
| Scenario A gateway | 74 | 36 | 38 (compliance 11, governance 13, onboarding 5, oversight 3, treasury 2, `/internal/*` 4) |
| Scenario B gateway | 99 | 60 | 39 (governance 12, compliance 7, onboarding 5, audit 1, `/internal/*` 14) |

Restricting to the documented `/api/v1` surface of Scenario A: 28 paths in `pvp/` plus 7 in
`auth/` = **62 of 69**; the 34 deferred are the administrative families above.

### 2.5 Authentication: CookieAuth everywhere, Bearer only where the platform allows it

**Decision.** `CookieAuth` (`apiKey` in cookie, name `access_token`, HttpOnly,
SameSite=Strict) is the sole credential for the entire `/api/v1` surface, declared in all
three contracts. `BearerAuth` is declared in `amm/` only, as an alternative on the twelve
Scenario B `/api/v2` `RequireAnyAuth` routes; **the two swap endpoints are cookie-only** and
are documented as such. `RelayAuth` is declared nowhere, because no `/internal/*` endpoint
is published. `CBWEB3_AUTH_TOKEN` was removed repository-wide.

**Rationale.** This is simply what the gateway does. The conformance suite now uses a
session-scoped cookie jar with four modes (`mock`, `direct`, `pki`, `bearer`), including the
two-step nonce + `wallet/bind` handshake that commercial banks actually perform.

**One security finding is recorded rather than papered over: the platform implements no
CSRF protection.** No CSRF header exists anywhere — confirmed by exhaustive search of both
specifications and the gateway source. `SameSite=Strict` on the session cookie is the sole
cross-site mitigation. This is written up in
`Toolbox/conformance/spec/security_checklist.md` as an open finding. It is a real
observation about the delivered system and you should expect a question about it.

### 2.6 Where the platform's own document is wrong, we describe the software and say so

**Decision.** The gateway's published OpenAPI document disagrees with the gateway binary in
23 catalogued places. The contracts describe the **binary**, each carrying a
`Spec divergence:` note naming the divergence and citing the Go source. Every instance is
collected in one canonical register, **`Toolbox/DIVERGENCES.md`** (rows A1–A11 for Scenario
A, B1–B12 for Scenario B, plus a table of the five incompatible pool-pair spellings).

**Rationale.** A contract that faithfully copies a document known to be wrong produces
clients that fail in production. A contract that silently "fixes" it makes us look like we
invented an API. The register is the third option: describe what runs, cite the line of Go
that proves it, and let a reader audit every claim.

**This must be stated as a known limitation in the DPG evidence, not presented as the
vendor's own contract.** Concretely: **51 of the 95 schemas** in the Scenario B contract
carry `x-cbweb3-source: implementation-observed`, meaning they were reconstructed from
delivered Go source because the platform's document declares them as free-form objects.
That is more than half of Scenario B. It is honest and it is annotated, but it is not the
vendor's normative artifact and we should never describe it as one.

**Five** of the divergences are blocking — a client written strictly against the vendor's
published document cannot work. Three are in Scenario A; two are carried as
`Spec divergence (blocking):` notes on the Scenario B AMM quote operations:

| Row | The document says | The gateway does |
|---|---|---|
| A1/A2 | `state: "ACCEPTED"` | `state: "FX_STATE_ACCEPTED"` — the protobuf enum name |
| A4 | **412** on an invalid FX state transition | **409 Conflict**. No payments or HTLC route emits 412 at all |
| A5/A6 | `GET /htlc/status` returns `{"lock": {…}}` with a `secret` | the **bare** record, with `counterparty_locked`/`amount`/`created_at` and **no** `secret` |
| B13 | `quote/exact-output` takes **`amountOut`** (camelCase) | reads **`amount_out`**; the documented name always yields `400 INVALID_REQUEST` |
| B14 | `quote/cross-currency` declares **zero parameters** | requires three (`source_currency`, `target_currency`, `amount_out`); following the document literally always yields 400 |

The two AMM rows are the only divergences the contracts themselves label `(blocking)`; the
three Scenario A rows were classified during this review. The word is now used for the same
thing in both places.

Each was verified directly in the delivered source during this report's own review (see §4).

The Scenario B circuit-breaker operations are **not** in this set. They are `Spec gap:` cases
— the platform declares no request or response body where the handler requires one — which
leaves an integrator without guidance rather than actively misleading them.

### 2.7 Two new CI gates, because the absence of them is what let this happen

**Decision.** `Toolbox/tools/validate_artifact_paths.py` asserts that every mock and vector
path resolves to a real path + method in one of the three contracts.
`Toolbox/tools/verify_hashlocks.py` recomputes every documented SHA-256 secret/hash-lock
pair. Both are wired into `.github/workflows/toolbox-ci.yml` as required jobs, and both
hard-fail if the script is missing.

**Rationale.** Nothing in CI ever cross-checked an artifact path against the contract. That
is precisely why seven fictional endpoints shipped green through four sessions.

### 2.8 The false hash lock — a correctness defect, now permanently guarded

Every pre-realignment artifact claimed `SHA-256('cbweb3-test-secret-2026') = 0x7f83b165…9069`
and instructed implementers to verify it. It was wrong twice over: the true digest of that
string is `000cd63c…fdcf`, and the string is not valid hex, so the gateway would have
rejected it outright — `SettleHTLC` hex-decodes the secret and hashes the **decoded bytes**.
An interim proposal to substitute `"hello"` → `2cf24dba…` was also unusable for the same
hex-decoding reason and was discarded. The kit now ships verified 32-byte hex preimages with
their true digests, and `verify_hashlocks.py` recomputes all 11 documented pairs on every CI
run. The discredited digest survives in the repository only as elided prose in retraction
notes, enforced by a rule in CI.

---

## 3. File manifest — counted from disk

`137 files changed, 26,867 insertions(+), 2,522 deletions(-)` against `HEAD` (`d12f9d7`) — of which 26,337 insertions are realignment artifacts and the remainder is this report.

### Created — 94 files (93 realignment artifacts + this report)

| Area | Files | Detail |
|---|---:|---|
| Contracts | 9 | `auth/`, `amm/` (spec + README + CHANGELOG each), `pvp/openapi_pvp_v2.3.0.yaml` |
| Divergence register | 1 | `Toolbox/DIVERGENCES.md` — 23 source-traced rows |
| Mocks | 55 | `auth/` 5, `pvp/` 20 new, `amm/` 31, plus 4 READMEs (`mocks/`, `auth/`, `amm/`, + `pvp/` rewritten) |
| Test vectors | 8 | `pvp_reserves`, 5 `amm/*`, `auth_vectors`, plus 3 READMEs (`test-vectors/`, `amm/`, `auth/`) |
| Conformance | 8 | `helpers.py`, `tests/auth/`, `tests/amm/` (4 modules), `test_pvp_reserves.py`, 2 package markers |
| Tooling | 3 | `tools/validate_artifact_paths.py`, `tools/verify_hashlocks.py`, `tools/README.md` |
| Sandbox | 1 | `tutorials/03-hub-swap-mock.md` — Scenario B had no tutorial, though three existing pages linked to one |

### Modified — 34 files

Contracts `pvp/README.md` + `pvp/CHANGELOG.md`; conformance `conftest.py`, `pytest.ini`,
`README.md`, `spec/conformance_requirements.md`, `spec/security_checklist.md`,
`test_pvp_fx_agreement.py`, `test_pvp_htlc.py`, 2 package markers; vectors
`pvp_fx_agreement_vectors.json`, `pvp_htlc_vectors.json`, `pvp/README.md`; mocks
`pvp/README.md`; all 4 `sandbox/devnet-guide/` pages, both existing tutorials,
`sandbox/README.md`, `sample-configs/.env.example`, `sample-configs/prism-config.md`;
`Toolbox/README.md`, `Toolbox/CONTRIBUTING.md`, `Toolbox/docs/onboarding/README.md`; and at
repository root `README.md`, `CHANGELOG.md`, `AGENTS.md`,
`.github/workflows/toolbox-ci.yml`, three `.github/ISSUE_TEMPLATE/*.yml`.

### Deleted — 9 tracked files

`Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml`, six `mocks/pvp/happy-path/*.json`, two
`mocks/pvp/timeout-refund/*.json`. Every one carried a non-existent path, a `Bearer` header
and/or the false hash.

Also deleted, untracked and therefore invisible to git: **`Toolbox/REALIGNMENT-PLAN.md`**.
It was a working document and must not reach the PR. Four files referenced it and were
updated. It is gone from disk.

### Counts as shipped

| Artifact | Count | Detail |
|---|---:|---|
| Contracts | **3** | all `v2.3.0` |
| Paths / operations | **88 / 99** | auth 8/8, pvp 28/32, amm 52/59 |
| Public (unauthenticated) operations | **16** | auth 4, amm 12, pvp 0 |
| Reference mocks | **59** | auth 5; pvp 23 (17 happy, 3 timeout, 3 error); amm 31 (18 happy, 8 provisioning, 2 residue, 3 error) |
| Distinct contract operations exercised by mocks | **38 of 99** | template-aware resolution |
| Test vectors | **140** in 9 files | 66 happy-path, 61 error, 13 edge-case |
| Conformance tests | **91** | 54 `mock_safe`, 37 `live_only`; auth 10, pvp 40, amm 41 |
| Divergence rows | **23** | A1–A11 (Scenario A), B1–B12 (Scenario B) |

---

## 4. Verification — what was actually executed

Every result below was observed in this session, on the current working tree, after the
final edits. Nothing is reported from another agent's summary.

**Independence note.** Three Prism mock servers were already running from earlier in this
work, but all three had been started **before** the contracts were last modified
(contracts touched 00:36:50, newest Prism started 00:33:33). They were serving stale
documents, so any earlier "tests pass" claim was not evidence. All were killed and
restarted against the current files before the runs below.

| # | Check | Command | Result |
|---|---|---|---|
| 1 | OpenAPI lint | `spectral lint --ruleset .spectral.yml --fail-severity=hint` on all 3 contracts | **PASS** — "No results with a severity of 'hint' or higher found!" |
| 2 | Mock meta-schema | `ajv validate --spec=draft2020` × 59 | **PASS** — 0 failures |
| 3 | Vector meta-schema | `ajv validate --spec=draft2020` × 9 | **PASS** — 0 failures |
| 4 | Artifact-path gate | `python3 Toolbox/tools/validate_artifact_paths.py` | **PASS** — 77 path templates; 59 mocks + 140 vectors; every path resolves; exit 0 |
| 5 | Hash-lock gate | `python3 Toolbox/tools/verify_hashlocks.py` | **PASS** — 11 pairs recomputed; exit 0 |
| 6 | False-hash rule | full 64-char discredited digest, repo-wide grep | **PASS** — absent |
| 7 | Conformance collection | `pytest --collect-only`, no server | **PASS** — 91 collected, 0 errors |
| 8 | Conformance vs Prism | `pytest -m "mock_safe"` against freshly restarted 4010/4011/4012 | **PASS** — **54 passed, 37 deselected** |
| 9 | CI's own selector | `pytest -m "mock_safe and happy_path"` | **PASS** — 39 passed, 52 deselected |
| 10 | CI smoke probes | every `curl` from the `conformance-smoke` job, replayed by hand | **PASS** — incl. the negative case: no cookie → **401** |
| 11 | No invented paths | set difference of our 88 paths against both platform specs | **PASS** — **0** paths exist in our contracts that exist in neither gateway spec |
| 12 | Stale-reference sweep | `openapi_pvp_v0.1.0`, `/fx/agreement`, `/htlc/lock`, `Bearer`, `CBWEB3_AUTH_TOKEN`, `7f83b165` | **PASS** — every surviving hit is a deliberate historical/retraction note or a correct statement about the 12 bearer-capable `/api/v2` routes |
| 13 | Markdown links | all 43 in-repo `.md` files, relative links, URL-decoded | **PASS** — 0 broken (1 was broken; fixed, see below) |
| 14 | Divergence spot-check | A4, A5/A6, A8/A9 traced to delivered Go source | **CONFIRMED** — `payment.go:1126` `FailedPrecondition → StatusConflict`; the gateway's only `StatusPreconditionFailed` is `onboarding_proxy.go:465`; `HTLCStatus` struct has `counterparty_locked`/`amount`/`created_at` and no `secret`; `mint_tx_hash` at `payment_grpc.go:276,345` |
| 15 | Scenario B spot-check | `listLiquidityCommits` fix | **CONFIRMED** — `liquidity_handler.go:409-411` requires `pool_pair` and 400s without it; the contract now marks it `required: true` and declares 400/500 |

### Five errors I found and corrected while verifying

These were live in the tree when I began. I fixed them and re-ran every gate above.

1. **The published conformance-test count was wrong — again.** Four places said **90**; the
   real number is **91**, and three marker sub-counts were also off by one (`error` 26→27,
   `amm` 40→41, `registry` 10→11, `scenario_b` 40→41). A test had been added during the
   repair pass without re-deriving the totals. Corrected in `README.md`,
   `Toolbox/README.md` and `Toolbox/conformance/README.md`. This is the same failure mode
   the realignment set out to fix, recurring inside the fix, which is worth noting: these
   counts need a CI gate of their own (§6.6).
2. **A broken link.** `Toolbox/test-vectors/amm/README.md` pointed at `../DIVERGENCES.md`,
   which resolves to a non-existent file; corrected to `../../DIVERGENCES.md`.
3. **The pool-pair spelling count was never fully reconciled.** The register settled on
   **five** spellings, but `.env.example`, `amm_registry_vectors.json` and two AMM mock
   fixtures still said "three". All four corrected.
4. **An unsupported provenance claim survived in a fixture.** `04_confirm_pair.json`
   asserted "The delivered platform's own clients use exactly this form" — a claim the
   repair pass removed from the AMM README but not from the fixture. Replaced with the
   actual citation (`normalizeSovereignPoolPair`, `central_bank_pool_client.go`).
5. Every gate in the table above was then re-run on the edited tree. All still green.

### What was NOT verified, and why

* **Nothing has been run against a real gateway.** All 54 passing tests ran against Prism,
  which serves our own contracts back to us. That proves internal consistency — contract,
  mocks, vectors and tests agree — and it proves nothing about the live estate. The 37
  `live_only` tests (state transitions, balance deltas, residue reconciliation, timelock
  ordering) **have never executed anywhere.** This is the single largest gap in the
  evidence and it is the subject of §7 / guidelines §4.2.
* **The `drift-check` CI job has never run.** It is `workflow_dispatch`-only and requires a
  gateway URL. Its logic was read and reviewed but not executed.
* **The Toolbox CI workflow has never run in GitHub Actions on this branch**, because
  nothing is committed. Every job was replayed by hand locally, including the exact `curl`
  probes, but a green local replay is not a green Actions run.
* **The 51 `implementation-observed` Scenario B schemas were not exhaustively re-derived.**
  A sample was traced to source (checks 14–15). The remainder rest on the contract-authoring
  and repair passes' work.
* **No live PKI handshake was exercised.** `pki` auth mode is implemented against the
  documented flow; it requires a real participant P-256 key and certificate.

---

## 5. Known gaps

### 5.1 Things the platform does not express, so the contracts cannot either

Recorded explicitly in the relevant README rather than filled with a plausible guess:

* **No decimals count for tCeBM/fCeBM anywhere.** `/api/v1` payment amounts are 7-digit
  strings with no unit statement; `/api/v2` amounts are explicitly 18-decimal base units. A
  client moving a figure from a deposit into a swap cannot do so safely. Compounding trap:
  a transfer limit's `max_amount` is a human decimal on input and wei on output, under the
  same field name.
* **No idempotency mechanism at all** — no `Idempotency-Key`, no `If-Match`/ETag, no
  operation-level header parameters. Replay safety is per-endpoint and payload-keyed. A
  generic retry wrapper is not part of this API.
* **No machine-readable error code on `/api/v1`.** The shape is `{error: "<free-form
  string>"}`, sometimes a human sentence and sometimes a code, in the same field.
  Conformance therefore asserts on **status code only**, never on error text.
* **No `EXPIRED` FX state**, despite an `expiry_date` on every agreement and a documented
  background expiration worker. Expiry has no representable terminal state.
* **Two unreconciled HTLC state vocabularies** — `HTLCLock.state` is prefixed
  (`HTLC_STATE_LOCKED`), the `/htlc/search` `state` query parameter is bare (`LOCKED`). Both
  are mirrored verbatim; neither is normalised.
* **Five incompatible pool-pair spellings**, none marked canonical. `pair_id` must be
  treated as an opaque string read from `GET /api/v2/amm/pairs` and never constructed.
* **Five Scenario A routes are wired by the router but appear in no OpenAPI document** —
  `/api/v1/statement`, `/api/v1/identities/*`, `/internal/v1/identities/*`,
  `POST /internal/v1/payments/pvp-legs`, `GET /internal/v1/payments/pvp-credits`. Recorded
  as a gap; nothing is modelled on them.
* **No write path for relay-delivered FX state changes.** Scenario A's `Internal` tag claims
  the relay delivers them idempotently, but the only registered internal FX route is a GET.
  HTLC has no relay endpoints whatsoever; cross-spoke secret propagation is prose with no
  HTTP endpoint.
* **Undiscoverable deployment behaviour.** `FX_RATE_TOLERANCE_PCT` and
  `FX_AGREEMENT_HTLC_STRICT` change server behaviour, and Pente on/off materially changes FX
  behaviour, with no capability endpoint to distinguish deployments.
* **The commercial-bank gateway is a proxy, and that is invisible in the paths.** The same
  path is served locally on a central-bank gateway and proxied on a commercial-bank gateway,
  where the proxy injects the requester's address, CSR, key and PoP signature. What the
  client must send differs; the path does not.

### 5.2 Scenario B coverage

39 of 99 Scenario B paths are unpublished — governance, compliance, onboarding, audit and
fourteen `/internal/*` routes, five of which are the AMM's own cross-currency relay hops.
More materially, **51 of 95 Scenario B schemas are reconstructed from Go source** because
the platform declares them as free-form objects (§2.6). Circuit-breaker and registry
*mutations* are contract-tested but deliberately **not** exercised live, because triggering
them halts a live corridor.

### 5.3 What needs the real gateway, not Prism

The 37 `live_only` tests, all unexecuted: the reserve prelude producing a non-zero balance;
FX propose → accept → settle as a state machine; asymmetric HTLC timelock ordering
(safety-critical, currently enforced only by prose in review); secret reveal and refund
after expiry; AMM quote TTL (15 seconds — quote and swap must be issued programmatically in
one step); swap polled to `COMPLETED`; residue returned and reconciled; bridge position
reaching `BURNED`/`RELEASED`; `hub-reconciliation` reporting `balanced == true`. Also
unverifiable against a mock: role-based 403s (needs a second credential set), and the real
`bridge_state` vocabulary, which lives only in Go domain code — the platform published
`PENDING`/`CLOSED`, neither of which exists.

### 5.4 Two smaller carried-forward items

* `Toolbox/schemas/mock.schema.json` still requires `body` on a response. The plan called
  for dropping it so bodiless responses become expressible. **Not blocking** — all 59 mocks
  validate as-is — but it will bite the first person who mocks a 204.
* ~~The guessed service ports in `.env.example` were flagged for verification against the
  platform compose topology.~~ **Verified 2026-08-21 and correct**: `18080` (bank-a) and
  `38080` (central-bank-a) match the `"18080:8080"` / `"38080:8080"` mappings in the
  platform compose files and the `servers` block of the gateway spec. `4010`–`4012` are our
  own Prism mock ports. No change needed.

---

## 6. What you must decide or do

### 6.1 Commit the work — nothing is committed

This is the only item that blocks everything else. The tree is fully staged: **94 additions,
34 modifications, 9 deletions; 0 files unstaged; 0 untracked under `Toolbox/`.** CI checks
out the commit, not the working tree, so **as of this moment the CI gates would fail on
their own guard clauses** ("required gate is missing"), because `Toolbox/tools/` does not
exist in any commit.

```
git commit -m "Realign Toolbox with delivered API Gateway v2.3.0"
```

Per `lfdt_ai_disclosure_commits`: this is an LFDT lab, so use an `Assisted-by:` trailer, not
`Co-Authored-By: Claude`.

### 6.2 Ratify the versioning decision

The migration guidelines proposed `v0.2.0`; we shipped `v2.3.0` tracking the gateway
(§2.3). It is a better scheme, but it is *your* decision to record, because it is the one
place this work knowingly departs from the written guidelines.

### 6.3 Delete six stray Python wheels from the repository root

`attrs-*.whl`, `jsonschema-*.whl`, `jsonschema_specifications-*.whl`, `referencing-*.whl`,
`rpds_py-*.whl`, `typing_extensions-*.whl`. They are untracked, **not** covered by
`.gitignore`, and a `git add -A .` at the root would sweep them into the PR. I left them
alone rather than delete files I did not create.

```
rm /home/kornatis/cbweb3/*.whl
```

### 6.4 Decide whether the CSRF finding is disclosed or remediated

The platform has no CSRF protection (§2.5). It is currently written up as an open finding
in the security checklist. That is the honest position, but it will be read by DPGA
reviewers and possibly by central-bank security teams. You need to decide whether it stays
a disclosed finding, becomes a vendor change request (which breaks the zero-burden
constraint), or is mitigated at the deployment layer.

### 6.5 Decide how the 23 divergences are communicated to the vendor

Nothing was sent. Five are blocking — a client written strictly against GL/AH's own
published OpenAPI document cannot operate the FX state machine, cannot read HTLC status,
and cannot operate the Scenario B circuit breaker (which declares no request body at all
while requiring one). This is genuinely useful information for them and costs them nothing
to receive. But sending it *is* a form of burden, and it touches the vendor negotiation.
Options: attach `DIVERGENCES.md` as an informational annex to the next D-review; raise it as
a documentation defect against the delivered OpenAPI artifacts; or hold it entirely.
**Recommendation: send it as an annex, framed as "we have already absorbed all of this on
our side; here is what we learned, for your documentation backlog."** It preserves the
zero-burden posture while giving you a concrete, source-cited quality finding on record.

### 6.6 Consider a count gate

Published artifact counts drifted twice — once before this work (16 vs 17 test methods),
once *during* it (90 vs 91). A ten-line CI check that re-derives the counts and diffs them
against the READMEs would close it permanently. Not written; flagging it as a small,
high-value follow-up.

### 6.7 Open the PR, and decide who reviews it

137 files. The reviewable core is the three contracts plus `DIVERGENCES.md`; everything else
follows from them. Suggest the review focus there and treat the fixtures as generated
output.

---

## 7. Consequences for the monorepo import

Against §4.1 of `localdocs/monorepo-migration-guidelines.md` — "API contract reconciliation
— highest priority", described there as "the largest single item in this document".

### Closed by this work

| §4.1 requirement | Status | Evidence |
|---|---|---|
| **1.** A written diff per endpoint — paths, schemas, error codes, auth — covering **both** scenarios | **Closed** | `Toolbox/DIVERGENCES.md` (23 source-traced rows) + 60 `Spec divergence:` notes inside the three contracts + per-contract READMEs and CHANGELOGs. Both scenarios covered, not just A |
| **2.** A decision per divergence; platform surface wins; Toolbox re-cut with mocks, vectors and conformance tests updated | **Closed** | Every divergence resolved in the platform's favour. 0 invented paths. Mocks (59), vectors (140) and tests (91) all rebuilt. Vendor burden: zero. *Version number deviates — see §6.2* |
| **4.** Clarify how one contract covers two scenarios | **Closed** | Answered explicitly: it cannot. Three contracts, split by scenario, with the byte-identical auth surface published once (§2.2) |
| **§4.4** Publish `contracts/amm/` if the AMM surface is stable | **Closed** | `contracts/amm/openapi_amm_v2.3.0.yaml` — 52 paths / 59 operations, published on the same terms as PvP |

The premise of §4.1 is also now obsolete in a good way. It states the community builds
against `openapi_pvp_v0.1.0.yaml` "backed by 8 reference mocks, 13 test vectors and 16
executable conformance tests" and that the two contracts "do not match on a single path".
That file is deleted; the numbers are now 59 / 140 / 91; and every published path exists in
a delivered gateway. **§4.1 should be rewritten in the guidelines to reflect this before the
document goes to GL/AH again** — as written, it hands them an out-of-date accusation.

### Remaining open

| Item | Status | What is needed |
|---|---|---|
| **§4.1 item 3** — one normative contract, *continuously validated* against the gateway so the two cannot drift again | **Partially open** | The mechanism exists — a `drift-check` job that fetches each gateway's self-served `GET /openapi.yaml` (confirmed wired at `scenario-a/router.go:38`, `scenario-b/router.go:42`) and diffs the path sets. But it is `workflow_dispatch`-only, needs a reachable gateway URL, and **has never run**. Making it scheduled or PR-blocking requires a staging gateway CI can reach — a decision for you and LNet infrastructure |
| **§4.2** — conformance tests run against the real platform in CI | **Fully open** | Unchanged by this work. The suite is *ready* for it: 37 `live_only` tests exist, four auth modes including PKI are implemented, and a `CBWEB3_PROFILE` fixture skips rather than fails on route groups a deployment never registered. What is missing is the environment. The guidelines call this "the single strongest piece of evidence available for the DPGA criterion on open standards and best practices", and I agree — but today it is capability, not evidence |
| **§4.4** — `contracts/compliance/` | **Open by decision** | The directory no longer exists. Compliance is outside the integrator scope boundary (§2.4). If the DPG submission wants it, it is a new piece of work, not a gap in this one |
| **§4.3** — user manuals reach the published documentation site | **Untouched** | Out of scope here; belongs to the import itself |

### Is the repository ready to receive the import?

**Yes, for the contract-reconciliation blocker specifically.** The Toolbox now describes the
system that is actually going to land under `platform/`, so importing the vendor tree will
not create a contradiction between `Toolbox/contracts/` and `platform/*/api-gateway/docs/`.
Three practical properties were verified for the import:

* `Toolbox/` writes nothing outside itself, and nothing under `Toolbox/` or `.github/`
  references the platform clone by absolute path — checked, clean. Guidelines §2 rule 1
  holds.
* `.github/workflows/toolbox-ci.yml` is path-filtered to `Toolbox/**`, so the import will
  not trigger it and the platform's own workflows will not trigger these gates.
* The `drift-check` job is written to read `Toolbox/contracts/*/openapi_*.yaml` by glob, so
  it will keep working once the gateway specs live in-repo — and at that point it can diff
  against the local file instead of an HTTP endpoint, which removes the "needs a reachable
  gateway" obstacle to §4.1 item 3 entirely. **That is the cheapest path to closing item 3
  and it becomes available the moment the import lands.**

**The blockers to the import that this work does not touch** remain as recorded in
guidelines §1: DCO sign-offs (§1.3, confirmed blocker — 0 sign-offs across the vendor
history), and the AI-authorship decision (§1.4 — 1,059 `Co-Authored-By` AI trailers that
become public the moment the history does). Neither is affected by anything here.

---

## 8. Candid summary

The contract layer is solid and I would defend it: every path is real, every divergence is
source-cited, and the two new gates make the original failure mode unrepeatable. The
fixture layer is internally consistent and machine-checked end to end.

What I would not claim in front of a DPGA reviewer without qualification:

1. **"Verified against the platform" means verified against the platform's *specification
   and source code*, not against a running system.** Not one request in this work reached a
   live gateway. Everything green is green against a mock generated from our own contracts.
2. **More than half of the Scenario B contract is reverse-engineered.** 51 of 95 schemas
   are `implementation-observed` because the vendor's document declares them as free-form
   objects. It is annotated and auditable, and it is still not the vendor's normative
   artifact.
3. **The published counts drifted during the repair pass itself.** I caught it and fixed it,
   but it says something about how easily this kind of document rots without a gate.

The right next step, in order: commit (§6.1), delete the wheels (§6.3), open the PR, and
then get a staging gateway URL so the 37 `live_only` tests can run for the first time.
Until that happens, the Toolbox is *correct by construction* but not *confirmed by
observation*, and those are different claims.
