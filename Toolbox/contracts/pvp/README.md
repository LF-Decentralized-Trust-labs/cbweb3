# PvP Settlement — Interface Contract (Scenario A)

| | |
|---|---|
| **File** | `openapi_pvp_v2.3.0.yaml` |
| **Scenario** | **A** — single-ledger, spoke-to-spoke |
| **Platform version** | CBWeb3 API Gateway **v2.3.0** |
| **Operations** | 32 across 28 paths (5 Token + 15 Escrow + 8 FX Agreement + 6 HTLC, counting `POST` and `GET` on a shared path separately) |
| **Depends on** | [`../auth/openapi_auth_v2.3.0.yaml`](../auth/README.md) for session bootstrap |
| **Status** | Normative. Derived from the delivered platform gateway. |

## What this contract covers

Payment-versus-Payment settlement: a cross-currency payment executed as an **atomic swap
of two tokenised central bank money (tCeBM) legs**, coordinated by a bilateral FX
agreement and carried out by a pair of dual-layer Hash Time-Locked Contracts —
**plus everything a bank needs in order to hold the tCeBM those HTLCs lock.**

| Surface | Paths | Why it is here |
|---|---|---|
| Token — `/api/v1/token/*` | 5 | The balances an HTLC lock moves; the probes a settlement test asserts on. |
| Reserve lifecycle — `/api/v1/payments/{deposits,escrows,redeems}*` | 9 paths / 15 operations | How a bank **obtains** tCeBM. Hard precondition, not an optional privacy step. |
| FX agreement — `/api/v1/payments/fx/agreements*` | 7 | Agreeing the terms of the swap. |
| HTLC — `/api/v1/htlc/*` | 6 | Executing the swap atomically. |

### Why the reserve lifecycle is in a "PvP" contract

`POST /api/v1/htlc/lock` locks tCeBM the calling bank **must already hold**, and the only
way to obtain tCeBM is deposit → approve → fiat-exchange → escrow → approve. A conformance
suite that cannot run those steps can assert response shapes against a mock and nothing
more — which is exactly the deficiency this contract set exists to remove.

**Token** — `/api/v1/token/*`

| Operation | Method + path |
|---|---|
| `mintToken` | `POST /api/v1/token/mint` |
| `burnToken` | `POST /api/v1/token/burn` |
| `transferToken` | `POST /api/v1/token/transfer` |
| `getTokenBalance` | `GET /api/v1/token/balance` |
| `getFiatBalance` | `GET /api/v1/token/fiat-balance` |

**Reserve lifecycle (Escrow)** — `/api/v1/payments/*`

| Operation | Method + path |
|---|---|
| `registerDeposit` | `POST /api/v1/payments/deposits` |
| `listDeposits` | `GET /api/v1/payments/deposits` |
| `approveDeposit` | `POST /api/v1/payments/deposits/approve` |
| `rejectDeposit` | `POST /api/v1/payments/deposits/reject` |
| `requestFiatExchange` | `POST /api/v1/payments/deposits/fiat-exchange` |
| `requestEscrow` | `POST /api/v1/payments/escrows` |
| `listEscrows` | `GET /api/v1/payments/escrows` |
| `approveEscrow` | `POST /api/v1/payments/escrows/approve` |
| `rejectEscrow` | `POST /api/v1/payments/escrows/reject` |
| `requestRedeem` | `POST /api/v1/payments/redeems` |
| `listRedeems` | `GET /api/v1/payments/redeems` |
| `approveRedeem` | `POST /api/v1/payments/redeems/approve` |
| `rejectRedeem` | `POST /api/v1/payments/redeems/reject` |

**FX Agreements** — `/api/v1/payments/fx/agreements*`

| Operation | Method + path |
|---|---|
| `proposeFXAgreement` | `POST /api/v1/payments/fx/agreements` |
| `listFXAgreements` | `GET /api/v1/payments/fx/agreements` |
| `getFXAgreement` | `GET /api/v1/payments/fx/agreements/{tradeId}` |
| `acceptFXAgreement` | `POST /api/v1/payments/fx/agreements/{tradeId}/accept` |
| `rejectFXAgreement` | `POST /api/v1/payments/fx/agreements/{tradeId}/reject` |
| `cancelFXAgreement` | `POST /api/v1/payments/fx/agreements/{tradeId}/cancel` |
| `settleFXAgreement` | `POST /api/v1/payments/fx/agreements/{tradeId}/settle` |
| `listFXAgreementEvents` | `GET /api/v1/payments/fx/agreements/{tradeId}/audit` |

