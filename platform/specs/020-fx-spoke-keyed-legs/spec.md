# Feature Specification: FX Agreement Spoke-Keyed Legs

**Feature Branch**: `020-fx-spoke-keyed-legs`
**Created**: 2026-06-26
**Status**: Draft
**Scope**: Scenario A — Enhanced Correspondent Banking

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Propose FX Agreement with explicit spoke routing (Priority: P1)

A settlement agent proposes an FX Agreement identifying the origin spoke and destination spoke by their IDs, and specifying a Paladin identity for each leg. The system routes and validates each leg by spoke ID, not by positional label ("A"/"B").

**Why this priority**: This is the gating change for N-spoke scalability. Without it, every new spoke requires code edits. All downstream stories depend on this.

**Independent Test**: Can be tested end-to-end by proposing an FX Agreement with `source_spoke_id=spoke-brl`, `dest_spoke_id=spoke-usd`, `source_receiver=<identity>`, `dest_receiver=<identity>` and asserting the agreement is stored and readable with those spoke-keyed fields.

**Acceptance Scenarios**:

1. **Given** a ProposeFXAgreementRequest with `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` filled, **When** the request is submitted, **Then** the agreement is persisted with those four fields and the legacy positional fields are absent.
2. **Given** a ProposeFXAgreementRequest where `source_spoke_id` or `dest_spoke_id` is empty, **When** the request is submitted, **Then** it is rejected with a validation error identifying the missing field.
3. **Given** an existing agreement, **When** `GetFXAgreement` is called, **Then** the response includes `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` and does not include `spoke_a_receiver` or `spoke_b_receiver`.

---

### User Story 2 — Relay routes HTLC legs by destination spoke ID (Priority: P1)

The Cacti relay reads `dest_spoke_id` from the FX Agreement to determine which spoke to route the counterparty HTLC lock to, replacing the hardwired `spokeA`/`spokeB` assumption.

**Why this priority**: Without relay routing by spoke ID, the data-model change does not deliver runtime value — the relay would still fail with more than two spokes.

**Independent Test**: Can be tested by verifying that the relay correctly parses `dest_spoke_id` / `source_spoke_id` from an agreement event and selects the correct spoke endpoint.

**Acceptance Scenarios**:

1. **Given** an FX Agreement with `dest_spoke_id=spoke-usd`, **When** the relay receives the lock event from spoke-brl, **Then** it routes the counterparty lock to the endpoint registered for `spoke-usd`.
2. **Given** an FX Agreement event missing `dest_spoke_id`, **When** the relay processes it, **Then** it logs an error and skips the event without crashing.

---

### User Story 3 — Existing agreement records remain accessible after migration (Priority: P2)

Agreements created before this change (with positional fields) are migrated to the new spoke-keyed schema so that operations (accept, settle, cancel) on in-flight agreements continue to work.

**Why this priority**: Backfill is required for production correctness but does not block new-agreement flows in a fresh environment; it is critical for staged deployments.

**Independent Test**: Can be tested by running the backfill script against a seeded database and asserting that all rows have non-empty `source_spoke_id`/`dest_spoke_id` and that the old columns are absent.

**Acceptance Scenarios**:

1. **Given** a database with rows having `spoke_a_receiver` / `spoke_b_receiver`, **When** the migration runs, **Then** all rows gain `source_spoke_id='spoke-a'`, `dest_spoke_id='spoke-b'`, `source_receiver=<former spoke_a_receiver>`, `dest_receiver=<former spoke_b_receiver>`, and the old columns are dropped.
2. **Given** a migrated agreement, **When** `AcceptFXAgreement` is called, **Then** the operation succeeds using the migrated fields.

---

### Edge Cases

