# Scenario B — Hub-and-Spoke Settlement — Test Vectors

**76 vectors in 5 files** against
[`../../contracts/amm/openapi_amm_v2.3.0.yaml`](../../contracts/amm/openapi_amm_v2.3.0.yaml)
(52 paths / 59 operations, 12 of them public).

Scenario B settles through an **International Hub**: value is bridged from a spoke to the Hub
as a wrapped `W-tCeBM_<CUR>` token, swapped on an AMM pool, bridged out on the beneficiary's
spoke, and the unspent slippage buffer is returned as a separate residue leg.

> **Scenario B has no HTLC and no FX agreement.** Neither exists in the delivered gateway. The
> eight orphan `LockHTLC*` / `HTLCLock` schemas in the Scenario B platform document are
> unreferenced copy-paste residue from Scenario A — a grep for `htlc` across the entire
> Scenario B Go tree returns one stale comment and no handler, no route, no service. They are
> deliberately not published and nothing here exercises them.

| File | Covers | hp | err | edge | total |
|---|---|---|---|---|---|
| `amm_reserves_vectors.json` | Scenario B reserve lifecycle — **different paths from Scenario A** | 7 | 3 | 1 | **11** |
| `amm_registry_vectors.json` | credential-free discovery, hub currency and pair registries | 8 | 6 | 2 | **16** |
| `amm_swap_vectors.json` | quotes, cross-currency swap, settlement and residue verification | 6 | 9 | 2 | **17** |
| `amm_bridge_vectors.json` | lock-mint, burn-unlock, liquidity commits, hub reconciliation | 8 | 6 | 1 | **15** |
| `amm_governance_vectors.json` | circuit breaker, transfer limits, oversight disclosure, sovereign add | 10 | 6 | 1 | **17** |

`amm_governance_vectors.json` is an addition beyond the realignment plan's manifest: the
governance, oversight and sovereign-liquidity surface carries the platform's cleanest
malformed-amount and quorum assertions and did not belong inside the registry file.

## Run them in this order

1. **Discovery, no credentials** — `amm_registry_vectors.json` `registry-hp-01..05`.
   `hub-config` → `hub/currencies` → `amm/pairs` → `pool/{pair}/status` is `ACTIVE` →
   `circuit-breaker/status` is not halted.
2. **Reserve prelude, mandatory** — `amm_reserves_vectors.json`. Without spoke tCeBM there is
   nothing to bridge in and no payment can be made.
3. **Corridor provisioning, Central Banks** — `registry-hp-06..08` then
   `amm_bridge_vectors.json` `bridge-hp-01`, `bridge-hp-06`, `bridge-hp-07`, `bridge-hp-08`.
   Register currencies → propose (CB of token A) → confirm (CB of token B) → lock-mint →
   mint-and-approve → commit liquidity per side → poll until `EXECUTED`.
4. **Payment** — `amm_swap_vectors.json`. Quote (**15-second TTL**) → swap → poll to `COMPLETED`.
5. **Settlement verification** — `swap-hp-04` and `bridge-hp-05`.

**The quote lives 15 seconds.** Issue the quote and the swap programmatically in one step; a
fixture that pauses between them will flake. `quote_id` is optional, and omitting it skips quote
validation entirely.

## Settlement verification is the point

`swap-hp-04` is what makes this a settlement suite rather than an API suite. Asserting
`status == "COMPLETED"` is **not** conformance. Five things must hold:

1. The payer's balance falls by **`amount_in`, not `max_amount_in`**.
2. `residue_amount` equals `max_amount_in − amount_in`.
3. `residue_status` is `RETURNED`.
4. The bridge-out position reaches `BURNED` or `RELEASED`.
5. `GET /api/v2/amm/hub-reconciliation` reports `balanced: true` and `unexplained: "0"`.

