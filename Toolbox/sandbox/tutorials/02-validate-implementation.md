# Tutorial 2 — Validate an implementation

**Time:** ~15 minutes · **Prerequisites:** Python 3.9+, `pytest`, `requests` ·
**Scenario:** both

Tutorials [01](01-pvp-settlement-mock.md) and [03](03-hub-swap-mock.md) drive the API by
hand. This one runs the executable conformance suite — against a mock, and then against a
real gateway.

---

## 1. Install

```bash
pip install pytest requests
# PKI mode additionally needs:  pip install cryptography
```

---

## 2. Run it against a mock

Start the mocks (see [mock-server-setup.md](../devnet-guide/mock-server-setup.md)):

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &
```

Then:

```bash
cd Toolbox/conformance
CBWEB3_AUTH_MODE=mock CBWEB3_PROFILE=mock \
  pytest tests/ -m "mock_safe and happy_path" -v
```

That is the exact invocation CI runs. Both defaults are `mock`, so the environment
variables are shown for clarity rather than necessity.

You will see a mixture of `PASSED` and `SKIPPED`, and **the skips are the point** — read
the next section before concluding anything from a green run.

---

## 3. The two marker tiers, and why a green mock run proves less than it looks

Markers come in two orthogonal tiers. Every test carries one from each.

| Tier | Marker | Meaning |
|---|---|---|
| **Execution** | `mock_safe` | A single request; asserts status code and response shape. Runs against Prism **and** a live gateway. |
| | `live_only` | Multi-step state transitions, balance deltas, residue reconciliation. **Skipped automatically in `mock` mode**, because Prism is stateless. |
| **Category** | `happy_path` / `error` / `edge_case` | What kind of case it is. |

Plus domain markers (`auth`, `pvp`, `amm`), sub-domain markers (`reserves`, `fx_agreement`,
`htlc`, `registry`, `amm_swap`, `bridge`), scenario markers (`scenario_a`, `scenario_b`) and
`bearer_ok`. Run `pytest --markers` for the authoritative list; `pytest.ini` sets
`--strict-markers`, so a typo in a marker name fails the run rather than silently selecting
nothing.

**A mock is a shape checker, not a settlement checker.** Prism has no state, so it cannot
tell you that a payer was debited `amount_in` rather than `max_amount_in`, that residue was
returned, or that an HTLC refund actually restored a balance. Those assertions all live in
`live_only`. A fully green `mock_safe` run means your client sends and parses the right
shapes — nothing more.

Useful selections:

```bash
pytest tests/ -m "scenario_a"                # Scenario A only
pytest tests/ -m "amm and not live_only"     # Scenario B, mock-compatible
pytest tests/ -m "error"                     # error handling only
pytest tests/auth/ -v                        # just the session surface
```

---

## 4. Point it at a real gateway

### Configuration

| Variable | Purpose |
|---|---|
| `CBWEB3_BASE_URL` | **Bare origin**, no `/api/v1` suffix. Default `http://localhost:4010` |
| `CBWEB3_AUTH_BASE_URL` / `CBWEB3_PVP_BASE_URL` / `CBWEB3_AMM_BASE_URL` | Optional per-domain overrides. Against a real gateway all three default to `CBWEB3_BASE_URL`, because one gateway serves the whole surface |
| `CBWEB3_AUTH_MODE` | `mock` \| `direct` \| `pki` \| `bearer`. Default `mock` |
| `CBWEB3_PROFILE` | `mock` \| `scenario-a-bank` \| `scenario-a-cb` \| `scenario-b-bank` \| `scenario-b-cb` |
| `CBWEB3_CLIENT_ID` / `CBWEB3_CLIENT_SECRET` | `direct` and `pki` modes |
| `CBWEB3_USER_ID` / `CBWEB3_KEY_PEM` / `CBWEB3_CERT_PEM` | `pki` mode |
| `CBWEB3_NONCE_ENCODING` | `pki` mode: `utf8` (default) or `hex`. Whether the nonce is signed as text or as its decoded bytes. The platform does not state which it expects — if `wallet/bind` returns `401 NONCE_SIGNATURE_MISMATCH` with an otherwise valid key, try the other one |
| `CBWEB3_BEARER_TOKEN` | `bearer` mode |
| `CBWEB3_TIMEOUT` | Per-request timeout, seconds. Default `15` |

> **`CBWEB3_AUTH_TOKEN` no longer exists anywhere in this repository.** The `/api/v1`
> surface is authenticated by an `access_token` **HttpOnly cookie**, not by a bearer token,
> so the suite carries the credential in a `requests.Session` cookie jar. A static header
> dict cannot hold a session.

A starting point:
[`../sample-configs/.env.example`](../sample-configs/.env.example).

### `direct` mode — governance, supervisor, NOC

```bash
export CBWEB3_BASE_URL=https://gateway.example.org
export CBWEB3_AUTH_MODE=direct
export CBWEB3_PROFILE=scenario-a-cb
export CBWEB3_CLIENT_ID=your-client-id
export CBWEB3_CLIENT_SECRET=your-client-secret

cd Toolbox/conformance
pytest tests/ -v
```

