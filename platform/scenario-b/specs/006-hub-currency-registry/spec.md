# Feature Specification: Hub Currency Registry

**Feature Branch**: `006-hub-currency-registry`  
**Created**: 2026-05-20  
**Status**: Draft  
**Input**: User description: "New on-chain currency registry on the international hub so central banks can register their currencies, enabling pair proposal discovery; paired with SideA/SideB tracking on pair proposals."

## Clarifications

### Session 2026-05-20

- Q: What enforces uniqueness in CurrencyEntry — symbol only, tokenAddress only, or both independently? → A: Both `symbol` AND `tokenAddress` are independently unique keys. Registering a symbol already taken or a tokenAddress already registered under any symbol must be rejected.
- Q: How should SideA/SideB be exposed in the pair API response — rename, add both, or semantic alias only? → A: Semantic alias only — `proposer_cb` IS SideA and `confirmer_cb` IS SideB. No new fields are added; the existing field names are the canonical API surface and their role semantics (proposer = SideA, confirmer = SideB) are documented.

- Q: Should `POST /api/v2/amm/pairs/propose` validate `token_a_address` against `CurrencyRegistry` as a mandatory gate? → A: No — advisory only. The `CurrencyRegistry` exists for discovery; pair proposal authorization is enforced exclusively on-chain via `PairRegistry` + `IdentityRegistry`. No off-chain gate against the registry.
- Q: Should the remove endpoint identify the currency by symbol, by tokenAddress, or support both? → A: By symbol — endpoint path is `DELETE /api/v2/hub/currencies/{symbol}` (e.g., `/BRL`). Symbol is already a unique key (Q1) and is human-readable.
- Q: What happens if SideA (proposer) and SideB (confirmer) are the same Central Bank? → A: Blocked by structural design — `PairRegistry.confirmPair` requires `msg.sender == getCentralBankOf(tokenB)`, which by definition is different from the tokenA issuer (proposer). Self-confirmation is architecturally impossible; this guarantee is inherited from the existing contract and must be documented explicitly.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — CB Registers Its Currency on the Hub (Priority: P1)

A Central Bank operator (e.g., CB-A, issuer of BRL) wants to register its currency on the international hub so that other central banks can discover it, read the token address, and propose a trading pair.

**Why this priority**: Without a discoverable currency registry, pair proposals require out-of-band coordination (offline address exchange). This story delivers the entire discovery foundation for the hub.

**Independent Test**: Can be tested end-to-end by calling the register endpoint and then calling the list endpoint, verifying the currency appears with the correct symbol, country name, token address, and CB identifier.

**Acceptance Scenarios**:

1. **Given** CB-A is authenticated and holds the issuer role for its tCeBM token, **When** it submits a register request with symbol "BRL", country "Brazil", token address, and its CB identifier, **Then** the currency entry is stored on-chain and returned in the currency list.
2. **Given** the same CB-A tries to register "BRL" again, **When** it submits the same register request, **Then** the system rejects it with a conflict error indicating the currency already exists.
3. **Given** a CB that is NOT the issuer of the token tries to register, **When** it submits a register request for that token address, **Then** the system rejects it with an authorization error.
4. **Given** CB-B (issuer of EUR) also registers its currency, **When** any CB calls the list endpoint, **Then** both BRL and EUR entries appear in the response.

---

### User Story 2 — CB Removes Its Currency from the Hub (Priority: P2)

A Central Bank operator wants to deregister its own currency from the hub registry (e.g., because the token was migrated to a new address or the country left the hub).

**Why this priority**: Data hygiene and sovereignty — a CB must be able to retract its own registration without requiring a hub administrator.

**Independent Test**: Can be tested by registering a currency, then calling the remove endpoint and confirming the currency no longer appears in the list.

**Acceptance Scenarios**:

1. **Given** CB-A has already registered "BRL", **When** it submits a `DELETE /api/v2/hub/currencies/BRL` request, **Then** the entry is removed from on-chain storage and no longer returned in the currency list.
2. **Given** CB-B tries to remove CB-A's "BRL" registration, **When** it submits `DELETE /api/v2/hub/currencies/BRL`, **Then** the system rejects it with an authorization error.
3. **Given** no currency is registered under symbol "XYZ", **When** any CB submits `DELETE /api/v2/hub/currencies/XYZ`, **Then** the system returns a not-found error.

---

