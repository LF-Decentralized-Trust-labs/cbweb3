# Phase 0 Research: Circuit-Breaker Transaction-Hash Visibility

**Feature**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05

All questions were resolved by direct inspection of the repository at `develop`. No external research was required — every unknown was a question about what the existing code does.

---

## R1. Where does the on-chain transaction hash actually come from, per action?

**Decision**: Treat the four V2 breaker operations as three different cases, not one uniform case.

| Operation | Chain-facing reality | What must change |
|---|---|---|
| Pause | `Client.PauseCircuitBreaker` returns the tx hash; `ammAdapter.PauseCircuitBreaker` passes it through; `CircuitBreakerService.Pause` receives it as `txRef`, persists it to `OnChainTxRef`, then returns only `error` | Service return + status + handler + UI |
| Propose resume | `Client.ProposeResume` submits the tx, gets a receipt, then extracts and returns **the proposal ID from the `LogResumeProposed` topic** — the receipt (and thus the tx hash) is discarded | Shared client must return both; then adapter, service, handler, UI |
| Sign resume | `Client.SignResume` returns `(string, error)` where the string is the tx hash from `evm.SubmitTx`; `ammAdapter.SignResume` discards it with `_, err = c.SignResume(...)` and returns bare `error` | Adapter interface must widen; then service, handler, UI |
| Execute resume | `ammAdapter.ExecuteResume` performs **no chain call at all** and returns `nil` — the contract auto-unpauses inside `signResume` once quorum is met | Nothing. It has no hash by design |

**Rationale**: The rescope plan asserted the adapter "already has the hash in hand" for every action. That is true for pause and sign-resume, but not for propose-resume, and meaningless for execute-resume. Building on the plan's assumption would have produced a propose-resume response with either a missing hash or — worse — the proposal ID mislabelled as one.

**Alternatives considered**:
- *Re-query the chain for the hash after the fact.* Rejected: adds a round-trip, is racy, and the value is already in the receipt.
- *Return the proposal ID as `tx_hash` for propose-resume.* Rejected outright: they are different identifiers with different meanings; conflating them would corrupt the audit trail the feature exists to provide.
- *Skip propose-resume.* Offered to the project owner as an option and declined in favour of full coverage.

**Consequence**: `ExecuteResume` being a no-op is also the root cause of the known `{"state":"LIVE"}` caveat — the handler reports resumed after calling something that does nothing, which is correct only because the contract already resumed during the final signature. This is why the caveat is real for quorum > 2 (FR-020).

---

## R2. Should the hash be rendered as a hyperlink to a block explorer?

**Decision**: Render a plain, copyable, selectable value. No link.

**Rationale**: No block-explorer base URL is configured anywhere in Scenario B — no `VITE_*EXPLORER*` variable exists in any `.env.example`, and no frontend source references one. Introducing a link would require inventing configuration that the deployment does not have.

**Alternatives considered**:
- *Add an explorer URL variable now.* Rejected as speculative: no explorer is deployed for these networks, so the variable would be inert — precisely the anti-pattern this feature is removing elsewhere (FR-018).
- *Hard-code a public explorer.* Rejected: these are permissioned Besu networks, not public chains.

**Forward compatibility**: FR-008 requires the presentation to become a link later without changing how the value is obtained, so the store holds a bare hash string and only the page decides how to render it.

---

## R3. How should the at-a-glance badge be made consistent with the breaker page?

**Decision**: Repoint the badge at the V2 on-chain status source that the Circuit Breaker page already uses.

**Rationale**: The badge is fed by `stores/circuit-breaker.store.ts`, which calls `governanceApi.getCircuitBreaker` — a V1 call that branches on `useMocks` and returns `mockDb.getCircuitBreaker()` in the default configuration. The V2 path (`httpClientV2`) has no mock branch and reads on-chain `IsPaused()` as source of truth. So in a default run the chrome can claim one thing while the page claims another.

**Alternatives considered**:
- *Un-gate the V1 endpoint (remove its `useMocks` branch).* Rejected: leaves two independent read paths that can still drift, and silently changes behaviour for any other V1 consumer.
- *Leave the badge alone.* Rejected: a control surface that contradicts itself about whether the network is halted is a safety problem, and the fix is small.

**Open follow-up (not in scope)**: once the badge moves to V2, the V1 `getCircuitBreaker` api/store may become dead. Whether to delete them is left to implementation review rather than pre-decided here, since other consumers must be checked at the time.

---

## R4. Is the Scenario A governance mock path live or dead?

**Decision**: Dead. Document the Scenario A governance portal as live-backed, with no selectable mock mode.

**Rationale**: In `scenario-a/frontend/apps/governance/src`, `useMocks` is exported from `http-client.ts` and consumed by **nothing**; `services/mocks/mock-db.ts` is imported by **nothing**. By contrast, in `scenario-b/frontend/apps/governance/src`, `useMocks` is consumed by `governance.api.ts` and `registry.api.ts`. The originating ticket attributes the mock-defaulting behaviour to Scenario A; it belongs to Scenario B.

**Consequence for wording**: because the Scenario A cleanup (Workstream 2) is out of scope, the corrected manual must describe current behaviour without promising that the inert flag or unused mock data will be removed. FR-014 states this constraint explicitly.

---

## R5. Does any of this require regenerating the protocol contract?

**Decision**: No.

**Rationale**: The affected flow is the REST V2 governance path (`/api/v2`, Fiber handlers, `httpClientV2`). The compliance service's legacy `ToggleCircuitBreaker` proto RPC is a different, older path and is untouched. `buf`/`protoc` are not in the toolchain, so avoiding proto changes was a hard design constraint — and it is satisfied without compromise.

---

## R6. Is a database migration needed?

**Decision**: No.

**Rationale**: `domain.CircuitBreakerSignature` already declares `OnChainTxRef string \`gorm:"column:on_chain_tx_ref"\``. The column exists. The gap is that only `Pause` populates it — `ProposeResume` and `SignResume` construct their signature records without it. Populating an existing column is not a migration.

**Consequence**: status must read the pair's latest signature row to answer "what was the most recent action's hash", which is only correct once all three write paths populate the column.
