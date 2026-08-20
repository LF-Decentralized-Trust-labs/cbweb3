# Tutorial 1 — Scenario A: PvP settlement against a mock

**Time:** ~25 minutes · **Prerequisites:** Node.js 18+, `curl`, `jq` · **Scenario:** A
(single-ledger, spoke-to-spoke)

You will walk a complete cross-border payment-versus-payment settlement — reserve prelude,
FX agreement, both HTLC legs, settlement, and the refund branch — using nothing but `curl`
against a local mock of the delivered CBWeb3 API Gateway v2.3.0 contract.

Every path, field name and status code below is taken from
[`Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml`](../../contracts/pvp/openapi_pvp_v2.3.0.yaml),
which mirrors the Scenario A gateway as delivered.

> **Scenario B has none of this.** No HTLC endpoints, no FX agreement — it settles through
> the International Hub instead. If that is your integration, go to
> [tutorial 03](03-hub-swap-mock.md).

---

## Setup

Start the Scenario A mock from the repository root:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010
```

Optionally, in a second terminal, the shared auth contract (only needed for step 0):

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012
```

Then, in a working terminal:

```bash
export PVP=http://localhost:4010
export AUTH=http://localhost:4012
export COOKIE='Cookie: access_token=SYNTHETIC_COOKIE_CI'
export JSON='Content-Type: application/json'
```

> **Paths are complete.** Contract paths carry their own `/api/v1` prefix and the
> `servers:` entries are bare origins. `http://localhost:4010/api/v1/htlc/lock` is the real
> path — nothing is stripped and nothing is re-prefixed. Never append `/api/v1` to
> `CBWEB3_BASE_URL`.

---

## The synthetic corridor

Identities on the platform are Paladin `name@node` strings — not wallet addresses, not
bank codes.

| Role | Identity |
|---|---|
| Initiator / originator | `funded_operator@spoke-a-bank-a` |
| Settlement agent | `funded_operator@spoke-a-bank-c` |
| Responder / counterparty | `funded_operator@spoke-b-bank-d` |
| Beneficiary | `funded_operator@spoke-b-bank-b` |

Terms: **1,000,000 BRL → 850,000 ARS at rate 0.85**.

Amounts and rates are decimal **strings**, never JSON numbers. **No decimals count is
stated anywhere on the platform** for tCeBM or fCeBM — treat balances as opaque big
integers and compare deltas rather than assuming a scale factor.

---

## Step 0 — Authenticate

The whole `/api/v1` surface is guarded by an `access_token` **HttpOnly cookie**. There is
no bearer token on `/api/v1`, and there is **no CSRF header anywhere** on the platform —
`SameSite=Strict` is the sole cross-site mitigation.

Against Prism, any cookie value works, because Prism validates the *presence* of a cookie
credential and never its value. That is the `$COOKIE` variable you exported above.

Against a **real** gateway there are two flows, and which one you get is decided by your
role, not by which endpoint you call:

**Direct flow** — governance, supervisor and NOC roles. `Set-Cookie` does the work:

```bash
curl -s -c jar.txt -X POST "$AUTH/api/v1/auth/login" \
  -H "$JSON" \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}' | jq .
```
```json
{
  "accessToken": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600,
  "tokenType": "Bearer"
}
```

**PKI flow** — what a **commercial bank** actually performs. The *same* endpoint, the
*same* `200`, a completely different body: no tokens, no cookies, just a nonce to sign.

```bash
curl -s -X POST "$AUTH/api/v1/auth/login" \
  -H "$JSON" -H 'Prefer: example=pkiNonceFlow' \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}' | jq .
# → {"nonce":"9f2c7a1de4b83056af11c2d9e7b40a3c"}
```

You then sign that nonce with your P-256 participant key and bind the wallet, which is the
call that actually sets the cookies:

```bash
curl -s -c jar.txt -X POST "$AUTH/api/v1/auth/wallet/bind" \
  -H "$JSON" \
  -d '{
        "user_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
        "nonce_signature_hex": "3045022100c1f0...022043ab...",
        "cert_pem": "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----\n"
      }' | jq .
```

> **A known upstream defect, mirrored rather than fixed.** The `200` on
> `POST /api/v1/auth/login` is typed as `AuthResponse`, which has no `nonce` property and
> *requires* `accessToken`/`expiresIn`/`tokenType`. A strict response validator therefore
> rejects a perfectly correct PKI login. The Toolbox contract models the nonce branch as a
> documented `oneOf` marked `x-cbweb3-source: implementation-observed`, because the shape is
> recovered from the handler and the platform's prose, not from its OpenAPI document.

Confirm the session and — importantly — read your roles, because they decide which of the
calls below your session may actually make:

```bash
curl -s -b jar.txt "$AUTH/api/v1/auth/me" | jq .
# → {"subject":"f47ac10b-...","roles":["ROLE_COMMERCIAL_BANK"],"country":"BR","bankId":"341", ...}
```

For the rest of this tutorial we use the synthetic mock cookie.

---

## Step 1 — The reserve prelude (do not skip it)

`POST /api/v1/htlc/lock` locks tCeBM the bank **must already hold**. tCeBM is obtained only
by completing the reserve lifecycle. Skipping this is the single most common integration
failure.

### 1a. Register a deposit (commercial-bank gateway)

```bash
curl -s -X POST "$PVP/api/v1/payments/deposits" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"amount":"10000000"}' | jq .
```
```json
{ "deposit_id": "dep-a1b2c3d4", "status": "PENDING" }
```

`requester_besu_address` and `requester_paladin_identity` are optional: on a
commercial-bank gateway the proxy injects the caller's identity, so sending them is
neither required nor honoured.

### 1b. Approve it (Central Bank gateway, `ROLE_TREASURY`)

```bash
curl -s -X POST "$PVP/api/v1/payments/deposits/approve" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"deposit_id":"dep-a1b2c3d4"}' | jq .
```
```json
{ "status": "approved" }
```

Note the status code: **`201`**, not `200`.

### 1c. Exchange fiat for fCeBM

```bash
curl -s -X POST "$PVP/api/v1/payments/deposits/fiat-exchange" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"deposit_id":"dep-a1b2c3d4"}' | jq .
```
```json
{ "tx_hash": "0x1234..." }
```

> **Scenario divergence — do not copy this path into a Scenario B client.** Scenario A
> serves `POST /api/v1/payments/deposits/`**`fiat-exchange`**; Scenario B serves
> `POST /api/v1/payments/deposits/`**`exchange`**. Neither gateway serves both. It is the
> one structural difference that prevents the reserve lifecycle from being shared between
> the two contracts.

### 1d. Request tokenisation escrow (fCeBM → tCeBM)

```bash
curl -s -X POST "$PVP/api/v1/payments/escrows" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"amount":"5000000"}' | jq .
```
```json
{ "escrow_id": "esc-a1b2c3d4", "status": "PENDING" }
```

### 1e. Approve the escrow (Central Bank gateway)

```bash
curl -s -X POST "$PVP/api/v1/payments/escrows/approve" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"escrow_id":"esc-a1b2c3d4"}' | jq .
```
```json
{ "burn_tx_hash": "0x1234...", "zeto_mint_tx_hash": "0x5678..." }
```

Two hashes, because two things happened: fCeBM (ERC-20 on Besu) was **burned**, and the
equivalent tCeBM (Zeto privacy token on Paladin) was **minted**.

### 1f. Confirm the balance

```bash
curl -s "$PVP/api/v1/token/balance" -H "$COOKIE" | jq .
# → {"balance":"9000000"}
```

Only now can you lock.

