# `pvp/` — Scenario A: PvP settlement by FX agreement and dual-layer HTLC

Contract: [`../../contracts/pvp/openapi_pvp_v2.3.0.yaml`](../../contracts/pvp/openapi_pvp_v2.3.0.yaml)
· Scenario: **A**, single-ledger, spoke-to-spoke.

Scenario A settles a cross-currency payment as an **atomic swap of two tokenised central bank
money legs**, agreed bilaterally and executed by a pair of hash time-locked contracts. Scenario B
has no FX agreement surface and no HTLC endpoints whatsoever — see [`../amm/`](../amm/).

## The corridor these fixtures trace

Costa Rica → Peru. **5,000,000 CRC for 37,000 PEN at 0.0074**, on 18 June 2026.

| Actor | Paladin identity | Spoke |
|---|---|---|
| Banco de San José — payer, originator | `funded_operator@spoke-crc-bank-sanjose` | `spoke-crc` |
| Banco de Heredia — settlement agent, receives the CRC leg | `funded_operator@spoke-crc-bank-heredia` | `spoke-crc` |
| Banco de Lima — counterparty, custodian | `funded_operator@spoke-pen-bank-lima` | `spoke-pen` |
| Banco de Callao — beneficiary, receives the PEN leg | `funded_operator@spoke-pen-bank-callao` | `spoke-pen` |
| BCCR — Banco Central de Costa Rica | Central Bank gateway | `spoke-crc` |

BCCR and BCRP are real pilot participants. The four commercial banks are invented, named after
cities so no real institution is implied. The identity form is the platform's own
(`<operator>@<spoke>-<bank>`), and it is load-bearing: the gateway parses the spoke out of it and
**refuses an HTLC lock whose receiver is not on the local spoke**.

## `happy-path/` — seventeen steps, one payment

### Reserve prelude (1–7): a bank cannot lock tokens it does not hold

| # | Call | Result |
|---|---|---|
| 01 | `POST /api/v1/payments/deposits` | `deposit_id`, record `PENDING` |
| 02 | `POST /api/v1/payments/deposits/approve` | BCCR confirms 10,000,000 CRC of fiat arrived |
| 03 | `POST /api/v1/payments/deposits/fiat-exchange` | **fCeBM minted** — first on-chain movement |
| 04 | `GET /api/v1/token/fiat-balance` | `10000000` — the fiat ERC-20, on Besu |
| 05 | `POST /api/v1/payments/escrows` | Reserve Tokenisation requested for 8,000,000 |
| 06 | `POST /api/v1/payments/escrows/approve` | **fCeBM burned on Besu, tCeBM minted on Paladin** |
| 07 | `GET /api/v1/token/balance` | `8000000` tCeBM — the settlement precondition is met |

The two tokens are not interchangeable and the distinction is the single most common source of
confusion in this platform's own documentation. **fCeBM** is an ERC-20 fiat token on Besu;
**tCeBM** is a Zeto privacy token on Paladin, and it is what an HTLC locks. Steps 11 and 12 fail
outright without step 06.

### Agreeing the terms (8–10)

| # | Call | Result |
|---|---|---|
| 08 | `POST /api/v1/payments/fx/agreements` | `PROPOSED`; server-generated `trade_id` (a UUID) |
| 09 | `POST …/{tradeId}/accept` | `ACCEPTED` — **called on the spoke-pen gateway**, a different host |
| 10 | `GET …/{tradeId}` | the terms both HTLC legs must match |

`trade_id` is deliberately omitted from the request so the server generates one, and `on_behalf`
is omitted because the handler hard-codes it to `false` and ignores whatever is sent. Rate
consistency is enforced: `|counter_amount / origin_amount − rate|` must be within
`FX_RATE_TOLERANCE_PCT` (default 0.1%), and `37000 / 5000000 = 0.0074` exactly. Every party
identity is checked against the Paladin roster before the proposal is accepted.

### The atomic swap (11–14)

| # | Call | Timelock | Result |
|---|---|---|---|
| 11 | `POST /api/v1/htlc/lock` | `now + 3600` → `1781781300` | initiator locks 5,000,000 CRC; gateway generates the secret and returns `hash_lock` **and** `secret` |
| — | *hash lock crosses out-of-band* | | **there is no API endpoint for this anywhere on the platform** |
| 12 | `POST /api/v1/htlc/lock-with-hash` | `now + 1800` → `1781779800` | responder locks 37,000 PEN against the same hash lock, on the spoke-pen gateway |
| 13 | `GET /api/v1/htlc/status/{contractId}` | | `counterparty_locked: true` — the mirror leg exists |
| 14 | `POST /api/v1/htlc/settle` | | initiator reveals the secret; both legs settle |

> **The timelock ordering is safety-critical and enforced only by prose.** The initiator's
> `1781781300` **must** exceed the responder's `1781779800`, so the responder can reclaim its
> tokens before the initiator regains the ability to. Nothing in any schema enforces this.
> **A fixture, vector or tutorial that inverts the ordering models an unsafe swap in which the
> initiator can claim the responder's leg after the responder has lost the ability to refund,
> and must be rejected in review.**

Step 13 is not decoration. The gateway refuses to settle while `counterparty_locked` is false,
because revealing the secret before the mirror lock exists would let the initiator take the
responder's leg while owing nothing. Step 14 produces state changes on a gateway you never
contacted: the Cacti relay picks the revealed secret up and settles the spoke-pen leg. That
broadcast has no HTTP endpoint either.

