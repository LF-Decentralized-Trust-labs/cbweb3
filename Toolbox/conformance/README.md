# Conformance Tests — CBWeb3 Toolbox

Executable tests that check an implementation against the three published Toolbox
contracts for **CBWeb3 API Gateway v2.3.0**:

| Contract | Scenario | Paths / operations |
|---|---|---|
| `contracts/auth/openapi_auth_v2.3.0.yaml` | shared (A and B) | 8 / 8 |
| `contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — single-ledger, spoke-to-spoke | 28 / 32 |
| `contracts/amm/openapi_amm_v2.3.0.yaml` | B — hub-and-spoke, International Hub | 52 / 59 |

**91 test methods.** Every assertion is traceable to one of those three files. Where
the platform documents no behaviour, this suite asserts none — the gap is named in
the test's docstring instead.

---

## Authentication — the thing most likely to trip you up

The gateway does **not** use `Authorization: Bearer` on `/api/v1`. It uses an
**HttpOnly cookie named `access_token`**, obtained through a login flow:

```
CookieAuth:  type: apiKey, in: cookie, name: access_token
             HttpOnly, SameSite=Strict, Path=/
             Secure follows the gateway's COOKIE_SECURE env var
```

* `BearerAuth` exists **only** in the Scenario B contract, and only as an
  *alternative* to the cookie on the 12 `/api/v2` operations guarded by
  `RequireAnyAuth`. **The two cross-currency swap endpoints are cookie-only.**
* **There is no CSRF mechanism.** No `X-CSRF-Token`, no double-submit cookie, no
  synchroniser token, no Origin/Referer check — an exhaustive grep of both platform
  specs and the gateway source finds nothing. `SameSite=Strict` is the sole
  mitigation. This suite sends no CSRF header, and the absence is recorded as a
  finding in [`spec/security_checklist.md`](spec/security_checklist.md).
* `CBWEB3_AUTH_TOKEN` no longer exists anywhere in this repository.

One endpoint, `POST /api/v1/auth/login`, behaves two completely different ways and
**the server decides, not the client**:

| Flow | Roles | Response |
|---|---|---|
| **direct** | `ROLE_GOVERNANCE`, `ROLE_SUPERVISOR`, `ROLE_NOC` | 200 `AuthResponse` **plus** `Set-Cookie` |
| **PKI nonce** | `ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY` | 200 `{"nonce": "<hex>"}` — **no tokens, no cookies** |

In the PKI flow the client signs the nonce with the private key matching its
CB-issued participant certificate and posts
`/api/v1/auth/wallet/bind {user_id, nonce_signature_hex, cert_pem}`, which sets the
cookies. **A fixture that stops after `/auth/login` never holds a session** — and
commercial banks, the actors an external integrator is most likely building for,
always perform the two-step dance.

---

## Quick start

```bash
pip install pytest requests          # cryptography is needed for pki mode only
cd Toolbox/conformance
```

### Against a Prism mock (what CI runs)

Prism serves no login flow — it is stateless and returns static examples — so mock
mode **performs no login at all** and injects a synthetic cookie straight into the
jar. That works because Prism validates only the *presence* of an apiKey-in-cookie
credential, never its value, while a request *without* the cookie still yields 401,
which is a useful negative assertion.

```bash
# three instances, one per contract
npx @stoplight/prism-cli mock ../contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock ../contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock ../contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &

pytest -m mock_safe                  # 53 tests, no configuration needed
```

`CBWEB3_AUTH_MODE` defaults to `mock` and `CBWEB3_BASE_URL` to
`http://localhost:4010`; the 4011/4012 fan-out is automatic. Override any of the
three with `CBWEB3_PVP_BASE_URL` / `CBWEB3_AMM_BASE_URL` / `CBWEB3_AUTH_BASE_URL`.

### Against a real gateway — direct login

```bash
export CBWEB3_BASE_URL=https://gateway.example.org     # bare origin, no /api/v1
export CBWEB3_AUTH_MODE=direct
export CBWEB3_PROFILE=scenario-a-cb
export CBWEB3_CLIENT_ID=cb-a
export CBWEB3_CLIENT_SECRET=…
pytest -m scenario_a
```

### Against a real gateway — PKI login (what a commercial bank does)