> **The gateway split is asymmetric and it matters.** The *create* half of deposits,
> escrows and redeems is registered only on a **commercial-bank** gateway, which proxies to
> the Central Bank. The `approve` / `reject` / `fiat-exchange` half is registered only on a
> **Central Bank** gateway and is gated on `ROLE_TREASURY`. Against a real deployment,
> steps 1a/1d and 1b/1c/1e need **two sessions against two hosts**. One Prism process
> serves both, which is convenient and slightly dishonest — remember it when you move to a
> devnet.

---

## Step 2 — Propose the FX agreement

```bash
curl -s -X POST "$PVP/api/v1/payments/fx/agreements" \
  -H "$JSON" -H "$COOKIE" \
  -d '{
        "trade_id": "trade-a1b2c3d4",
        "counterparty_b": "funded_operator@spoke-b-bank-d",
        "originator": "funded_operator@spoke-a-bank-a",
        "settlement_agent": "funded_operator@spoke-a-bank-c",
        "custodian": "funded_operator@spoke-b-bank-d",
        "beneficiary": "funded_operator@spoke-b-bank-b",
        "origin_amount": "1000000",
        "counter_amount": "850000",
        "origin_currency": "BRL",
        "counter_currency": "ARS",
        "rate": "0.85",
        "expiry_date": 1711590000
      }' | jq .
```
```json
{ "trade_id": "trade-a1b2c3d4", "tx_hash": "0xabcd1234..." }
```

The agreement is created in state `PROPOSED`. Only `counterparty_b`, the two amounts, the
two currencies, `rate` and `expiry_date` are required; `trade_id` is optional and generated
if you omit it.

Server-side checks you cannot see from the response: the two currencies must differ,
`expiry_date` must be a future Unix timestamp **in seconds**, and
`|counter_amount / origin_amount − rate|` must fall within `FX_RATE_TOLERANCE_PCT`.

> **Open question, recorded not resolved.** `FX_RATE_TOLERANCE_PCT` is a server environment
> variable exposed by **no endpoint**. A client cannot discover the tolerance it is being
> held to. The same is true of `FX_AGREEMENT_HTLC_STRICT`, and of whether Pente privacy
> groups are enabled — all three change server behaviour and none is discoverable.

---

## Step 3 — The counterparty accepts

```bash
curl -s -X POST "$PVP/api/v1/payments/fx/agreements/trade-a1b2c3d4/accept" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"on_behalf": false}' | jq .
```
```json
{ "tx_hash": "0xabcd1234..." }
```

`PROPOSED → ACCEPTED`. The body is **optional** — a `POST` with no body means "accept in my
own name". Only now may HTLC locks be associated with the agreement.

`reject` and `cancel` are the terminal branches, at
`POST .../fx/agreements/{tradeId}/reject` and `.../cancel`.

---

## Step 4 — Read the agreement and its audit trail

```bash
curl -s "$PVP/api/v1/payments/fx/agreements/trade-a1b2c3d4" -H "$COOKIE" | jq .
```
```json
{
  "agreement": {
    "trade_id": "trade-a1b2c3d4",
    "originator": "funded_operator@spoke-a-bank-a",
    "counterparty_b": "funded_operator@spoke-b-bank-d",
    "origin_amount": "1000000",
    "counter_amount": "850000",
    "origin_currency": "BRL",
    "counter_currency": "ARS",
    "rate": "0.85",
    "expiry_date": 1711590000,
    "state": "ACCEPTED"
  }
}
```

Note the envelope: the record is nested under `agreement`, not returned bare.

```bash
curl -s "$PVP/api/v1/payments/fx/agreements/trade-a1b2c3d4/audit" -H "$COOKIE" | jq .
```
```json
{
  "events": [
    {
      "id": 1,
      "trade_id": "trade-a1b2c3d4",
      "from_state": "PROPOSED",
      "to_state": "ACCEPTED",
      "actor": "funded_operator@spoke-b-bank-d",
      "occurred_at_unix": 1711500000,
      "notes": "KYC verified — agreement accepted",
      "tx_hash": "0xabcd1234...",
      "source": "LOCAL_API"
    }
  ],
  "total": 1
}
```

