# Phase 1 Data Model: Circuit-Breaker Transaction-Hash Visibility

**Feature**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05

No schema migration is required. This document records the entities the feature touches, the one field that changes meaning in practice, and the rules that govern it.

---

## Persisted entities (existing — no migration)

### `CircuitBreakerSignature`

One row per breaker write action. Already carries the field this feature needs.

| Field | Type | Today | After |
|---|---|---|---|
| `SignatureID` | string (UUID) | set on all three actions | unchanged |
| `ControlID` | string | pool pair | unchanged |
| `EventKind` | enum | `CBEventKindPause` \| `CBEventKindResume` | unchanged |
| `RequestID` | string | pause: the tx ref (conflated); resume: the on-chain proposal ID | pause: **the proposal-free actions keep tx ref out of this field**; resume: proposal ID. See rule D-3 |
| `SignerBankID` | string | acting institution | unchanged |
| `SignerWallet` | string | always `""` | unchanged (out of scope) |
| `SignaturePayload` | []byte | signature bytes | unchanged |
| **`OnChainTxRef`** | string, column `on_chain_tx_ref` | **populated only by `Pause`** | **populated by all three write actions** |
| `SignedAt` | time | set on all three | unchanged — becomes the ordering key for "most recent action" |

**Validation rules**:
- **D-1**: `OnChainTxRef` is populated whenever a chain call produced a hash, and left empty when no chain is wired. Empty means "no on-chain reference exists", never "unknown".
- **D-2**: The pair's current hash is read from the AMM's own breaker events, not from this table. Each central bank runs its own gateway and database and records only the actions it performed itself, so the row with the greatest `SignedAt` for a `ControlID` answers "this gateway's last action", and two central banks inspecting the same pair would cite different transactions. The table remains the fallback where no AMM is wired, which is also the only place it can be authoritative — there, local actions are the whole record.
- **D-3**: `RequestID` and `OnChainTxRef` are distinct identifiers and must not be used interchangeably. `Pause` currently assigns `RequestID: txRef`, which is a conflation; the feature must not extend that pattern to the resume actions, where `RequestID` genuinely carries the proposal ID needed for signing.

### `scenario_b_risk_control_states`

Per-pair breaker state projection. **Unchanged** by this feature — no new column. The latest hash is derived from the signature rows rather than denormalised here, so there is one writer and no risk of the two disagreeing.

---

## Transport entities (changed)

### `CircuitBreakerStatus` (Go struct → JSON)

| Field | JSON | Change |
|---|---|---|
| `Pair` | `pair` | — |
| `State` | `state` | — |
| `PauseInitiator` | `pause_initiator,omitempty` | — |
| `PauseReason` | `pause_reason,omitempty` | — |
| `ResumeRequestID` | `resume_request_id,omitempty` | — |
| `ResumeSignatures` | `resume_signatures,omitempty` | — |
| `ResumeQuorum` | `resume_quorum,omitempty` | — |
| **`TxHash`** | **`tx_hash,omitempty`** | **NEW** — the pair's most recent action hash (rule D-2) |

### Chain-facing interface (`AMMCircuitBreakerCaller`)

| Method | Today | After |
|---|---|---|
| `PauseCircuitBreaker` | `(string, error)` | unchanged — already returns the hash |
| `ProposeResume` | `(string, error)` → proposal ID | **`(proposalID string, txHash string, err error)`** |
| `SignResume` | `error` | **`(txHash string, err error)`** |
| `ExecuteResume` | `error` | unchanged — no chain call, no hash |
| `IsPaused` | `(bool, error)` | unchanged |
| `ActiveResumeProposal` | `(string, int, int, error)` | unchanged |

The shared `amm.Client.ProposeResume` must likewise return the receipt's `TxHash` alongside the proposal ID it already extracts from the `LogResumeProposed` topic.

### Service method returns

| Method | Today | After |
|---|---|---|
| `Pause` | `error` | `(txHash string, err error)` |
| `ProposeResume` | `(string, error)` | `(requestID string, txHash string, err error)` |
| `SignResume` | `error` | `(txHash string, err error)` |
| `ExecuteResume` | `error` | unchanged |
| `GetStatus` | `(*CircuitBreakerStatus, error)` | unchanged signature; struct gains `TxHash` |

---

## Frontend model

**Two** types need the field, not one — `pause` and `resume-sign` are typed as `CircuitBreakerV2Status`, while `resume-request` returns its own shape:

```ts
// circuit-breaker-v2.types.ts
export interface CircuitBreakerV2Status {
  …
  tx_hash?: string;   // absent ⇒ no on-chain reference for this action/environment
}

export interface ProposeResumeResponse {
  request_id: string;
  state: CbState;
  tx_hash?: string;   // distinct from request_id — see rule D-3
}
```

**Store**: the hash rides on `cbStatus`; no new store field is needed. `pause` and `signResume` already replace `cbStatus` wholesale, so they carry it for free.

**The one trap**: `proposeResume`'s reducer hand-rebuilds `cbStatus` field by field rather than replacing it, and today already drops `resume_signatures` and `resume_quorum`. It must be updated to carry `tx_hash: response.tx_hash`, or the proposal's hash is lost the instant it arrives. This is the single most likely place for the feature to silently half-work.

The page renders the hash as selectable text with a copy control; absence renders as a neutral "no on-chain reference" state, never as an empty box or a dash that could read as a truncated value (FR-009).

---

## Derived: network-wide breaker condition

Not stored anywhere. Computed for the chrome indicator, because the indicator makes a network-wide claim ("swaps are globally halted") while authoritative status is per-pair.

| Input | Source |
|---|---|
| Set of pairs | `ammPairsApi.getPairs()` — already used by the Circuit Breaker page |
| Per-pair state | `circuitBreakerV2Api.getStatus(pair)` — the authoritative, non-mockable read |
| Refresh | existing `usePolling` hook (15 s on the breaker page) |

**Derivation rules**:
- **N-1**: halted if **any** known pair reports `HALTED` (FR-012).
- **N-2**: operational only if the pair set is known and **no** pair reports `HALTED`.
- **N-3**: if the pair set is empty or unavailable, the condition is indeterminate — never asserted as halted (FR-014).
- **N-4**: a pair whose status cannot be read is treated as not-known-halted, and the indeterminacy is surfaced rather than swallowed.

Consumers — `components/layout/AppLayout.tsx`, `components/layout/Sidebar.tsx`, `pages/DashboardPage.tsx` — read only the halted/operational condition via `hooks/useCircuitBreaker.ts`. None reads the V1-only `updatedAt`/`updatedBy`, so those fields can be dropped without loss.

---

## State transitions (unchanged)

`LIVE → HALTED` on pause · `HALTED → RESUME_PENDING` on propose · `RESUME_PENDING → LIVE` when quorum is met during the final signature.

The feature adds **no** transition and changes **no** guard. It attaches an on-chain reference to each transition-causing action. Per FR-025 the decision model is untouched: pause remains 1-of-N, resume remains 2-of-N.