```bash
export CBWEB3_BASE_URL=https://bank-a.gateway.example.org
export CBWEB3_AUTH_MODE=pki
export CBWEB3_PROFILE=scenario-a-bank
export CBWEB3_CLIENT_ID=bank-a
export CBWEB3_CLIENT_SECRET=…          # issued once by onboarding Phase 3
export CBWEB3_USER_ID=f47ac10b-58cc-4372-a567-0e02b2c3d479
export CBWEB3_KEY_PEM=/path/participant-key.pem
export CBWEB3_CERT_PEM=/path/participant-cert.pem
pytest -m scenario_a
```

### Against Scenario B `/api/v2` with a bearer token

```bash
export CBWEB3_AUTH_MODE=bearer
export CBWEB3_BEARER_TOKEN=eyJ…
pytest -m bearer_ok                    # every other test is skipped: no cookie
```

---

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `CBWEB3_BASE_URL` | `http://localhost:4010` | Bare origin. **No `/api/v1` suffix** — tests use full contract paths. |
| `CBWEB3_AUTH_MODE` | `mock` | `mock` \| `direct` \| `pki` \| `bearer` |
| `CBWEB3_PROFILE` | `mock` | `mock` \| `scenario-a-bank` \| `scenario-a-cb` \| `scenario-b-bank` \| `scenario-b-cb` |
| `CBWEB3_CLIENT_ID`, `CBWEB3_CLIENT_SECRET` | — | `direct` and `pki` |
| `CBWEB3_USER_ID`, `CBWEB3_KEY_PEM`, `CBWEB3_CERT_PEM` | — | `pki` only. The PEM variables accept a path or the PEM text itself. |
| `CBWEB3_NONCE_ENCODING` | `utf8` | `utf8` \| `hex` — see "Known ambiguities" below. |
| `CBWEB3_BEARER_TOKEN` | — | `bearer` only |
| `CBWEB3_AUTH_BASE_URL`, `CBWEB3_PVP_BASE_URL`, `CBWEB3_AMM_BASE_URL` | `CBWEB3_BASE_URL` | Per-domain overrides (used by the three-instance Prism setup). |
| `CBWEB3_TIMEOUT` | `15` | Per-request timeout, seconds. |
| `CBWEB3_REFUND_WAIT_SECONDS` | `20` | Real-time wait in the HTLC timeout branch. |
| `CBWEB3_SOURCE_CURRENCY`, `CBWEB3_TARGET_CURRENCY`, `CBWEB3_BENEFICIARY_BANK_ID` | — | Required by the live Scenario B settlement tests; they skip without them. |
| `CBWEB3_SWAP_AMOUNT_OUT`, `CBWEB3_SWAP_MAX_AMOUNT_IN`, `CBWEB3_SWAP_POLL_SECONDS` | contract examples / `120` | Live swap tuning. |

CLI equivalents: `--base-url`, `--auth-mode`, `--profile`.

---

## Markers

Three orthogonal axes. Combine them freely: `pytest -m "mock_safe and scenario_b"`.

### Execution tier — the one that matters most

| Marker | Count | Selects |
|---|---:|---|
| `mock_safe` | 53 | Single-request shape/status assertions. Pass against **both** Prism and a live gateway. |
| `live_only` | 37 | Multi-step state transitions, settlement verification, balance deltas, residue reconciliation. **Auto-skipped** when `CBWEB3_AUTH_MODE=mock`, because Prism cannot persist state. |
| `bearer_ok` | 5 | Operations where `BearerAuth` is a declared alternative to the cookie. The only tests that run under `CBWEB3_AUTH_MODE=bearer`. |

### Category

| Marker | Count |
|---|---:|
| `happy_path` | 59 |
| `error` | 27 |
| `edge_case` | 5 |

### Domain and scenario

| Marker | Count | Module(s) |
|---|---:|---|
| `auth` | 10 | `tests/auth/test_auth_session.py` |
| `pvp` | 40 | the three `tests/pvp/` modules |
| `amm` | 41 | the four `tests/amm/` modules |
| `reserves` | 24 | `test_pvp_reserves.py` (14) + `test_amm_reserves.py` (10) |
| `fx_agreement` | 14 | `test_pvp_fx_agreement.py` |
| `htlc` | 12 | `test_pvp_htlc.py` |
| `registry` | 11 | `test_amm_registry.py` |
| `amm_swap` | 12 | `test_amm_swap.py` |
| `bridge` | 8 | `test_amm_bridge.py` |
| `scenario_a` | 40 | Skipped against a `scenario-b-*` profile. |
| `scenario_b` | 41 | Skipped against a `scenario-a-*` profile. |

