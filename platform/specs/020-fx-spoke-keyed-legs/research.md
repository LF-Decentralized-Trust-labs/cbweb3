# Research: FX Agreement Spoke-Keyed Legs

**Feature**: 020-fx-spoke-keyed-legs  
**Phase**: 0 — Research & Unknowns Resolution

---

## 1. Spoke identity — how does the payment-orchestrator know its own spoke ID?

**Decision**: Derive from existing `spokePrefix` field on the gRPC server struct.

**Rationale**: `main.go:232` already calls `identity.SpokePrefix(paladinIdentity)` and stores the result in `server.SpokePrefix` (e.g., `"spoke-a"`). This value is available at request-handling time as `s.spokePrefix`. No new env var or config key is needed.

**How it is used in the new routing logic**: Replace the positional receiver comparison at `server.go:1666-1676` with a direct spoke-ID comparison:
```
if fx.DestSpokeID == s.spokePrefix → we are the destination; localReceiver = fx.DestReceiver
else                                → we are the source;      localReceiver = fx.SourceReceiver
```

**Alternatives considered**:
- Add a dedicated `SPOKE_ID` env var → rejected; `spokePrefix` is already derived from `PALADIN_IDENTITY` and is the canonical identity anchor for the server.

---

## 2. Proto field numbering — which field numbers to use for the new fields?

**Decision**:
- `FXAgreement`: `source_spoke_id=21`, `dest_spoke_id=22`, `source_receiver=23`, `dest_receiver=24`. Reserve old numbers with `reserved 17, 18; reserved "spoke_a_receiver", "spoke_b_receiver";`
- `ProposeFXAgreementRequest`: `source_spoke_id=16`, `dest_spoke_id=17`, `source_receiver=18`, `dest_receiver=19`. Reserve old numbers with `reserved 14, 15; reserved "spoke_a_receiver", "spoke_b_receiver";`

**Rationale**: Proto3 prohibits reusing field numbers of removed fields. `FXAgreement` uses fields 1–20 (field 20 = `contract_address`); new fields start at 21. `ProposeFXAgreementRequest` uses fields 1–15; new fields start at 16. `reserved` declarations prevent old-client wire corruption.

**Alternatives considered**:
- Reuse numbers 17/18 and 14/15 → prohibited by proto3 wire-compatibility rules; would silently corrupt messages from clients that still have cached descriptor pools.
- Use a nested `FXLeg` message (`repeated FXLeg legs = 21`) → rejected; the bilateral (two-leg) pairwise model is fixed for Scenario A; flat fields are simpler and sufficient.

---

## 3. Database migration strategy — AutoMigrate vs raw SQL?

**Decision**: Raw SQL migration file executed at startup before AutoMigrate, wrapping the column additions in an `IF NOT EXISTS` guard for idempotency.

**Rationale**: GORM's `AutoMigrate` adds new columns but **never drops existing ones** (`spoke_a_receiver`, `spoke_b_receiver`). Dropping columns requires explicit SQL DDL. The existing pattern (`fx_agreement_gorm.go:28`) runs AutoMigrate unconditionally at startup — adding a migration SQL step before AutoMigrate is the least-invasive approach that allows the drop.

**Migration sequence** (executed at service startup, in order):
1. `ALTER TABLE fx_agreements ADD COLUMN IF NOT EXISTS source_spoke_id TEXT NOT NULL DEFAULT ''` (× 4 columns)
2. `UPDATE fx_agreements SET source_spoke_id='spoke-a', dest_spoke_id='spoke-b', source_receiver=spoke_a_receiver, dest_receiver=spoke_b_receiver WHERE source_spoke_id=''` (idempotent backfill)
3. `ALTER TABLE fx_agreements DROP COLUMN IF EXISTS spoke_a_receiver, DROP COLUMN IF EXISTS spoke_b_receiver` (only after backfill)
4. Same three steps for `fx_agreement_events` if it carries those columns (verify during implementation).
5. AutoMigrate runs after — sees the new columns already present, no-ops on them.

**Alternatives considered**:
- GORM AutoMigrate only (add new model fields, keep old ones) → rejected; leaves dead columns permanently and the old fields would reappear in GORM-generated SQL via the model.
- External migration tool (golang-migrate, goose) → rejected; no migration runner is currently in the codebase and introducing one is out of scope for this change.

---

## 4. Relay leg parsing — what needs to change in `htlc-relay.ts`?

**Decision**: Update the FXAgreement interface and parsing in `htlc-relay.ts` at lines 148–149 and 607–608/747–748.

**Current state** (verified):
```typescript
// Interface (line 148-149)
spoke_a_receiver: string;
spoke_b_receiver: string;

// Parsing from chain event (line 607-608)
spokeAReceiver: (a["spoke_a_receiver"] as string) ?? "",
spokenBReceiver: (a["spoke_b_receiver"] as string) ?? "",

// Payload to counterpart (line 747-748)
spoke_a_receiver: event.spokeAReceiver,
spoke_b_receiver: event.spokenBReceiver,
```

**New state**:
- Add `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` to the interface; remove old fields.
- Update `resolveCounterpart` / spoke selection to use `dest_spoke_id` instead of the hardwired `spokeA`/`spokeB` config keys — **this is the relay routing fix from track B** and is in scope for this change since it touches the same lines.

**Note**: The relay's config-level spoke registry (`config.ts:36-53`) is a separate concern (Phase 2 relay routing). This change only updates which agreement fields the relay reads — the relay still routes to the statically configured counterpart spoke in the "quick path" (bilateral) phase.

---

## 5. Frontend type update — scope?

**Decision**: Update `scenario-a/frontend/apps/bank/src/types/fx-agreement.types.ts` to replace `spoke_a_receiver`/`spoke_b_receiver` with the four new fields.

**Scope**: Type definition only (no logic). Any UI that currently displays these fields will need a label update, but this is a cosmetic concern handled with the type change.

---

## 6. Test-first requirement — where does the failing test go?

**Decision**: Add a new test case to `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/fx_onchain_test.go`.

**Rationale**: That file already covers `ProposeFXAgreement` on-chain flows. The new test constructs a `ProposeFXAgreementRequest` with `source_spoke_id`/`dest_spoke_id`/`source_receiver`/`dest_receiver` and asserts they are stored and readable. The test will fail to compile until the proto is updated — that is the intended "red" state.

**Additional tests required** (per constitution Principle V):
- Unit test for the new spoke-ID routing logic in `server.go` (replaces the `isLocalReceiver` spoke-prefix comparison test at `relay_test.go`).
- Repository test in `gorm_repos_test.go` asserting the four new columns are persisted correctly.
