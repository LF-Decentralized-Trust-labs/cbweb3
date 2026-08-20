# Settlement Flow Walkthrough

What actually happens, call by call, in each of the two CBWeb3 settlement scenarios.

> **Paths are shown in full**, exactly as the contracts declare them and exactly as a
> gateway (or a Prism mock of a contract) serves them: `POST /api/v1/htlc/lock`. There is no
> base path to prepend and nothing is stripped.
>
> **Authentication is a cookie.** Every call below except the ones marked *public* requires
> an `access_token` HttpOnly cookie. There is no bearer token on `/api/v1`.

---

## Which scenario are you in?

| | **Scenario A** (single-ledger, spoke-to-spoke) | **Scenario B** (hub-and-spoke) |
|---|---|---|
| Atomicity mechanism | Two HTLCs sharing one hash lock | Orchestrated bridge-in → Hub AMM swap → bridge-out |
| Terms agreed by | A bilateral FX agreement record | An AMM pool quote with a 15-second TTL |
| Who signs on the hub | there is no hub | the Central Banks, never the commercial bank |
| HTLC endpoints | 6 | **zero** |
| FX agreement endpoints | 7 | **zero** |

Scenario B genuinely has no HTLC and no FX agreement. The `LockHTLC*` schemas that appear in
the Scenario B platform document are unreferenced copy-paste residue and are deliberately
not published by the Toolbox.

---

# Scenario A — PvP settlement with dual-layer HTLCs

PvP settlement makes **both legs of a cross-currency payment settle atomically** — either
both complete or both are reclaimed. That eliminates principal risk.

## Participants (synthetic)

The identities are Paladin `name@node` strings, not wallet addresses and not bank codes.

| Role | Identity | Spoke |
|---|---|---|
| Initiator (originator) | `funded_operator@spoke-a-bank-a` | Spoke-A |
| Settlement agent | `funded_operator@spoke-a-bank-c` | Spoke-A |
| Responder (counterparty) | `funded_operator@spoke-b-bank-d` | Spoke-B |
| Beneficiary | `funded_operator@spoke-b-bank-b` | Spoke-B |

Terms: **1,000,000 BRL → 850,000 ARS at rate 0.85**. Amounts and rates are decimal
**strings**, never numbers, and no decimals count is stated anywhere on the platform — treat
balances as opaque big integers and compare deltas.

---

## Step 0 — the reserve prelude (do not skip it)

`POST /api/v1/htlc/lock` locks tCeBM the bank **must already hold**. tCeBM is obtained only
by completing the reserve lifecycle:

```
  Commercial bank gateway            Central Bank gateway
          │                                   │
          │  POST /api/v1/payments/deposits   │      → PENDING, returns deposit_id
          ├──────────────────────────────────>│
          │  POST .../deposits/approve        │      → 201 {"status":"approved"}
          │  POST .../deposits/fiat-exchange  │      → mints fCeBM (ERC-20, Besu)
          │                                   │
          │  POST /api/v1/payments/escrows    │      → PENDING, returns escrow_id
          ├──────────────────────────────────>│
          │  POST .../escrows/approve         │      → burns fCeBM, mints tCeBM (Zeto)
          │                                   │
          │  GET /api/v1/token/balance        │      → non-zero. Only now can you lock.
```

**The split is asymmetric and it matters.** The `POST` (create) half of deposits, escrows and
redeems is registered only on a **commercial-bank** gateway, which proxies to the Central
Bank. The `approve` / `reject` / `fiat-exchange` half is registered only on a **Central Bank**
gateway and gated by `ROLE_TREASURY`. No single base URL can drive the whole lifecycle: a
test that creates a deposit and approves it needs two sessions against two hosts.

`Scenario divergence, so this cannot be shared:` Scenario A serves
`POST /api/v1/payments/deposits/`**`fiat-exchange`**; Scenario B serves
`POST /api/v1/payments/deposits/`**`exchange`**. Neither gateway serves both.

---

## The settlement flow