The 10 `auth` tests carry no scenario marker: the authentication surface is
byte-identical between the two platform specs, so they run against either gateway.

`--strict-markers` is on. An unregistered marker is an error, not a warning.

---

## Why a 404, 501, 502 or 503 is not a failure

Route groups are **registered conditionally per deployment**. Scenario A wires
HTLC/Token/FX only when the payment orchestrator is configured; the escrow
*approve* half only on Central Bank gateways (`PaymentProxyHandler == nil`) and the
*create* half only on commercial-bank gateways; Scenario B registers each `/api/v2`
group only when its backing service is injected.

So an endpoint's absence yields **404, and no role will fix it**. Every test that
touches a conditionally-wired route declares the profile(s) it needs and **skips**
rather than fails. Likewise `501` (feature not configured), `502` (upstream
dependency unreachable) and `503` (capability disabled) are first-class deployment
outcomes and are treated as SKIP.

Without this the suite would be unusable against any real estate.

---

## What this suite verifies

* **Status codes**, exactly, from the contract's declared response set.
* **Field names and required-ness**, exactly as the contract spells them —
  including the mixed casing that must never be normalised (`clientId`/`clientSecret`
  on `/auth/login` beside `user_id`/`client_secret` on `/auth/pki-login`).
* **Cookie authentication end to end**, including both login flows, the
  `HttpOnly`/`SameSite=Strict` attributes, and 401 without the cookie.
* **Amount typing**: every amount, balance, reserve and limit is a JSON *string*,
  never a number.
* **Timestamp typing**: `expiry_date`, `time_lock`, `created_at`/`valid_until` on
  quotes are int64 **Unix seconds**; `created_at`/`updated_at` on records are RFC
  3339 strings. The two are never conflated.
* **Closed vocabularies** the contract actually commits to: FX states, HTLC lock
  states, reserve states, `pool_status`.
* **Settlement, not just API shape** (live mode): the reserve prelude produces a
  non-zero tCeBM balance; a cross-currency swap debits `amount_in` and not
  `max_amount_in`; the residue returns to the payer; both bridge legs are reported;
  the Hub reconciles.
* **The verified hash-lock pair**, recomputed at run time, using the platform's own
  derivation: the secret is **32 random bytes carried as hex**, and the hash lock is
  `SHA-256` of the **decoded bytes**, not of the hex text
  (`payment-orchestrator/.../server.go:264-270` for lock, `:492-497` for settle,
  where a non-hex secret is rejected outright as `invalid secret hex`). The
  superseded artefacts published a **false** digest and instructed integrators to
  verify it; it is purged from this repository. Note this supersedes the plan's
  proposed `"hello"` pair, which is mathematically correct but unusable against a
  real gateway on both counts.
* **The safety-critical timelock ordering**: the initiator's HTLC lock must outlive
  the responder's. Nothing in the schema enforces it, so the suite asserts it on its
  own request data.

## What this suite does **not** verify, and why

* **The `error` string.** The `/api/v1` error model is `{error: "<free-form>"}` with
  **no machine-readable code**; the same field carries a human sentence in one place
  and a code (`INVALID_CERTIFICATE_CHAIN`) in another. `ErrorCodeResponse {error,
  code}` is declared in both platform specs and referenced by **zero** operations,
  while the live `/api/v2` handlers emit `error_code`. Assertions are on status codes
  only, except on `/api/v2` where `error_code` is genuinely required by the schema.
* **Refund before expiry.** The platform documents no status code for it.
* **Sending the prefixed `HTLC_STATE_*` vocabulary to the `/api/v1/htlc/search` `state`
  query parameter**, which declares the bare `LOCKED|SETTLED|REFUNDED` set. Two
  vocabularies coexist, the platform never reconciles them, and what happens when
  you cross them is undocumented.
* **FX expiry.** There is no `EXPIRED` state despite `expiry_date` on every
  agreement and a documented `SYSTEM_JOB` "background expiration worker" event
  source. Expiry has no representable terminal state.
* **Bridge position state membership.** The real vocabulary (`LOCKING`, `ACTIVE`,
  `BURNING`, `BURNED`, `RELEASED`, `RECONCILIATION_REQUIRED`) lives in Go domain code
  and appears in no OpenAPI document; the platform's own example suggests
  `PENDING`/`CLOSED`, which do not exist. The field is asserted to be a non-empty
  string and nothing more.
