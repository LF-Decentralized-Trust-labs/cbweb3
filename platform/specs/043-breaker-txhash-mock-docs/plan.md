# Implementation Plan: Circuit-Breaker Transaction-Hash Visibility and Mock-vs-Live Documentation Reconciliation

**Branch**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/043-breaker-txhash-mock-docs/spec.md`

## Summary

Carry the on-chain transaction hash produced by each Scenario B circuit-breaker write action outward — from the chain-facing client, through the api-gateway service and HTTP handlers, into the governance portal — and render it on the Circuit Breaker page as a copyable raw hash. Replace the mock-capable at-a-glance breaker indicator with a condition **derived** from the same on-chain V2 status the page uses — halted if any pair is halted — so the chrome can no longer contradict the page. Separately, correct four inaccurate mock-vs-live statements across `docs/user-manuals/`, fix the stale bank Settings copy, and delete an inert `VITE_USE_MOCKS` from two bank `.env.example` files.

The transaction hash already exists at the chain boundary for two of the three write actions and is discarded; for the third (propose resume) it is present in the receipt but the client returns only the proposal identifier. No new chain round-trips are introduced anywhere.

## Technical Context

**Language/Version**: Go 1.26+ (api-gateway, shared blockchain client); TypeScript 5.x / React 18 (governance portal)
**Primary Dependencies**: Fiber v2 (HTTP), GORM v2 + Postgres (persistence), go-ethereum (EVM client), Zustand (portal state), existing `httpClientV2` (`/api/v2`)
**Storage**: Postgres — existing `CircuitBreakerSignature` rows (`on_chain_tx_ref` column already present) and `scenario_b_risk_control_states`. No schema change required.
**Testing**: `go test` for the api-gateway service and handlers via `make test.api-gateway`. Frontend: **Vitest in its default Node environment only** — the governance app declares `vitest` alone, has no `test` block in `vite.config.ts`, and has no `jsdom`/`happy-dom` or `@testing-library/react`. Component-rendering tests are therefore not available and FR-026 forbids adding the dependencies, so portal coverage is store- and api-level (the `features/liquidity/format.test.ts` pattern), with rendering verified manually per quickstart.
**Target Platform**: Linux containers via Docker Compose (Scenario B stack)
**Project Type**: Web application — Go backend services plus React frontends
**Performance Goals**: No change to breaker action latency. Hash propagation adds no chain call, so pause/resume timings are unchanged.
**Constraints**: No proto or `buf`/`protoc` regeneration — the affected path is the REST V2 flow, not the legacy compliance proto. Hash rendering must never gate the breaker action. No block-explorer URL is configured, so the hash renders as a plain value. No new runtime or dev dependencies (FR-026). Every new `.go`/`.ts` file must carry `// SPDX-License-Identifier: Apache-2.0` as its first line — mandatory per `CLAUDE.md` and gated in CI by the `License Headers` workflow (FR-027).
**Scale/Scope**: 1 shared chain client, 4 api-gateway files (+2 test files, one of them new), 9 governance portal files (types, api, V2 store, page, hook, V1 store, three indicator consumers) plus 2 new portal test files, 5 documentation files, 2 env templates, 1 bank portal page. Roughly 30 changed files.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Assessment | Verdict |
|---|---|---|
| **I. Scenario-Scoped Independence** | All code changes are Scenario B (`scenario-b/backend/...`, `scenario-b/frontend/...`, `backend/shared/blockchain/scenariob/amm/` — the shared client is already scenario-partitioned by path). The only Scenario A touches are one manual line and one `.env.example` line, both documentation-class and split into their own commit/PR per FR-024. No PR spans both scenarios' code. | PASS |
| **II. Privacy by Design** | A transaction hash is a public ledger identifier. No PII, no amounts, no plaintext value data is added to any surface. Breaker actions carry no counterparty data. | PASS |
| **III. Atomic Settlement Guarantee** | Untouched. FR-025 forbids altering the breaker decision model: pause stays 1-of-N, resume stays 2-of-N quorum. No settlement path, token path or partial-settlement behaviour is modified. | PASS |
| **IV. Compliance Gate Before Participation** | Untouched. No authentication, authorisation or compliance check is added, removed or bypassed. The badge change alters only which read source feeds a display, not any gate. | PASS |
| **V. Test-First at Every Layer** | Every implementation task is preceded by a failing test. Backend: Go tests asserting `tx_hash` presence/absence across all four V2 responses. Frontend: store- and api-level Vitest cases asserting the hash is captured from each response, retained across a status refresh, and that the network-wide condition is derived correctly. Component rendering is verified manually because the portal has no DOM test environment and FR-026 forbids adding one — the trade is stated in the spec's Assumptions rather than left implicit. No contract changes, so no Foundry work. | PASS (with a stated limit) |
| **VI. Observability and Auditability** | Directly strengthened. The constitution names "circuit-breaker state transitions" as a Scenario B lifecycle event that must be reconstructible. Surfacing the on-chain hash makes each transition traceable to a ledger record. Two observability gaps are *recorded but deliberately not fixed* (FR-022 premature-LIVE for quorum > 2, FR-023 absent audit-trail entries) — documenting a known gap is not swallowing it. | PASS (improves) |

