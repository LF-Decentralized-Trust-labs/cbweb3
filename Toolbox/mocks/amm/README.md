# `amm/` — Scenario B: hub-and-spoke settlement over the International Hub

Contract: [`../../contracts/amm/openapi_amm_v2.3.0.yaml`](../../contracts/amm/openapi_amm_v2.3.0.yaml)
· Scenario: **B**, hub-and-spoke.

> **Scenario B has no HTLC endpoints and no FX agreement surface.** Not one gateway path
> contains `htlc`, and a search of the whole Go tree finds a single stale comment and no handler,
> route or service. The eight `LockHTLC*` schemas in the Scenario B platform document are
> unreferenced copy-paste residue from Scenario A. Atomic-swap settlement lives in
> [`../pvp/`](../pvp/).

Settlement here is: **Reserve Tokenisation → bridge-in (lock spoke tCeBM, mint Hub W-token) →
Hub AMM swap → bridge-out → residue return.**

## The corridor these fixtures trace

Costa Rica → Chile. **1,000 CLP delivered for 542.75 CRC**, on 18 June 2026, over a corridor
opened the day before.

| Actor | Identifier |
|---|---|
| BCCR — Banco Central de Costa Rica, issuer of CRC | `central_bank_crc`, `spoke-crc` |
| Banco Central de Chile, issuer of CLP | `central_bank_clp`, `spoke-clp` |
| Banco de San José — payer, commercial | `bank-sanjose` |
| Banco de Valparaíso — beneficiary | `bank-valparaiso` |

`/api/v2` uses **lowercase, unprefixed** realm roles (`central_bank`, `commercial_bank`, `mlp`)
while `/api/v1` uses `ROLE_`-prefixed ones; both namespaces coexist in the same token's `roles`
array. The platform document's `ROLE_CENTRAL_BANK_SCENARIO_B` is the name of a Go constant, not a
role — no JWT will ever contain it.

## `provisioning/` — what two Central Banks must do before anyone can pay

Eight steps, none of which a commercial bank performs. One Central Bank can never open a corridor
alone; that is a design property, not an inconvenience.

| # | Call | Who |
|---|---|---|
| 01 | `POST /api/v2/hub/currencies` — register `W-tCeBM_CRC` | BCCR |
| 02 | `POST /api/v2/hub/currencies` — register `W-tCeBM_CLP` | Banco Central de Chile |
| 03 | `POST /api/v2/amm/pairs/propose` — only the CB of token A may propose | BCCR |
| 04 | `POST /api/v2/amm/pairs/confirm` — only the CB of token B may confirm; pair becomes `ACTIVE` | Chile |
| 05 | `POST /api/v2/bridge/lock-mint` — bridge 500,000 CRC of own reserves; poll to `ACTIVE` | BCCR |
| 06 | `POST /api/v2/amm/token/mint-and-approve` | BCCR |
| 07 | `POST /api/v2/amm/liquidity/commit` — **its own side only** | BCCR |
| 08 | `GET /api/v2/amm/liquidity/commits/{commit_id}` — poll until `EXECUTED` | BCCR |

Commit-reveal is the path the platform's own end-to-end test uses: each CB commits its side, a
Cacti watcher observes the match, and the relay executes both together. The alternative sovereign
path (`deposit-side` / `finalize` / `reclaim-side`) is not interchangeable with it. An earlier
dual-sided `liquidity/add` that let one CB supply both sides was removed as a sovereignty breach
and does not exist.

Send only `pool_pair` and `amount` on step 07: `provider_id`, `side` and `w_token_address` are
labelled deprecated backward-compatibility fields in the delivered handler. **Which side a
Central Bank owns is resolved on-chain, not chosen by the caller** — step 08 shows `side: "A"`
because the resolver decided it.

## `happy-path/` — eighteen steps, one cross-border payment

### Discovery (01–05): five calls, zero credentials

`hub-config` → `hub/currencies` → `amm/pairs` → `pool/{pair}/status` is `ACTIVE` →
`circuit-breaker/status` is `LIVE`. Twelve `/api/v2` operations in this contract are fully
public; these are the natural zero-onboarding entry point to an unknown CBWeb3 network.

Two traps live here. **Always pass `pair` to the circuit breaker** — it is per pair, and omitting
the parameter does not report "the network": the handler substitutes the literal `BRL-USD`, which
on most deployments is not a registered pair at all, so an omitted parameter can return a
healthy-looking `LIVE` for a pair nobody trades. And a `LIVE` answer is not proof no peer has
halted: when the chain cannot be read the gateway falls back to its local projection, and to
`LIVE` when it has no record.

### Reserve prelude (06–11): mandatory, and silently skippable in a sandbox

