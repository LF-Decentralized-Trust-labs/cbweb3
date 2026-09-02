# Changelog — Hub-and-Spoke Settlement Contract (Scenario B)

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Contract versions track the **CBWeb3 API Gateway version** they describe, so this file
starts at 2.3.0.

## [2.3.0] — 2026-08-20

### Corrected before publication (2026-08-21)

An independent verification pass against the delivered Scenario B handlers found nine
places where this contract described the platform's OpenAPI *document* while claiming
`x-cbweb3-source: implementation-observed`. All are corrected; every one is traced to
delivered Go source and recorded in [`../../DIVERGENCES.md`](../../DIVERGENCES.md).

- `listLiquidityCommits`: the query parameter is **`pool_pair`** and is **mandatory**
  (400 without it); the body is `{commits, count}`, not a bare array.
- `LiquidityCommit`: the persisted `domain.PoolCommit` has GORM tags but **no JSON tags**,
  so it marshals with **PascalCase** keys — `CommitID`, `PoolPair`, `Side`, `Status`,
  `CounterpartCommitID`, `OnChainCommitID` (base64, not hex), `CreatedAt`, `ExpiresAt`.
  Applies to `getLiquidityCommit` and to the `commits` array.
- `listLiquidityPositions`: `pool_pair` is **mandatory**, `provider_id` is an
  undocumented optional filter, the body is `{pool_pair, positions, count}`, and each
  item is the ten-field `LPPositionDTO` keyed **`provider_bank_id`** (not `provider_id`).
  400 / 500 / 501 declared.
- `getLPBalance`: `total_supply` renamed to **`lp_total_supply`**; `share_percentage` and
  `holder` added; 501 and 502 declared.
- `commitLiquidity`: **201**, not 200. The body is the eight-field `CommitResult`.
  409 (`error_code`), 403 (`NOT_AUTHORIZED_LP`), 422 (`code`: `COMMIT_SIDE_NOT_CONFIGURED`,
  `BRIDGE_POSITION_NOT_ACTIVE`, `INSUFFICIENT_BALANCE`, `ON_CHAIN_COMMIT_FAILED`) and 500
  declared.
- `proposePair` / `confirmPair`: the success bodies are `ProposePairResult`
  `{pair_id, status, tx_hash, amm_address}` and `ConfirmPairResult`
  `{pair_id, status, tx_hash}` — **not** the nine-field `AmmPair` registry record, which
  is now used only for `listPairs` items. 404 and the 422 fallback declared on both.
- `registerHubCurrency` / `removeHubCurrency`: the bodies are
  `RegisterHubCurrencyResult` / `RemoveHubCurrencyResult` `{symbol, tx_hash}`. `HubCurrency`
  is now used only for `listHubCurrencies` items.
- `PoolStatusResponse`: `pending_commits[]` is `PendingCommitSummary` and
  `counterpart_commit` is `CounterpartCommit` — two distinct shapes, neither of them the
  commit record. `suggested_match_amount` is empty when no FX rate is available.
- `bridgeLockMint` 422 and `sovereignAddLiquidity` 400/422 no longer **require** a
  machine-readable code: only some branches emit one. Each is an `anyOf` of the coded and
  bare shapes. `sovereignAddLiquidity`'s 500 declared.

### Changed

- The pool-pair spelling count is settled at **five** across every artefact, enumerated
  identically with source citations (`tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`,
  `W-BRL-ARS`, `W-BRL-W-ARS`, `BRL-USD`). Previously three files said three and two said
  four, and none named the fourth.

### Added

- Initial publication. Derived from the delivered **CBWeb3 API Gateway v2.3.0** OpenAPI
  document (`scenario-b/backend/services/api-gateway/docs/openapi.yaml`, 4,970 lines),
  cross-checked against the gateway's Go router, handlers, services and domain packages,
  and against its shipped end-to-end integration test.
