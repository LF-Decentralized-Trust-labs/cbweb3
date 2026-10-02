# Phase 1 Data Model: Scenario A AMM Retirement

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06

No schema migration. No persisted field changes. This is a deletion feature: the model changes are the **disappearance** of in-memory and configuration structures, plus one interface that narrows from two implementations to one. This document records what goes, what stays, and the rules that keep the halt control behaving identically.

---

## Persisted state (unchanged — no migration)

### Compliance parameters backing the halt flag

The halt decision is stored as compliance parameters, not as a dedicated table.

| Parameter key | Meaning | After |
|---|---|---|
| `circuit_breaker_paused` | Whether the system is halted | **unchanged** |
| `circuit_breaker_updated_by` | Who last changed it | **unchanged** |
| `circuit_breaker_updated_at` | When it last changed | **unchanged** |

**Rules**:

- **D-1**: These parameters are already the source of truth whenever no swap engine is wired, which is the Scenario A default. Retirement makes them the **only** source of truth. No key is added, renamed or removed.
- **D-2**: A recorded halt state MUST survive the retirement unchanged, including in an environment that previously ran the on-chain path. The retirement changes which code reads the value, never the value.
- **D-3**: No migration, backfill or data repair is implied. Any change to these keys would indicate a defect in the retirement, not a required step.

---

## Removed structures

### Chain-client configuration (deleted)

The swap-engine chain client and its configuration struct disappear entirely.

| Structure | Today | After |
|---|---|---|
| Chain-client config carrying the engine address, RPC URL, chain id and signing key | Constructed when the address and key are both present | **deleted** |
| Generated contract bindings for the engine | Present under the client package | **deleted** |
| The engine's client constructor and its `Breaker` implementation | Built at service start-up, else nil | **deleted** |

### Provisioning address record (field removed)

| Field | Today | After |
|---|---|---|
| Swap-engine address on the deployed-addresses record | Optional; empty unless an operator records it, parsed from the deployed-addresses file | **removed** |
| Swap-engine address on the entity-environment structure | Optional; rendered into the environment template | **removed** |
| Swap-engine address assigned by the environment-rendering step | Copied from the address record | **removed** |

**Rules**:

- **D-4**: Reading a **previously written** provisioning record that still contains the address MUST NOT fail. An unknown key in that file is ignored, not rejected, so an operator upgrading from an older provisioning run is not blocked.
- **D-5**: The rendered environment MUST no longer emit the variable. An environment that still supplies it externally is simply ignored — the variable becomes inert-and-undocumented rather than inert-and-advertised.

---

## Narrowed interface: the halt control

This is the only behavioural structure that changes shape, and it narrows rather than widens.

| Aspect | Today | After |
|---|---|---|
| Breaker abstraction consumed by the compliance service | An interface with a chain-backed implementation, injected as nil when unconfigured | **removed** — no abstraction, no injection |
| Service field holding it | Present, nilable | **removed** |
| Service constructor parameter | Accepts the breaker | **removed** |
| Halt state read | On-chain when wired, else parameters | **parameters only** |
| Halt state write | On-chain pause / resume-vote when wired, else parameters | **parameters only** |
| Externally visible behaviour in Scenario A | Parameters path (the engine is never wired) | **identical** |

**Rules**:

- **D-6**: The two RPC operations, their gateway routes and handlers, and the governance status indicator are **retained unchanged**. Only the implementation behind them narrows.
- **D-7**: No residual branch may remain that could select a chain path. A nil-check guarding a path that no longer exists is dead code and must go with it, otherwise the retirement leaves behind exactly the kind of misleading structure it exists to remove.
- **D-8**: The interface **contract definition** is untouched. The request and response shapes, the field carrying the transaction reference included, stay exactly as they are — so no regeneration is required and none may be performed. The transaction-reference field simply always arrives empty in Scenario A, which is already true today.

---

## Removed portal structures

| Structure | Status today | After |
|---|---|---|
| Governance breaker screen | Exists, exported from the governance **page barrel**, reachable by no route | **deleted**, with its barrel export |
| Bank swap and liquidity screens | Referenced **only** by two commented route lines and one commented navigation entry. The bank app has **no page barrel** — pages are imported directly | **deleted**, with the commented entries. No page-barrel edit exists to make here |
| Bank swap-engine data access, state store and types | Present and re-exported by three barrels (stores, services/api, types); data access delegates unconditionally to the sample data set, with no live path | **deleted**, with all three barrel exports |
| Bank swap-engine hook | Present but **fully orphaned** — exported by no barrel and imported by nothing | **deleted**, with no consumer to update |
| Bank sample data set | Contains engine types, a module-level pool value and three engine methods, interleaved with unrelated sample data | **partially edited** — engine content removed, everything else retained. This is the one portal file that is edited rather than deleted |
| Bank dashboard pool refresh | Invoked on every load; result never read | **deleted** |
| Governance inert mock switch and unimported sample data set | Exported and shipped; consumed by nothing | **deleted** |

**Rules**:

- **D-9**: Removing the pool refresh MUST change no rendered output. This is verifiable rather than aspirational: the variable holding the result is commented out, and every card that displayed it sits inside a commented block.
- **D-10**: Removing the inert mock switch and sample data set MUST change no behaviour, because nothing imports either.
- **D-11**: Commented-out blocks unrelated to the swap engine or the mock plumbing are **left alone**. The scope is these surfaces, not every commented block encountered.
- **D-12**: No unused import, export or type may survive removal in either portal — the build and lint gates are what prove it. Three bank barrels re-export the engine's modules and must be edited; a fourth barrel that the originating plan implies (a bank page barrel) does not exist, so an implementer must not go looking for it.
- **D-13**: The bank sample data set is **edited, not deleted**. It is shared with unrelated sample data, so scoping the edit to the engine's types, pool value and three methods is what keeps the rest of the portal working.

---

## What is deliberately NOT modelled here

- **No new entity.** The feature introduces no structure of any kind.
- **No on-chain state.** An already-deployed engine instance is unaffected by deleting its source; it stops being referenced. No retraction, migration or cleanup is modelled because none is possible or needed.
- **No cross-scenario structure.** The other scenario's swap engine, its bindings and its client are separate files and are not part of this model.

---

## State transitions

The halt flag's transitions are **unchanged**: not-halted → halted on a pause, halted → not-halted on a resume. The feature removes one of the two implementations that could effect those transitions in principle, leaving the one that effects them in practice. No transition is added, removed or re-guarded.