`deposits` → `approve` → `exchange` → `escrows` → `approve` → non-zero `token/balance`. The
cross-currency bridge-in handler refuses a swap from a bank that does not already hold sufficient
tCeBM: *"insufficient tokenized reserves: bank %s holds %s tCeBM, need %s — complete Reserve
Tokenisation first"*. **Enforcement is disabled when the gateway runs without
`PAYMENT_GRPC_ADDR`**, so a sandbox can accept a swap that production would refuse — which is
exactly why a conformance run must include these steps regardless of whether it can get away
without them.

Note the path: Scenario B serves the fiat exchange at `deposits/`**`exchange`**, Scenario A at
`deposits/`**`fiat-exchange`**. No gateway serves both. That single divergence is why the Toolbox
publishes the reserve lifecycle once per scenario rather than sharing it — a shared document
would have to declare both paths and would therefore lie.

### The payment (12–15)

| # | Call | Result |
|---|---|---|
| 12 | `GET /api/v2/amm/quote/cross-currency` | `amount_in` 542.75 CRC for `amount_out` 1,000 CLP — **15-second TTL** |
| 13 | `POST /api/v2/amm/swap/cross-currency` | `swap_id`, status `BRIDGE_IN_PROGRESS` |
| 14 | `GET /api/v2/amm/swap/cross-currency/{id}` | `SWAP_IN_PROGRESS` — fields appear as the orchestration reaches them |
| 15 | same | `COMPLETED`, `residue_status: RETURNED`, 41 seconds end to end |

**The quote lives fifteen seconds**, so quote and swap must be issued programmatically in one
step; a test that pauses between them will flake. The quote endpoint is a blocking
spec-vs-implementation divergence: the platform document declares **zero** parameters while the
handler requires three, so following the document literally always yields 400. And the swap is
**cookie-only** — `RequireCookieAuth`, not `RequireAnyAuth` — so the bearer token that works on
the bridge and liquidity routes will not work here.

The numbers are reproducible from fixture 04. For reserves 500,000 / 925,000 and a 30 bps fee,
the constant-product exact-output input is
`reserve_a * amount_out * 10000 // ((reserve_b − amount_out) * 9970) + 1`, which is exactly the
`542753802533140547183` the quote returns.

### Settlement verification (16–18) — the part that makes it a settlement test

| # | Call | Assertion |
|---|---|---|
| 16 | `GET /api/v2/bridge/positions` | bridge-in carried the **full** `max_amount_in`; the RESIDUE position reached `RELEASED` |
| 17 | `GET /api/v1/token/balance` | payer debited `amount_in`, **not** `max_amount_in` |
| 18 | `GET /api/v2/amm/hub-reconciliation` | `balanced: true`, `unexplained: "0"` |

## The residue mechanic, which must be taught rather than glossed

A cross-currency swap is exact-**output**: the payer names what the beneficiary receives and a
ceiling `max_amount_in`. Because the realised input is unknown until the AMM executes, the
orchestrator **bridges in the full `max_amount_in` first**. The unspent remainder is burned on
the Hub and returned to the payer on its source spoke as a *separate* bridge position with
`leg = RESIDUE`.

```
max_amount_in   548181340558471952655
− amount_in     542753802533140547183
= residue         5427538025331405472      ← must come back, or the payer is short
```

`residue_status` is the only field that makes an over-debit visible.
[`residue/`](residue/) shows what it looks like when it fails:

| # | File | Shows |
|---|---|---|
| 01 | `01_swap_status_return_failed.json` | `status: COMPLETED` **and** `residue_status: RETURN_FAILED` — the beneficiary was paid in full, the payer is 5.43 CRC short |
| 02 | `02_hub_reconciliation_unbalanced.json` | the same event from BCCR's side: `unexplained` non-zero, `balanced: false`, the position stranded in `RECONCILIATION_REQUIRED` |

**A client that asserts only on `status == COMPLETED` reports fixture 01 as a clean payment.**
Conformance must assert residue reconciliation.

## Why one POST produces asynchronous, multi-Central-Bank state

The swap is sovereign-delegated: the commercial bank never holds a W-token and never signs on the
Hub. Behind the single public POST, four relay-only steps run on Central Bank gateways — the
payer's issuing CB performs the bridge-in (only it may mint `W-<source>`), that same CB signs the
Hub AMM leg, the **beneficiary's** CB performs the bridge-out (CB-A burning CB-B's tokens would
be a sovereignty breach), and the issuing CB returns the residue. Those `/internal/amm/*`
endpoints are deliberately out of scope for this kit, but an integrator cannot understand the
asynchrony without knowing they exist — nor why `BRIDGE_OUT_FAILED` is expressible at all: the
swap succeeded and the delivery did not, and that requires manual intervention rather than a retry.

