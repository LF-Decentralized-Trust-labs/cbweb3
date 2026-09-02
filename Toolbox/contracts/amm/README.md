# Hub-and-Spoke Settlement — Interface Contract (Scenario B)

| | |
|---|---|
| **File** | `openapi_amm_v2.3.0.yaml` |
| **Scenario** | **B** — hub-and-spoke, "International Hub" |
| **Platform version** | CBWeb3 API Gateway **v2.3.0** |
| **Operations** | 59 across 52 paths |
| **Depends on** | [`../auth/openapi_auth_v2.3.0.yaml`](../auth/README.md) for session bootstrap |
| **Status** | Normative. Derived from the delivered platform gateway. |

## Scenario B has no HTLC and no FX agreement

Stated plainly because it contradicts what earlier Toolbox material assumed: **not one of
the Scenario B gateway's 100 paths contains `htlc`, and there is no `/fx/agreement`
anywhere.** A search of the whole Go tree turns up a single stale comment and no handler,
route or service.

The Scenario B platform document does declare eight `LockHTLC*` / `HTLCLock` component
schemas — under a `# --- HTLC schemas ---` comment, referenced by nothing. They are
copy-paste residue from Scenario A and are deliberately **not** published here. Atomic-swap
settlement lives in [`../pvp/`](../pvp/README.md).

## What this contract covers

Scenario B settles a cross-currency payment as: **Reserve Tokenisation → bridge-in →
Hub AMM swap → bridge-out → residue return.**

| Domain | Operations | Prefix |
|---|---|---|
| Token balances | 2 | `/api/v1/token/*` |
| Escrow lifecycle (deposit / tokenise / redeem) | 13 | `/api/v1/payments/*` |
| AMM quotes and swaps | 9 | `/api/v2/amm/{quote,swap,pool,hub-*}` |
| Bridge (lock-mint / burn-unlock / positions) | 3 | `/api/v2/bridge/*` |
| Liquidity provisioning and reconciliation | 13 | `/api/v2/amm/{liquidity,lp-balance,hub-reconciliation}` |
| AMM token preparation | 2 | `/api/v2/amm/token/*` |
| Hub currency registry | 4 | `/api/v2/hub/*` |
| AMM pair registry | 3 | `/api/v2/amm/pairs*` |
| Circuit breaker | 4 | `/api/v2/governance/circuit-breaker/*` |
| Transfer limits | 3 | `/api/v2/governance/transfer-limits*` |
| Oversight (disclosure quorum) | 3 | `/api/v2/oversight/*` |

`operationId` values match the platform's own operation IDs exactly.

Note that `/api/v1` and `/api/v2` are **not API generations**. The prefix encodes the
subsystem: `/api/v1` is the core identity/compliance/token/escrow surface shared with
Scenario A; `/api/v2` is the Hub-and-Spoke AMM feature set, built later and never
backported. `/api/v1` is not deprecated — the gateway serves 44 v1 paths and 40 v2 paths
side by side.

## The settlement flow

**Part 0 — corridor provisioning (Central Banks, once per corridor).** Each CB registers
its currency (`registerHubCurrency`), which deploys its Hub W-token. The CB of tokenA
proposes the pair (`proposePair`); the CB of tokenB confirms it (`confirmPair`), activating
the corridor. **Two Central Banks are required by design** — one CB can never open a
corridor alone.

**Part 1 — make the pool ACTIVE (Central Banks).** Each CB bridges its own reserves
(`bridgeLockMint` → poll `listBridgePositions` for `bridge_state: ACTIVE`), mints and
approves (`mintAndApprove`), then supplies **only its own side**, by either:

