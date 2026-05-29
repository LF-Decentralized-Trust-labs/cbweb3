# Research: CB Liquidity Frontend Refactor (009)

**Phase 0 output** — all NEEDS CLARIFICATION resolved before Phase 1 design.

---

## Decision 1: `cancelCommit` — does it still send `provider_id`?

**Decision**: Yes. The frontend MUST continue sending `provider_id` as a query parameter.  
**Rationale**: The `CancelCommit` handler in `liquidity_handler.go` was NOT updated in 008-fix-cb-liquidity. It still reads `c.Query("provider_id")` and returns HTTP 400 if absent. The frontend works around this by deriving the value from Zustand store state (`currentBankId` or auth profile `bankId`) rather than requiring the caller to supply it explicitly. The public store action signature is simplified to `cancelActiveCommit(commitId: string)` while the internal API call still sends `?provider_id=<bankId>`.  
**Alternatives considered**: Removing the param entirely → would break cancel with HTTP 400. Backend fix → out of scope for this frontend-only refactor.

---

## Decision 2: `GET /api/v2/amm/liquidity/commits` field casing

**Decision**: PascalCase. The `CommitListItem` type must use PascalCase field names: `CommitID`, `PoolPair`, `ProviderID`, `Side`, `Amount`, `Status`, `OnChainCommitID`, `CounterpartCommitID`, `CreatedAt`, `ExpiresAt`.  
**Rationale**: `domain.PoolCommit` has only `gorm:` struct tags and no `json:` struct tags. Go's standard JSON encoder outputs struct field names verbatim (i.e., the Go identifier casing). `OnChainCommitID` is `*[]byte` and serializes as a base64 string or `null`.  
**Alternatives considered**: Adding a snake_case mapping layer at the API client — unnecessary complexity when a typed `CommitListItem` with PascalCase keys is cleaner and accurate.

---

## Decision 3: `GET /api/v2/amm/liquidity/positions` — auto-filter by JWT or explicit param?

**Decision**: Explicit `provider_id` param required. For wizard Step 4, the frontend MUST pass the authenticated CB's `bankId` as `provider_id`.  
**Rationale**: The `ListPositions` handler does NOT auto-filter by JWT. When `provider_id` is omitted, it calls `FindByPoolPair` and returns all positions for the pool. The frontend MUST pass `provider_id` explicitly to show only the current CB's positions. Response uses snake_case DTO fields (`lp_id`, `provider_bank_id`, `token_a_contributed`, `token_b_contributed`, etc.) with a top-level `pool_pair` field.  
**Alternatives considered**: Omitting `provider_id` and filtering client-side — fragile and shows other CBs' positions.

---

## Decision 4: `SOVEREIGN_FLOW_PHASE` — add `CANCELLED` state or reuse `FAILED`?

**Decision**: Add `CANCELLED` as a distinct terminal phase in `SOVEREIGN_FLOW_PHASE`.  
**Rationale**: `CANCELLED` is a first-class terminal state returned by the backend (commit cancelled by operator or system). Using `FAILED` would conflate two different user outcomes (intentional cancellation vs. error/timeout). The wizard must display different messaging for each.  
**Alternatives considered**: Reusing `FAILED` — confusing UX for the operator who cancelled intentionally.

---

## Decision 5: Phase machine — initial `sovereignPhase` value

**Decision**: Keep `SOVEREIGN_FLOW_PHASE.LOCK_MINT` as the initial value. Add `IDLE` to the type enum as described in FR-023, but initialize the store to `LOCK_MINT` to preserve existing wizard auto-start behavior.  
**Rationale**: The current wizard starts directly at Step 1 (LOCK_MINT). Introducing a new `IDLE` value would require a UI change to show a "start" button — beyond the scope of this refactor. The spec's FR-023 lists `IDLE` in the phase machine but the existing behavior of initializing to `LOCK_MINT` is simpler and functionally correct.  
**Alternatives considered**: Adding `IDLE` pre-state → additional UI change not in spec scope.

---

## Decision 6: `on_chain_commit_id` type

**Decision**: `on_chain_commit_id: string | null` in `CommitResult`. The API returns a base64-encoded string (from `*[]byte`) or `null`.  
**Rationale**: Backend `OnChainCommitID` field is `*[]byte`. When nil, JSON is `null`; when set, it's a base64 string. The UI should display `null` as "pending on-chain confirmation" (SC-009).  
**Alternatives considered**: `string | undefined` — less precise; `null` is explicit from the Go encoder.

---

## Decision 7: `waitForBridgeActive` — keep as a first-class step or fallback only?

**Decision**: Keep as first-class step invoked in the `lockMint` store action. The spec (FR-019) says `lockMint` success → transitions `sovereignPhase` to `BRIDGE_ACTIVE`. The `waitForBridgeActive` function is called after a successful `lockMint` response to poll for bridge `ACTIVE` state before allowing Step 2.  
**Rationale**: The prior architecture used `waitForBridgeActive` only as a fallback when `BRIDGE_POSITION_NOT_ACTIVE` was returned by commit. Now it is a required intermediate step between Step 1 (lock-mint) and Step 2 (commit).  
**Alternatives considered**: Keeping the fallback-only pattern — would leave a gap where Step 2 is enabled before the bridge is ready.

---

## Summary Table

| Question | Answer | Source |
|---|---|---|
| `cancelCommit` needs `provider_id`? | Yes, send as query param; derive from store state | spec clarifications |
| `listCommits` field casing? | PascalCase (no `json:` tags on domain struct) | spec clarifications |
| `listPositions` auto-filter by JWT? | No; pass `provider_id` explicitly | spec clarifications |
| Add `CANCELLED` phase? | Yes, distinct terminal state | spec FR-023 |
| Initial phase value? | Keep `LOCK_MINT` (no `IDLE` pre-state in wizard) | scope decision |
| `on_chain_commit_id` type? | `string \| null` | backend `*[]byte` behavior |
| `waitForBridgeActive` role? | First-class step in `lockMint` action | spec FR-019/FR-023 |