**HTLC** — `/api/v1/htlc/*`

| Operation | Method + path |
|---|---|
| `lockHTLC` | `POST /api/v1/htlc/lock` |
| `lockHTLCWithHash` | `POST /api/v1/htlc/lock-with-hash` |
| `settleHTLC` | `POST /api/v1/htlc/settle` |
| `refundHTLC` | `POST /api/v1/htlc/refund` |
| `getHTLCStatus` | `GET /api/v1/htlc/status/{contractId}` |
| `searchHTLC` | `GET /api/v1/htlc/search` |

`operationId` values match the platform's own operation IDs exactly.

## The flow

**Step 0 — reserve prelude, on each side, before any settlement is possible:**

```
Bank (commercial gateway)                       Central Bank gateway (ROLE_TREASURY)
        |                                                    |
        |── POST /payments/deposits ────────────────────────>|   PENDING
        |<── POST /payments/deposits/approve ────────────────|   APPROVED
        |<── POST /payments/deposits/fiat-exchange ──────────|   fCeBM minted on Besu
        |     GET /token/fiat-balance  > 0                   |
        |── POST /payments/escrows ─────────────────────────>|   PENDING  (Reserve Tokenisation)
        |<── POST /payments/escrows/approve ─────────────────|   fCeBM burned, tCeBM minted
        |     GET /token/balance  > 0                        |
```

**Steps 1–7 — the atomic swap:**

```
Bank-A (Spoke-A, initiator)                     Bank-D (Spoke-B, responder)
        |                                                    |
        |── POST /api/v1/payments/fx/agreements ────────────>|   FX_STATE_PROPOSED
        |<── POST /api/v1/payments/fx/agreements/{id}/accept |   FX_STATE_ACCEPTED
        |                                                    |
        |── POST /api/v1/htlc/lock  (secret server-side) ────|   timelock now+3600s
        |        returns hash_lock                           |
        |·········· hash_lock travels OUT-OF-BAND ··········>|   (no API endpoint)
        |                                                    |
        |<── POST /api/v1/htlc/lock-with-hash ───────────────|   timelock now+1800s
        |                                                    |
        |── POST /api/v1/htlc/settle  (reveals the secret) ──|
        |·········· Cacti relay broadcasts the secret ······>|   Spoke-B settles
        |                                                    |
        |                    SETTLED                         |
```

**Timeout path:** if the secret is never revealed, each side calls
`POST /api/v1/htlc/refund` after **its own** timelock expires.

Every path in this diagram is written in full. There is no unprefixed `/htlc/...` or
`/fx/...` surface on any CBWeb3 gateway; that shape belonged to the withdrawn
`openapi_pvp_v0.1.0.yaml` and describes an API that never existed.

**Unwind (optional):** `POST /api/v1/token/transfer` the tCeBM to the Central Bank, then
`POST /api/v1/payments/redeems` quoting that transfer's `tx_hash`, then the Central Bank
calls `POST /api/v1/payments/redeems/approve` to mint fCeBM back.

### The timelock ordering rule is safety-critical

The initiator's timelock **must** be longer than the responder's (defaults: 1 hour versus
30 minutes). The platform expresses this in prose only — nothing in any schema enforces
it. **A test vector or tutorial that inverts the ordering models an unsafe swap** in which
the initiator can claim the responder's leg after the responder has lost the ability to
refund.

## No single gateway serves the whole contract

Route registration is conditional on how a gateway is wired, and the reserve lifecycle in
particular is split across two gateway roles **asymmetrically**:

| Operation group | Commercial-bank gateway | Central Bank gateway |
|---|---|---|
| `POST` deposits / escrows / redeems | **served** (proxies to the CB) | **404** — the CB receives creations over its relay-internal path |
| `GET` deposits / escrows / redeems | served | served |
| `approve` / `reject` / `fiat-exchange` | **404** | served, `ROLE_TREASURY` |
| `POST /api/v1/token/burn` | **404** | served, `ROLE_TREASURY` |
| `POST /api/v1/token/mint` | served, **no role gate** | served, `ROLE_TREASURY` |
| HTLC, FX, `token/transfer`, both balances | served | served |