- What happens when a proto client built against the old schema (with `spoke_a_receiver`) sends a request? The server must reject it with a clear error (field no longer present; reserved field numbers prevent silent corruption).
- What happens when both old and new field names are present in a client request? Proto3 reserved fields prevent this at the serialization layer.
- What happens if the backfill script is run twice? It must be idempotent (no-op on already-migrated rows).
- What happens when `source_spoke_id == dest_spoke_id`? The orchestrator must reject with a validation error (intra-spoke FX is not supported).

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST replace `spoke_a_receiver` (field 17) and `spoke_b_receiver` (field 18) in `FXAgreement` with `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` using new field numbers, and MUST mark the old numbers as reserved.
- **FR-002**: The system MUST replace `spoke_a_receiver` (field 14) and `spoke_b_receiver` (field 15) in `ProposeFXAgreementRequest` with `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`, and MUST mark the old numbers as reserved.
- **FR-003**: The payment-orchestrator MUST validate that `source_spoke_id`, `dest_spoke_id`, `source_receiver`, and `dest_receiver` are all non-empty on every `ProposeFXAgreement` call.
- **FR-004**: The payment-orchestrator MUST identify the local leg by comparing `source_spoke_id` and `dest_spoke_id` against the spoke's own ID (configuration, not hardcoded value), replacing the positional `spoke-a`/`spoke-b` string comparison.
- **FR-005**: The database schema MUST be updated via a migration that adds the four new columns, backfills existing rows (mapping `spoke_a_receiver` → `source_spoke_id='spoke-a'`, `source_receiver`; `spoke_b_receiver` → `dest_spoke_id='spoke-b'`, `dest_receiver`), and drops the old columns.
- **FR-006**: The Cacti relay MUST parse `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` from agreement events, replacing the hardcoded `spoke_a_receiver`/`spoke_b_receiver` parsing.
- **FR-007**: The frontend type definitions MUST be updated to expose the four new fields and remove the two legacy fields.
- **FR-008**: A failing test MUST be written before any production code is changed (test-first rule from constitution).
- **FR-009**: The backfill migration MUST be idempotent.

### Key Entities

- **FXAgreement**: A bilateral foreign-exchange settlement agreement. Key change: legs are now identified by `source_spoke_id` + `source_receiver` and `dest_spoke_id` + `dest_receiver` instead of positional "A/B" labels.
- **ProposeFXAgreementRequest**: The input message for proposing a new agreement. Receives the same field replacements as `FXAgreement`.
- **FXLeg (implicit)**: The concept of a settlement leg keyed by spoke ID. Not a new message type — expressed as four flat fields on the two messages above.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero references to `spoke_a_receiver` or `spoke_b_receiver` remain in production code (proto, Go, TypeScript) after the change — verifiable by `grep`.
- **SC-002**: All existing tests pass after the migration, and the new test added in FR-008 passes.
- **SC-003**: A fresh FX Agreement proposal with two independently provisioned spokes (identified by distinct `source_spoke_id` / `dest_spoke_id`) completes the full settlement lifecycle (propose → accept → lock → settle) without code modification.
- **SC-004**: The database migration runs to completion on a database seeded with legacy rows and leaves no rows with null or empty `source_spoke_id` / `dest_spoke_id`.
- **SC-005**: The relay correctly routes a lock event to the destination spoke identified by `dest_spoke_id`, confirmed by integration test or manual verification against two independently provisioned spokes.

---

## Assumptions

- The existing two-spoke environment uses `spoke-a` and `spoke-b` as canonical spoke IDs; the backfill uses these as the literal values for `source_spoke_id` and `dest_spoke_id` on legacy rows.
- Correspondent banking in Scenario A remains pairwise (two legs per agreement); this change makes legs spoke-keyed but does not introduce N-leg agreements.
- The local spoke ID is available to the payment-orchestrator via an environment variable (`SPOKE_ID`) already present or to be added as part of this change.
- The Cacti relay's spoke registry (its config) already knows the spoke endpoints by ID — this change updates which field the relay reads to select the destination, not how the registry is populated.
- Proto3 reserved field semantics guarantee that old clients sending `spoke_a_receiver` / `spoke_b_receiver` will have those values silently dropped (not error), but the server validation in FR-003 will reject the request for missing required fields.
- This change is scoped entirely to Scenario A. Scenario B's `payment_orchestrator.proto` is a separate file and must not be touched.