**The residue mechanic must be taught, not glossed.** A cross-currency swap bridges in the
**full `max_amount_in`**, because the realised input is unknown until execution. The unspent
remainder is burned on the Hub and returned to the payer on its source spoke as a separate
bridge position with `leg=RESIDUE`. **`residue_status = RETURN_FAILED` means the payer has not
been made whole** — `swap-edge-02` covers a `COMPLETED` swap that still leaves the payer short.
The sibling case is the distinct `BRIDGE_OUT_FAILED` code on a `500`: the Hub swap **succeeded**
and the beneficiary delivery did not, so it is never safe to blind-retry.

## Blocking spec-vs-implementation defects

The Toolbox mirrors the **implementation**, because the platform surface wins. Every deviation
is footnoted in the vector that exercises it.

| Ref | Operation | Document says | Handler does | Vector |
|---|---|---|---|---|
| **QUOTE-XC** | `GET /api/v2/amm/quote/cross-currency` | **zero** parameters | requires `source_currency`, `target_currency`, `amount_out` | `swap-err-01` |
| **QUOTE-EO** | `GET /api/v2/amm/quote/exact-output` | `amountOut` | reads only `amount_out` | `swap-err-02` |
| **ENVELOPE** | `hub/currencies`, `amm/pairs`, `bridge/positions` | bare JSON array | wrapped `{currencies:…}`, `{pairs:…}`, `{positions:…}` | `registry-hp-02/03`, `bridge-hp-02` |
| **POOL-422** | `GET /api/v2/amm/pool/{pair}/status` | `404` for an unavailable pool | `422 POOL_STATUS_UNAVAILABLE` | `registry-err-06` |
| **CURRENCY-201** | `POST /api/v2/hub/currencies` | `200`, no schema | `201` with the service result | `registry-hp-06` |
| **BREAKER-BODY** | `pause`, `resume-request`, `resume-sign` | **no `requestBody` at all** | all three require one | `breaker-hp-01/02/03`, `breaker-err-01` |
| **BREAKER-PAIR** | `GET .../circuit-breaker/status` | no `pair` parameter | undeclared `pair`, defaulting to the literal `BRL-USD` | `registry-edge-01` |
| **BRIDGE-STATE** | `bridge/positions` `state` example | `PENDING, ACTIVE, CLOSED` | `PENDING` and `CLOSED` **do not exist** | `bridge-hp-02/03` |

`swap-err-01` and `swap-err-02` are deliberately runnable against both a live gateway and a
Prism mock served from the upstream document, because the two **disagree** — the mock returns
`200`, the gateway returns `400`. The disagreement is the finding.

## Traps with settlement consequences

**Never construct a pool-pair identifier.** Five incompatible spellings appear across the
delivered artefacts — `tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`, `W-BRL-ARS`, the
quote-style `W-BRL-W-ARS` and the circuit breaker's `BRL-USD` fallback — and none is documented
as canonical. All five are enumerated with source citations in
[`../../DIVERGENCES.md`](../../DIVERGENCES.md). Take `pair_id` verbatim from
`GET /api/v2/amm/pairs`.

**Always send `pair` to the circuit-breaker status endpoint.** Omitting it reports on the
literal `BRL-USD`, so a client can read a healthy `LIVE` for a pair nobody trades while the pair
it *is* trading is halted, and then be surprised by a `422 CIRCUIT_BREAKER_HALTED` on the swap
(`registry-edge-01`).

**Always send `is_token_a` to `sovereign-add`.** Omitting it does not mean "unspecified", it
means **side B** (`sovereign-hp-01`).

**Transfer limits change unit under the same field name.** `max_amount` is a human decimal on
input (`"100000.00"`) and **wei** on output (`"100000000000000000000000"`). A client that reads
a limit and writes it back unchanged shrinks it by 10^18 (`limits-hp-01`).

**A `200` on `resume-sign` does not mean trading resumed.** Branch on `state`: `LIVE` means the
resume executed, `RESUME_PENDING` on the same `200` means it did not (`breaker-edge-01`).

