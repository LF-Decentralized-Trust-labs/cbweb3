<!-- SPDX-License-Identifier: Apache-2.0 -->
# Divergence register

**One canonical list.** Every place where the CBWeb3 platform's published OpenAPI document
disagrees with the gateway that platform actually ships. The mocks, the test vectors, the
conformance suite and the interface contracts all point here instead of each keeping a private
table that drifts from the others.

## The rule

**The delivered gateway wins.** It is in production use by central banks. Where the document and
the binary disagree, the Toolbox contracts describe the binary and say so in a
`Spec divergence:` note on the operation or schema concerned. Nothing here is a change request
for the vendor; every correction is on the Toolbox side.

Two words are used precisely:

* **`Spec divergence`** — the document states something and the gateway does something else.
* **`Spec gap`** — the document states nothing (a free-form object, an undeclared status code)
  and the Toolbox fills it from delivered source.

---

## Scenario A — PvP settlement, reserves, HTLC

All eleven rows are traced to delivered source. Every one is now reflected in
`contracts/pvp/openapi_pvp_v2.3.0.yaml`, so the contract, the mocks, the vectors and the
conformance suite describe a single shape.

| # | Operation | Platform document says | Delivered gateway does | Source |
|---|---|---|---|---|
| A1 | `getFXAgreement`, `listFXAgreements` | `state: "ACCEPTED"` | `state: "FX_STATE_ACCEPTED"` — the protobuf enum name | `api-gateway/internal/adapters/payment/payment_grpc.go` — `ag.State.String()` |
| A2 | `listFXAgreementEvents` | `from_state`/`to_state`/`source` bare | `FX_STATE_*` / `FX_EVENT_SOURCE_*`, and `FX_STATE_INVALID` is real — it is the `from_state` of the propose event | same file, `fxAgreementEventToResult`; `payment-orchestrator/…/server.go` line ~1164 |
| A3 | `listFXAgreementEvents` | `actor` is a party identity; `notes` carries detail | `actor` is the literal `"payment-orchestrator"`; `notes` is never written | `payment-orchestrator/…/server.go` — `appendFXAuditEvent` |
| A4 | FX `accept` / `reject` / `cancel` / `settle` | **412** Precondition Failed | **409** Conflict. No payments or HTLC route emits 412 at all — the gateway's only 412 is `/pki-login`'s missing-certificate branch | `handlers/payment.go` — `grpcErrorToHTTP`, `codes.FailedPrecondition → StatusConflict`; `handlers/onboarding_proxy.go:465` |
| A5 | `getHTLCStatus` | `{"lock": { … }}` | the **bare** record at the top level. `searchHTLC` *does* wrap, as `{"locks": [...], "total": n}` | `handlers/payment.go` — `return c.JSON(result)` |
| A6 | `getHTLCStatus` / `searchHTLC` record | has `secret`; lacks `counterparty_locked`, `amount`, `created_at` | exactly the reverse | `adapters/payment/payment_grpc.go` — `type HTLCStatus` |
| A7 | `lockHTLC` 201 | no `secret` | returns `secret` **once**; no other route ever exposes it | same file — `type LockHTLCResult` |
| A8 | `requestFiatExchange` 201 | `{tx_hash}` | `{mint_tx_hash}` | same file — `type FiatExchangeResult` |
| A9 | `approveEscrow` 201 | `{burn_tx_hash, zeto_mint_tx_hash}` | `{burn_tx_hash, mint_tx_hash}` | same file — `type ApproveEscrowResult` |
| A10 | `registerDeposit` / `requestEscrow` / `requestRedeem` 201 | `{id, status}` | `{id}` only | same file — `RegisterDepositResult`, `RequestEscrowResult`, `RequestRedeemResult` |
| A11 | FX `accept` / `reject` / `cancel` / `settle` 200 | `{tx_hash}` | `{trade_id, tx_hash}` | same file — `FXAgreementResult`, every other field `omitempty` |

Two further Scenario A behaviours are gaps rather than contradictions, and are documented on the
operations themselves:

* **`proposeFXAgreement` requires eleven fields, not seven.** `source_spoke_id`,
  `dest_spoke_id`, `source_receiver` and `dest_receiver` are enforced by the orchestrator
  (`InvalidArgument` → 400), one hop past the gateway's own body check.
* **Reserve and HTLC-lock validation surfaces as 500, not 400.** Those handlers check only that
  the body parses and then wrap every downstream error as `StatusInternalServerError` instead of
  routing it through `grpcErrorToHTTP`. The declared 400 covers unparseable JSON only.

Vectors that assert these: `fx-hp-03`, `fx-edge-01` (A1/A2); `fx-err-07/08/09` (A4);
`htlc-hp-03`, `htlc-hp-05` (A5/A6); `reserve-hp-03` (A8); `reserve-hp-05` (A9);
`reserve-hp-01/04` (A10); `fx-err-03` (eleven required fields);
`reserve-err-02/04`, `htlc-err-01/02/03/04/07/08`, `htlc-edge-03` (500-not-400).

---

## Scenario B — AMM, bridge, registries