```
  Bank-A (Spoke-A)                                  Bank-D (Spoke-B)
  initiator                                         responder
       │                                                 │
       │ 1. POST /api/v1/payments/fx/agreements          │   → PROPOSED
       ├────────────────────────────────────────────────>│
       │                                                 │
       │ 2. POST .../fx/agreements/{tradeId}/accept      │   → ACCEPTED
       │<────────────────────────────────────────────────┤
       │                                                 │
       │ 3. POST /api/v1/htlc/lock                       │   timelock now+3600s
       ├──────┐  server generates the secret,            │   returns hash_lock
       │      │  derives hash_lock, locks tCeBM          │
       │<─────┘                                          │
       │                                                 │
       │ 4. hash_lock crosses OUT OF BAND                │   no API endpoint exists
       │ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─>│
       │                                                 │
       │ 5. POST /api/v1/htlc/lock-with-hash             │   timelock now+1800s
       │                                          ┌──────┤   (SHORTER — see below)
       │                                          └─────>│
       │                                                 │
       │ 6. POST /api/v1/htlc/settle  (reveals secret)   │
       ├─────────────────────────────────────────────────┤
       │        the Cacti relay broadcasts the revealed  │
       │        secret to Spoke-B, which settles the     │
       │        mirror leg with no call from you         │
       │                                                 │
       │ 7. agreement reaches SETTLED                    │
```

### Step 1 — propose the FX agreement

**Who:** the originating bank. **Call:** `POST /api/v1/payments/fx/agreements`.

Creates the agreement in `PROPOSED`. The server checks that
`|counter_amount / origin_amount − rate|` is within `FX_RATE_TOLERANCE_PCT` (default 0.1%),
that the two currencies differ, and that `expiry_date` is a future Unix timestamp **in
seconds**. Returns `{"trade_id": …, "tx_hash": …}`.

`FX_RATE_TOLERANCE_PCT` is a server environment variable exposed by no endpoint. A client
cannot discover the tolerance it is being held to.

### Step 2 — the counterparty accepts

**Who:** the `counterparty_b` party. **Call:** `POST /api/v1/payments/fx/agreements/{tradeId}/accept`.

`PROPOSED → ACCEPTED`. The request body is optional — a `POST` with no body means "accept in
my own name". Only now may HTLC locks be associated with the agreement.

`reject` and `cancel` are the terminal branches. `GET .../{tradeId}/audit` returns the state
transition trail, including transitions attributed to a `SYSTEM_JOB` actor.

### Step 3 — the initiator locks

**Who:** the initiating bank. **Call:** `POST /api/v1/htlc/lock`.

**The gateway generates the secret internally.** You do not choose it and you do not send it.
The server derives a SHA-256 `hash_lock` from it, records the lock metadata on the public
layer (Besu) and locks the token amount on the private layer (Zeto/Paladin). The response
carries `contract_id` and `hash_lock`.

Default `time_lock` is **now + 3600s**.

Note the naming trap reproduced from the platform: the optional field linking the lock to an
FX agreement is called `agreement_id`, but its **value is an FX `trade_id`**.

### Step 4 — the hash lock crosses out of band

**There is no API endpoint for this.** The initiator conveys `hash_lock` to the responder
off-chain. The Toolbox does not invent a call for it.

### Step 5 — the responder locks

**Who:** the responding bank. **Call:** `POST /api/v1/htlc/lock-with-hash`, supplying the
`hash_lock` received in step 4. No secret is generated or stored on this side — the secret is
known only to the initiator until it settles.

Default `time_lock` is **now + 1800s**.

> ### The timelock ordering rule is safety-critical
>
> **The initiator's timelock must be longer than the responder's.** If it is not, the
> initiator can wait for the responder's lock to expire, refund nothing, and still claim the
> responder's leg with the secret — while the responder has lost its own refund window.
>
> Nothing in any schema enforces this. The platform states it in prose only. A test vector,
> mock or tutorial that inverts the ordering models an **unsafe swap** and must be rejected
> in review.

### Step 6 — the initiator settles, revealing the secret

**Call:** `POST /api/v1/htlc/settle` with `contract_id` and `secret`. The contract checks
`SHA-256(secret) == hash_lock`, then calls Zeto `transferLocked()` to release the private
tokens.

**This one call produces state on a gateway you never contacted.** Revealing the secret
publishes it; the Hyperledger Cacti relay broadcasts it to the other spoke, which settles the
mirror leg automatically. There is no API endpoint for that broadcast either.

### Step 7 — observing the result

- `GET /api/v1/htlc/status/{contractId}` — the full lock record.
- `GET /api/v1/htlc/search` — filter by `agreement_id`, `sender`, `receiver` or `state`.
- `GET /api/v1/token/balance` — the settlement assertion that matters.