* *cooperative commit-reveal* — `commitLiquidity`, then poll `getLiquidityCommit` until
  `EXECUTED` (this is the path the platform's own end-to-end test uses); or
* *sovereign escrow-and-finalize* — `depositLiquiditySide` / `finalizeLiquidityCommit` /
  `reclaimLiquiditySide`, where the side is auto-resolved on-chain and cannot be chosen.

Both replaced an earlier dual-sided `/api/v2/amm/liquidity/add` that let one CB supply both
sides; it was removed as a sovereignty breach and does not exist.

**Part 2 — the payer bank obtains tokenised reserves. Mandatory.**
`registerDeposit` → `approveDeposit` → `requestFiatExchange` (mints **fCeBM**, the ERC-20
fiat token on Besu) → `requestEscrow` → `approveEscrow` (burns fCeBM, mints **tCeBM**, the
Zeto privacy token on Paladin). This is **Reserve Tokenisation**, and it is a hard
precondition: the bridge-in handler refuses to move value for a bank that does not already
hold sufficient tCeBM. Enforcement is *disabled* when the gateway runs without
`PAYMENT_GRPC_ADDR`, so a sandbox can silently accept a swap production would refuse —
conformance runs must include the escrow step.

**Part 3 — the cross-border payment (the only part a third party writes code for).**
`listPairs` → `getPoolStatus` (must be `ACTIVE`) → `getV2CircuitBreakerStatus` (must not be
halted) → `getCrossCurrencyQuote` (**15-second TTL**) → `swapCrossCurrency` →
poll `getCrossCurrencySwapStatus` until `COMPLETED` or `FAILED`.

**Part 4 — settlement verification.** The payer's tCeBM balance should have decreased by
`amount_in`, **not** by `max_amount_in`; the beneficiary's balance should be non-zero; the
CB treasury's `getHubReconciliation` should report `balanced: true` at rest.

## Two things that surprise every first integrator

### 1. The slippage buffer is bridged in full, and the remainder comes back separately

A cross-currency swap is exact-**output**. Because the realised input is unknown until the
AMM executes, the orchestrator bridges in the **entire `max_amount_in`** first. The unspent
remainder is then burned on the Hub and returned to the payer as a *separate* bridge
position with `leg = RESIDUE`.

The payer's true net debit is `amount_in`. **`residue_status` is what makes an over-debit
visible:** `RETURN_FAILED` means the buffer is still stranded on the Hub swap signer and the
payer has **not** been made whole. Do not treat `status == COMPLETED` as sufficient proof of
correct settlement.

### 2. One POST produces asynchronous state on two other Central Banks' gateways

The swap is **sovereign-delegated**: the commercial bank never holds a W-token and never
signs on the Hub. Behind the single public POST, four relay-only steps run — the payer's
issuing CB does the bridge-in and signs the Hub AMM leg, the **beneficiary's** CB does the
bridge-out (CB-A burning CB-B's tokens would be a sovereignty breach), and the issuing CB
returns the residue.

Those four endpoints are out of scope here, but knowing they exist explains why failure can
surface as `BRIDGE_OUT_FAILED` — the swap succeeded, the delivery did not, and manual
reconciliation is required.

## How implementation-observed material is marked

**The `/api/v2` surface is deliberately under-specified upstream.** A block comment in the
platform's own document says: *"Schemas below are intentionally permissive (free-form
objects) where the handler request/response shape is service-defined; see the handler
packages for exact field contracts."* Roughly thirty `/api/v2` operations declare
`{type: object, additionalProperties: true}`.

Publishing those as bare untyped objects would make this contract useless to the external
implementers it exists for. So where the real shape was recovered from the delivered Go
handlers or from the platform's own executable end-to-end test, it is published — and
**every such schema carries `x-cbweb3-source: implementation-observed`** plus a
`Spec gap:` note in its description.

Schemas *without* that marker are transcribed from the platform's OpenAPI document. The
distinction matters: the vendor may change an implementation-observed shape without
touching its OpenAPI, so a conformance suite should treat those assertions as softer.

**Nothing was invented.** Every field published here is one a delivered handler, service
struct or shipped end-to-end test actually reads or writes; where a shape was recoverable
from none of those, the object is left open and the contract says so rather than guessing.

Five operations deserve a specific note, because the platform document gives them **no
body at all** and a client written from that document alone cannot call them: the three
circuit-breaker writes (`pauseCircuitBreaker`, `proposeResume`, `signResume`) declare no
`requestBody` upstream yet their handlers require one and answer 400 or 422 without it,
and `openDisclosure` / `signDisclosure` declare no response body upstream yet return one.
Their shapes — along with `sovereignAddLiquidity`, `approveAMM`, `mintAndApprove`,
`getHubLiquidityConfig`, `getV2CircuitBreakerStatus` and `getDisclosureStatus` — are
published here from the delivered code, each marked
`x-cbweb3-source: implementation-observed`.

## Divergences from the platform's own OpenAPI document

Each is marked inline in the contract with a `Spec divergence:` paragraph. Where they
conflict, **this contract documents what a client actually observes.**

| # | Where | Platform document says | Delivered gateway does |
|---|---|---|---|
| 1 | `getExactOutputQuote` | Query parameter `amountOut` (camelCase) | Handler reads **`amount_out`**. A client following the document always gets `400 INVALID_REQUEST`. **Blocking.** |
| 2 | `getCrossCurrencyQuote` | **Zero** parameters declared | Handler requires `source_currency`, `target_currency`, `amount_out`; accepts `pool_pair`, `max_slippage_pct`. Following the document literally always yields 400. **Blocking.** |
| 3 | `listBridgePositions` | 200 body is a bare JSON array | Returns `{"positions": [...]}` |
| 4 | `listHubCurrencies` | 200 body is a bare JSON array | Returns `{"currencies": [...]}` |
| 5 | `listPairs` | 200 body is a bare JSON array | Returns `{"pairs": [...]}` |
| 6 | `bridgeLockMint` | 200 | Returns **201** |
| 7 | `registerHubCurrency` | 200 | Returns **201** |
| 8 | `proposePair` | 200 | Returns **201** |
| 9 | `getPoolStatus` | 404 for an unavailable pool | Returns **422** `POOL_STATUS_UNAVAILABLE`. Both are declared so a client handles either. |
| 10 | `pauseCircuitBreaker`, `proposeResume`, `signResume` | **No `requestBody` declared at all** | All three require a JSON body; an empty POST is rejected. `pause` needs `pair`, `bank_id`, `reason_code`. **Blocking.** |
| 11 | `sovereignAddLiquidity` | 200 | Returns **201** |
| 12 | `mintAndApprove` | 403 means "insufficient role" | 403 is also the anti-cross policy refusal `CROSS_CB_MINT_PROHIBITED` when the recipient is another Central Bank's address |
| 13 | `commitLiquidity` | — (free-form body) | `provider_id`, `side` and `w_token_address` are **deprecated** in the handler: the provider comes from the session and the side is resolved on-chain. Send `pool_pair` and `amount` only. |
| 14 | `removeLiquidity` | — (free-form body) | `fraction_bps` is **optional**; omitting it means a full withdrawal. A `provider_bank_id` that disagrees with the session is 403 `PROVIDER_ID_MISMATCH`. |

Additionally, a large family of error paths is entirely undocumented upstream and is
declared here: 422 on the swap and bridge endpoints (`TRANSFER_LIMIT_EXCEEDED`,
`POOL_NOT_ACTIVE`, `CIRCUIT_BREAKER_HALTED`, `BRIDGE_IN_FAILED`, `SWAP_FAILED`,
`INSUFFICIENT_POOL_LIQUIDITY`, `ZK_VALIDATION_FAILED`), 409 and 404 on the currency and
pair registries, and 404 `HUB_LIQUIDITY_NOT_CONFIGURED`.

Three further upstream errors are corrected in wording rather than shape: the escrow
summaries read "tokenization escrow (tCeBM → tCeBM)" and "redemption (tCeBM → tCeBM)",
identical on both sides and therefore meaningless, and `getFiatBalance` calls the ERC-20
fiat balance "tCeBM". This contract writes **fCeBM (ERC-20 on Besu) ↔ tCeBM (Zeto on
Paladin)**.

**This is quoting the platform, not editorialising.** The Scenario B text is the product of
a bad global `fCeBM` → `tCeBM` find-replace: the **Scenario A** gateway document — the same
vendor, the same release — carries the correct wording for the identical operations
("Request tokenization escrow (fCeBM → tCeBM)", "Request redemption (tCeBM → fCeBM)",
"Burns the requester's fCeBM on Besu and mints equivalent tCeBM"). The delivered Scenario B
Go code and the platform's own shipped integration test agree with Scenario A
("Phase 3b: Reserve Tokenisation — Bank A converts fCeBM → tCeBM"). No Toolbox author chose
this terminology; it is transcribed from the platform's own correct copy of the same text.

## Why the reserve lifecycle is duplicated here and in `../pvp/`

`/api/v1/payments/{deposits,escrows,redeems}*` appears in **both** scenario contracts. That
duplication is deliberate, and extracting a shared `reserves/` contract was rejected: the
two gateways' reserve blocks are not the same API. Scenario A serves
`POST /payments/deposits/`**`fiat-exchange`** where Scenario B serves
`POST /payments/deposits/`**`exchange`**; Scenario A's requests carry
`requester_paladin_identity` and its redeem request declares `zeto_transfer_tx_hash`
(proxy-injected, not client-supplied), neither of which exists here; Scenario B's escrow request carries a `deposit_id` that
Scenario A's does not; and four transaction-hash response fields are named differently on
each side (`tx_hash`/`mint_tx_hash`, `zeto_mint_tx_hash`/`mint_tx_hash`,
`fiat_mint_tx_hash`/`mint_tx_hash`). A single shared document would have had to declare
both variants, telling an implementer that both exist on one gateway. Neither does.
See [`../pvp/README.md`](../pvp/README.md) for the full comparison table.

## Four error shapes coexist — the contract models the union honestly

| Shape | Where |
|---|---|
| `{error}` (`ErrorResponse`) | the `/api/v1` surface and the platform's shared responses |
| `{error, error_code, details?, recommended_action?}` (`AmmErrorResponse`) | v2 AMM / bridge / swap handlers |
| `{error, code}` (`RegistryErrorResponse`) | currency and pair registry handlers |
| `{error, error_code, hint}` (`SovereignSeedError`) | sovereign liquidity seeding — and, oddly, the 501 of `listCrossCurrencySwaps` |

The platform additionally declares a component schema `ErrorCodeResponse` (`{error, code}`)
that **no operation references**. It is dead and is not republished.

## Roles, and a naming trap

`/api/v2` uses a **separate role namespace** from `/api/v1`: lowercase, unprefixed realm
roles `central_bank`, `commercial_bank`, `mlp`.

> The platform document writes "Requires ROLE_CENTRAL_BANK_SCENARIO_B" on the
> transfer-limit endpoints. **That string is the name of a Go constant, not a role.** A JWT
> will never contain such a claim; the real value is `central_bank`. The same applies to
> `commercial_bank` and `mlp`.

The platform's shared 403 response is also described as "requires CENTRAL_BANK role" while
being referenced by operations the router gates on `commercial_bank`, or on
`commercial_bank` **or** `central_bank`, or on the liquidity-provider roles. The real gate
is stated per operation in this contract.

`BearerAuth` **is** declared here — unlike in the Scenario A contract — because Scenario B
genuinely accepts it, but only on the `/api/v2` routes guarded by `RequireAnyAuth`
(machine-to-machine Central Bank flows). **The swap endpoints are cookie-only.**

Twelve of this contract's 59 operations are fully public (`security: []`) — both quotes,
pool status, hub config, hub liquidity config, the currency and pair registry reads, hub
token supply, circuit-breaker status, LP positions, LP balance and disclosure status. They
are the natural entry point for a "discover the network" tutorial that needs no
onboarding, and they are the only public operations in the whole Toolbox contract set
apart from `/healthz` and the three public auth operations.

## No single gateway exposes this whole surface

Route registration is conditional on per-gateway dependency injection:

* `registerDeposit` / `requestEscrow` / `requestRedeem` exist **only** on commercial-bank
  gateways (which proxy to their Central Bank);
* the matching `approve` / `reject` / `exchange` operations exist **only** on Central Bank
  gateways;
* `/api/v2` groups appear only when their backing service is wired.

So the create half and the approve half of the escrow lifecycle live on **different hosts**,
and a 404 may mean "this gateway is not that kind of node" rather than "non-conformant".
**Every conformance suite must declare a gateway profile** (commercial bank / central bank /
hub) before it can assert anything.

## Using it with Prism

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml --port 4011

# Discover the corridor (unauthenticated in the real gateway too)
curl -s http://127.0.0.1:4011/api/v2/amm/pairs
curl -s http://127.0.0.1:4011/api/v2/hub/currencies
curl -s 'http://127.0.0.1:4011/api/v2/amm/pool/W-tCeBM_BRL%2FW-tCeBM_ARS/status'

# Quote, then settle
curl -s 'http://127.0.0.1:4011/api/v2/amm/quote/cross-currency?source_currency=BRL&target_currency=ARS&amount_out=1000000000000000000000'

curl -s -X POST http://127.0.0.1:4011/api/v2/amm/swap/cross-currency \
  -H 'Content-Type: application/json' \
  -d '{"source_currency":"BRL","target_currency":"ARS",
       "pool_pair":"W-tCeBM_BRL/W-tCeBM_ARS",
       "amount_out":"1000000000000000000000",
       "max_amount_in":"1200000000000000000000",
       "beneficiary_bank_id":"bank-galicia"}'

