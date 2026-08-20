# Changelog — PvP Settlement Contract (Scenario A)

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

**Versioning changed at 2.3.0.** Toolbox contract versions now track the **CBWeb3 API
Gateway version** they describe, rather than carrying an independent series. This makes
the provenance of every artefact checkable at a glance: `openapi_pvp_v2.3.0.yaml`
describes API Gateway v2.3.0 and nothing else.

## [2.3.0] — 2026-08-20

### Corrected before publication (2026-08-21)

An independent verification pass against the delivered Scenario A gateway found this
contract following the platform's OpenAPI *document* in places where the shipped binary
does something else. **The delivered gateway wins.** All are corrected and catalogued as
rows A1–A11 of [`../../DIVERGENCES.md`](../../DIVERGENCES.md).

- **FX state vocabulary is prefixed.** `FXAgreement.state`, `from_state`, `to_state` are
  `FX_STATE_*` (including `FX_STATE_INVALID`, the `from_state` of the propose event) and
  `source` is `FX_EVENT_SOURCE_*`, because the gateway emits protobuf enum names. The
  `state` **query parameter** keeps the bare vocabulary — the orchestrator strips an
  `FX_STATE_` prefix, so both spellings are accepted on input; only the output is prefixed.
- **412 → 409 on `accept` / `reject` / `cancel` / `settle`.** `grpcErrorToHTTP` maps gRPC
  `FailedPrecondition` to `409 Conflict`. No payments or HTLC route emits 412 at all; the
  gateway's only 412 is `/pki-login`'s missing-certificate branch. The unreachable 412 on
  `lockHTLCWithHash` is removed.
- **`getHTLCStatus` returns the bare record**, not `{"lock": {…}}`. `searchHTLC` really
  does wrap, as `{"locks": [...], "total": n}`.
- **`HTLCLock` has no `secret`.** The delivered `HTLCStatus` struct does not declare one.
  It carries `counterparty_locked`, `amount` and `created_at`, all three now documented.
  `LockHTLCResponse` gains `secret` — the one and only place it is ever returned.
- **`requestFiatExchange` returns `mint_tx_hash`**, not `tx_hash`; **`approveEscrow`
  returns `mint_tx_hash`**, not `zeto_mint_tx_hash`. Both were mislabelled as a
  Scenario A vs Scenario B difference; they are document-vs-implementation, and the two
  scenarios agree with each other.
- **Creation responses carry the id only.** `registerDeposit`, `requestEscrow` and
  `requestRedeem` do not return `status`.
- **FX action responses carry `{trade_id, tx_hash}`**, not `tx_hash` alone.
- **`FXAgreementEvent.actor` is the literal `"payment-orchestrator"`** and `notes` is
  never written, so the audit trail does not identify who acted. Use `source` to
  distinguish a direct call from a governance call.
- **`proposeFXAgreement` requires eleven fields, not seven** — `source_spoke_id`,
  `dest_spoke_id`, `source_receiver` and `dest_receiver` are enforced by the orchestrator.
- **`requestRedeem` takes `amount` only.** The commercial-bank proxy performs the tCeBM
  transfer itself and injects `zeto_transfer_tx_hash`, `requester_besu_address` and
  `requester_paladin_identity`. The document presents the hash as a client obligation; it
  is not one.
- **Reserve and HTLC-lock validation surfaces as 500, not 400.** Those handlers check only
  that the body parses and wrap every downstream error as 500. The declared 400 covers
  unparseable JSON only.
- The `/api/v1` prefix is written in full everywhere, including the README sequence
  diagram. No `/htlc/...` or `/fx/...` shorthand remains — that was the shape of the
  withdrawn `openapi_pvp_v0.1.0.yaml`.

### Removed

- **`openapi_pvp_v0.1.0.yaml` is deleted, not deprecated.**

### Added

- **`openapi_pvp_v2.3.0.yaml` — a complete replacement**, derived from the delivered
  CBWeb3 API Gateway v2.3.0 OpenAPI document
  (`scenario-a/backend/services/api-gateway/docs/openapi.yaml`), cross-checked against
  the gateway's Go router.
- 32 operations across 28 paths, covering the Scenario A settlement path end to end:
  - the **token surface** (`mint`, `burn`, `transfer`, `balance`, `fiat-balance`) — the
    balances an HTLC lock actually moves;
  - the **reserve lifecycle** (`deposits`, `deposits/approve`, `deposits/reject`,
    `deposits/fiat-exchange`, `escrows`, `escrows/approve`, `escrows/reject`, `redeems`,
    `redeems/approve`, `redeems/reject`, plus the three list endpoints) — how a bank
    obtains the tCeBM it will lock;
  - the full **FX agreement lifecycle** (propose / list / get / accept / reject / cancel /
    settle / audit);
  - the full **HTLC lifecycle** (lock / lock-with-hash / settle / refund / status /
    search).

  `operationId` values match the platform's own.
