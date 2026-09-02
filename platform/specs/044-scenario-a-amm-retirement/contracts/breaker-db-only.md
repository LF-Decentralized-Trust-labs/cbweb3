# Contract: Scenario A Circuit Breaker (unchanged surface, database-only behind it)

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06

**The headline of this document is that nothing changes.** This feature deletes a swap engine and the chain-backed implementation of the halt control that sat on it. The externally visible contract — the two gRPC operations, the two gateway routes, their request and response shapes, and their status codes — is **unchanged in every respect**.

That is recorded here explicitly because "we removed a whole implementation and the contract did not move" is a claim a reviewer should be able to check quickly, and because the constraint is hard rather than stylistic.

---

## Why the contract must not move

The compliance contract definition lives in Scenario A's own copy at `apis/proto/compliance/v1/compliance.proto`, separate from Scenario B's. Removing the two breaker operations from it would be scenario-isolated — but it is **blocked**:

- `buf`, `protoc` and `protoc-gen-go` are not installed in this environment.
- The generation target is `cd apis/proto && buf generate`.
- The generated `compliance.pb.go` and `compliance_grpc.pb.go` are committed to the repository.

A contract change therefore could not be regenerated, compiled, or verified here. FR-011 makes this a hard requirement: the definition must come out of this feature byte-for-byte unchanged.

**Verification**: `git diff` on `apis/proto/` and on the generated files must be empty at the end of the feature.

---

## gRPC operations (unchanged)

| Operation | Request | Response | Change |
|---|---|---|---|
| `GetCircuitBreakerStatus` | empty | status response | **none** |
| `ToggleCircuitBreaker` | pause flag + reason | state + transaction reference | **none** |

**Rules**:

- **C-1**: Both operations remain declared, registered and reachable. Retirement removes an implementation, not a capability.
- **C-2**: Request and response shapes are unchanged, **including** the response field carrying an on-chain transaction reference. That field is not removed even though nothing can populate it after retirement — removing it would be a contract change.
- **C-3**: The transaction reference is **always empty** in Scenario A after retirement. This is not a regression: it is already empty today, because the toolkit never wires an engine, so the database path already runs. Callers that treat it as optional are unaffected; a caller that required it would already be broken today.
- **C-4**: Status codes and error semantics are unchanged. In particular, the failure mode that reported an on-chain operation failure can no longer occur, because there is no on-chain operation — but no new failure mode replaces it.

---

## Gateway routes (unchanged)

| Route | Purpose | Change |
|---|---|---|
| `GET /governance/circuit-breaker/status` | Read the halt state | **none** |
| `POST /governance/circuit-breaker/toggle` | Set the halt state | **none** |

**Rules**:

- **C-5**: Both routes keep their paths, methods, bodies and response shapes. The governance portal calls them exactly as it does today.
- **C-6**: The Scenario A portal's status call has no mock branch, so it already reads the real backend. Nothing in the portal's data access changes.

---

## Behaviour before and after

| | Before retirement (Scenario A as deployed) | After retirement |
|---|---|---|
| State read | Compliance parameters — the engine is never wired, so the on-chain branch is not taken | Compliance parameters |
| State write | Compliance parameters | Compliance parameters |
| Transaction reference returned | Empty | Empty |
| Recorded halt state | Preserved across restarts | **Identical** |
| Governance status badge | Reports the halt state | **Identical** |

The right-hand column is the left-hand column. That is the entire point: the retirement deletes a path that Scenario A does not take.

---

## Contract test checklist

- [ ] `GetCircuitBreakerStatus` returns the recorded halt state, before and after retirement, with the same shape
- [ ] `ToggleCircuitBreaker` sets the flag in both directions and reports the resulting state
- [ ] The transaction reference is empty and its absence raises no error
- [ ] Both gateway routes respond with unchanged paths, status codes and bodies
- [ ] A halt recorded before the change reads back identically after it
- [ ] `git diff` on `apis/proto/` and the generated Go files is **empty**
- [ ] No response contains a field that was not there before, and none has lost one