curl -s http://127.0.0.1:4011/api/v2/amm/swap/cross-currency/swp-1a2b3c4d
```

Add `--errors` to make Prism enforce request validation. Use
`-H 'Prefer: example=<name>'` to select a named example — for instance
`example=transferLimit` on the swap's 422.

Validate before committing changes:

```bash
npx @stoplight/spectral-cli lint --ruleset .spectral.yml --fail-severity=error \
  Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml
```

## Conventions reproduced verbatim

* **Amounts are strings, and the unit is not uniform.** `/api/v1` says "big integer as
  string"; `/api/v2` says "base units (decimal string)"; `TransferLimitRecord.max_amount`
  is **wei** while the `max_amount` you *send* to create that same limit is
  **human-decimal** (`"100000.00"`). No decimals count is asserted anywhere except
  `SovereignSupply.decimals`. **Do not assume a scale factor.**
* **Timestamps are mixed:** RFC 3339 on record metadata, `YYYY-MM-DD` on the swap-history
  filters, and raw **Unix integers** on quote `created_at` / `valid_until`.
* **Pair identifiers are opaque.** Five incompatible conventions appear across the
  delivered artefacts — `tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`, `W-BRL-ARS`, the
  quote-style `W-BRL-W-ARS` and the circuit breaker's `BRL-USD` fallback — and none is
  documented as canonical. All five are enumerated with source citations in
  [`../../DIVERGENCES.md`](../../DIVERGENCES.md). **Always take a pair id from `listPairs`;
  never construct one.** The `mocks/amm/` fixtures use the sovereign form (`W-CRC-CLP`)
  because the pool-status route is a single path segment; the examples in this contract use
  the slash form with `%2F`.
* **Caller identity is never taken from the body.** `payer_bank_id`, `payer_id` and
  `owner_bank_id` are marked `deprecated` here because the handlers ignore them and derive
  identity from the session. Do not teach clients to send them.
* **The bridge-state vocabulary is not in the platform document at all** — it is
  `LOCKING | ACTIVE | BURNING | BURNED | RELEASED | RECONCILIATION_REQUIRED`, recovered
  from `internal/domain`. The document's own example on the `state` filter reads
  `PENDING, ACTIVE, CLOSED`; **`PENDING` and `CLOSED` do not exist.** Do not copy it.

## Open questions — flagged, not resolved

These are genuine gaps in the delivered specification. None is filled with a guess.

* **Which `pool_pair` form is canonical?** Five incompatible conventions coexist across
  the platform's spec, schemas, handlers and shipped end-to-end test, with none declared
  canonical.
  This contract treats pair identifiers as opaque strings obtained from `listPairs`, but
  the platform should declare a canonical form.
* **What decimals scale do tCeBM and fCeBM use?** 18 is implied by the word "wei" but
  never asserted, and transfer limits are human-decimal on input yet wei on output for the
  same logical field. Fixtures must not assume a scale factor.
* **Which error-code spelling is authoritative?** `ErrorCodeResponse` `{error, code}` is
  declared upstream and referenced by nothing; the AMM/bridge/swap handlers emit
  `error_code`; the registry handlers emit `code`; the `/api/v1` surface emits no code at
  all. Conformance can assert reliably only on status codes.
* **How should a client safely retry?** There is **no idempotency mechanism** on the public
  surface: no `Idempotency-Key`, no `If-Match`/`ETag`, and `X-Correlation-Id` is
  server-generated and overrides any client-supplied value. The idempotency that does exist
  is server-side and endpoint-specific — the relay-internal hub swap returns
  `status: "duplicate"` for an already-swapped position and **409** for one carrying a
  failed claim, and spoke registration reports `already_registered: true` — but none of
  that is reachable from a client. A cross-currency swap that times out client-side cannot
  be retried safely by any documented means.
* **Scenario B's relay auth is mid-migration** (ECDSA signature preferred, shared secret
  fallback) while the platform's OpenAPI documents only the bare shared secret,
  understating the security model. Out of Toolbox scope, since no `/internal` route is
  published here, but it should be confirmed before any future internal-surface
  documentation.
* **There is no CSRF protection anywhere on the platform**, and this contract therefore
  declares none. The sole mitigation is `SameSite=Strict` on the auth cookies. Recorded as
  a security finding rather than papered over.

## Deliberately out of scope

* **All `/internal/*` routes** — the Cacti relay endpoints under `/internal/amm`
  (matched-commit execution, cross-currency bridge-in / hub-swap / bridge-out /
  residue-return), the payment relay under `/internal/v1/payments`, spoke self-registration
  under `/internal/v1/spokes`, and the transfer-limit pre-auth relay under
  `/internal/v2/transfer-limits`. They are Central-Bank-gateway-only and must never be
  exposed publicly.

  They are also **mid-migration in a way the platform document does not state**: it presents
  relay auth as a plain `X-Relay-Auth` shared secret, while the delivered middleware prefers
  per-CB ECDSA signatures (`X-Relay-Key-Id` / `X-Relay-Timestamp` / `X-Relay-Signature`)
  with replay protection, falling back to the shared secret only while signatures are not
  mandatory. Documenting them as a bare shared secret would understate the security model —
  a further reason to keep them out of the community surface.

* **Compliance, onboarding, governance-portal and identity endpoints**, which share the
  gateway but belong to other Toolbox domains.
* **The Scenario B HTLC schemas**, which are unreferenced residue (see the top of this file).
