# Conformance Requirements — CBWeb3 Toolbox

> Contracts covered: `auth` / `pvp` / `amm` at **v2.3.0** | Revised: 2026-08-20
> Target: **CBWeb3 API Gateway v2.3.0**, scenarios A and B

---

## 1. Purpose

This document defines what it means for an implementation to be **conformant** with
the Toolbox interface contracts: the testing levels, the pass/fail criteria, and the
scope boundaries.

It is deliberately narrow. The Toolbox mirrors a delivered platform; it does not
specify one. Where the platform documents no behaviour, conformance requires
nothing.

---

## 2. Scope

### In scope

| Contract | Scenario | Paths / ops | Conformance tests |
|---|---|---|---|
| `contracts/auth/openapi_auth_v2.3.0.yaml` | shared (A and B) | 8 / 8 | 10 |
| `contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — single-ledger | 28 / 32 | 40 |
| `contracts/amm/openapi_amm_v2.3.0.yaml` | B — hub-and-spoke | 52 / 59 | 40 |
| **Total** | | **88 / 99** | **90** |

Covered domains: authentication and session; the reserve lifecycle (deposits,
escrows, redeems, balances) on both scenarios; the FX agreement lifecycle and
dual-layer HTLC (Scenario A); AMM quotes, cross-currency swaps, bridging, hub
registries and pool status (Scenario B).

### Out of scope

* **Every `/internal/*` route**, on all three prefixes and both scenarios. They are
  relay endpoints guarded by a shared secret and are never client-callable. They are
  explained in the contract prose and modelled nowhere.
* **Compliance, governance, onboarding, supervisor and oversight write paths**
  (~42 Scenario A paths). Deferred, not rejected.
* Performance, load and stress testing.
* Network-level security (TLS configuration, CORS, firewall rules).
* Smart contract bytecode verification and Zeto privacy proofs.
* Multi-node relay coordination and event ordering.

### Explicitly not conformance criteria

Because the platform does not define them:

* The text of any `error` string. The `/api/v1` error model is
  `{error: "<free-form>"}` with **no** machine-readable code.
* Refunding an HTLC before its timelock expires — no status code is documented.
* Sending the prefixed `HTLC_STATE_*` vocabulary to the `/api/v1/htlc/search` `state` query
  parameter, which declares the bare `LOCKED|SETTLED|REFUNDED` set.
* An `EXPIRED` FX state — none exists.
* Membership of `bridge_state` in any vocabulary — the real one lives in Go domain
  code and is published in prose only.
* Any decimals or scale factor for tCeBM/fCeBM on `/api/v1`.
* Idempotent replay of any operation. No `Idempotency-Key`, `If-Match` or ETag
  exists anywhere.

---

## 3. Testing levels

### Level 1 — Artefact validation (static)

| Check | Tool | In CI |
|---|---|---|
| All three OpenAPI contracts are valid and lint-clean | Spectral (`--fail-severity=error`) | Yes |
| Mock JSON conforms to `schemas/mock.schema.json` | ajv-cli | Yes |
| Vector JSON conforms to `schemas/vector.schema.json` | ajv-cli | Yes |
| Every mock/vector path resolves to a real contract path+method | `tools/validate_artifact_paths.py` | **Required by plan §5.7; not yet on disk** |
| Every documented `SHA-256(secret) = hashlock` pair recomputes | `tools/verify_hashlocks.py` | **Required by plan §5.7; not yet on disk** |

> **Hash-lock derivation.** A pair verifies as `hash_lock == SHA-256(bytes.fromhex(secret))`.
> The secret is 32 random bytes carried as hex and the digest is taken over the
> **decoded bytes**, never over the hex text — the settle handler hex-decodes the
> secret first and rejects a non-hex value as `invalid secret hex`. A verifier that
> hashes the UTF-8 text will report every correct pair as a mismatch.

**Pass:** zero errors. The two pending gates are the ones whose absence let a fully
invented API ship green for four sessions; Level 1 is not complete without them.

### Level 2 — Mock conformance (Prism)

Runs the `mock_safe` tier — **53 tests** — against three Prism instances, one per
contract (4010 pvp / 4011 amm / 4012 auth).

Prism is **stateless** and serves static examples, so Level 2 proves that the tests,
the contracts and the contracts' own examples agree. It proves nothing about
settlement.

```bash
pytest -m mock_safe            # CBWEB3_AUTH_MODE defaults to mock
```

**Pass:** exit code 0 with 53 passed, 37 deselected.

Authentication at this level is a documented escape hatch: no login is performed and
a synthetic cookie is injected into the jar, because Prism validates only the
*presence* of the `access_token` cookie credential, never its value. A request
*without* the cookie still yields 401, which the suite asserts 12 times.

### Level 3 — Implementation conformance (real gateway)

Runs the full suite against a provisioned deployment, with a real login
(`CBWEB3_AUTH_MODE=direct` or `pki`) and a declared `CBWEB3_PROFILE`.

```bash
CBWEB3_AUTH_MODE=pki CBWEB3_PROFILE=scenario-a-bank pytest -m scenario_a
```

**Pass:** every test that is neither skipped nor deselected passes.

**A skip is not a failure and not a pass.** A conformance report must state which
tests skipped and why. Legitimate skip reasons are exactly:

| Reason | Meaning |
|---|---|
| Profile mismatch | The deployment does not register that route group. Route groups are wired conditionally per gateway role. |
| `501` | Feature not configured in this deployment. |
| `502` | An upstream dependency is unreachable — an estate problem. |
| `503` | Capability disabled in this deployment. |
| Missing corridor configuration | The live Scenario B settlement tests need `CBWEB3_SOURCE_CURRENCY`, `CBWEB3_TARGET_CURRENCY` and `CBWEB3_BENEFICIARY_BANK_ID`. |
| Empty registry | No AMM pair is provisioned, so no pool-addressed test can run. |

---

## 4. What "pass" means

An implementation is **conformant with the CBWeb3 Toolbox contracts at v2.3.0** if,
for every non-skipped test:

1. The HTTP **status code** matches one of those the contract declares for that
   operation.
2. Every field the contract marks **required** is present in the response.
3. **Typing** holds: amounts, balances, reserves, limits and rates are JSON
   *strings*; `expiry_date`, `time_lock` and the quote `created_at`/`valid_until` are
   int64 Unix seconds; record `created_at`/`updated_at`/`expires_at` are RFC 3339
   strings.
4. Values of fields the contract declares as a **closed enum** fall inside it —
   FX agreement state, HTLC lock state, reserve status, `pool_status`. Fields the
   contract declares as free strings are *not* constrained, even where an observed
   vocabulary is documented in prose.
5. Authentication behaves as declared: the `access_token` cookie authorises,
   its absence yields 401, and `Authorization: Bearer` is accepted **only** on the
   12 Scenario B `/api/v2` `RequireAnyAuth` operations.
6. In Level 3, settlement actually happens: the reserve prelude yields a non-zero
   tCeBM balance; a cross-currency swap debits `amount_in` (not `max_amount_in`),
   returns the residue, reports both bridge legs, and leaves the Hub reconciled.

Field **values** beyond the above are not asserted, because the contracts publish
them as examples rather than as guarantees.

---

## 5. Test execution requirements

### Environment

* Python 3.9+
* `pip install pytest requests` — plus `cryptography` for `CBWEB3_AUTH_MODE=pki`
* Node 18+ for Prism at Level 2
* Network access to the target gateway, or localhost for the mocks

### Configuration

Full variable list in [`../README.md`](../README.md). The essentials:

| Variable | Default | Meaning |
|---|---|---|
| `CBWEB3_BASE_URL` | `http://localhost:4010` | Bare origin, **no `/api/v1` suffix** |
| `CBWEB3_AUTH_MODE` | `mock` | `mock` \| `direct` \| `pki` \| `bearer` |
| `CBWEB3_PROFILE` | `mock` | Gateway role; governs which routes may skip |

`CBWEB3_AUTH_TOKEN` **no longer exists**. The gateway does not use bearer tokens on
`/api/v1`.

### Markers

Selection is three-axis: category (`happy_path` / `error` / `edge_case`), execution
tier (`mock_safe` / `live_only` / `bearer_ok`) and domain-plus-scenario (`auth`,
`pvp`, `amm`, `reserves`, `fx_agreement`, `htlc`, `registry`, `amm_swap`, `bridge`,
`scenario_a`, `scenario_b`). `--strict-markers` is enabled.

---

## 6. Versioning

* Contract filenames and `info.version` track **the gateway version they describe**,
  not an independent Toolbox version. All three are at `2.3.0`.
* `openapi_pvp_v0.1.0.yaml` described an API that never existed on any CBWeb3
  gateway. It is **deleted**, with no shim, alias or path compatibility layer. There
  were no consumers to break.
* A future gateway release produces new contracts, new vectors and new tests at the
  gateway's version number.
* An implementation must state which gateway version it targets. Passing at one
  version implies nothing about another.

---

## 7. Reporting conformance

An implementation that passes Level 3 may report conformance by opening a PR that
states:

1. The gateway version and scenario targeted, and the `CBWEB3_PROFILE` used.
2. Full pytest output, including the **skip list with reasons** — a run with many
   skips and no failures is not a full pass, and must not be presented as one.
3. The Toolbox commit hash the run was executed against.