`source` also takes the value `SYSTEM_JOB`, attributed to a background expiration worker.

> **There is no `EXPIRED` state.** The five FX states are `PROPOSED`, `ACCEPTED`,
> `REJECTED`, `CANCELLED`, `SETTLED`. Every agreement carries an `expiry_date` and the audit
> trail documents an expiration worker, yet expiry has **no representable terminal state**.
> This is an open question against the platform, not something the Toolbox papers over.

---

## Step 5 — The initiator locks

```bash
TIME_LOCK_INITIATOR=$(( $(date +%s) + 3600 ))

curl -s -X POST "$PVP/api/v1/htlc/lock" \
  -H "$JSON" -H "$COOKIE" \
  -d "{
        \"agreement_id\": \"trade-a1b2c3d4\",
        \"receiver\": \"funded_operator@spoke-a-bank-c\",
        \"amount\": \"1000000\",
        \"time_lock\": $TIME_LOCK_INITIATOR
      }" | jq .
```
```json
{
  "contract_id": "htlc-a1b2c3d4",
  "hash_lock": "0xabcdef1234567890...",
  "htlc_tx_hash": "0x1234...",
  "zeto_tx_hash": "0x5678..."
}
```

Three things to internalise here:

1. **The gateway generates the secret.** You do not choose it and you do not send it. The
   server derives the SHA-256 `hash_lock` from it, records the lock on the public layer
   (Besu) and locks the tokens on the private layer (Zeto/Paladin).
2. **`agreement_id` carries an FX `trade_id`.** The field name and its value disagree. This
   is the platform's naming, reproduced verbatim.
3. **The `hash_lock` in that example response is an ellipsis**, not a real digest — the
   platform's own example is truncated. Do not copy it into a fixture. Read the real value
   from the response of your own lock call.

Default `time_lock` is now + 3600s.

---

## Step 6 — The hash lock crosses out of band

**There is no API endpoint for this.** The initiator conveys `hash_lock` to the responder
off-platform. The Toolbox does not invent a call for it, and neither should you.

---

## Step 7 — The responder locks with the received hash

```bash
TIME_LOCK_RESPONDER=$(( $(date +%s) + 1800 ))

curl -s -X POST "$PVP/api/v1/htlc/lock-with-hash" \
  -H "$JSON" -H "$COOKIE" \
  -d "{
        \"agreement_id\": \"trade-a1b2c3d4\",
        \"receiver\": \"funded_operator@spoke-b-bank-d\",
        \"amount\": \"1000000\",
        \"time_lock\": $TIME_LOCK_RESPONDER,
        \"hash_lock\": \"93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2\"
      }" | jq .
```
```json
{
  "contract_id": "htlc-b9c8d7e6",
  "hash_lock": "a3f1b2c4d5e6f7890123456789abcdef0123456789abcdef0123456789abcdef01",
  "htlc_tx_hash": "0xabcd1234...",
  "zeto_tx_hash": "0xef567890..."
}
```

No secret is generated or stored on this side. The secret is known only to the initiator
until it settles.

> ### ⚠ The timelock ordering rule is safety-critical
>
> **The initiator's timelock (3600s) must be longer than the responder's (1800s).**
>
> If it is not, the initiator can wait for the responder's lock to expire, decline to
> refund, and still claim the responder's leg with the secret — while the responder has
> already lost its refund window. That is an unsafe swap.
>
> **Nothing in any schema enforces this.** The platform states it in prose only. Any test
> vector, mock or tutorial that inverts the ordering must be rejected in review.