The fixture POSTs `/api/v1/auth/login`, lets `Set-Cookie` populate the jar, then asserts
`GET /api/v1/auth/me` returns `200` and reads `roles` to decide which families the run may
exercise.

### `pki` mode — what a commercial bank actually does

```bash
export CBWEB3_AUTH_MODE=pki
export CBWEB3_PROFILE=scenario-a-bank
export CBWEB3_USER_ID=f47ac10b-58cc-4372-a567-0e02b2c3d479
export CBWEB3_KEY_PEM=/path/to/participant-key.pem
export CBWEB3_CERT_PEM=/path/to/participant-cert.pem
```

`/api/v1/auth/login` returns `{"nonce": "<hex>"}` with **no tokens and no cookies**; the
fixture signs the nonce with the P-256 participant key (DER, hex) and POSTs
`/api/v1/auth/wallet/bind`, which is the call that sets the cookies.

Never point this at production credentials, and never commit key material.

### `bearer` mode — a narrow slice only

```bash
export CBWEB3_AUTH_MODE=bearer
export CBWEB3_BEARER_TOKEN=...
pytest tests/ -m bearer_ok -v
```

`BearerAuth` is declared only in the Scenario B contract, and only as an alternative to the
cookie on the twelve `/api/v2` `RequireAnyAuth` routes. **The two cross-currency swap
endpoints are cookie-only.** Every test that is not marked `bearer_ok` is skipped in this
mode.

---

## 5. Reading the results

### `SKIPPED` is usually information, not failure

Three things produce skips, and all three are legitimate:

1. **`live_only` in `mock` mode.** Prism cannot hold state.
2. **Wrong scenario.** `scenario_a` tests skip against a Scenario B profile and vice versa.
   The two gateways genuinely serve different surfaces — Scenario B has no HTLC endpoints
   at all.
3. **Route not registered on this deployment.** Route groups are wired conditionally per
   gateway role, so a `404` may mean *"this gateway is not that kind of node"* rather than
   *"non-conformant"*. The profile fixture turns that case into a skip.

### The reserve lifecycle needs two hosts

The *create* half of deposits, escrows and redeems is registered only on a
**commercial-bank** gateway; the `approve` / `reject` / `exchange` half only on a **Central
Bank** gateway, gated on `ROLE_TREASURY`. **No single base URL can drive the whole
lifecycle.** Against a real deployment, run the suite twice with two profiles and two
sessions, or expect the approve-side tests to skip.

### Assert on status codes, never on error text

The `/api/v1` error model is `{"error": "<free-form string>"}` with **no machine-readable
code** — the same field carries human sentences and code-like tokens. The suite asserts
status codes only. If you are writing new tests, do the same, and do **not** resurrect the
`HTLC_HASH_MISMATCH` / `HTLC_EXPIRED` codes that earlier Toolbox material invented: no
CBWeb3 gateway has ever emitted them.

---

## 6. What conformance does and does not prove

**It proves** your implementation serves the contract's paths and methods, requires the
`access_token` cookie where the contract says so, returns the documented status codes, and
— in `live_only` mode against a real gateway — that value actually moves: balances change,
HTLC refunds restore funds, swap residue comes back.

**It does not prove** performance or scale, on-chain finality, cryptographic correctness of
signatures or Zeto proofs, regulatory compliance, or that your deployment's environment
variables (`FX_RATE_TOLERANCE_PCT`, `FX_AGREEMENT_HTLC_STRICT`, Pente on/off) match anyone
else's. **None of those three is discoverable through any endpoint** — a client cannot tell
which behaviour it is talking to.

---

## 7. The same gates in CI

`.github/workflows/toolbox-ci.yml` runs, on every push and pull request touching `Toolbox/`:

| Job | What it enforces |
|---|---|
| `lint-openapi` | `spectral lint --fail-severity=error` on **every** `Toolbox/contracts/*/openapi_*.yaml` |
| `validate-schemas` | `ajv` on every mock and every test-vector file |
| `validate-artifact-paths` | Every mock and vector `path` must resolve to a real path **and method** in one of the three contracts |
| `verify-hashlocks` | Every documented secret/hash-lock pair is recomputed; the discredited `7f83b165…` value must be absent from the repository |
| `conformance-smoke` | Three Prism instances, cookie-authenticated probes, a `401`-without-cookie negative assertion, then `-m "mock_safe and happy_path"` |
| `drift-check` (manual) | Fetches a live gateway's self-served `GET /openapi.yaml` and diffs its path set against our contracts |

The last three did not exist before the 2026-08 realignment, and their absence is precisely
why a contract describing seven endpoints that were never implemented shipped green for
four sessions. Run them locally before opening a PR:

```bash
python Toolbox/tools/validate_artifact_paths.py
python Toolbox/tools/verify_hashlocks.py
```

---

## Next steps

- [Conformance suite README](../../conformance/README.md) — full coverage table
- [Conformance requirements](../../conformance/spec/conformance_requirements.md) — what
  "conformant" formally means
- [Security checklist](../../conformance/spec/security_checklist.md) — including the
  recorded finding that the platform implements **no CSRF mechanism**, with
  `SameSite=Strict` as the sole mitigation
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — adding your own vectors and tests