> **Open question, reproduced rather than resolved.** Two HTLC state vocabularies coexist and
> the platform never reconciles them. The `state` **field** of a lock record is prefixed —
> `HTLC_STATE_INVALID | HTLC_STATE_PENDING | HTLC_STATE_LOCKED | HTLC_STATE_SETTLED |
> HTLC_STATE_REFUNDED` — while the `state` **query parameter** of `/api/v1/htlc/search` is
> bare: `LOCKED | SETTLED | REFUNDED`. From the specification alone a client cannot tell
> which to send to the filter. Resolve it against a live gateway before relying on it.

---

## What if something goes wrong?

### Timeout / refund path

If the secret is never revealed, **each side reclaims its own leg independently** by calling
`POST /api/v1/htlc/refund` after its own timelock expires. Because the responder's timelock
is the shorter one, the responder always regains the ability to refund first. That ordering
*is* the safety property of the whole scheme.

### Errors: assert on status codes, not on message text

The `/api/v1` error model is `{"error": "<free-form string>"}`. **There is no
machine-readable error code.** The string is sometimes a human sentence
(`"invalid state transition"`) and sometimes a code-like token
(`INVALID_CERTIFICATE_CHAIN`, `NONCE_SIGNATURE_MISMATCH`) — in the same field. A
`ErrorCodeResponse {error, code}` schema is declared in both platform specifications and
referenced by **zero** operations.

| Situation | Status | Notes |
|---|---|---|
| No `access_token` cookie | `401` | On every non-public operation |
| HTLC mutation from an oversight-only session | `403` | Undeclared upstream; the router gates `lock`, `lock-with-hash`, `settle` and `refund` on `ROLE_BANK` / `ROLE_COMMERCIAL_BANK` / `ROLE_TREASURY` / `ROLE_GOVERNANCE` |
| Accepting an agreement that is not `FX_STATE_PROPOSED`, or has expired | `409` | The platform document declares `412`; the delivered gateway maps gRPC `FailedPrecondition` to `409 Conflict`. See `DIVERGENCES.md` row A4 |
| Calling `lock-with-hash` with a body that parses but is rejected downstream | `500` | That handler never consults a certificate and never returns `412`; it wraps every downstream error as 500 |
| Approve/reject without `ROLE_TREASURY` | `403` | Reserve lifecycle only |
| Route not registered on this gateway role | `404` | **Not** a conformance failure — see below |

**A 404 may mean "wrong kind of node".** Route groups are wired conditionally: Scenario A
registers HTLC / token / FX only when the payment orchestrator is present, the escrow
*approve* half only on a Central Bank gateway, and the *create* half only on a commercial-bank
gateway. A conformance run must declare a gateway profile and **skip**, not fail, when the
deployment cannot serve a route.

**No `EXPIRED` FX state exists.** Every agreement carries an `expiry_date`, and the audit
trail documents a `SYSTEM_JOB` background expiration worker — but there is no terminal state
that represents expiry. This is recorded as an open question, not resolved by guesswork.

**There is no idempotency mechanism.** No `Idempotency-Key`, no `If-Match`/ETag, no
operation-level header parameters anywhere. Replay safety is per-endpoint and payload-keyed.
`X-Correlation-Id` is server-generated and overrides any client value, so it cannot serve as
one. A generic retry wrapper is not part of this API.

---

## Cryptographic reference values

The hash lock in a real Scenario A swap is **generated by the gateway**, not chosen by the
client — `POST /api/v1/htlc/lock` returns it. The pair below exists so that fixtures for
`POST /api/v1/htlc/lock-with-hash` and any local SHA-256 implementation can be checked
against a value you can recompute yourself:

**The secret is 32 bytes carried as 64 lowercase hex characters**, and the gateway
**hex-decodes it before hashing**: `hash_lock = SHA-256(hex_decode(secret))`. It does *not*
hash the ASCII of the string you send. A human-readable passphrase is therefore not a valid
secret — anything that is not 64 hex characters is rejected with
`400 "hash_lock must be a 64-char hex string (32 bytes)"`. This is the single most common
way a first HTLC integration fails.

| Field | Value |
|---|---|
| Secret (hex, 32 bytes) | `d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c` |
| Hash algorithm | SHA-256 over the **decoded bytes** |
| Hash lock | `93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2` |