> ### The secret is 32 bytes of hex, and the gateway hex-decodes it
>
> `hash_lock` is **64 lowercase hex characters (32 bytes)**, and the gateway computes
> `hash_lock = SHA-256(hex_decode(secret))`. It hashes the **decoded bytes**, not the ASCII
> of the string you send. A human-readable passphrase is not a valid secret: anything that
> is not 64 hex characters is rejected with
> `400 "hash_lock must be a 64-char hex string (32 bytes)"`.
>
> The pair used above is the one the Toolbox fixtures ship, and you can recompute it:
>
> ```bash
> printf 'd2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c' | xxd -r -p | sha256sum
> # 93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2
>
> # or, without xxd:
> python3 -c "import hashlib;print(hashlib.sha256(bytes.fromhex('d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c')).hexdigest())"
> ```
>
> **Two traps in the response above, both the platform's own.** Its `hash_lock` example is
> **66 hex characters** while its own description says 64 — it is not a valid digest and
> must not be copied literally; and the `POST /api/v1/htlc/lock` example truncates its
> `hash_lock` to an ellipsis.
>
> Earlier Toolbox material published a hash lock that was **not the SHA-256 of anything**,
> was not even valid hex, and instructed implementers to verify it.
> `Toolbox/tools/verify_hashlocks.py` now recomputes every documented pair in CI so that
> cannot recur.

---

## Step 8 — The initiator settles, revealing the secret

```bash
curl -s -X POST "$PVP/api/v1/htlc/settle" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"contract_id":"htlc-b9c8d7e6","secret":"d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c"}' | jq .
```
```json
{ "htlc_tx_hash": "0x1234...", "zeto_tx_hash": "0x5678..." }
```

The contract checks `SHA-256(secret) == hash_lock`, then calls Zeto `transferLocked()` to
release the private tokens.

**This one call produces state on a gateway you never contacted.** Revealing the secret
publishes it; the Hyperledger Cacti relay broadcasts it to the other spoke, which settles
the mirror leg automatically. There is no API endpoint for that broadcast, and there are no
HTLC relay endpoints at all — the propagation is described only in prose.

---

## Step 9 — Observe the result

```bash
curl -s "$PVP/api/v1/htlc/status/htlc-a1b2c3d4" -H "$COOKIE" | jq .
```
```json
{
  "lock": {
    "contract_id": "htlc-a1b2c3d4",
    "sender": "funded_operator@spoke-a-bank-a",
    "receiver": "funded_operator@spoke-a-bank-c",
    "hash_lock": "0xabcdef1234567890...",
    "time_lock": 1711503600,
    "secret": "",
    "zeto_lock_ref": "zeto-utxo-8f21c0",
    "state": "HTLC_STATE_LOCKED"
  }
}
```

```bash
curl -s "$PVP/api/v1/htlc/search?agreement_id=trade-a1b2c3d4&state=LOCKED" -H "$COOKIE" | jq .
```

> **Two state vocabularies coexist, and the platform never reconciles them.** The `state`
> **field** of a lock record is prefixed —
> `HTLC_STATE_INVALID | HTLC_STATE_PENDING | HTLC_STATE_LOCKED | HTLC_STATE_SETTLED |
> HTLC_STATE_REFUNDED` — while the `state` **query parameter** of `/api/v1/htlc/search` is
> bare: `LOCKED | SETTLED | REFUNDED`. From the specification alone you cannot tell which
> to send to the filter. The Toolbox mirrors both verbatim rather than normalising one away.
> Verify against a live gateway before relying on it.

The assertion that actually matters is the balance:

```bash
curl -s "$PVP/api/v1/token/balance" -H "$COOKIE" | jq .
```

---

## The timeout branch

If the secret is never revealed, **each side reclaims its own leg independently** after its
own timelock expires:

```bash
curl -s -X POST "$PVP/api/v1/htlc/refund" \
  -H "$JSON" -H "$COOKIE" \
  -d '{"contract_id":"htlc-b9c8d7e6"}' | jq .
```
```json
{ "htlc_tx_hash": "0x1234...", "zeto_tx_hash": "0x5678..." }
```

Because the responder's timelock is the shorter one, the responder always regains the
ability to refund first. **That ordering is the safety property of the whole scheme.**

---

## Errors: assert on status codes, never on message text