### User Story 3 — CB Discovers Currencies to Propose a Pair (Priority: P2)

A Central Bank operator (e.g., CB-A) wants to browse all registered currencies on the hub to identify another CB's token address and propose a trading pair.

**Why this priority**: This is the primary consumer of the registry — it feeds `POST /api/v2/amm/pairs/propose` without requiring off-band communication.

**Independent Test**: Can be tested standalone by calling the list endpoint and confirming it returns the registered currencies with all fields needed to populate the propose-pair request (`token_a_address`, `token_b_address`, `amm_address`).

**Acceptance Scenarios**:

1. **Given** BRL and EUR are both registered, **When** CB-A calls the currency list endpoint, **Then** it receives entries containing symbol, country name, token address, and CB identifier for each currency.
2. **Given** the list result, **When** CB-A uses the EUR token address from the list in a `POST /api/v2/amm/pairs/propose` request, **Then** the propose pair call succeeds (authorization on pair proposal is determined by the token issuer, not the registry).

---

### User Story 4 — Pair Proposal Tracks SideA / SideB (Priority: P3)

When a Central Bank proposes a pair, it is recorded as **SideA**. When another CB confirms the pair, it is recorded as **SideB**. Subsequent hub operations (liquidity provision, LP queries) can use these roles to distinguish the two CB counterparties.

**Why this priority**: This enriches existing pair records with explicit counterparty roles, enabling clearer governance and downstream LP operations, but does not block the registry feature.

**Independent Test**: Can be tested by proposing a pair (CB-A = SideA) and confirming it (CB-B = SideB), then calling the pair list endpoint and verifying `side_a` and `side_b` fields are populated.

**Acceptance Scenarios**:

1. **Given** CB-A proposes the BRL-EUR pair, **When** the proposal succeeds, **Then** `pair_proposals` records CB-A as SideA and the pair status is PROPOSED.
2. **Given** the BRL-EUR pair is PROPOSED with CB-A as SideA, **When** CB-B confirms it, **Then** CB-B is recorded as SideB and the pair status transitions to ACTIVE.
3. **Given** a pair is ACTIVE with SideA and SideB set, **When** the pair list endpoint is called, **Then** each pair entry exposes `side_a` and `side_b` identifier fields.
4. **Given** CB-A tries to propose a pair for the same token combination that is already PROPOSED or ACTIVE, **When** it submits the proposal, **Then** the system rejects it with a conflict error.

---

### Edge Cases

- ~~What happens when a CB registers a token address that is already registered under a different symbol?~~ → Resolved (Q1): the contract rejects the call — tokenAddress is a unique key independent of symbol.
- What happens when a CB deregisters a currency that is currently referenced by an active pair proposal?
- What happens when the hub IdentityRegistry does not have a CB mapped to the given token address?
- ~~How does the system handle a propose-pair request if the `token_a_address` in the request does not match any registered currency in the CurrencyRegistry?~~ → Resolved (Q3): the proposal proceeds normally — the registry is advisory/discovery only; authorization is enforced on-chain by `PairRegistry` + `IdentityRegistry`.
- ~~What happens if SideA and SideB are the same Central Bank (i.e., a CB tries to confirm its own proposal)?~~ → Resolved (Q5): structurally impossible — `PairRegistry.confirmPair` requires the confirmer to be the on-chain issuer of `tokenB`; since `tokenA` and `tokenB` must have different issuers for a valid pair, a CB cannot confirm its own proposal.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The hub MUST provide an on-chain smart contract (`CurrencyRegistry`) that stores, per currency entry: currency symbol (e.g., "BRL"), country name (e.g., "Brazil"), token address (the tCeBM address on the hub), and CB identifier (the human-readable name of the issuing Central Bank).
- **FR-002**: A Central Bank MUST only be authorized to register or remove its own currency entry; authorization is derived from the same issuer mapping used by the existing `IdentityRegistry` (`setCentralBankOf`), ensuring a CB can only act for tokens it issued.
- **FR-003**: The backend MUST expose an endpoint for an authenticated CB to register its currency on the `CurrencyRegistry` contract.
- **FR-004**: The backend MUST expose a `DELETE /api/v2/hub/currencies/{symbol}` endpoint for an authenticated CB to remove its own currency from the `CurrencyRegistry` contract, identifying the entry by its currency symbol (e.g., `BRL`).
- **FR-005**: The backend MUST expose an endpoint to list all currently registered currencies, returning all fields (symbol, country name, token address, CB identifier).
- **FR-006**: The `CurrencyRegistry` contract MUST revert if (a) the currency symbol is already registered under any entry, OR (b) the token address is already registered under any symbol — both are independently unique keys.
- **FR-007**: The `CurrencyRegistry` contract MUST revert if the caller's on-chain address does not match the issuer of the given token address (as recorded in `IdentityRegistry`).
- **FR-008**: The pair proposal flow (`POST /api/v2/amm/pairs/propose`) MUST record the proposing CB as SideA, stored in the existing `proposer_cb` field.
- **FR-009**: The pair confirmation flow (`POST /api/v2/amm/pairs/confirm`) MUST record the confirming CB as SideB, stored in the existing `confirmer_cb` field.
- **FR-010**: The pair list endpoint (`GET /api/v2/amm/pairs`) MUST document that `proposer_cb` represents SideA and `confirmer_cb` represents SideB. No additional fields are required; the semantic equivalence is the deliverable.
- **FR-011**: The system MUST prevent proposing a new pair between the same token-A and token-B combination if an existing PROPOSED or ACTIVE pair already exists for those tokens (regardless of pair ID label).
- **FR-012**: The `CurrencyRegistry` must be a standalone contract (distinct from `PairRegistry` and `IdentityRegistry`), accessible to all hub participants for read operations.

