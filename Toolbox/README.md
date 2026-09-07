# CBWeb3 Toolbox

The **CBWeb3 Toolbox** is a curated set of **integration-ready, reusable artifacts** that
help implementers and contributors build, test and validate interoperability and
privacy-aware flows in the CBWeb3 ecosystem.

This is **not** a full product implementation. It is a shared **integration kit**:
interface contracts, reference mocks, test vectors, executable conformance tests and
sandbox guidance that reduce ambiguity and accelerate integration across participants.

> ### What the Toolbox describes
>
> Every artifact here mirrors **CBWeb3 API Gateway v2.3.0 as delivered** by
> GoLedger/AguilaHub and in use by central banks and commercial banks on the pilot estate.
> The contracts are transcriptions of the delivered gateway specifications, not designs.
> Where the platform is ugly, the Toolbox is ugly in the same way; where the platform is
> silent, the Toolbox says so rather than guessing. Shapes recovered from delivered source
> rather than from the platform's own OpenAPI document are marked
> `x-cbweb3-source: implementation-observed`.

---

## Two scenarios. Read this before anything else.

CBWeb3 settles cross-border payments **two different ways**, and they share almost nothing
at the API level beyond authentication and the reserve lifecycle.

| | **Scenario A** — single-ledger, spoke-to-spoke | **Scenario B** — hub-and-spoke |
|---|---|---|
| Settlement mechanism | FX agreement + a pair of dual-layer HTLCs | Escrow → bridge lock-mint → Hub AMM swap → bridge-out → residue return |
| Contract | `contracts/pvp/` | `contracts/amm/` |
| HTLC endpoints | 6 | **none** |
| FX agreement endpoints | 7 | **none** |
| Atomicity comes from | one hash lock shared by two locks | orchestration across two Central Bank gateways |

Scenario B genuinely has **no HTLC surface and no FX agreement**. The `LockHTLC*` schemas
that appear in the Scenario B platform document are unreferenced copy-paste residue and are
deliberately not published here.

---

## What the Toolbox provides

### In scope
- **Interface contracts** — OpenAPI 3.0.3 specifications scoped per domain, with versioning,
  error models and request/response examples
- **Reference mocks** — canonical JSON request/response fixtures for each flow, enabling
  development with no live blockchain infrastructure
- **Test vectors** — deterministic fixtures (input → expected output) with preconditions,
  categories and validation guidance
- **Conformance tests** — executable checks that verify compliance with the contracts,
  runnable against a mock or against a real gateway
- **Sandbox / devnet guidance** — non-sensitive sample configurations and golden-path
  tutorials for both scenarios
- **Validation tooling** — CI gates that cross-check every artifact against the contracts

### Out of scope
- Production secrets, keys, real customer data
- **All `/internal/*` endpoints**, on both scenarios and all three prefixes. They are
  authenticated by a relay credential, are never client-callable, and are described in
  contract prose only
- Full business logic implementations or production deployments
- Anything that requires privileged access or sensitive runtime material

---

## Repository structure

```text
Toolbox/
│
├── README.md                       # This document — scope, principles, conventions
├── CONTRIBUTING.md                 # Contribution workflow, quality requirements
├── DIVERGENCES.md                  # Canonical register: where the platform document and the delivered gateway disagree
│
├── contracts/                      # Interface contracts (OpenAPI 3.0.3, one per domain)
│   ├── auth/                       # Shared session surface (Scenario A and B)
│   │   ├── openapi_auth_v2.3.0.yaml
│   │   ├── CHANGELOG.md
│   │   └── README.md
│   ├── pvp/                        # Scenario A: token, reserves, FX agreement, HTLC
│   │   ├── openapi_pvp_v2.3.0.yaml
│   │   ├── CHANGELOG.md
│   │   └── README.md
│   └── amm/                        # Scenario B: token, reserves, AMM, bridge, hub,
│       ├── openapi_amm_v2.3.0.yaml #             governance, oversight
│       ├── CHANGELOG.md
│       └── README.md
│
├── mocks/                          # Reference mocks (static JSON fixtures)
│   ├── auth/                       # Both login flows, session introspection
│   ├── pvp/                        # happy-path/ · timeout-refund/ · errors/
│   └── amm/                        # happy-path/ · provisioning/ · residue/ · errors/
│
├── test-vectors/                   # Deterministic test fixtures
│   ├── auth/
│   ├── pvp/                        # reserves · fx_agreement · htlc
│   └── amm/                        # reserves · registry · swap · bridge · governance
│
├── conformance/                    # Executable conformance suite
│   ├── pytest.ini
│   ├── README.md
│   ├── spec/                       # conformance_requirements.md · security_checklist.md
│   └── tests/{auth,pvp,amm}/
│
├── schemas/                        # Meta-schemas the fixtures are validated against
│   ├── mock.schema.json
│   └── vector.schema.json
│
├── tools/                          # Validation helpers, run as CI gates
│   ├── README.md
│   ├── validate_artifact_paths.py  # Every mock/vector path must exist in a contract
│   └── verify_hashlocks.py         # Every documented SHA-256 pair is recomputed
│
├── sandbox/                        # Sandbox / devnet guidance — NO secrets
│   ├── devnet-guide/
│   ├── sample-configs/
│   └── tutorials/                  # 01 Scenario A · 02 conformance · 03 Scenario B
│
└── docs/
    └── onboarding/
```