The `/api/v1` error model is `{"error": "<free-form string>"}`. **There is no
machine-readable error code.** The same field carries human sentences
(`"invalid state transition"`) and code-like tokens (`INVALID_CERTIFICATE_CHAIN`,
`NONCE_SIGNATURE_MISMATCH`). An `ErrorCodeResponse {error, code}` schema is declared in
both platform specifications and referenced by **zero** operations.

Try the 401 branch — no cookie:

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$PVP/api/v1/htlc/settle" \
  -H "$JSON" -d '{"contract_id":"htlc-a1b2c3d4","secret":"d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c"}'
# → 401
```

And the invalid-state-transition branch, which is a **409 Conflict**:

```bash
curl -s -H 'Prefer: code=409' -H "$COOKIE" -H "$JSON" \
  -X POST "$PVP/api/v1/payments/fx/agreements/trade-a1b2c3d4/accept" | jq .
# → {"error":"invalid state transition"}
```

> **Why 409 and not 412.** The platform's own OpenAPI document declares `412 Precondition
> Failed` for this refusal. No payments or HTLC route in the delivered gateway can emit 412 —
> the orchestrator raises gRPC `FailedPrecondition` and `grpcErrorToHTTP` maps that to `409`.
> (The single genuine 412 in the whole Scenario A gateway belongs to `/pki-login`, for a
> missing participant certificate.) The Toolbox contract declares `409`, so Prism serves `409`
> here and a live gateway answers `409` too. See [`../../DIVERGENCES.md`](../../DIVERGENCES.md)
> row **A4**, `mocks/pvp/errors/02_invalid_state_transition.json`, and vectors
> `fx-err-07/08/09`.

| Situation | Status |
|---|---|
| No `access_token` cookie | `401` |
| HTLC mutation from a session without `ROLE_BANK` / `ROLE_COMMERCIAL_BANK` / `ROLE_TREASURY` / `ROLE_GOVERNANCE` | `403` |
| Accepting an agreement that is not `FX_STATE_PROPOSED`, or has expired | `409` — the document's `412` is unreachable |
| Any other invalid FX transition (`reject`, `cancel`, `settle`) | `409` |
| `lock-with-hash` with a body that parses but is rejected downstream | `500` — that handler never consults a certificate and never returns `412` |
| Approve/reject without `ROLE_TREASURY` | `403` |
| Route not registered on this gateway role | `404` — **not** a conformance failure |
| On-chain scan failed during `htlc/search` | `502` |

**A `404` may mean "wrong kind of node".** Route groups are wired conditionally per
deployment. A conformance run must declare a gateway profile and **skip**, not fail, when
the deployment cannot serve a route.

**There is no idempotency mechanism** — no `Idempotency-Key`, no `If-Match`/ETag, no
operation-level header parameters anywhere. Replay safety is per-endpoint and payload-keyed.
A generic retry wrapper is not part of this API.

---

## What the mock cannot show you

- **State.** Prism answers every request from the contract's examples. Creating an
  agreement does not make it retrievable, and the `contract_id` you get back is the same
  one every time.
- **Credential validation.** Presence of a cookie is checked; validity, roles and expiry
  are not.
- **The relay.** Step 8's cross-spoke secret propagation involves a second gateway that
  does not exist in your sandbox.
- **Role and deployment shape.** One process serves both the commercial-bank and the
  Central Bank halves of the reserve lifecycle.

---

## Next steps

- [Tutorial 2 — validate an implementation](02-validate-implementation.md) with the
  conformance suite
- [Tutorial 3 — Scenario B hub swap](03-hub-swap-mock.md), the other settlement mechanism
- [Settlement flow walkthrough](../devnet-guide/flow-walkthrough.md) — the same flow with
  more architectural context
- [The contract itself](../../contracts/pvp/openapi_pvp_v2.3.0.yaml) — 28 paths,
  32 operations, and a README documenting every divergence from the platform's own document
