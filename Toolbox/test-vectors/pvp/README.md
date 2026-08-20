# Scenario A — PvP Settlement — Test Vectors

**54 vectors in 3 files** against
[`../../contracts/pvp/openapi_pvp_v2.3.0.yaml`](../../contracts/pvp/openapi_pvp_v2.3.0.yaml)
(28 paths / 32 operations, zero public).

Scenario A is the single-ledger, spoke-to-spoke scenario: two banks on two spokes settle a
cross-currency payment atomically with a **dual-layer HTLC**, coordinated by an **FX agreement**.
It is the only scenario with an FX agreement surface and the only one with HTLC.

| File | Covers | hp | err | edge | total |
|---|---|---|---|---|---|
| `pvp_reserves_vectors.json` | deposit → approve → fiat-exchange → escrow → approve → balance | 7 | 5 | 1 | **13** |
| `pvp_fx_agreement_vectors.json` | propose, accept, reject, cancel, settle, get, list, audit | 7 | 11 | 1 | **19** |
| `pvp_htlc_vectors.json` | initiator lock, responder lock-with-hash, settle, refund, status, search | 7 | 11 | 4 | **22** |

## Run them in this order

The three files are one flow, not three independent suites.

1. **Reserve prelude** — `pvp_reserves_vectors.json`. A bank cannot HTLC-lock tCeBM it does not
   hold. A conformance run that skips this can never execute a real lock, only a 401.
2. **FX agreement** — `pvp_fx_agreement_vectors.json`. `PROPOSED → ACCEPTED` before any lock.
3. **HTLC** — `pvp_htlc_vectors.json`. Initiator locks, hash crosses **out of band**, responder
   locks with that hash, initiator settles and reveals the secret.

There is **no API endpoint that transfers the hash lock between spokes**, and no relay endpoint
of any kind in the HTLC family. Cross-spoke secret propagation happens through the Cacti relay
and is described only in prose: settling on Spoke-A produces state changes on a gateway you
never contacted.

## The safety-critical invariant

> **The initiator's timelock MUST be longer than the responder's**, so the responder can always
> refund before the initiator regains the ability to. The defaults encode it — `now+3600` for
> `POST /api/v1/htlc/lock`, `now+1800` for `POST /api/v1/htlc/lock-with-hash` — but **nothing on
> the server enforces the relationship.** A client that supplies an explicit responder timelock
> longer than the initiator's is accepted and has built an unsafe swap.

`htlc-hp-01` and `htlc-hp-02` carry explicit timestamps that preserve the ordering
(`1798765200` = 2027-01-01T00:00:00Z + 3600s, `1798763400` = the same anchor + 1800s).
`htlc-edge-04` covers the default path and asserts the invariant directly.
**Any vector or mock that inverts this ordering must be rejected in review.**

## The hash lock

```
secret     6362776562332d746f6f6c626f782d68746c632d766563746f722d30312d6f6b   (64 hex, 32 bytes)
hash_lock  cd0a60b0332435927cb5648fe6d22a7be02f3d2cfacb1d4a0e8cce0b7a8d7338   (64 hex, NO 0x)
```

`hash_lock = SHA-256(hex_decode(secret))`. The gateway hex-decodes the secret and hashes the
**decoded bytes** — it does **not** hash the ASCII of the string you send. Verify:

```bash
python3 -c "import hashlib;print(hashlib.sha256(bytes.fromhex('6362776562332d746f6f6c626f782d68746c632d766563746f722d30312d6f6b')).hexdigest())"
# cd0a60b0332435927cb5648fe6d22a7be02f3d2cfacb1d4a0e8cce0b7a8d7338
```

Those 32 bytes are the ASCII of `cbweb3-toolbox-htlc-vector-01-ok`, so `sha256` of that literal
string gives the same digest — a second, independent way to check the pair. The wrong secret in
`htlc-err-01` is the ASCII of `cbweb3-toolbox-htlc-vector-01-no` and hashes to something else,
which is what makes it a genuine mismatch rather than a formatting error.

The pair is **Toolbox-chosen and independently verifiable**. It is *not* taken from the platform
repository: a grep for either digest across the delivered source returns nothing. Do not
attribute it to the vendor in DPG evidence.

**Retracted.** Every pre-realignment Toolbox artefact claimed that
`SHA-256('cbweb3-test-secret-2026')` was `0x7f83b165…9069` — retracted, and purged from every
artefact. That claim was false in two independent ways: the real digest of that ASCII string is
`000cd63c…fdcf`, **and** the string is not valid hex, so the gateway would have rejected it with
"invalid secret hex" before hashing anything. `tools/verify_hashlocks.py` recomputes the
`syntheticSecrets` block of `pvp_htlc_vectors.json` in CI so this cannot recur.

## Two HTLC state vocabularies — reproduced, not reconciled

`HTLCLock.state` uses the **prefixed** enum
`HTLC_STATE_INVALID | HTLC_STATE_PENDING | HTLC_STATE_LOCKED | HTLC_STATE_SETTLED | HTLC_STATE_REFUNDED`,
while the `state` **query parameter** on `GET /api/v1/htlc/search` uses the **bare** enum
`LOCKED | SETTLED | REFUNDED`. The platform reconciles them nowhere. Send the bare form
(`htlc-hp-06`), read the prefixed form (`htlc-hp-03`). Do not normalise.

## Contract defects these vectors expose

### FX-409 — the FX state machine returns `409`, not the documented `412`