### Key Entities

- **CurrencyEntry**: Represents a registered currency on the hub. Attributes: `symbol` (e.g., "BRL"), `countryName` (e.g., "Brazil"), `tokenAddress` (on-chain address of the tCeBM token), `proposerCB` (human-readable CB identifier, e.g., "central_bank_a"). **Uniqueness**: both `symbol` and `tokenAddress` are independently unique — no two entries may share either value.
- **PairProposal** *(existing, semantically clarified)*: No schema change. `proposerCB` ≡ SideA (the CB that initiated the pair proposal); `confirmerCB` ≡ SideB (the CB that approved it). These equivalences are documented in the API contract and enforced by the service layer.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A Central Bank can complete currency registration (end-to-end, including on-chain confirmation) and have the entry visible in the currency list within the normal transaction confirmation time of the hub network.
- **SC-002**: A Central Bank can discover all registered hub currencies from a single API call and use the returned data directly to populate a pair proposal request, without any additional out-of-band data exchange.
- **SC-003**: Unauthorized registration or removal attempts (wrong issuer) are rejected 100% of the time at the contract layer, without any off-chain workaround possible.
- **SC-004**: After pair confirmation, both SideA and SideB identifiers are visible in the pair list response, enabling downstream consumers to distinguish the two CB counterparties without additional lookups.
- **SC-005**: Duplicate pair proposals (same token-A + token-B already in PROPOSED or ACTIVE state) are rejected before reaching the on-chain contract.

## Assumptions

- The `IdentityRegistry` contract already tracks which on-chain address is the issuer (central bank) of each tCeBM token via `setCentralBankOf(tokenAddress, cbAddress)`. The `CurrencyRegistry` contract will call `IdentityRegistry` to verify issuer authority.
- The `CurrencyRegistry` is deployed on the hub network (same chain as `PairRegistry` and `IdentityRegistry`).
- The CB identifier in the currency entry (`proposerCB` field, e.g., `"central_bank_a"`) is a human-readable label provided by the caller; it is stored as-is and not validated against an authoritative name registry.
- The `CurrencyRegistry` is a discovery tool only. It does not gate, block, or authorize any other operation (pair proposal, liquidity provision, etc.). All access control remains in `PairRegistry` and `IdentityRegistry` on-chain.
- The deregistration of a currency that is referenced by an existing PROPOSED or ACTIVE pair is out of scope for v1 — governance rules for that scenario will be defined separately.
- SideA ≡ `proposer_cb` and SideB ≡ `confirmer_cb` in all layers (DB, service, API response). No new columns, no new response fields. The feature deliverable is documenting and enforcing this semantic contract.
- Self-confirmation (SideA == SideB) is structurally prevented by the `PairRegistry` contract: `proposePair` is authorized by `getCentralBankOf(tokenA)` and `confirmPair` by `getCentralBankOf(tokenB)`. A valid pair always involves two distinct token issuers; no additional off-chain guard is needed.