### Verification (15–17)

| # | Call | Result |
|---|---|---|
| 15 | `GET /api/v1/htlc/search?agreement_id=…` | the local leg, now `HTLC_STATE_SETTLED`, secret public |
| 16 | `GET …/{tradeId}/audit` | `INVALID → PROPOSED → ACCEPTED → SETTLED`, the last delivered by the relay |
| 17 | `GET /api/v1/token/balance` | `3000000` — down **exactly** the 5,000,000 origin leg |

Step 17, not a 200 from settle, is what makes a conformance run a settlement test. Only one lock
comes back from search: the responder's leg lives on the spoke-pen orchestrator and is not
indexed here.

## `timeout-refund/` — the same corridor, the counterparty never locks

A separate trade, so nothing here contradicts the happy path.

| # | Call | Result |
|---|---|---|
| 01 | `POST /api/v1/htlc/refund` 30 min early | **500** `rpc error: code = FailedPrecondition desc = time lock has not expired yet` |
| 02 | `GET /api/v1/htlc/status/{contractId}` past expiry | still `HTLC_STATE_LOCKED` — **nothing expires by itself** |
| 03 | `POST /api/v1/htlc/refund` after expiry | 200, tokens unlocked back to the sender |

Two facts worth internalising. A refund that is merely early is reported as a **500**, because
the gateway maps every unhandled gRPC code that way and passes the raw transport string through
into `error`. And there is no `EXPIRED` state and no job that sets one: expiry is a permission to
refund, not a transition, so a client must compare `time_lock` against its own clock. The
delivered domain also carries `SETTLING` and `REFUNDING` states that appear in no published
enum — treat the HTLC state vocabulary as open.

## `errors/` — the three failures an integrator will actually meet

| # | Call | Status | Body |
|---|---|---|---|
| 01 | `POST /api/v1/htlc/lock` with no cookie | 401 | `{"error": "missing access_token cookie"}` |
| 02 | `POST …/{tradeId}/accept` on an `ACCEPTED` agreement | **409** | raw gRPC precondition string |
| 03 | `POST /api/v1/htlc/settle` from a supervisor session | 403 | `{"error": "insufficient permissions", "required_role": [...]}` |

Fixture 02 records divergence **A4**: the *platform document* declares **412** here and the
gateway returns **409**. The Toolbox contract now declares **409** as well, so the fixture, the
contract and the estate agree; a client generated from `contracts/pvp` handles the status it will
actually receive. See [`../../DIVERGENCES.md`](../../DIVERGENCES.md). Fixture 03's 403 is declared in no upstream document at all; the router gates
the four mutating HTLC routes to `ROLE_BANK`, `ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY` and
`ROLE_GOVERNANCE` so that an oversight-only session cannot drive value movement, and the body
carries an undocumented `required_role` array. A second, different 403 exists on the same routes
for a correctly-roled caller who is not a counterparty of that HTLC.

**Assert on status codes, never on error strings.** The `/api/v1` error model is
`{"error": "<free-form message>"}` with no machine-readable code anywhere; the platform declares
an `ErrorCodeResponse` schema that **no operation references**. The invented codes
`HTLC_HASH_MISMATCH`, `HTLC_EXPIRED`, `HTLC_NOT_EXPIRED`, `HTLC_ALREADY_SETTLED` and
`AGREEMENT_NOT_ACCEPTED` that earlier Toolbox material used exist nowhere on this platform and
must not be resurrected.

## Which gateway serves what

No single host serves this whole flow, and the `notes` of every fixture says which one it means.

| Half | Registered on | A wrong host answers |
|---|---|---|
| `POST` deposits / escrows / redeems | commercial-bank gateway (proxies to the CB) | 404 |
| `approve` / `reject` / `fiat-exchange` | Central Bank gateway, `ROLE_TREASURY` | 404 |
| spoke-crc legs (steps 1–8, 11, 13–17) | Banco de San José / BCCR | — |
| spoke-pen legs (steps 9, 12) | Banco de Lima | — |

A 404 from a conformance run therefore often means "this gateway is not that kind of node", not
"the implementation is non-conformant". On a commercial-bank gateway the create-half calls are
**proxied**, and the proxy overwrites `requester_besu_address` and `requester_paladin_identity`
from the verified caller — which is why no request body in this directory sends them.

## Recorded as absent, not filled in with a guess

* No `EXPIRED` FX state, despite `expiry_date` on every agreement and a documented `SYSTEM_JOB`
  expiration worker.
* No write path for relay-delivered FX state changes: the Internal tag describes one, but the
  only registered internal FX route is a `GET`.
* No HTLC relay endpoints at all; cross-spoke secret propagation is prose with no HTTP surface.
* No decimals count for tCeBM or fCeBM anywhere. Amounts here use the platform's own literal
  example magnitudes; compare deltas, never assume a scale factor.
* Two HTLC state vocabularies that the platform never reconciles — records use
  `HTLC_STATE_LOCKED`, the `/api/v1/htlc/search` `state` **query parameter** takes bare `LOCKED`. Both
  are reproduced exactly as published.
* Five routes the Scenario A router wires but documents nowhere, including the only two endpoints
  on the entire platform whose names contain "pvp" — both relay-internal.