| # | Operation | Platform document says | Delivered gateway does | Source |
|---|---|---|---|---|
| B1 | `listPairs`, `listHubCurrencies` | a bare JSON array | `{"pairs": [...]}` / `{"currencies": [...]}` | `handlers/pair_handler.go`, `handlers/currency_handler.go` |
| B2 | `listLiquidityCommits` | optional query `pool`; bare array | mandatory query **`pool_pair`** (400 without it); `{"commits": [...], "count": n}` | `handlers/liquidity_handler.go` — `ListCommits` |
| B3 | `listLiquidityCommits` items, `getLiquidityCommit` | *(free-form)* | **PascalCase** keys — `CommitID`, `PoolPair`, … — because `domain.PoolCommit` has GORM tags but no JSON tags | `internal/domain/pool_commit.go`; corroborated by `frontend/apps/governance/src/types/liquidity.types.ts` |
| B4 | `listLiquidityPositions` | no parameters; bare array | mandatory `pool_pair`, optional `provider_id`; `{"pool_pair", "positions", "count"}`, items keyed `provider_bank_id` | `handlers/liquidity_handler.go` — `ListPositions` |
| B5 | `getLPBalance` | *(free-form)* | `{lp_shares, lp_total_supply, share_percentage, holder}` — note `lp_total_supply`, not `total_supply` | same file — `GetLPBalance` |
| B6 | `commitLiquidity` | **200**, free-form | **201** with the eight-field `CommitResult`; plus 409/403 (`error_code`) and 422 (`code`) | same file — `CommitLiquidity`; `services/liquidity_provision_service.go` |
| B7 | `proposePair` / `confirmPair` | **200**, free-form | **201** / 200 with `{pair_id, status, tx_hash, amm_address}` / `{pair_id, status, tx_hash}` — *not* the registry record | `services/pair_service.go` |
| B8 | `registerHubCurrency` / `removeHubCurrency` | **200** / no body | **201** `{symbol, tx_hash}` / 200 `{symbol, tx_hash}` | `services/currency_service.go` |
| B9 | `getPoolStatus` | *(free-form)* | `pending_commits[]` is `PendingCommitSummary` and `counterpart_commit` is `CounterpartCommit` — two different shapes, neither of them the commit record | `services/pool_status_service.go` |
| B10 | `bridgeLockMint` | **200** | **201** | `handlers/bridge_handler.go` — `LockMint` |
| B11 | `sovereignAddLiquidity` | **200** | **201** | `handlers/liquidity_handler.go` — `SovereignAddLiquidity` |
| B12 | `bridge/positions` `state` filter example | `PENDING, ACTIVE, CLOSED` | `PENDING` and `CLOSED` do not exist; the real set is `LOCKING, ACTIVE, BURNING, BURNED, RELEASED, RECONCILIATION_REQUIRED` | `internal/domain` |

**Machine-readable error codes are not uniformly present.** `bridgeLockMint`'s 422 carries
`error_code` on the transfer-limit branch and nothing on the enqueue branch;
`sovereignAddLiquidity`'s 400 carries `code` only on the not-a-sovereign-pair branch. The
contract declares those two statuses as `anyOf` of the coded and bare shapes. A client must
never require the code.

**Three spellings of the same idea coexist in one gateway:** `error_code` on the AMM, bridge and
swap handlers; `code` on the registries and the liquidity handler's coded refusals; nothing at
all on `/api/v1`. The liquidity handler mixes both across statuses of the *same* operation.

---

## Pool-pair identifiers: five spellings, none canonical

Take `pair_id` verbatim from `GET /api/v2/amm/pairs`. **Never construct one.** Every spelling
below is real and present in the delivered artefacts:

| Form | Example | Where it appears |
|---|---|---|
| Symbol-joined | `tCeBM_BRL-tCeBM_ARS` | Scenario B `docs/openapi.yaml`, Postman collection |
| Slash-separated | `W-tCeBM_BRL/W-tCeBM_ARS` | Scenario B `docs/openapi.yaml`, Postman collection |
| Sovereign | `W-BRL-ARS` | `handlers/cross_currency_bridge_out_handler.go`, sovereign seeding, NOC backend |
| Quote-style | `W-BRL-W-ARS` | `services/swap_quote_generator.go` — normalised to the sovereign form by `normalizeSovereignPoolPair` in `central_bank_pool_client.go` |
| Bare-currency | `BRL-USD` | `handlers/governance_scenariob_handler.go:136` — the circuit breaker's default when `pair` is omitted; `services/pair_router.go` |

The **sovereign** form is the only one that fits `/api/v2/amm/pool/{pair}/status`, a single path
segment, without percent-encoding. That is why `mocks/amm/` uses it (`W-CRC-CLP`) while the
contract examples and the tutorials use the slash form with `%2F`. Both are legitimate; neither
is canonical; the currency symbols in `hub/currencies` are **not** the components of a pair id.

---

## What this register is not

It does not list the platform's *internal* inconsistencies that the Toolbox merely reproduces —
`HTLCLock.state` prefixed while `searchHTLC`'s `state` filter is bare, `ErrorCodeResponse`
declared and referenced by zero operations, amounts documented as "big integer as string" in one
place and "base units (decimal string)" in another. Those live in the contract READMEs, because
they are properties of the platform surface rather than disagreements with it.