```bash
printf 'd2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c' | xxd -r -p | sha256sum
# 93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2

# or, without xxd:
python3 -c "import hashlib;print(hashlib.sha256(bytes.fromhex('d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c')).hexdigest())"
```

This pair is **chosen by the Toolbox**, not taken from the platform. It is published because
it is independently verifiable in one line, and it is the pair the `pvp` mock fixtures ship;
the `pvp` test vectors carry their own, derived identically.
`Toolbox/tools/verify_hashlocks.py` recomputes every documented pair in CI, so a wrong digest
can never ship again — earlier Toolbox material published a hash lock that was not the
SHA-256 of anything, was not even valid hex, and told implementers to verify it.

---

# Scenario B — cross-currency settlement through the International Hub

Scenario B has no HTLCs. A commercial bank makes **one** call, and it fans out into four
relay-only, sovereignty-preserving steps across two Central Bank gateways.

```
  Bank (Spoke BRL)        Its Central Bank        Hub AMM        Beneficiary's Central Bank
        │                        │                   │                      │
        │ POST /api/v2/amm/swap/cross-currency       │                      │
        ├───────────>│           │                   │                      │
        │            │ bridge-in: lock spoke tCeBM,  │                      │
        │            │ mint Hub W-token              │                      │
        │            ├──────────────────────────────>│                      │
        │            │           │  Hub swap, signed by that same CB        │
        │            │           │                   │                      │
        │            │           │  bridge-out: the BENEFICIARY's CB burns  │
        │            │           │  the W-token and delivers on its spoke   │
        │            │           │                   ├─────────────────────>│
        │            │           │                   │                      │
        │            │  residue return: the unspent slippage buffer comes   │
        │            │  back to the payer as a SEPARATE bridge position     │
        │<───────────┤           │                   │                      │
```

The commercial bank never holds a W-token and never signs on the Hub. CB-A burning CB-B's
tokens would be a sovereignty breach, which is why the *beneficiary's* Central Bank performs
the bridge-out. That is why one `POST` produces asynchronous state on gateways you never
called, and why partial failure is expressible.

## 1. Discovery — no credentials at all

| Call | Purpose |
|---|---|
| `GET /api/v2/amm/hub-config` | Hub contract addresses (AMM, pair registry, currency registry) |
| `GET /api/v2/hub/currencies` | Symbol → Hub W-token address, and which CB proposed it |
| `GET /api/v2/amm/pairs` | **The only supported source of a valid `pool_pair` value** |
| `GET /api/v2/amm/pool/{pair}/status` | `pool_status` must be `ACTIVE` before you swap |
| `GET /api/v2/governance/circuit-breaker/status` | A halted breaker makes the swap fail |

