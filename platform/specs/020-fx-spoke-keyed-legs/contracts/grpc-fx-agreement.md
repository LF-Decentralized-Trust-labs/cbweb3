# gRPC Interface Contract: FXAgreement Spoke-Keyed Fields

**Service**: `PaymentOrchestratorService`  
**Proto file**: `scenario-a/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto`  
**Change type**: Breaking — positional fields removed, spoke-keyed fields added

---

## Changed Messages

### `FXAgreement` (read/response message)

New fields added; old positional fields removed and reserved:

```proto
message FXAgreement {
  // ... unchanged fields (1-16, 19-20) ...
  reserved 17, 18;
  reserved "spoke_a_receiver", "spoke_b_receiver";
  string source_spoke_id = 21; // spoke ID of the initiating leg (e.g. "spoke-brl")
  string dest_spoke_id   = 22; // spoke ID of the receiving leg  (e.g. "spoke-usd")
  string source_receiver = 23; // Paladin identity on source spoke (e.g. "funded_operator@spoke-brl-bank-c")
  string dest_receiver   = 24; // Paladin identity on dest spoke   (e.g. "funded_operator@spoke-usd-bank-b")
}
```

### `ProposeFXAgreementRequest` (write/request message)

```proto
message ProposeFXAgreementRequest {
  // ... unchanged fields (1-13) ...
  reserved 14, 15;
  reserved "spoke_a_receiver", "spoke_b_receiver";
  string source_spoke_id = 16;
  string dest_spoke_id   = 17;
  string source_receiver = 18;
  string dest_receiver   = 19;
}
```

---

## Validation Rules (enforced by server)

| Field | Rule |
|-------|------|
| `source_spoke_id` | Non-empty string. Must not equal `dest_spoke_id`. |
| `dest_spoke_id` | Non-empty string. Must not equal `source_spoke_id`. |
| `source_receiver` | Non-empty string. Must be a valid Paladin identity format (`<name>@<spoke>-<entity>`). |
| `dest_receiver` | Non-empty string. Must be a valid Paladin identity format. |

---

## Wire Compatibility

- Old clients that send `spoke_a_receiver` (field 14) or `spoke_b_receiver` (field 15) in `ProposeFXAgreementRequest` will have those values **silently dropped** by proto3 deserialization (reserved fields are ignored on the wire). The server validation (FR-003) will then reject the request with `codes.InvalidArgument` for missing required fields.
- Clients **must** be updated to use the new field names before sending requests to a server running this version.
- The `reserved` declarations prevent accidental field-number reuse in future schema changes.

---

## Error Responses

| Condition | gRPC Status Code | Message |
|-----------|-----------------|---------|
| `source_spoke_id` empty | `INVALID_ARGUMENT` | `"source_spoke_id is required"` |
| `dest_spoke_id` empty | `INVALID_ARGUMENT` | `"dest_spoke_id is required"` |
| `source_receiver` empty | `INVALID_ARGUMENT` | `"source_receiver is required"` |
| `dest_receiver` empty | `INVALID_ARGUMENT` | `"dest_receiver is required"` |
| `source_spoke_id == dest_spoke_id` | `INVALID_ARGUMENT` | `"source_spoke_id and dest_spoke_id must be different"` |