Every invalid FX state transition returns **`409 Conflict`**. The platform document declares
`412`, and it is not right:

* the orchestrator raises `codes.FailedPrecondition` for every bad transition
  (`payment-orchestrator/internal/grpc/server/server.go:1187`, `:1262`, `:1319`, `:1367`);
* `grpcErrorToHTTP` maps `FailedPrecondition` → `fiber.StatusConflict`
  (`api-gateway/internal/http/handlers/payment.go:1126-1127`);
* the **only** place the Scenario A gateway emits `412` at all is
  `onboarding_proxy.go:465` — the missing-participant-certificate case, which is what the `412`
  on `lockHTLCWithHash` and on `POST /api/v1/auth/pki-login` actually means.

`fx-err-07`, `fx-err-08` and `fx-err-09` assert **409**, and the contract now declares 409 on
all four FX actions with a `Spec divergence:` note naming the document's 412. **No vector in the
repository asserts a status its contract does not declare.** A vector asserting `412` would fail
against every real deployment, which is exactly the class of failure this realignment exists to
eliminate. Catalogued as row **A4** in [`../../DIVERGENCES.md`](../../DIVERGENCES.md).

### RESERVE-500 — a missing required field is `500`, not `400`

The reserve handlers and the two HTLC lock handlers return `400` **only** for a body that fails
to parse. A body that parses but omits a required field goes downstream, the orchestrator answers
`InvalidArgument`, and the gateway returns `fiber.StatusInternalServerError` unconditionally
instead of using its own `grpcErrorToHTTP` mapper (`payment.go:599-602`, `:250-253`, `:282-285`,
`:303-306`, `:323-326`). Both `400` and `500` are declared on those operations, so the vectors
stay inside the contract while documenting the trap.

The reachable `400` on those routes is a **type** error, not an absence: `reserve-err-01` sends
`"amount": 10000000` as a JSON number where the handler expects a string.

### FX-REQUIRED — four "optional" fields are mandatory

`source_spoke_id`, `dest_spoke_id`, `source_receiver` and `dest_receiver` are listed as optional
by the platform document and therefore by the contract. The delivered service requires all four
and requires the two spoke ids to differ (`server.go:1009-1021`). A client sending exactly the
contract's required set **always** gets `400`. See `fx-err-03`; every happy-path FX vector sends
all four.

### DEPOSIT-STATUS — the creation responses carry only an id

`RegisterDepositResponse` and `RequestEscrowResponse` each carry a single field in the delivered
proto (`payment_orchestrator.proto:375-377`, `:421-423`). The contract mirrors the platform
document's extra `status: "PENDING"` property, which is never emitted. `reserve-hp-01` and
`reserve-hp-04` assert only the id.

### HTLC-403 has two independent sources, and the contract describes one

`403` on a mutating HTLC route can mean the **router role gate** (`ROLE_BANK`,
`ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY`, `ROLE_GOVERNANCE` — `htlc-err-10`) **or** the
per-record **counterparty check** in the handler (`htlc-err-09`, `payment.go:358-365`). Only the
first is described in the contract. The same counterparty check also guards
`GET /api/v1/htlc/status/{contractId}`, where the contract declares **no `403` at all** — that
gap is recorded here and deliberately not asserted.

## Gaps recorded, not filled

* **No `EXPIRED` FX state.** Every agreement carries `expiry_date` and the audit trail documents
  a `SYSTEM_JOB` background expiration worker, yet the state enum has no `EXPIRED` member and no
  distinct terminal state. A client must compare `expiry_date` itself (`fx-edge-01`). The gate
  that *is* enforced sits at HTLC lock time (`htlc-err-08`).
* **No `404` anywhere in the HTLC family.** An unknown `contract_id` surfaces as `500`
  (`htlc-err-07`), because the gateway's counterparty pre-check maps only `PermissionDenied` to
  `403` and everything else — including `NotFound` — to `500`.
* **No write path for relay-delivered FX state.** The `Internal` tag claims the relay delivers FX
  state changes idempotently, but the only registered internal FX route is a `GET`. Nothing is
  modelled on it.
* **`FX_RATE_TOLERANCE_PCT` and `FX_AGREEMENT_HTLC_STRICT`** change server behaviour and are
  exposed by no endpoint, so a client cannot discover the tolerance in force (`fx-err-05`) or
  whether strict HTLC mode is on (`htlc-err-08`). Pente on/off changes FX behaviour materially
  with no capability endpoint to distinguish deployments (`fx-err-08`).
* **No decimals count for tCeBM or fCeBM.** `/api/v1` amounts are opaque big-integer strings with
  no stated unit. Compare them as integer strings, never as floats, and never assume 18.
* **Five routes are wired by the Scenario A router but appear in no OpenAPI document** —
  `GET /api/v1/statement`, `/api/v1/identities/*`, `/internal/v1/identities/*`,
  `POST /internal/v1/payments/pvp-legs`, `GET /internal/v1/payments/pvp-credits`
  (`router.go:149-158`, `:240-243`). Notably the last two are the only endpoints on the entire
  platform whose names contain "pvp", and both are relay-internal. Nothing is modelled on them.
* **A third HTLC identifier form.** The compliance surface describes an HTLC contract id as
  0x-prefixed hex while every HTLC endpoint uses the `htlc-a1b2c3d4` form — and the delivered
  generator actually emits a 64-character unprefixed hex string. Treat `contract_id` as opaque.