**Never construct a pool-pair identifier.** Five incompatible spellings appear across the
delivered artefacts (`tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`, `W-BRL-ARS`, the
quote-style `W-BRL-W-ARS` and the circuit breaker's `BRL-USD` fallback) and none is documented
as canonical — see [`../../DIVERGENCES.md`](../../DIVERGENCES.md). Treat `pair_id` as an opaque
string taken verbatim from `GET /api/v2/amm/pairs`.

**Always pass `pair` to the circuit-breaker status call.** Omitting it does not report "the
network" — the handler substitutes the literal `BRL-USD`, which on most deployments is not a
registered pair, so an omitted parameter can return a healthy-looking `LIVE` for a pair
nobody trades.

`Spec divergences, mirrored:` `hub/currencies`, `amm/pairs` and `bridge/positions` are
documented upstream as bare JSON arrays but actually return wrapped objects
(`{"currencies": […]}`, `{"pairs": […]}`, `{"positions": […]}`). The Toolbox models the
wrapped form, which is what a client receives.

## 2. Reserve prelude — mandatory, and enforced

Identical in shape to Scenario A (with `deposits/exchange` instead of
`deposits/fiat-exchange`), and it is not optional: the bridge-in handler refuses a swap with
*"insufficient tokenized reserves: bank %s holds %s tCeBM, need %s — complete Reserve
Tokenisation first"*.

## 3. Corridor provisioning — Central Bank work

Only needed once per corridor: `POST /api/v2/hub/currencies` → `POST /api/v2/amm/pairs/propose`
(the CB of token A) → `POST /api/v2/amm/pairs/confirm` (the CB of token B) →
`POST /api/v2/bridge/lock-mint` → `POST /api/v2/amm/token/mint-and-approve` →
`POST /api/v2/amm/liquidity/commit` for each side → poll
`GET /api/v2/amm/liquidity/commits/{commit_id}` until `EXECUTED`.

## 4. Quote, then swap — in one step

`GET /api/v2/amm/quote/cross-currency` requires `source_currency`, `target_currency` and
`amount_out`.

> `Spec divergence (blocking):` the platform document declares **zero parameters** for this
> operation while the handler requires three. A client written strictly against the upstream
> document always gets 400. The same class of defect affects
> `GET /api/v2/amm/quote/exact-output`, documented as taking `amountOut` while the handler
> reads `amount_out`.

**The quote lives 15 seconds.** Issue the quote and the swap programmatically in one step; a
walkthrough that pauses between them will simply expire.

`POST /api/v2/amm/swap/cross-currency` returns a `swap_id`. This endpoint is **cookie-only**
(`RequireCookieAuth`) — the bearer token that works on other `/api/v2` routes will not work
here. Do not send `payer_bank_id`: caller identity is deprecated in the body and is taken
from the session.

Poll `GET /api/v2/amm/swap/cross-currency/{id}` until `status` is `COMPLETED` or `FAILED`.
The intermediate vocabulary — `QUOTING`, `BRIDGE_IN_PROGRESS`, `SWAP_IN_PROGRESS`,
`BRIDGE_OUT_PROGRESS` — is **not in the platform's OpenAPI at all**; it is recovered from Go
domain code and is published as documented strings, not as an enum the platform has committed
to. The same is true of bridge position state (`LOCKING`, `ACTIVE`, `BURNING`, `BURNED`,
`RELEASED`, `RECONCILIATION_REQUIRED`) — and the upstream example values `PENDING` and
`CLOSED` do not exist.

## 5. Settlement verification — this is what makes it a settlement test

`status == COMPLETED` alone is not enough.

| Assertion | Why |
|---|---|
| The payer was debited **`amount_in`**, not `max_amount_in` | The full `max_amount_in` is bridged in before the swap runs, because the realized input is unknown until execution |
| `residue_status` is `RETURNED` | The unspent slippage buffer is burned on the Hub and returned to the payer on its source spoke as a **separate** bridge position (`leg=RESIDUE`). `RETURN_FAILED` means the payer has **not** been made whole |
| The bridge-out position reached `BURNED` / `RELEASED` | Delivery actually happened |
| The beneficiary balance is non-zero | The payment arrived |
| `GET /api/v2/amm/hub-reconciliation` reports `balanced == true` | Nothing is stranded |

## 6. Errors on `/api/v2`

Unlike `/api/v1`, the v2 handlers do emit a machine-readable code — under three different
spellings across the surface (`code`, `error_code`, and Scenario A's undocumented
`required_role` array). None of the 422 codes below is declared in the platform document;
they are observed in the delivered handlers and are marked as such in the contract.

| Situation | Status | `error_code` |
|---|---|---|
| Daily transfer limit exhausted | `422` | `TRANSFER_LIMIT_EXCEEDED` |
| Pool has no liquidity on one side | `422` | `POOL_NOT_ACTIVE` |
| A Central Bank has paused the pair | `422` | `CIRCUIT_BREAKER_HALTED` |
| Not enough depth for the requested output | `422` | `INSUFFICIENT_POOL_LIQUIDITY` |
| No session cookie | `401` | `UNAUTHENTICATED` |
| **Hub swap succeeded, delivery did not** | `500` | `BRIDGE_OUT_FAILED` — **do not retry**; reconcile with the beneficiary Central Bank |

---

## Next steps

- **Scenario A, hands on:** [Tutorial 1: PvP settlement against a mock](../tutorials/01-pvp-settlement-mock.md)
- **Scenario B, hands on:** [Tutorial 3: Hub swap against a mock](../tutorials/03-hub-swap-mock.md)
- **Understand the rules:** [Conformance Requirements](../../conformance/spec/conformance_requirements.md)
- **See the contracts:**
  [pvp](../../contracts/pvp/openapi_pvp_v2.3.0.yaml) ·
  [amm](../../contracts/amm/openapi_amm_v2.3.0.yaml) ·
  [auth](../../contracts/auth/openapi_auth_v2.3.0.yaml)
