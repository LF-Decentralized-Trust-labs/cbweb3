# Contract: Scenario B Governance Circuit Breaker (V2 REST)

**Feature**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05
**Base**: `/api/v2` (served by `governance_scenariob_handler.go`, consumed by `httpClientV2`)

This is the only external contract the feature changes. The change is **additive**: one optional field on four responses. No request body, route, status code or error shape changes. No protobuf contract is involved.

---

## Common field

| Field | Type | Presence | Meaning |
|---|---|---|---|
| `tx_hash` | string | **optional** — omitted entirely when no on-chain reference exists | The on-chain transaction hash of the action, or for `status`, of the pair's most recent action |

**Rules**
- **C-1**: Omitted, never `""`. An empty string would be indistinguishable from "unknown"; omission means "no on-chain reference exists" (FR-009).
- **C-2**: Never a link. A bare hash string; presentation is the client's concern (FR-008).
- **C-3**: Never load-bearing. Absence must not change the action's outcome, status code, or the `state` value (FR-010).
- **C-4**: Backward compatible. Existing clients that ignore unknown fields are unaffected.

---

## `POST /api/v2/governance/circuit-breaker/pause`

Request: unchanged — `{ pair, bank_id, reason_code }`

```jsonc
// 200 — before
{ "state": "HALTED", "pair": "W-BRL-W-ARS" }
// 200 — after
{ "state": "HALTED", "pair": "W-BRL-W-ARS", "tx_hash": "0x9a5b…f446" }
```

Errors unchanged: `400` invalid body / missing `pair`, `bank_id`, `reason_code`; `422` on-chain failure.

---

## `POST /api/v2/governance/circuit-breaker/resume-request`

```jsonc
// 200 — before
{ "state": "RESUME_PENDING", "request_id": "0x3f1c…" }
// 200 — after
{ "state": "RESUME_PENDING", "request_id": "0x3f1c…", "tx_hash": "0x77ab…" }
```

**`request_id` and `tx_hash` are different identifiers and both are required.** `request_id` is the proposal ID from the `LogResumeProposed` topic and is the value a co-signer must submit to `resume-sign`; `tx_hash` is the transaction that created the proposal. Substituting one for the other breaks signing (rule D-3).

---

## `POST /api/v2/governance/circuit-breaker/resume-sign`

```jsonc
// 200 — quorum not yet met
{ "state": "RESUME_PENDING", "request_id": "0x3f1c…", "tx_hash": "0xbb12…" }
// 200 — quorum met
{ "state": "LIVE", "pair": "W-BRL-W-ARS", "tx_hash": "0xbb12…" }
```

`tx_hash` is the hash of **this signature's** transaction in both cases. Note that when quorum is met the contract unpauses inside that same signing transaction, so this hash is also the transaction that resumed the pair.

> **Known limitation (FR-020, documented not fixed).** The `LIVE` response is produced after calling `ExecuteResume`, which performs no chain call because the contract auto-unpauses during the final signature. This is correct for a 2-of-N quorum but would report `LIVE` prematurely for any quorum greater than 2. Revisit only if quorum ever exceeds 2.

---

## `GET /api/v2/governance/circuit-breaker/status?pair=…`

```jsonc
// 200 — after
{
  "pair": "W-BRL-W-ARS",
  "state": "RESUME_PENDING",
  "pause_initiator": "central-bank-a",
  "pause_reason": "incident-2026-08-05",
  "resume_request_id": "0x3f1c…",
  "resume_signatures": 1,
  "resume_quorum": 2,
  "tx_hash": "0x77ab…"        // pair's most recent action, any institution
}
```

`tx_hash` is the `on_chain_tx_ref` of the most recent `CircuitBreakerSignature` for the pair by `SignedAt`, regardless of signer (rule D-2). This is what lets the value survive a reload and appear identically to every central bank (FR-005, FR-006).

`state` continues to be sourced from on-chain `IsPaused()` plus the active proposal; the hash does not become a source of truth for state.

---

## No-chain environments

Every route behaves exactly as before, with `tx_hash` omitted from all four responses. Actions still succeed, states still transition through the off-chain projection, and no error is raised for the missing hash (FR-009, FR-010).

---

## Contract test checklist

- [ ] `pause` returns non-empty `tx_hash` when chain-wired
- [ ] `resume-request` returns **both** `request_id` and `tx_hash`, and they differ
- [ ] `resume-sign` returns `tx_hash` in both the pending and quorum-met responses
- [ ] `status` echoes the pair's latest action hash, including after a simulated reload
- [ ] `status` reflects an action taken by a *different* institution
- [ ] all four omit `tx_hash` entirely (key absent, not `""`) in no-chain mode
- [ ] all four keep their existing `state` values and status codes unchanged in both modes
