# Data Model: FX Agreement Spoke-Keyed Legs

**Feature**: 020-fx-spoke-keyed-legs  
**Phase**: 1 — Design

---

## 1. Proto Changes (`payment_orchestrator.proto`)

### `FXAgreement` message — field diff

| Action | Field | Number | Type | Notes |
|--------|-------|--------|------|-------|
| REMOVE | `spoke_a_receiver` | 17 | string | Positional; number reserved |
| REMOVE | `spoke_b_receiver` | 18 | string | Positional; number reserved |
| ADD | `source_spoke_id` | 21 | string | e.g. `"spoke-brl"` |
| ADD | `dest_spoke_id` | 22 | string | e.g. `"spoke-usd"` |
| ADD | `source_receiver` | 23 | string | Paladin identity on source spoke |
| ADD | `dest_receiver` | 24 | string | Paladin identity on dest spoke |

Reserved declarations to add:
```proto
reserved 17, 18;
reserved "spoke_a_receiver", "spoke_b_receiver";
```

### `ProposeFXAgreementRequest` message — field diff

| Action | Field | Number | Type | Notes |
|--------|-------|--------|------|-------|
| REMOVE | `spoke_a_receiver` | 14 | string | Positional; number reserved |
| REMOVE | `spoke_b_receiver` | 15 | string | Positional; number reserved |
| ADD | `source_spoke_id` | 16 | string | |
| ADD | `dest_spoke_id` | 17 | string | |
| ADD | `source_receiver` | 18 | string | |
| ADD | `dest_receiver` | 19 | string | |

Reserved declarations to add:
```proto
reserved 14, 15;
reserved "spoke_a_receiver", "spoke_b_receiver";
```

---

## 2. Domain Struct (`internal/domain/fx.go`)

### `FXAgreement` struct — field diff

| Action | Field | Type | Notes |
|--------|-------|------|-------|
| REMOVE | `SpokeAReceiver` | string | |
| REMOVE | `SpokeBReceiver` | string | |
| ADD | `SourceSpokeID` | string | |
| ADD | `DestSpokeID` | string | |
| ADD | `SourceReceiver` | string | |
| ADD | `DestReceiver` | string | |

---

## 3. GORM Model (`internal/repository/fx_agreement_models.go`)

### `FXAgreementModel` struct — field diff

| Action | Field | Type | GORM Tag | Column |
|--------|-------|------|----------|--------|
| REMOVE | `SpokeAReceiver` | string | `column:spoke_a_receiver` | `spoke_a_receiver` |
| REMOVE | `SpokeBReceiver` | string | `column:spoke_b_receiver` | `spoke_b_receiver` |
| ADD | `SourceSpokeID` | string | `column:source_spoke_id` | `source_spoke_id` |
| ADD | `DestSpokeID` | string | `column:dest_spoke_id` | `dest_spoke_id` |
| ADD | `SourceReceiver` | string | `column:source_receiver` | `source_receiver` |
| ADD | `DestReceiver` | string | `column:dest_receiver` | `dest_receiver` |

---

## 4. Database Schema (`fx_agreements` table)

### Migration SQL (idempotent, runs at startup before AutoMigrate)

```sql
-- Step 1: Add new columns
ALTER TABLE fx_agreements ADD COLUMN IF NOT EXISTS source_spoke_id TEXT NOT NULL DEFAULT '';
ALTER TABLE fx_agreements ADD COLUMN IF NOT EXISTS dest_spoke_id   TEXT NOT NULL DEFAULT '';
ALTER TABLE fx_agreements ADD COLUMN IF NOT EXISTS source_receiver TEXT NOT NULL DEFAULT '';
ALTER TABLE fx_agreements ADD COLUMN IF NOT EXISTS dest_receiver   TEXT NOT NULL DEFAULT '';

-- Step 2: Backfill existing rows (idempotent: WHERE source_spoke_id = '' guard)
UPDATE fx_agreements
SET
  source_spoke_id = 'spoke-a',
  dest_spoke_id   = 'spoke-b',
  source_receiver = spoke_a_receiver,
  dest_receiver   = spoke_b_receiver
WHERE source_spoke_id = '';

-- Step 3: Drop legacy columns (after backfill is confirmed)
ALTER TABLE fx_agreements DROP COLUMN IF EXISTS spoke_a_receiver;
ALTER TABLE fx_agreements DROP COLUMN IF EXISTS spoke_b_receiver;
```

**Note**: Verify whether `fx_agreement_events` also carries these columns. If so, apply the same migration to that table (events are read-only audit records; backfill with the same spoke-a/spoke-b defaults).

---

## 5. Server Routing Logic (`internal/grpc/server/server.go`)

### Spoke-leg identification — before and after

**Before** (`server.go:1666-1676`):
```go
// This orchestrator is on spoke-a. Counterparty lock is on spoke-b (SpokeBReceiver).
if fx.SpokeBReceiver == payload.Receiver {
    localReceiver = fx.SpokeAReceiver
}
// This orchestrator is on spoke-b. Counterparty lock is on spoke-a (SpokeAReceiver).
if fx.SpokeAReceiver == payload.Receiver {
    localReceiver = fx.SpokeBReceiver
}
```

**After**:
```go
// Identify local leg by comparing dest/source spoke ID against this server's spoke prefix.
if fx.DestSpokeID == s.spokePrefix {
    localReceiver = fx.DestReceiver
} else {
    localReceiver = fx.SourceReceiver
}
```

**Invariant**: `s.spokePrefix` is already set from `PALADIN_IDENTITY` at startup (`main.go:232`). No new configuration required.

---

## 6. Relay Type Changes (`interop/hub-and-spoke/cacti/src/htlc-relay.ts`)

### FXAgreement interface — field diff (lines 148–149)

| Action | Field | Type |
|--------|-------|------|
| REMOVE | `spoke_a_receiver` | string |
| REMOVE | `spoke_b_receiver` | string |
| ADD | `source_spoke_id` | string |
| ADD | `dest_spoke_id` | string |
| ADD | `source_receiver` | string |
| ADD | `dest_receiver` | string |

### Parsing from chain event (lines 607–608)

| Before | After |
|--------|-------|
| `spokeAReceiver: (a["spoke_a_receiver"] as string) ?? ""` | `sourceSpokeId: (a["source_spoke_id"] as string) ?? ""` |
| `spokenBReceiver: (a["spoke_b_receiver"] as string) ?? ""` | `destSpokeId: (a["dest_spoke_id"] as string) ?? ""` |
| *(absent)* | `sourceReceiver: (a["source_receiver"] as string) ?? ""` |
| *(absent)* | `destReceiver: (a["dest_receiver"] as string) ?? ""` |

### Payload to counterpart (lines 747–748)

| Before | After |
|--------|-------|
| `spoke_a_receiver: event.spokeAReceiver` | `source_spoke_id: event.sourceSpokeId` |
| `spoke_b_receiver: event.spokenBReceiver` | `dest_spoke_id: event.destSpokeId` |
| *(absent)* | `source_receiver: event.sourceReceiver` |
| *(absent)* | `dest_receiver: event.destReceiver` |

---

## 7. Frontend Type (`frontend/apps/bank/src/types/fx-agreement.types.ts`)

Replace the two positional fields with the four spoke-keyed fields in the `FXAgreement` TypeScript interface.
