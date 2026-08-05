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
- **D-2**: `SignedAt` orders actions within a pair. The pair's current hash is the `OnChainTxRef` of the row with the greatest `SignedAt` for that `ControlID`.
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

```ts
// circuit-breaker-v2.types.ts
tx_hash?: string;   // absent ⇒ no on-chain reference for this action/environment
```

Store holds the latest known hash for the selected pair, refreshed from status polling so it survives reload (FR-006). The page renders it as text with a copy control; absence renders as a neutral "no on-chain reference" state, never as an empty box or a dash that could read as a truncated value (FR-009).

---

## State transitions (unchanged)

`LIVE → HALTED` on pause · `HALTED → RESUME_PENDING` on propose · `RESUME_PENDING → LIVE` when quorum is met during the final signature.

The feature adds **no** transition and changes **no** guard. It attaches an on-chain reference to each transition-causing action. Per FR-023 the decision model is untouched: pause remains 1-of-N, resume remains 2-of-N.
