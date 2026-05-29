# Research: Hub Currency Registry

**Feature**: 006-hub-currency-registry  
**Date**: 2026-05-20  
**Status**: Complete — all NEEDS CLARIFICATION resolved

---

## Decision 1: Solidity Storage & Iteration Pattern for `getAllCurrencies()`

**Decision**: Use the same dual-mapping + ordered array pattern already in `PairRegistry.sol`.

**Rationale**:
- `PairRegistry` uses `mapping(bytes32 => PairEntry) _pairs` + `string[] _pairIds` to enable O(n) iteration on a fixed-size set.
- `CurrencyRegistry` will use:
  - `mapping(bytes32 => CurrencyEntry) _bySymbolKey` — primary storage keyed by `keccak256(symbol)`
  - `mapping(address => bytes32) _tokenToSymbolKey` — secondary index to enforce unique tokenAddress
  - `mapping(bytes32 => bool) _symbolExists` — existence guard
  - `string[] _symbols` — ordered list for `getAllCurrencies()` iteration
- On remove: mark `_symbolExists[key] = false` and `delete _tokenToSymbolKey[token]`; the entry remains in `_symbols` but is skipped during iteration (tombstone pattern, same trade-off as PairRegistry's PROPOSED filter in `getAllActivePairs`).

**Alternatives considered**:
- OZ `EnumerableSet` — would work but adds a dependency and pattern divergence from existing contracts. Rejected for consistency.
- Raw delete from `_symbols` array — O(n) shift, expensive on large sets. Rejected.

---

## Decision 2: Go ABI Encoding for `CurrencyEntry[]`

**Decision**: Use inline `tuple[]` ABI definition with named components, identical to the `getAllActivePairs` pattern in `pairRegistryABI`.

**Rationale**:
- The existing `pairRegistryABI` encodes `PairEntry[]` as `tuple[]` with named string/address components:
  ```json
  {"name":"","type":"tuple[]","components":[{"name":"pairId","type":"string"},{"name":"ammAddress","type":"address"},…]}
  ```
- `CurrencyEntry` maps to the same pattern:
  ```json
  {"name":"","type":"tuple[]","components":[{"name":"symbol","type":"string"},{"name":"countryName","type":"string"},{"name":"tokenAddress","type":"address"},{"name":"proposerCB","type":"string"}]}
  ```
- go-ethereum `abi.Unpack` handles this natively when mapped to a matching Go struct.

**Alternatives considered**:
- ABI-gen from JSON artifact — more robust for large contracts, but overkill for a 4-function contract and inconsistent with the existing hand-coded ABI approach. Rejected.

---

## Decision 3: FR-011 Duplicate Pair Check by tokenA+tokenB Combination

**Decision**: Add a new repository method `FindByTokenPair(ctx, tokenA, tokenB string) (*PairProposal, error)` that queries the DB using a bi-directional token check. Check is performed in `pair_service.ProposePair` *before* submitting on-chain.

**Rationale**:
- The existing check `FindByPairID(ctx, pairID)` only guards against duplicate `pair_id` labels. A CB could propose "BRL-EUR2" for the same token pair as "BRL-EUR" and bypass this check.
- The correct guard is: reject if any existing row has `(token_a_address = $1 AND token_b_address = $2) OR (token_a_address = $2 AND token_b_address = $1)` with status IN ('PROPOSED', 'ACTIVE').
- Order-agnostic check is needed because two CBs may disagree on which token is "A" vs "B".
- This is an off-chain soft guard; the on-chain `PairRegistry` guards by `pairId` only, so the off-chain check is the only protection for token-pair duplicates.

**SQL**:
```sql
SELECT * FROM pair_proposals
WHERE (token_a_address = $1 AND token_b_address = $2)
   OR (token_a_address = $2 AND token_b_address = $1)
  AND status IN ('PROPOSED', 'ACTIVE')
LIMIT 1;
```

**Alternatives considered**:
- On-chain guard in `PairRegistry.sol` — would require adding a `mapping(bytes32 => bool) _tokenPairExists` keyed by `keccak256(abi.encode(tokenA, tokenB))`. This would be the ideal long-term approach but is a larger contract change outside this feature's scope. Recorded as a tech debt item.

---

## Decision 4: `proposerCB` Field Stored On-Chain

**Decision**: Store `proposerCB` as a `string` on the `CurrencyRegistry` contract. It is a human-readable label (e.g., `"central_bank_a"`) provided by the caller and not validated against any authoritative registry.

**Rationale**:
- The spec (Q&A clarification) confirms this field is a free-form label, not validated.
- Gas cost for a short string (≤32 chars typical) is negligible for an admin-only write operation.
- Keeping it on-chain makes the contract the single source of truth, avoiding sync complexity.

**Alternatives considered**:
- Store only the on-chain CB address and resolve the label off-chain — adds backend complexity and breaks the "discovery" UX goal where the list endpoint should return all human-readable metadata without extra lookups. Rejected.

---

## Decision 5: No DB Table for Currency Entries (On-Chain Read-Through)

**Decision**: The backend reads currency entries directly from the `CurrencyRegistry` contract via `getAllCurrencies()`. No `hub_currencies` PostgreSQL table is created.

**Rationale**:
- Currency entries have no complex lifecycle (no PROPOSED→ACTIVE transition, no event subscription needed).
- The `CurrencyRegistry` is a write-rarely, read-often registry. On-chain reads are cheap (view functions, no gas).
- Avoids an event-sync goroutine similar to `PairRouter`, which would add complexity for marginal benefit.
- Consistent with how the project queries the hub chain for discovery data (e.g., `ManualOracle` price reads).

**Alternatives considered**:
- DB cache table `hub_currencies` with event sync — would improve read latency and offline resilience. Deferred to a future enhancement if read performance becomes an issue.

---

## Decision 6: SideA/SideB — Semantic Documentation Only

**Decision**: No code changes are needed for FR-008/009. The existing `pair_service.ProposePair` already records `req.ProposerCB` into `domain.PairProposal.ProposerCB`, and `ConfirmPair` already calls `repo.Activate(ctx, pairID, req.ConfirmerCB, confirmedAt)`. FR-010 is satisfied by updating the OpenAPI/handler docs to label `proposer_cb` ≡ SideA and `confirmer_cb` ≡ SideB.

**Rationale**: The Q2 clarification confirmed this is a semantic alias only, with zero schema/API changes. Code audit confirmed the semantics are already implemented correctly.

---

## Summary of Resolved Unknowns

| Unknown | Resolution |
|---------|------------|
| Solidity iteration for `getAllCurrencies()` | Dual-mapping + `string[]` tombstone pattern (same as PairRegistry) |
| Go ABI encoding for `CurrencyEntry[]` | Inline `tuple[]` ABI (same as `pairRegistryABI`) |
| FR-011 duplicate pair by token combination | New `FindByTokenPair` repo method + SQL bi-directional check |
| `proposerCB` on-chain | `string` field on contract, free-form label |
| DB table for currencies | None — on-chain read-through via `getAllCurrencies()` |
| SideA/SideB code change | None — semantic documentation only; code already correct |