So **no single base URL can drive the whole reserve lifecycle**: a test that creates a
deposit and approves it needs two sessions against two hosts. Everything above additionally
requires the payment orchestrator to be wired (`PaymentHandler != nil`); when it is not,
routes are never registered and a client gets **404**, not 501. A 404 during a conformance
run may therefore mean *"this gateway is not that kind of node"* rather than *"the
implementation is non-conformant"*. Declare a gateway profile before asserting.

## Why the reserve lifecycle is duplicated in `../amm/` instead of shared

The two scenarios' reserve blocks look structurally identical and are not. Sharing one
document would have required declaring endpoints and fields that no single gateway serves.
The real divergences:

| | Scenario A (this contract) | Scenario B (`../amm/`) |
|---|---|---|
| Fiat exchange path | `POST /payments/deposits/`**`fiat-exchange`** | `POST /payments/deposits/`**`exchange`** |
| Deposit / escrow / redeem requests | carry `requester_paladin_identity` | do not |
| Create responses | carry `status` alongside the id | id only |
| Escrow request | no `deposit_id` | carries `deposit_id` |
| Redeem request | declares `zeto_transfer_tx_hash` — but the proxy injects it; the client sends `amount` only | field does not exist |
| Fiat-exchange response field | `tx_hash` | `mint_tx_hash` |
| Escrow-approval Paladin hash | `zeto_mint_tx_hash` | `mint_tx_hash` |
| Redeem-approval response field | `fiat_mint_tx_hash` | `mint_tx_hash` |
| Escrow / deposit record identity fields | `requester_besu_address` + `requester_paladin_identity` | `requester_id` + `besu_address` / `requester_besu_address` |
| Escrow record hash field | `zeto_mint_tx_hash` | `mint_tx_hash`, plus a `deposit_id` this side does not carry |

Duplication per scenario is the honest choice. It is deliberate.

While comparing the two, note that **Scenario A's own wording is the correct one**:
`fCeBM → tCeBM` for the escrow, `tCeBM → fCeBM` for the redeem. Scenario B's summaries
read "tCeBM → tCeBM" on both, the result of a bad `fCeBM` → `tCeBM` find-replace upstream.
When the Toolbox writes **fCeBM (ERC-20 on Besu) ↔ tCeBM (Zeto privacy token on Paladin)**
it is quoting this document, not correcting it.

## Using it with Prism

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010

curl -s -X POST http://127.0.0.1:4010/api/v1/payments/escrows \
  -H 'Content-Type: application/json' \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' \
  -d '{"amount":"5000000"}'

curl -s -X POST http://127.0.0.1:4010/api/v1/payments/fx/agreements \
  -H 'Content-Type: application/json' \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' \
  -d '{"counterparty_b":"funded_operator@spoke-b-bank-d",
       "origin_amount":"1000000","counter_amount":"850000",
       "origin_currency":"BRL","counter_currency":"ARS",
       "rate":"0.85","expiry_date":1711590000}'

curl -s -X POST http://127.0.0.1:4010/api/v1/htlc/lock \
  -H 'Content-Type: application/json' \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' \
  -d '{"receiver":"funded_operator@spoke-a-bank-c","amount":"1000000"}'

curl -s 'http://127.0.0.1:4010/api/v1/htlc/status/htlc-a1b2c3d4' \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI'
```

Paths carry their full `/api/v1` prefix and `servers:` entries are bare origins, so Prism
serves exactly the paths a real gateway does — nothing is stripped or re-prefixed.

Prism validates only the **presence** of the `access_token` cookie, never its value, so
any placeholder works against the mock. A request without the cookie gets 401 — a useful
negative assertion. Prism is also stateless: it cannot persist a created agreement, so
"create then get then accept" sequences are meaningless against it. Assert status and
shape only in mock mode.

Add `--errors` to make Prism validate requests against the contract instead of returning a
best-effort example.

Validate before committing changes:

```bash
npx @stoplight/spectral-cli lint --ruleset .spectral.yml --fail-severity=error \
  Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml
```

## Traps this contract reproduces on purpose

The platform surface is inconsistent in several places. Those inconsistencies are
**mirrored faithfully rather than harmonised**, because a client must match the real
server, not a tidier idea of it.

* **`agreement_id` carries a `trade_id`.** The HTLC's link to an FX agreement is named
  `agreement_id` on `lockHTLC`, `lockHTLCWithHash` and the `searchHTLC` filter — but the
  value is an FX `trade_id`. Cross-domain rename; real trap.
* **Two HTLC state vocabularies that do not match.** `HTLCLock.state` uses
  `HTLC_STATE_LOCKED | HTLC_STATE_SETTLED | HTLC_STATE_REFUNDED` (plus `_INVALID` and
  `_PENDING`); the `state` **query parameter** of `searchHTLC` uses bare
  `LOCKED | SETTLED | REFUNDED`. The platform never reconciles them, so **a client cannot
  tell from the specification alone which to send to the filter.** Both are published as
  written. **Open question — resolve against a live gateway before relying on the filter.**
* **camelCase paths, snake_case bodies.** `{tradeId}` / `{contractId}` in paths;
  `trade_id` / `contract_id` in JSON.
* **Amounts and rates are decimal strings** (`"1000000"`, `"0.85"`), never numbers.
  Timestamps are `int64` **Unix seconds** on chain-adjacent fields (`time_lock`,
  `expiry_date`, `occurred_at_unix`) and RFC 3339 strings on database metadata
  (`created_at`). Both conventions appear in this one contract.
* **No decimals count is stated anywhere** for tCeBM or fCeBM. 18 is implied by the word
  "wei" elsewhere on the platform but never asserted for these tokens. Compare balance
  deltas; **do not assume a scale factor** in fixtures.
* **Creation returns 201 — and so do several things that create nothing.**
  `approveDeposit`, `requestFiatExchange`, `approveEscrow` and `approveRedeem` all answer
  **201**, while the three `reject` operations answer 200. Mirrored, not smoothed.
* **Response-code asymmetry across the FX actions.** `accept` and `reject` document
  400/401/404/409/500; `cancel` and `settle` document 401/404/409/500 with **no 400**,
  because they declare no request body. Not smoothed over. (The platform document writes
  412 where this contract writes 409 — see the divergence table below.)
* **Almost nothing is `required` on the response side.** Only `LockHTLCResponse` and
  `LockHTLCWithHashLockResponse` declare required fields. `HTLCLock`, `FXAgreement`,
  `FXAgreementEvent`, every list wrapper and every record type declare none —
  conformance tests must not assume a response field is present.
* **No pagination anywhere.** `total` is the length of the returned array.
* **No `pattern` on any identifier.** `trade_id` is described as a UUID but exemplified as
  `trade-a1b2c3d4`. Do not validate with regexes the platform does not enforce.
* **No machine-readable error code.** The `/api/v1` error body is
  `{"error": "<free-form message>"}`. `ErrorCodeResponse` is declared in the platform
  document and referenced by nothing. Assert on status codes.

## Divergences from the platform's own OpenAPI document

| # | Where | Platform document says | Delivered gateway does |
|---|---|---|---|
| 1 | `lockHTLC`, `lockHTLCWithHash`, `settleHTLC`, `refundHTLC` | No 403 declared | The router gates all four to `ROLE_BANK`, `ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY`, `ROLE_GOVERNANCE`, so an oversight-only session cannot drive value movement. **403 is real** and is declared here. |
| 2 | `searchHTLC` | 200/401/500 | The supervisor scan path can return **502** `on-chain scan failed: …`. Declared here. |
| 3 | `mintToken` | No 403 declared | On a **Central Bank** gateway the router applies `RequireRole(ROLE_TREASURY)`; on a commercial-bank gateway the same route has no role gate at all. **403 is real on one deployment shape** and is declared here. |
| 4 | 412 on the four FX actions | One shared 412 response whose example is about a missing onboarding certificate | The gateway returns **409 Conflict**: the orchestrator raises gRPC `FailedPrecondition` and `grpcErrorToHTTP` maps it to 409. This contract declares 409 with a per-endpoint meaning. |
| 4b | 412 on `lockHTLCWithHash` | Missing participant certificate | **Unreachable.** That handler validates the body, deducts the transfer limit and returns 201/400/500; it never consults a certificate and never calls `grpcErrorToHTTP`. The 412 is not declared here. The one genuine 412 in the gateway belongs to `/pki-login`. |
| 5 | `POST /payments/{deposits,escrows,redeems}` documented as ordinary endpoints | Nothing said about gateway role | The create half is registered **only** on a commercial-bank gateway and the approve half **only** on a Central Bank gateway. Stated per operation here. |

## Open questions — flagged, not resolved

These are genuine gaps in the delivered specification. None is filled with a guess.

* Which HTLC `state` vocabulary does the `searchHTLC` filter accept — prefixed or bare?
  (For `listFXAgreements` the equivalent question *is* answered: the orchestrator strips an
  `FX_STATE_` prefix before matching, so both work. Nothing analogous exists for HTLC.)
* What state does an **expired** FX agreement land in? There is no `FX_STATE_EXPIRED` value,
  despite `expiry_date` and a real `FX_EVENT_SOURCE_SYSTEM_JOB` background expiration worker.
* Which spoke-ID family is canonical? The same platform schema uses `spoke-a`/`spoke-b` in
  server and identity examples but `spoke-brl`/`spoke-usd` for `source_spoke_id` /
  `dest_spoke_id`.
* Which HTLC contract-ID form does a client actually receive? The HTLC endpoints exemplify
  `htlc-a1b2c3d4`; `POST /api/v1/compliance/decrypt-transaction` (out of scope here)
  describes the same value as "0x-prefixed hex".
* `FX_RATE_TOLERANCE_PCT` and `FX_AGREEMENT_HTLC_STRICT` change server behaviour but are
  exposed by no endpoint. A client cannot discover the deployed configuration.
* FX behaviour differs materially between Pente-on and Pente-off deployments (`group_id`,
  `contract_address`, and `settleFXAgreement`'s "can also be called manually when Pente is
  not active") with no capability endpoint to tell them apart.
* Does Scenario A enforce an explicit "insufficient tokenized reserves" precondition on
  `lockHTLC` the way Scenario B's bridge-in handler does, or does the lock simply fail on
  an insufficient Zeto balance? Confirm before writing precondition text into a tutorial.
* There is **no idempotency mechanism** anywhere: no `Idempotency-Key`, no `If-Match` /
  `ETag`, and `X-Correlation-Id` is server-generated and overrides any client value. How a
  client should safely retry a `settle` is undefined.

## Deliberately out of scope

* **Internal relay endpoints.** The platform exposes exactly one relay-facing FX route,
  `GET /internal/v1/payments/fx/agreements`, protected by a shared-secret header and
  consumed by the Hyperledger Cacti bridge. It is not part of the community surface. There
  are **no** `/internal` routes for HTLC at all.
* **The cross-spoke choreography.** Conveying `hash_lock` from initiator to responder, and
  broadcasting the revealed secret from Spoke-A to Spoke-B, both happen out-of-band via
  the relay. Neither has an API endpoint in the delivered gateway. Nothing was invented to
  fill that gap.
* **The compliance, governance, supervisor, oversight, onboarding and treasury surface** —
  **34** of the Scenario A gateway's 69 documented `/api/v1` paths (11 `compliance`,
  13 `governance`, 5 `onboarding`, 3 `oversight`, 2 `treasury`). Deferred to a future
  `contracts/compliance/`, not forgotten.

  The other 35 are published: **28 here** (17 `payments`, 6 `htlc`, 5 `token`) and the
  **7 `auth` paths in [`../auth/`](../auth/)**. The three contracts together account for
  **62 of the 69**.
* **Routes the platform registers but never documents** — `GET /api/v1/statement`,
  `/api/v1/identities/*`, `/internal/v1/identities/*`,
  `POST /internal/v1/payments/pvp-legs`, `GET /internal/v1/payments/pvp-credits`. Recorded
  as a gap; nothing here is modelled on them.
* **Scenario B.** It has no FX agreement surface and no HTLC endpoints whatsoever — see
  [`../amm/`](../amm/README.md).

### A note on the name "PvP"

This directory keeps the name `pvp` because Scenario A's HTLC + FX flow *is*
payment-versus-payment in substance. Note that it does **not** correspond to the platform's
own `pvp`-named routes (`/internal/v1/payments/pvp-legs`, `/internal/v1/payments/pvp-credits`),
which are undocumented relay-internal endpoints and are out of scope.