**Bearer does not work on the swap routes.** Twelve `/api/v2` routes accept cookie *or* bearer
(bridge, liquidity, sovereign-add, token approvals, hub reconciliation — see `bridge-hp-03`).
The two cross-currency swap endpoints are **cookie-only** (`swap-err-04`).

**The role in a JWT is lowercase.** `central_bank`, `commercial_bank`, `mlp`.
`ROLE_CENTRAL_BANK_SCENARIO_B`, which the platform document writes in prose, is a Go constant
name and will never appear in a token.

## Three error models on one platform

| Family | Shape |
|---|---|
| `/api/v1` (reserves, balances) | `{error}` — **no machine-readable code at all** |
| `/api/v2` AMM, swap, bridge-limit | `{error, error_code}`, sometimes `+ details`, `+ recommended_action` |
| `/api/v2` registry (currencies, pairs) | `{error, code}` — different key name |

Some operations mix them: `bridge/lock-mint` returns `{error, error_code}` on its `422` while
its sibling `bridge/burn-unlock` returns a bare `{error}` on its own `422`
(`bridge-err-02` vs `bridge-err-03`). **Assert the status code.**

## Gaps recorded, not filled

* **Much of `/api/v2` is deliberately unspecified upstream** — fourteen endpoints declare
  free-form `additionalProperties` objects, including `swap/cross-currency` and
  `bridge/lock-mint`, and the platform says so in a comment. The contract fills these from
  handler source under `x-cbweb3-source: implementation-observed`, and the vectors assert those
  recovered shapes. This is a conscious, annotated deviation from the normative artefact and
  must be stated as a known limitation in DPG evidence, not presented as the platform's own
  contract.
* **No `QUOTE_EXPIRED` code.** An expired `quote_id` falls through the handler's string-matching
  classifier to a generic `500 INTERNAL_ERROR`, and the swap record is created and marked
  `FAILED` first — so a `500` there is **not** a no-op (`swap-edge-01`).
* **`INSUFFICIENT_POOL_LIQUIDITY` is hard-coded on the quote failure branch** regardless of the
  real cause, so it can mean unknown currency, no corridor, empty pool or a genuine shortfall
  (`swap-err-03`). Verify the corridor exists before quoting.
* **Bridge position state has no published enum.** The real vocabulary (`LOCKING`, `ACTIVE`,
  `BURNING`, `BURNED`, `RELEASED`, `RECONCILIATION_REQUIRED`) lives only in `internal/domain`,
  so the contract publishes it in prose rather than as an enum the platform has not committed to.
  `pool_status` is the one recovered vocabulary published as a real enum, because the gateway
  derives it exhaustively from two reserves and gates swaps on it.
* **No unit bridge between `/api/v1` and `/api/v2` amounts.** `/api/v1` amounts are opaque
  strings with no stated scale; `/api/v2` amounts are explicitly 18-decimal base units. The only
  `decimals` field on the entire CBWeb3 surface is on `GET /api/v2/hub/token/supply`, and it
  describes a Hub W-token, not spoke tCeBM (`registry-edge-02`). Do not generalise it.
* **`liquidity/commit` declares no `404` and no `422`**, so an unknown `pool_pair` has no
  documented status (`bridge-err-05` covers only the `400`).
* **The commercial-bank gateway is a proxy, and that is invisible in the paths.** The same path
  is served locally on a Central Bank gateway and proxied on a commercial-bank gateway, where
  the proxy injects the requester address, CSR, key and proof-of-possession. What a client must
  send differs; the `preconditions` block of each vector names which side it addresses.
* **Deployment-shape signals are skips, not failures.** `503` on `hub-reconciliation` and on
  `token/fiat-balance`, `501` on swap history and on `sovereign-add`, and a `404` from a route
  group the deployment never registered all mean "not this kind of node".