* **Fourteen `/api/v2` operations' field-level contracts**, including
  `swap/cross-currency` and `bridge/lock-mint`, which the platform declares as
  free-form objects and says so explicitly. Our contract fills them from handler
  source under `x-cbweb3-source: implementation-observed`; the tests assert only what
  that recovered shape supports.
* **Idempotency.** There is no `Idempotency-Key`, no `If-Match`/ETag and no
  operation-level header parameter anywhere in either spec. Replay safety is
  per-endpoint and payload-keyed. A generic retry wrapper is not part of this API.
* **Decimals.** No decimals count is stated anywhere for tCeBM/fCeBM. `/api/v1`
  amounts are 7-digit strings with no unit statement; `/api/v2` amounts are
  18-decimal base units. The suite uses the contracts' own literal example strings
  and **never converts a figure between the two surfaces**.
* **Role enforcement matrices.** Authorisation exists only in prose — no OAuth2
  scopes, no `x-` extension, no per-role security scheme — so a 403 cannot be
  provoked without a second credential set the suite has no way to provision.
* **Compliance, governance, onboarding and supervisor surfaces** (~42 Scenario A
  paths), and every `/internal/*` route. Out of scope for this release.
* **Circuit-breaker pause/resume, hub currency registration and pair
  propose/confirm** as standalone tests: they halt a live corridor or write to a
  shared registry.
* **Performance, TLS configuration, privacy proofs, and event ordering.**

---

## Known ambiguities the suite works around rather than resolves

| Ambiguity | How the suite handles it |
|---|---|
| The PKI nonce may be signed as its **hex text** or its **decoded bytes** — neither spec says which. | `CBWEB3_NONCE_ENCODING=utf8` (default) or `hex`. If `wallet/bind` returns 401 `NONCE_SIGNATURE_MISMATCH`, try the other. |
| `POST /auth/login`'s PKI 200 is typed as `AuthResponse`, which has no `nonce` property — a strict response validator fails a correct login. | Mirrored, not fixed: the contract models the nonce shape as a documented `oneOf` marked `x-cbweb3-source: implementation-observed`. |
| Pool-pair identifiers have **three** incompatible spellings, none canonical. | Never constructed. Always read from `GET /api/v2/amm/pairs` (`pair_id`) and treated as opaque. |
| `pair_id` contains a `/`, so it must be **percent-encoded** in the `{pair}` path segment but goes **unencoded** into `pool_pair` bodies and query parameters. Documented nowhere. | The path-parameter call encodes it; the body/query calls do not. |
| `GET /api/v2/amm/quote/exact-output` documents `amountOut`; the handler reads `amount_out`. | The suite sends `amount_out` and has an explicit `error` test proving the camelCase form is refused. |
| `GET /api/v2/amm/quote/cross-currency` documents **zero** parameters yet requires three. | Same treatment: the three are sent, and a 400 for the parameterless call is asserted. |
| `bridge/positions`, `hub/currencies` and `amm/pairs` are documented as bare arrays but return wrapped objects. | The wrapped form is asserted. |
| `GET /api/v2/governance/circuit-breaker/status` has an undeclared `pair` parameter that defaults to a literal `BRL-USD`. | `pair` is always sent explicitly. |
| Cookie `Max-Age` differs across the three `Set-Cookie` examples (300 / 900 / 3600). | No session TTL is ever hard-coded. |
| A `domain`-scoped cookie is silently dropped against a `localhost` origin (`http.cookiejar` rewrites the host to `localhost.local`). | The CI cookie is installed with no domain. Any cookie-jar client aimed at a localhost sandbox hits this. |

---

## Layout

```
conformance/
├── pytest.ini                      markers, --strict-markers
├── README.md                       this file
├── spec/
│   ├── conformance_requirements.md what "pass" means, scope, levels
│   └── security_checklist.md       OWASP API Top 10 adapted to CBWeb3
└── tests/
    ├── conftest.py                 session/cookie fixtures, 4 auth modes, profile gate
    ├── helpers.py                  contract-derived constants and assertion helpers
    ├── auth/test_auth_session.py                     10
    ├── pvp/test_pvp_reserves.py                      14
    ├── pvp/test_pvp_fx_agreement.py                  14
    ├── pvp/test_pvp_htlc.py                          12
    ├── amm/test_amm_reserves.py                      10
    ├── amm/test_amm_registry.py                      10
    ├── amm/test_amm_swap.py                          12
    └── amm/test_amm_bridge.py                         8
```

Tests are independent and order-free: preconditions come from fixtures, never from
another test having run first. `pytest --collect-only` succeeds with zero errors
with no server running.