## `errors/`

| # | Call | Status | Code |
|---|---|---|---|
| 01 | `POST /api/v2/amm/swap/cross-currency` | 422 | `TRANSFER_LIMIT_EXCEEDED` |
| 02 | `POST /api/v2/amm/swap/cross-currency` | 422 | `POOL_NOT_ACTIVE` |
| 03 | `GET /api/v1/token/balance` with no cookie | 401 | *(none — `/api/v1` has no codes)* |

Unlike `/api/v1`, the `/api/v2` handlers do carry a machine-readable `error_code`, and it is the
only stable thing to assert on in this part of the surface. Note the gateway spells the same idea
three ways: `error_code` in the AMM, bridge and swap handlers, `code` in the currency and pair
registries, and nothing at all on `/api/v1`. Fixture 01 also shows the transfer-limit unit trap:
the limit is quoted in **wei** in this message while the `max_amount` a Central Bank *sends* to
create that same limit is a human decimal like `"100000.00"` — one field name, two units,
depending on direction.

## Pair identifiers are opaque. Never construct one.

Five incompatible conventions appear across the delivered artefacts — `tCeBM_BRL-tCeBM_ARS`,
`W-tCeBM_BRL/W-tCeBM_ARS`, `W-BRL-ARS`, the quote-style `W-BRL-W-ARS` and the circuit breaker's
`BRL-USD` fallback — and **none is documented as canonical**. All five are enumerated with
source citations in [`../../DIVERGENCES.md`](../../DIVERGENCES.md). Always take `pair_id`
verbatim from `GET /api/v2/amm/pairs`.

These fixtures use the slash-free **sovereign** form `W-CRC-CLP` for a concrete, verifiable
reason: the pool route is `/api/v2/amm/pool/:pair/status`, a **single path segment**, which an
identifier containing a slash cannot occupy without percent-encoding.

That form is real, and here is its source rather than an appeal to authority: the delivered
`normalizeSovereignPoolPair` in
`scenario-b/backend/services/api-gateway/internal/services/central_bank_pool_client.go`
rewrites the quote-style `W-BRL-W-ARS` into `W-BRL-ARS` precisely so it can be spliced into that
single path segment, and the sovereign seeding, bridge-out and NOC paths use the `W-<A>-<B>`
shape throughout. The contract examples and the sandbox tutorials use the slash form with `%2F`
instead. **Both are legitimate; neither is canonical.**

Note deliberately that the currency symbols `W-tCeBM_CRC` and `W-tCeBM_CLP` are **not** the
components of the pair id — the two are unrelated opaque strings, and trying to derive one from
the other is the mistake this section exists to prevent.

## The one unresolved ambiguity, flagged rather than hidden

The platform documents `/api/v1` escrow amounts as "big integer as string" with **no decimals
count stated anywhere**, and `/api/v2` AMM amounts as 18-decimal base units. Both denominate the
same asset — the payer's spoke tCeBM. These fixtures express them in the same base units end to
end, because that is the only way a deposit, an escrow and a swap reconcile into one story. **The
platform never asserts that scale factor.** A client moving a figure from a deposit into a swap
cannot today do so safely on the strength of the documentation alone.

## Also recorded as absent or unstable

* **Bridge state has no published enum.** The real vocabulary — `LOCKING`, `ACTIVE`, `BURNING`,
  `BURNED`, `RELEASED`, `RECONCILIATION_REQUIRED` — lives only in Go domain code. The
  `PENDING, ACTIVE, CLOSED` example in the platform document names **two states that do not
  exist**. Fixture `happy-path/16` deliberately does not claim a terminal state for the settlement
  bridge-in position: `ACTIVE` is the last state the delivered code is documented to drive it to.
* **The bridge-out position is not visible from the payer's side.** It is created by the
  beneficiary's Central Bank on its own spoke; asserting on it needs that gateway.
* **`mirrored_asset` is an address while `native_asset` is a symbol** — one record, two kinds of
  value, and the platform's example shows a symbol for both.
* **Three response envelopes documented as bare arrays actually return wrapped objects**:
  `bridge/positions`, `hub/currencies` and `amm/pairs`. A client deserialising into an array per
  the document fails on all three.
* **Never send caller identity in a body.** `payer_bank_id`, `owner_bank_id`, `provider_id` and
  `requester_besu_address` are deprecated and overwritten from the session `bankId` claim.
* Much of `/api/v2` is deliberately under-specified upstream — the platform says so in a comment.
  Where the contract fills a shape from handler source it marks it
  `x-cbweb3-source: implementation-observed`, and these fixtures inherit those shapes.