- 59 operations across 52 paths covering the whole Scenario B community surface: token
  balances, the escrow lifecycle (deposit / Reserve Tokenisation / redeem), AMM quotes and
  swaps including the orchestrated cross-currency swap, bridge lock-mint / burn-unlock /
  positions, liquidity provisioning by both commit-reveal and sovereign
  escrow-and-finalize, hub reconciliation, the hub currency and AMM pair registries, the
  asymmetric circuit breaker, transfer limits, and oversight disclosure. `operationId`
  values match the platform's own.
- `CookieAuth` plus `BearerAuth` — the latter declared because Scenario B genuinely
  accepts bearer tokens, but **only** on the `/api/v2` routes guarded by `RequireAnyAuth`.
  The swap endpoints are cookie-only, and this is stated per operation. No CSRF scheme is
  declared, because the platform implements none.
- Four coexisting error shapes modelled honestly rather than normalised: `ErrorResponse`
  `{error}`, `AmmErrorResponse` `{error, error_code, …}`, `RegistryErrorResponse`
  `{error, code}` and `SovereignSeedError` `{error, error_code, hint}`.
- Fourteen documented divergences between the platform's OpenAPI document and the delivered
  gateway, marked inline and tabulated in `README.md` — three of them blocking for any
  client written strictly against the upstream document (`amountOut` vs `amount_out`, the
  cross-currency quote's three undeclared required parameters, and the three
  circuit-breaker writes that declare no request body upstream but require one).
- Extensive documentation of the slippage-buffer / residue mechanic, which is barely
  present upstream and is the difference between verifying a settlement and merely
  observing an HTTP 200.

### Notes on provenance

- The platform deliberately declares roughly thirty `/api/v2` request and response bodies
  as free-form objects, directing readers to the handler packages for the real field
  contracts. Where a real shape was recoverable from the delivered Go code or the
  platform's own executable end-to-end test, it is published here and marked with
  **`x-cbweb3-source: implementation-observed`**. Schemas without that marker are
  transcribed from the platform's OpenAPI document. The vendor may change an
  implementation-observed shape without touching its OpenAPI, so conformance assertions
  against those should be treated as softer.
- Eleven operations that the platform document leaves bodiless or free-form are published
  here from the delivered code, each marked `x-cbweb3-source: implementation-observed`:
  `pauseCircuitBreaker`, `proposeResume`, `signResume` and `getV2CircuitBreakerStatus`
  (the three writes declare **no `requestBody` upstream at all**, yet their handlers
  require one — a client built from the upstream document cannot operate the breaker);
  `openDisclosure`, `signDisclosure` and `getDisclosureStatus`; `sovereignAddLiquidity`;
  `mintAndApprove` and `approveAMM`; and `getHubLiquidityConfig`. Where a shape was
  recoverable from nowhere stable the object is left open instead. Nothing was invented
  to fill a gap.
- Three state vocabularies used by this surface — bridge state, swap operation status and
  pool status — are **absent from the platform's OpenAPI entirely** and were recovered from
  the delivered Go code. Bridge state (`internal/domain`) and swap operation status are
  documented in prose and left as **free strings**, rather than published as enums the
  platform has not committed to. Pool status is the one exception: it is published as an
  enum (`EMPTY | PENDING_COUNTERPART | ACTIVE`) marked
  `x-cbweb3-source: implementation-observed`, because the delivered gateway derives it
  exhaustively from the two reserves in `internal/services/pool_status_service.go` and
  gates swaps on it.

### Relationship to the previous Toolbox contracts

- There is **no compatibility guarantee** with the deleted `openapi_pvp_v0.1.0.yaml`, and
  no relationship to it: that contract described a flat `/fx/agreement` + `/htlc/*` surface
  that was never implemented on any gateway, and Scenario B has never had either an FX
  agreement surface or HTLC endpoints of any kind.
- No Toolbox contract for Scenario B existed before this one. The `contracts/amm/`
  directory was previously empty.