- The reserve lifecycle is included because **a bank cannot `POST /api/v1/htlc/lock`
  tokens it does not hold.** Without it a Scenario A conformance suite can only assert
  shapes against a mock — the exact deficiency this replacement exists to remove. It is
  duplicated per scenario rather than shared with `contracts/amm/`, because the two
  gateways' reserve blocks genuinely differ: a different fiat-exchange path
  (`fiat-exchange` vs `exchange`), a `requester_paladin_identity` field that exists only
  here, a `zeto_transfer_tx_hash` that is **required** here and absent there, and four
  differently-named transaction-hash response fields. One shared document would have had
  to declare endpoints and fields that no single gateway serves.
- Documentation of the **asymmetric deployment split**: the create half of the reserve
  lifecycle is registered only on a commercial-bank gateway and the approve half only on a
  Central Bank gateway, so no single base URL can drive the whole lifecycle.
- `CookieAuth` (`access_token`, HttpOnly, `SameSite=Strict`) as the security scheme.
- The real `ErrorResponse` (`{error}`).
- Rich endpoint descriptions written for implementers with no access to the platform
  repository, including the settlement choreography, the safety-critical timelock
  ordering rule, and the parts of the flow that have **no** API endpoint.
- Five documented divergences between the platform's OpenAPI document and the delivered
  gateway, marked inline and tabulated in `README.md` — including three undeclared 403s
  (the four mutating HTLC routes and, on a Central Bank gateway only, `mintToken`) and an
  undeclared 502 on `searchHTLC`.
- Eight **open questions** flagged rather than silently resolved, headed by the two
  unreconciled HTLC `state` vocabularies: the `HTLCLock.state` field is prefixed
  (`HTLC_STATE_LOCKED`) while the `searchHTLC` `state` query parameter is bare (`LOCKED`),
  and the platform never says which the filter accepts. See `README.md`.

## Why this is a replacement and not an upgrade

The previous contract, `openapi_pvp_v0.1.0.yaml` (2026-02-19, patched 2026-03-05),
**described an API that was never implemented.** It was drafted from an early pilot
sketch, not from the delivered system. Every one of its seven paths was wrong, and so was
its authentication model and its error model:

| | v0.1.0 (never existed) | v2.3.0 (delivered platform) |
|---|---|---|
| FX propose | `POST /fx/agreement` | `POST /api/v1/payments/fx/agreements` |
| FX accept | `POST /fx/agreement/{id}/accept` | `POST /api/v1/payments/fx/agreements/{tradeId}/accept` |
| FX get | `GET /fx/agreement/{id}` | `GET /api/v1/payments/fx/agreements/{tradeId}` |
| HTLC lock | `POST /htlc/lock` | `POST /api/v1/htlc/lock` |
| HTLC settle | `POST /htlc/settle` | `POST /api/v1/htlc/settle` |
| HTLC refund | `POST /htlc/refund` | `POST /api/v1/htlc/refund` |
| HTLC status | `GET /htlc/status/{id}` | `GET /api/v1/htlc/status/{contractId}` |
| Auth | `Authorization: Bearer` | `access_token` HttpOnly cookie (`CookieAuth`) |
| Errors | invented model with machine-readable codes | `{"error": "<free-form message>"}` |
| Field casing | camelCase (`agreementId`, `hashLock`, `timeLock`) | snake_case (`trade_id`, `hash_lock`, `time_lock`) |
| Identifiers | `0xCB002_CountryB` addresses | Paladin identities, `funded_operator@spoke-b-bank-d` |

The v0.1.0 contract also omitted operations the platform has always had — reject, cancel,
settle, the audit trail, HTLC search, and the responder-side `lock-with-hash` leg without
which a cross-spoke atomic swap cannot be completed at all.

### There are no compatibility guarantees with v0.1.0

None are offered, and none are needed: **v0.1.0 had no implementations.** No client, mock
or conformance suite outside this repository was built against it, because the endpoints
it declared did not exist on any CBWeb3 gateway. Nothing can break that was ever working.

Accordingly this release contains **no deprecation shims, no path aliases and no
backward-compatibility layer.** The old paths are gone. Anything in this repository still
referencing `openapi_pvp_v0.1.0.yaml` — mocks, test vectors, conformance tests, sandbox
tutorials — must be repointed at `openapi_pvp_v2.3.0.yaml` and reworked against the real
surface.

## Superseded history

The entries below are retained for the record. They describe the contract that was never
implemented and should not be used as a reference for any behaviour.

### [0.1.1] — 2026-03-05

- Fixed: added Error response schema to HTTP 410 on `/htlc/settle`.
- Fixed: added Error response schema to HTTP 409 on `/htlc/lock` and `/htlc/refund`.

### [0.1.0] — 2026-02-19

- Initial PvP contract, drafted from an early CBWeb3 pilot sketch.
- FX Agreement endpoints: create, accept, get details.
- HTLC endpoints: lock, settle, refund, status.
- Error model with machine-readable codes (the delivered platform has none).
- Example payloads using synthetic Country A ↔ Country B data.