**Result: all gates pass. No violations, so Complexity Tracking is empty.**

Note on FR-023: breaker actions currently write no governance audit-trail entries. This is a genuine pre-existing observability gap discovered during clarification. It is recorded for separate triage rather than absorbed here, because fixing it expands scope well beyond the R2-H-2 residual and was explicitly ruled out by the project owner. It is *not* a silent failure — nothing is swallowed; the actions are persisted to `CircuitBreakerSignature`.

## Project Structure

### Documentation (this feature)

```text
specs/043-breaker-txhash-mock-docs/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output — V2 breaker HTTP contract
│   └── circuit-breaker-v2.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
scenario-b/
├── backend/
│   ├── shared/blockchain/scenariob/amm/
│   │   └── client.go                                   # ProposeResume: also return receipt tx hash
│   └── services/api-gateway/
│       ├── internal/app/
│       │   └── amm_adapter.go                          # widen ProposeResume/SignResume returns
│       ├── internal/services/
│       │   ├── circuit_breaker_service.go              # propagate txRef; TxHash on status; persist on all three
│       │   └── circuit_breaker_service_test.go         # NEW FILE — needs SPDX header; failing first
│       └── internal/http/handlers/
│           ├── governance_scenariob_handler.go         # tx_hash in 4 JSON bodies
│           └── governance_scenariob_handler_test.go    # EXISTS — extend; holds a caller fake
└── frontend/apps/governance/src/
    ├── types/circuit-breaker-v2.types.ts               # tx_hash? on BOTH CircuitBreakerV2Status and ProposeResumeResponse
    ├── services/api/circuit-breaker-v2.api.ts          # pass tx_hash through all four calls
    ├── features/circuit-breaker/
    │   ├── circuit-breaker-v2.store.ts                 # proposeResume reducer must carry tx_hash
    │   └── circuit-breaker-v2.store.test.ts            # NEW — needs SPDX header
    ├── pages/CircuitBreakerPage.tsx                    # render + copy affordance
    ├── hooks/useCircuitBreaker.ts                      # SEAM — repoint to derived network-wide V2 condition
    ├── stores/circuit-breaker.store.ts                 # V1 store: derive from V2 per-pair statuses
    └── components/layout/{AppLayout,Sidebar}.tsx, pages/DashboardPage.tsx   # consume .state only

docs/user-manuals/
├── README.md                                           # status table: reattribute mock toggle
├── scenario-a/governance.md                            # line 4 — Scenario A commit
└── scenario-b/{governance.md,bank.md}                  # lines 5 — Scenario B commit

scenario-a/frontend/apps/bank/.env.example              # drop inert VITE_USE_MOCKS — Scenario A commit
scenario-b/frontend/apps/bank/.env.example              # drop inert VITE_USE_MOCKS — Scenario B commit
scenario-b/frontend/apps/bank/src/pages/SettingsPage.tsx # correct stale Environment copy
docs/R2-H-2.md                                          # annotate mis-scoping (FR-021)
```

**Structure Decision**: Existing Scenario B web-application layout — Go api-gateway plus React governance portal — with a shared, scenario-partitioned chain client. No new directories, packages or services. The work threads one value through four existing layers and corrects text in place.

## Phase 0 — Research

All unknowns were resolved by direct code inspection during specify/clarify; nothing required external research. Findings are consolidated in [research.md](./research.md). Summary of decisions:

1. **Where the hash comes from per action** — resolved by reading the client and adapter. Pause and sign-resume already produce it; propose-resume discards the receipt; execute-resume makes no chain call. Drives FR-002 through FR-004.
2. **Link vs plain value** — no `VITE_*EXPLORER*` variable exists anywhere in Scenario B, so a plain copyable value it is. Drives FR-008.
3. **Badge source** — the V1 `governanceApi.getCircuitBreaker` is `useMocks`-gated; V2 is not. Repoint rather than un-gate, so there is a single source of truth. Drives FR-011/FR-012.
4. **Scenario A mock path is inert** — nothing consumes `useMocks`, nothing imports `mock-db`. Determines the exact wording permitted by FR-016.
5. **No proto regeneration** — confirmed the V2 flow is REST end to end.

## Phase 1 — Design & Contracts

**Artifacts generated**: [research.md](./research.md), [data-model.md](./data-model.md), [contracts/circuit-breaker-v2.md](./contracts/circuit-breaker-v2.md), [quickstart.md](./quickstart.md).

Design decisions:

- **`tx_hash` is optional everywhere.** It is omitted (not empty-string) when no chain is wired, so clients can distinguish "no chain" from "not returned" per FR-009. Go uses `json:"tx_hash,omitempty"`; TypeScript uses `tx_hash?: string`.
- **Status carries the pair's latest hash**, sourced from the most recent `CircuitBreakerSignature` row for that pair by `SignedAt`, regardless of signer — per the clarified FR-005/FR-006. This is what makes the value survive reload and appear identically for every central bank.
- **Persist on all three write actions.** `OnChainTxRef` is currently written only by `Pause`; `ProposeResume` and `SignResume` must populate it too, otherwise the status lookup has nothing to read after a reload.
- **Widening is additive.** `ProposeResume` gains a second return value and `SignResume` changes from `error` to `(string, error)`. Both are internal interfaces (`AMMCircuitBreakerCaller`) with a single production implementation plus test fakes, so the blast radius is small and compile-checked. Go's implicit interfaces mean the fakes are structural — the compiler finds every one.
- **`ExecuteResume` stays unchanged** — it performs no chain call by design, so it has no hash to return.
- **Two frontend types carry the hash, not one.** `pause` and `resume-sign` are typed as `CircuitBreakerV2Status`, but `resume-request` returns the separate `ProposeResumeResponse`. Both need the optional field, or the resume proposal's hash is unrepresentable.
- **The hash rides on `cbStatus`; no new store field.** Because `pause`/`signResume` already replace `cbStatus` wholesale, they carry the hash for free. The exception is `proposeResume`, whose reducer hand-rebuilds `cbStatus` field by field and already silently drops `resume_signatures`/`resume_quorum` — it must be updated to carry the hash, or the proposal's hash is lost the moment it arrives.
- **The badge is a derived network-wide condition.** The indicator claims "swaps are globally halted" while V2 status is per-pair, so the fix is not a source swap but a derivation: fetch the pair list via the existing `ammPairsApi.getPairs()`, poll each pair's V2 status with the existing `usePolling` hook, and report halted if any pair is halted (FR-012). No new endpoint and no backend change. `hooks/useCircuitBreaker.ts` is the single seam — a one-line hook over the V1 store — so repointing it updates `AppLayout`, `Sidebar` and `DashboardPage` together. All three read only `.state`, so dropping the V1-only `updatedAt`/`updatedBy` costs nothing.

### Post-design Constitution re-check

Re-evaluated after the above: still **PASS on all six principles**. The design adds no dependency, no schema change, no proto change, no new service, and no cross-scenario coupling. Complexity Tracking remains empty.

## Complexity Tracking

No constitution violations. Table intentionally empty.

## Sequencing and PR split

Three commits/PRs, ordered so each is independently reviewable and scenario-isolated:

1. **Scenario B — breaker tx hash** (FR-001 to FR-010, plus FR-022, FR-026, FR-027). Backend chain client → adapter → service → handlers → portal. Ship and verify on a chain-wired stack.
2. **Scenario B — badge consistency** (FR-011 to FR-014). Separated from PR 1 because it touches shared portal chrome and introduces a derived network-wide condition, which deserves review on its own rather than riding along with the backend change.
3. **Docs + Scenario B text/config** (FR-015, FR-017 to FR-021, FR-023). Manual corrections, README table, bank SettingsPage copy, Scenario B `.env.example`, R2-H-2 annotation.
4. **Scenario A text/config only** (FR-016, plus the Scenario A `.env.example` line). Two lines, separated purely to keep the scenario-isolation rule clean.

Workstream 2 of the rescope plan — Scenario A vestigial-AMM retirement — is **out of scope** and remains available as separate future work.