The same three names — **`auth`**, **`pvp`**, **`amm`** — are used in `contracts/`,
`mocks/`, `test-vectors/` and `conformance/tests/`. There is no second naming axis.

---

## Available artifacts

### 1. Interface contracts

Contracts are **OpenAPI 3.0.3 specifications** scoped to a single domain. Each lives in its
own folder under `contracts/` with a `CHANGELOG.md` and a `README.md`.

| File | Purpose |
|------|---------|
| `openapi_<domain>_v<gateway-version>.yaml` | The specification: paths, schemas, error model, examples |
| `CHANGELOG.md` | Version history following [Keep a Changelog](https://keepachangelog.com/) |
| `README.md` | Scenario framing, operation index, divergences from the platform document, open questions |

**Currently available:**

| Domain | Folder | Scenario | Paths | Operations | Public ops | Covers |
|---|---|---|---|---|---|---|
| **Authentication** | `contracts/auth/` | shared (A **and** B) | 8 | 8 | 4 | `/healthz`, direct login, PKI nonce + wallet bind, refresh, logout, session introspection, client-secret rotation |
| **PvP settlement** | `contracts/pvp/` | A | 28 | 32 | 0 | Token surface, reserve lifecycle, FX agreement lifecycle, dual-layer HTLC |
| **Hub AMM** | `contracts/amm/` | B | 52 | 59 | 12 | Token surface, reserve lifecycle, AMM quote/swap, bridge lock-mint/burn-unlock, liquidity, hub registries, v2 governance and oversight |
| **Total** | | | **88** | **99** | **16** | |

`contracts/auth/` is published **once** because the whole authentication block is
byte-identical between the two delivered gateway specifications. Publishing it once removes
the only drift risk on the thing every integration starts with.

The **reserve lifecycle is deliberately duplicated** across `pvp/` and `amm/`. The two
gateways genuinely differ — Scenario A serves `POST /api/v1/payments/deposits/fiat-exchange`
and Scenario B serves `POST /api/v1/payments/deposits/exchange`, and neither serves both. A
single shared document would have to declare both paths and would therefore describe a
gateway that does not exist.

#### Authentication, in one paragraph

The whole `/api/v1` surface is authenticated by an **`access_token` HttpOnly cookie**
(`CookieAuth`: `apiKey`, `in: cookie`, `SameSite=Strict`). There is **no bearer token on
`/api/v1`**. `BearerAuth` is declared in `contracts/amm/` only, as an alternative on the
twelve `/api/v2` `RequireAnyAuth` routes; the two cross-currency swap endpoints are
cookie-only. **There is no CSRF mechanism anywhere on the platform** — no header, no
double-submit cookie, no Origin check. `SameSite=Strict` is the sole mitigation, and that
absence is recorded as a finding in `conformance/spec/security_checklist.md` rather than
papered over.

#### How to use a contract

**Validate the spec:**
```bash
# Using Spectral (recommended) — this is what CI runs, on all three
npx @stoplight/spectral-cli lint contracts/auth/openapi_auth_v2.3.0.yaml
npx @stoplight/spectral-cli lint contracts/pvp/openapi_pvp_v2.3.0.yaml
npx @stoplight/spectral-cli lint contracts/amm/openapi_amm_v2.3.0.yaml

# Using swagger-cli
npx @apidevtools/swagger-cli validate contracts/pvp/openapi_pvp_v2.3.0.yaml
```

**Run a mock server** — one Prism instance per contract:
```bash
npx @stoplight/prism-cli mock contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010
npx @stoplight/prism-cli mock contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011
npx @stoplight/prism-cli mock contracts/auth/openapi_auth_v2.3.0.yaml --port 4012
```

> **Paths are complete.** Contract paths carry their own `/api/v1` or `/api/v2` prefix and
> the `servers:` entries are bare origins, so Prism serves exactly the path a real gateway
> serves. Never append `/api/v1` to `CBWEB3_BASE_URL`.

**Generate client SDKs:**
```bash
npx @openapitools/openapi-generator-cli generate \
  -i contracts/pvp/openapi_pvp_v2.3.0.yaml \
  -g python \
  -o ./generated/python-pvp-client
```

---

### 2. Reference mocks

Mocks are **static JSON fixtures** representing canonical request/response pairs. They let
an integrator understand expected behaviour without running any infrastructure.

Each mock file has this shape:

```json
{
  "id": "mock-pvp-hp-01",
  "title": "Register a deposit (reserve prelude, step 1)",
  "description": "Human-readable explanation of this step",
  "request": {
    "method": "POST",
    "path": "/api/v1/payments/deposits",
    "headers": {
      "Content-Type": "application/json",
      "Cookie": "access_token=SYNTHETIC_COOKIE_CI"
    },
    "body": { "amount": "10000000" }
  },
  "response": {
    "status": 201,
    "headers": { "Content-Type": "application/json" },
    "body": { "deposit_id": "dep-a1b2c3d4", "status": "PENDING" }
  }
}
```

The session credential travels in `request.headers` as a `Cookie:` header, because the mock
meta-schema declares `request` with `additionalProperties: false` and has no `cookies` key.

**Currently available:**

| Domain | Folder | Files | Contents |
|---|---|---|---|
| **Authentication** | `mocks/auth/` | 5 | Direct login, PKI nonce, wallet bind, session introspection, 401 without a cookie |
| **PvP (Scenario A)** | `mocks/pvp/happy-path/` | 17 | Reserve prelude → FX agreement → both HTLC legs → settle → observe |
| | `mocks/pvp/timeout-refund/` | 3 | Refund too early, status after expiry, refund after expiry |
| | `mocks/pvp/errors/` | 3 | Missing cookie, invalid state transition, forbidden oversight session |
| **AMM (Scenario B)** | `mocks/amm/happy-path/` | 18 | Discovery → reserve prelude → quote → swap → verification |
| | `mocks/amm/provisioning/` | 8 | Currency registration, pair propose/confirm, lock-mint, mint-and-approve, liquidity commits |
| | `mocks/amm/residue/` | 2 | Residue returned, and `RETURN_FAILED` |
| | `mocks/amm/errors/` | 3 | Transfer limit, pool not active, and the unauthenticated case |
| **Total** | | **59** | |

#### The Scenario A happy path

```
Step  Call                                                    Actor            Purpose
────  ──────────────────────────────────────────────────────  ───────────────  ───────────────────────────
  1   POST /api/v1/payments/deposits                          Commercial bank  Register fiat deposit
  2   POST /api/v1/payments/deposits/approve                  Central Bank     Approve it (ROLE_TREASURY)
  3   POST /api/v1/payments/deposits/fiat-exchange            Central Bank     Mint fCeBM on Besu
  4   POST /api/v1/payments/escrows                           Commercial bank  Request tokenisation
  5   POST /api/v1/payments/escrows/approve                   Central Bank     Burn fCeBM, mint tCeBM
  6   GET  /api/v1/token/balance                              Commercial bank  Confirm non-zero balance
  7   POST /api/v1/payments/fx/agreements                     Originator       Propose terms → PROPOSED
  8   POST /api/v1/payments/fx/agreements/{tradeId}/accept    Counterparty     Accept → ACCEPTED
  9   POST /api/v1/htlc/lock                                  Initiator        Lock; gateway derives hash_lock
 10   (hash lock crosses OUT OF BAND — no API endpoint)       —                —
 11   POST /api/v1/htlc/lock-with-hash                        Responder        Lock with the received hash
 12   POST /api/v1/htlc/settle                                Initiator        Reveal the secret
 13   GET  /api/v1/htlc/status/{contractId}                   Either           Observe the result
```

Steps 1–6 are the **reserve prelude** and are not optional: a bank cannot HTLC-lock tCeBM it
does not hold.

> ### ⚠ The timelock ordering rule is safety-critical
>
> The **initiator's** timelock (step 9, `now + 3600`) **must exceed** the **responder's**
> (step 11, `now + 1800`). If it does not, the initiator can let the responder's lock
> expire, refund nothing, and still claim the responder's leg with the secret — while the
> responder has lost its refund window. **Nothing in any schema enforces this**; the
> platform states it in prose only. Any mock, vector or tutorial that inverts the ordering
> models an unsafe swap and must be rejected in review.

#### The Scenario B happy path

```
Step  Call                                                    Auth       Purpose
────  ──────────────────────────────────────────────────────  ─────────  ───────────────────────────────
  1   GET  /api/v2/amm/hub-config                             public     Hub contract addresses
  2   GET  /api/v2/hub/currencies                             public     Symbol → W-token address
  3   GET  /api/v2/amm/pairs                                  public     The ONLY source of a pool_pair
  4   GET  /api/v2/amm/pool/{pair}/status                     public     Must be ACTIVE
  5   GET  /api/v2/governance/circuit-breaker/status?pair=…   public     Must not be halted
  6   (reserve prelude — same six calls as Scenario A,        cookie     Mandatory; the bridge-in
       with deposits/exchange, not deposits/fiat-exchange)               handler enforces it
  7   GET  /api/v2/amm/quote/cross-currency                   public     15-second TTL
  8   POST /api/v2/amm/swap/cross-currency                    cookie     Cookie-only; returns swap_id
  9   GET  /api/v2/amm/swap/cross-currency/{id}               cookie     Poll to COMPLETED
 10   GET  /api/v2/amm/hub-reconciliation                     cookie|bearer  balanced == true
```

**Step 7 and step 8 must be issued programmatically in one step.** The quote lives 15
seconds; a walkthrough that pauses between them expires.

**`status == COMPLETED` is not a settlement assertion.** Verify that the payer was debited
`amount_in` and not `max_amount_in`, that `residue_status` is `RETURNED` (`RETURN_FAILED`
means the payer has **not** been made whole), that the bridge-out position reached
`BURNED`/`RELEASED`, and that hub reconciliation reports `balanced == true`.

#### How to use mocks

- **As a reference** — read the files in numeric order to follow a complete flow, including
  exact request bodies and expected responses.
- **Programmatically** — parse them in your test harness to feed requests and assert
  responses.
- **With Prism** — Prism serves the examples embedded in the OpenAPI document; the JSON
  fixtures add the scenarios a single example cannot express (errors, residue, timeout).

---

### 3. Test vectors

Test vectors are **deterministic fixtures** for validating that an implementation conforms
to a contract. Where mocks show what the API *looks like*, vectors define what **MUST
happen** under specific conditions.

```json
{
  "id": "htlc-err-01",
  "title": "Settle an HTLC with a secret that does not match the hash lock",
  "description": "The gateway must refuse to release the locked tokens.",
  "category": "error",
  "preconditions": {
    "authenticatedAs": "funded_operator@spoke-a-bank-a",
    "htlcExists": true,
    "htlcState": "HTLC_STATE_LOCKED",
    "timeLockNotExpired": true
  },
  "input": {
    "method": "POST",
    "path": "/api/v1/htlc/settle",
    "body": { "contract_id": "{{VALID_CONTRACT_ID}}", "secret": "not-the-secret" }
  },
  "expectedOutput": { "status": 500 },
  "notes": "Assert on the STATUS CODE only. The /api/v1 error model is {\"error\": \"<free-form string>\"} with no machine-readable code."
}
```

**Field reference:**

| Field | Description |
|-------|-------------|
| `id` | Unique identifier, `<domain>-<category>-<number>` |
| `title` | Short human-readable name |
| `description` | What is being tested and why |
| `category` | `happy-path`, `error` or `edge-case` |
| `preconditions` | State that MUST exist before the test runs |
| `input` | HTTP request to send |
| `expectedOutput` | Expected HTTP response |
| `notes` | Implementation guidance, provenance, or an upstream defect being mirrored |

**Placeholder conventions:**
- `{{NON_EMPTY_STRING}}` — assert a non-empty string; the exact value is implementation-specific
- `{{POSITIVE_INTEGER}}` — assert a positive integer
- `{{SAME_AS_INPUT}}` — assert the field echoes the corresponding input value
- `{{VALID_TRADE_ID}}` / `{{VALID_CONTRACT_ID}}` — a real id produced by an earlier step

> ### Never assert on error text
>
> The `/api/v1` error model is `{"error": "<free-form string>"}` and there is **no
> machine-readable error code**. The same field carries human sentences
> (`"invalid state transition"`) and code-like tokens (`INVALID_CERTIFICATE_CHAIN`). An
> `ErrorCodeResponse {error, code}` schema is declared in both platform documents and
> referenced by **zero** operations. `/api/v2` handlers do emit a code, under three
> different spellings (`code`, `error_code`, and an undocumented `required_role` array).
>
> Assert on **status codes**. Do not resurrect `HTLC_HASH_MISMATCH` or `HTLC_EXPIRED` —
> those were invented by earlier Toolbox material and no CBWeb3 gateway has ever emitted
> them.

**Currently available:**

| File | Domain | Vectors | Categories |
|------|--------|---------|------------|
| `auth/auth_vectors.json` | Authentication | 10 | 6 happy-path, 4 error |
| `pvp/pvp_reserves_vectors.json` | Scenario A reserves | 13 | 7 happy-path, 5 error, 1 edge-case |
| `pvp/pvp_fx_agreement_vectors.json` | Scenario A FX agreement | 19 | 7 happy-path, 11 error, 1 edge-case |
| `pvp/pvp_htlc_vectors.json` | Scenario A HTLC | 22 | 7 happy-path, 11 error, 4 edge-case |
| `amm/amm_reserves_vectors.json` | Scenario B reserves | 11 | 7 happy-path, 3 error, 1 edge-case |
| `amm/amm_registry_vectors.json` | Scenario B discovery | 16 | 8 happy-path, 6 error, 2 edge-case |
| `amm/amm_swap_vectors.json` | Scenario B quote and swap | 17 | 6 happy-path, 9 error, 2 edge-case |
| `amm/amm_bridge_vectors.json` | Scenario B bridge | 15 | 8 happy-path, 6 error, 1 edge-case |
| `amm/amm_governance_vectors.json` | Scenario B governance and oversight | 17 | 10 happy-path, 6 error, 1 edge-case |
| **Total** | | **140** | **66 happy-path, 61 error, 13 edge-case** |

#### How to validate an implementation

1. Deploy your implementation of the relevant contract.
2. For each vector file, iterate through the `vectors` array: set up the `preconditions`,
   send the `input`, assert the response matches `expectedOutput`.
3. All vectors applicable to your **gateway profile** must pass. A route your deployment
   never registered returns `404`, which is a *skip*, not a failure — route groups are wired
   conditionally per gateway role.

Or simply run the executable suite in `conformance/`, which does all of the above.

#### Cryptographic reference values

In a real Scenario A swap the **gateway generates the secret**: `POST /api/v1/htlc/lock`
returns the derived `hash_lock` and the client never chooses it. The pair below exists so
that `lock-with-hash` fixtures and local SHA-256 implementations can be checked against a
value anyone can recompute in one line:

**The secret is 32 bytes carried as 64 lowercase hex characters, and the gateway
hex-decodes it before hashing:** `hash_lock = SHA-256(hex_decode(secret))`. It does **not**
hash the ASCII of the string you send, so a human-readable passphrase is not a valid
secret — anything that is not 64 hex characters is rejected with
`400 "hash_lock must be a 64-char hex string (32 bytes)"`.

| Field | Value |
|---|---|
| Secret (hex) | `d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c` |
| Hash function | SHA-256 over the **decoded bytes** |
| Hash lock | `93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2` |

```bash
printf 'd2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c' | xxd -r -p | sha256sum
# 93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2

# or, without xxd:
python3 -c "import hashlib;print(hashlib.sha256(bytes.fromhex('d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c')).hexdigest())"
```

This pair is **chosen by the Toolbox**, not taken from the platform, and is published
precisely because it is independently verifiable. It is the pair the `pvp` mock fixtures
ship; the `pvp` test vectors carry their own, derived the same way. `tools/verify_hashlocks.py`
recomputes every documented pair in CI.

> **Why this section exists.** Every Toolbox artifact before the 2026-08 realignment
> published a hash lock that was **not the SHA-256 of anything**, and instructed
> implementers to verify it. Any integrator who followed that instruction got a mismatch.
> The CI gate makes a recurrence impossible.

---

### 4. Conformance tests

`conformance/` holds an executable pytest suite that runs against a Prism mock **or** a real
gateway. Two orthogonal marker tiers make that possible:

| Tier | Marker | Meaning |
|---|---|---|
| Execution | `mock_safe` | Single request; asserts status and shape. Runs against Prism and a live gateway |
| | `live_only` | State transitions, balance deltas, residue reconciliation. Skipped in mock mode — Prism is stateless |
| Category | `happy_path` / `error` / `edge_case` | What kind of case it is |

Plus domain (`auth`, `pvp`, `amm`), sub-domain (`reserves`, `fx_agreement`, `htlc`,
`registry`, `amm_swap`, `bridge`), scenario (`scenario_a`, `scenario_b`) and `bearer_ok`.

Four authentication modes are selected with `CBWEB3_AUTH_MODE`: `mock` (a synthetic cookie
injected into the jar, for CI), `direct` (client-id/secret login), `pki` (the two-step
nonce + wallet-bind dance a commercial bank actually performs) and `bearer` (the twelve
Scenario B `/api/v2` routes only). `CBWEB3_AUTH_TOKEN` **no longer exists anywhere in this
repository.**

**Currently available:**

| Module | Domain | Test methods |
|---|---|---|
| `tests/auth/test_auth_session.py` | Authentication | 10 |
| `tests/pvp/test_pvp_reserves.py` | Scenario A reserves | 14 |
| `tests/pvp/test_pvp_fx_agreement.py` | Scenario A FX agreement | 14 |
| `tests/pvp/test_pvp_htlc.py` | Scenario A HTLC | 12 |
| `tests/amm/test_amm_reserves.py` | Scenario B reserves | 10 |
| `tests/amm/test_amm_registry.py` | Scenario B discovery | 11 |
| `tests/amm/test_amm_swap.py` | Scenario B quote and swap | 12 |
| `tests/amm/test_amm_bridge.py` | Scenario B bridge and reconciliation | 8 |
| **Total** | | **91** |

Counts above are derived from the shipped files, not transcribed. Re-derive them before
changing them — the pre-realignment repository stated "16 test methods" in four places when
the true count was 17.

See [conformance/README.md](conformance/README.md) and
[sandbox/tutorials/02-validate-implementation.md](sandbox/tutorials/02-validate-implementation.md).

---

## Synthetic data policy

All artifacts use **synthetic data** exclusively.

Identities on the platform are Paladin `name@node` strings — not wallet addresses, not bank
codes.

| Role | Identity |
|---|---|
| Initiator / originator | `funded_operator@spoke-a-bank-a` |
| Settlement agent | `funded_operator@spoke-a-bank-c` |
| Responder / counterparty | `funded_operator@spoke-b-bank-d` |
| Beneficiary | `funded_operator@spoke-b-bank-b` |

- **Reference corridor:** 1,000,000 **BRL** → 850,000 **ARS** at rate **0.85**
- **Session credentials:** a synthetic `access_token` cookie value. There are **no bearer
  JWTs in `/api/v1` fixtures**, because the platform does not use them there
- **Transaction hashes, block numbers, contract and trade ids:** fabricated, and taken from
  the contracts' own examples wherever the platform publishes one
- **No real financial data, credentials, endpoints, key material or participant information
  is included**

> **Amounts are decimal strings, never JSON numbers — and the scale is not stated.** No
> decimals count is published anywhere for tCeBM or fCeBM. `/api/v1` payment amounts are
> 7-digit strings with no unit statement; `/api/v2` amounts are explicitly 18-decimal base
> units. **Moving a figure from a deposit into a swap is not safe** without a conversion you
> have derived yourself. Treat balances as opaque big integers and compare deltas. A
> compounding trap: a transfer limit's `max_amount` is a human decimal on input
> (`"100000.00"`) and wei on output (`"100000000000000000000000"`) — under the same field
> name.

---

## Known limitations and open questions

These are properties of the delivered platform. They are recorded here, and in the README of
the contract each belongs to, **because they are not resolvable by guesswork** — and a
Digital Public Goods submission that pointed at this kit should see them stated, not hidden.

| Area | Open question |
|---|---|
| FX lifecycle | There is **no `EXPIRED` state**, despite an `expiry_date` on every agreement and a documented background expiration worker |
| Error model | `/api/v1` has **no machine-readable error code**; `/api/v2` has three different spellings of one |
| Idempotency | **No mechanism exists** — no `Idempotency-Key`, no `If-Match`/ETag, no operation-level headers. Replay safety is per-endpoint and payload-keyed |
| Decimals | **No decimals count is stated** for tCeBM or fCeBM anywhere |
| HTLC state | **Two vocabularies coexist** and are never reconciled: the record field is prefixed (`HTLC_STATE_LOCKED`), the search query parameter is bare (`LOCKED`) |
| Pool pairs | **Five incompatible spellings**, none marked canonical (see [`DIVERGENCES.md`](DIVERGENCES.md)). Treat `pair_id` as opaque and read it from `GET /api/v2/amm/pairs` |
| Relay surface | Scenario A documents idempotent relay delivery of FX state changes, but the only registered internal FX route is a `GET`. HTLC has **no relay endpoints at all**; cross-spoke secret propagation is prose only |
| Undocumented routes | Five Scenario A routes are wired by the router and appear in no specification. They are recorded as a gap; nothing is modelled on them |
| Unspecified `/api/v2` | Fourteen endpoints declare free-form `additionalProperties` objects. The Toolbox fills them from handler source under `x-cbweb3-source: implementation-observed` — a conscious, annotated deviation |
| Spec-vs-implementation | `quote/exact-output` documents `amountOut` while the handler reads `amount_out`; `quote/cross-currency` documents zero parameters while requiring three; three responses documented as bare arrays actually return wrapped objects. The Toolbox mirrors the implementation and footnotes each case |
| Deployment discovery | `FX_RATE_TOLERANCE_PCT`, `FX_AGREEMENT_HTLC_STRICT` and Pente on/off change server behaviour and are exposed by **no endpoint** |
| Security | The platform implements **no CSRF mechanism**. `SameSite=Strict` is the sole mitigation, recorded as a finding in the security checklist |

---

## Design principles (quality bar)

- **Faithful to what is delivered** — accuracy against the real gateway outranks elegance.
  Never invent an endpoint, field, status code, enum value or schema that is not in the
  platform surface. If something is genuinely absent, say so
- **Reproducible** — artifacts must be verifiable locally and in CI
- **Non-sensitive by default** — samples must be safe to publish
- **Versioned** — a contract's version tracks the **gateway version it describes**, and its
  filename carries that version
- **Security-by-default** — OWASP-minded design; authn/z boundaries documented, not assumed
- **Operational clarity** — clear prerequisites, assumptions and error semantics

---

## Artifact conventions

### Contracts
- MUST include: **version**, **breaking change notes** (`CHANGELOG.md`), **error model** and
  **examples**
- MUST use stable, language-neutral formats (OpenAPI 3.0.3 / JSON Schema)
- MUST be scoped to a single domain (`auth/`, `pvp/`, `amm/`)
- MUST be **self-contained** — no cross-file `$ref`, so each is independently lintable and
  independently Prism-mockable
- MUST mark any shape not derived from the platform's own OpenAPI document with
  `x-cbweb3-source: implementation-observed`
- SHOULD be validatable with standard tools (Spectral, swagger-cli)

### Reference mocks
- MUST be **canonical**: deterministic responses aligned with the contract
- MUST include `id`, `title`, `description`, `request`, `response`
- MUST carry the session credential as a `Cookie:` request header, never `Authorization`
- MUST use only synthetic data
- MUST have an `id` matching `^mock-[a-z]+-[a-z]+-\d+$` — three lowercase segments
- SHOULD be organised by scenario (`happy-path/`, `timeout-refund/`, `residue/`, `errors/`)

### Test vectors
- MUST be deterministic and minimal
- MUST include `id`, `title`, `description`, `category`, `preconditions`, `input`,
  `expectedOutput`
- MUST cover happy paths, error cases and edge cases
- MUST assert on status codes rather than error strings
- SHOULD use the placeholder conventions above for implementation-specific values

### Conformance tests
- MUST state what "pass" means and what is explicitly out of scope
- MUST carry one execution-tier marker (`mock_safe` or `live_only`) and one category marker
- MUST use the shared `requests.Session` so the cookie jar persists
- SHOULD be runnable in CI and locally

### Every mock and vector path is checked
`tools/validate_artifact_paths.py` resolves every fixture's `path` **and method** against
the three contracts, and CI fails if one does not exist. That gate did not exist before the
2026-08 realignment, which is exactly how seven fictional endpoints shipped green for four
sessions.

A vector writes the contract's templated path verbatim and puts the concrete values in
`input.pathParams`, so the gate expands the one into the other before resolving. A
`{{RUNTIME}}` value stands for a single opaque segment; a literal is percent-encoded, which
is what keeps a slash-bearing `pair` identifier inside the one segment its template allows.
An unanswered placeholder — or a parameter answering none — is reported as the copy-paste
it is, not as a missing endpoint.

---

## How work is tracked

All work is tracked using the **GitHub Project** in this repository, with issues labeled by:

- `area:contracts`, `area:mocks`, `area:test-vectors`, `area:conformance`, `area:sandbox`,
  `area:docs`
- `prio:P0`, `prio:P1`, `prio:P2`
- `needs-owner` (when maintainers are seeking a contributor to lead the work)

---

## Roadmap

| Session | Topic | Status |
|---------|-------|--------|
| 1 | Toolbox charter, structure, contribution guide | ✅ Complete |
| 2 | Interface contracts + reference mocks + test vectors | ✅ Complete |
| 3 | Conformance tests + CI gates (security + compatibility) | ✅ Complete |
| 4 | Sandbox/devnet guidance + sample configs + onboarding | ✅ Complete |
| 5 | **Realignment to API Gateway v2.3.0** — three contracts, both scenarios, cookie auth, artifact-path and hash-lock CI gates | ✅ Complete (2026-08) |
| — | Monorepo import of the platform under `platform/` | Next |
| — | `contracts/compliance/` — the Scenario A compliance / governance / supervisor / oversight / onboarding surface (~42 paths) | **Deferred, not cancelled.** It serves supervisors and regulators rather than the settlement integrator this kit targets |

---

## Getting started

1. Read this charter for scope and conventions
2. **Decide which scenario you are integrating with** — A (`contracts/pvp/`) or B
   (`contracts/amm/`). They share only `contracts/auth/` and the reserve lifecycle
3. Start with `contracts/auth/` — every integration begins with a session
4. Read your scenario's contract `README.md` before the YAML: it lists the divergences from
   the platform's own document and the open questions
5. Run the tutorial: [sandbox/tutorials/01](sandbox/tutorials/01-pvp-settlement-mock.md)
   (Scenario A) or [03](sandbox/tutorials/03-hub-swap-mock.md) (Scenario B)
6. Validate with [sandbox/tutorials/02](sandbox/tutorials/02-validate-implementation.md)
7. Pick an issue from the Project board, ideally tagged `good-first-issue` or `needs-owner`
8. Follow [CONTRIBUTING.md](CONTRIBUTING.md)

---

## License

Unless explicitly stated otherwise, the Toolbox follows the repository's license
(Apache-2.0). Contributions must be compatible with that license.

---

## Security notes

- Apply OWASP-style thinking (input validation, least privilege, secure defaults, safe
  logging).
- If you discover a security vulnerability, **do not open a public issue**. Follow the
  repository's security reporting process (see `SECURITY.md` if present) or contact
  maintainers privately.
