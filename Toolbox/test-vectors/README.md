# Test Vectors — CBWeb3 Toolbox

Deterministic input/output fixtures for **CBWeb3 API Gateway v2.3.0**. Each vector states a
precondition, a request, and the status code and response shape the gateway actually returns,
so a third party can check an implementation without reading Go source.

Everything here is derived from the three published contracts:

| Contract | Scenario | Vector directory |
|---|---|---|
| `contracts/auth/openapi_auth_v2.3.0.yaml` | shared (A and B) | [`auth/`](auth/) |
| `contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — single-ledger, spoke-to-spoke | [`pvp/`](pvp/) |
| `contracts/amm/openapi_amm_v2.3.0.yaml` | B — hub-and-spoke, International Hub | [`amm/`](amm/) |

**140 vectors in 9 files** — 66 happy-path, 61 error, 13 edge-case.

| File | Covers | hp | err | edge | total |
|---|---|---|---|---|---|
| `auth/auth_vectors.json` | both login flows, `/auth/me`, refresh, logout | 6 | 4 | 0 | **10** |
| `pvp/pvp_reserves_vectors.json` | deposit → approve → exchange → escrow → balance | 7 | 5 | 1 | **13** |
| `pvp/pvp_fx_agreement_vectors.json` | propose / accept / reject / cancel / settle / audit | 7 | 11 | 1 | **19** |
| `pvp/pvp_htlc_vectors.json` | dual-layer HTLC lock, settle, refund, search | 7 | 11 | 4 | **22** |
| `amm/amm_reserves_vectors.json` | Scenario B reserve lifecycle (different paths) | 7 | 3 | 1 | **11** |
| `amm/amm_registry_vectors.json` | credential-free discovery, currency and pair registries | 8 | 6 | 2 | **16** |
| `amm/amm_swap_vectors.json` | quotes, cross-currency swap, residue reconciliation | 6 | 9 | 2 | **17** |
| `amm/amm_bridge_vectors.json` | lock-mint, burn-unlock, liquidity commits, reconciliation | 8 | 6 | 1 | **15** |
| `amm/amm_governance_vectors.json` | circuit breaker, transfer limits, oversight disclosure | 10 | 6 | 1 | **17** |
| **Total** | | **66** | **61** | **13** | **140** |

---

## How to run them

### 1. Validate the files themselves

Every file must validate against [`../schemas/vector.schema.json`](../schemas/vector.schema.json).
This runs in CI (`.github/workflows/toolbox-ci.yml`, job `validate-schemas`):

```bash
npm install -g ajv-cli ajv-formats
for v in Toolbox/test-vectors/*/*.json; do
  ajv validate -s Toolbox/schemas/vector.schema.json -d "$v" --spec=draft2020
done
```

Two further CI gates apply to this directory:

* **`tools/validate_artifact_paths.py`** — every `input.path` + `input.method` must resolve to a
  real path and method in one of the three contracts. It expands `pathParams` into `path` first,
  and enforces the convention in both directions: a `{placeholder}` with no `pathParams` entry
  fails, and so does a `pathParams` entry naming no placeholder. All 140 vectors pass this today.
* **`tools/verify_hashlocks.py`** — recomputes every documented secret/hash-lock pair. The only
  pair in this directory is the `syntheticSecrets` block of `pvp/pvp_htlc_vectors.json`; see
  [`pvp/README.md`](pvp/README.md) for its shape and its derivation rule.

### 2. Execute them against a mock

```bash
cd Toolbox
prism mock contracts/pvp/openapi_pvp_v2.3.0.yaml  --port 4010   # Scenario A
prism mock contracts/amm/openapi_amm_v2.3.0.yaml  --port 4011   # Scenario B
prism mock contracts/auth/openapi_auth_v2.3.0.yaml --port 4012  # shared
```

A mock can only confirm **status code and response shape**. Prism is stateless: it cannot serve
a nonce/bind handshake, cannot hold an FX agreement in `ACCEPTED`, and cannot move a balance.
Vectors whose `preconditions` describe prior state are `live_only` in practice.

Prism validates only the *presence* of the `access_token` cookie, never its value, so inject a
synthetic one and rely on the `401` vectors as the negative assertion:

```python
session.cookies.set("access_token", "SYNTHETIC_COOKIE_CI")
```

### 3. Execute them against a real gateway

```bash
cd Toolbox/conformance
CBWEB3_BASE_URL=https://<gateway> CBWEB3_AUTH_MODE=pki pytest
```

The conformance suite in [`../conformance/`](../conformance/) is the executable form of these
vectors; the JSON here is the language-neutral form for implementers not using Python.

---

## Reading a vector

```jsonc
{
  "id": "fx-err-07",              // ^[a-z]+-[a-z]+-\d+$ — family, category, number
  "title": "...",
  "description": "...",
  "category": "happy-path | error | edge-case",
  "preconditions": { ... },       // state that MUST exist first — free-form object
  "input":  { "method", "path", "pathParams"?, "query"?, "headers"?, "body"? },
  "expectedOutput": { "status", "body"?, ... },
  "notes": "..."                  // divergences, traps, and open questions
}
```

`{{PLACEHOLDER}}` values are runtime substitutions, not literals. `{{NON_EMPTY_STRING}}`,
`{{ARRAY}}`, `{{BIGINT_STRING}}` and friends are assertions about **shape**, not equality —
identifiers, transaction hashes and timestamps are implementation-specific. Values written
literally (`"ACCEPTED"`, `"HTLC_STATE_LOCKED"`, `"cd0a60b0…"`, `201`) **are** exact.

`input.path` is the contract's templated path, verbatim, so that CI can resolve it; the concrete
values go in `pathParams`. Query parameters go in `query`, never appended to `path`.

---

## Five rules these vectors encode

1. **Assert on the status code, never on the error text.** The `/api/v1` error model is
   `{error: "<free-form string>"}` with **no machine-readable code at all**. `/api/v2` adds a code
   under three different key names — `error_code` on the AMM and swap family, `code` on the
   registry family, and an undocumented `required_role` array on Scenario A's 403s.
   `ErrorCodeResponse` is declared in both platform specs and referenced by zero operations.
2. **Never invent an endpoint, field, status code or error code.** Where the platform documents
   no behaviour for a case, the gap is named in `notes` rather than filled with a plausible guess.
   The invented codes `HTLC_HASH_MISMATCH`, `HTLC_EXPIRED` and `AGREEMENT_NOT_ACCEPTED` from
   pre-realignment Toolbox material exist nowhere on the platform and are gone.
3. **Never send caller identity in a request body.** `payer_bank_id`, `payer_id`,
   `owner_bank_id`, `requester_besu_address` and friends are deprecated; handlers derive identity
   from the session. Two vectors (`reserve-edge-01`, `bridge-edge-01`) prove the value is
   overwritten rather than honoured.
4. **Never construct a pool-pair identifier.** Five incompatible spellings appear across the
   delivered artefacts and none is canonical — see [`../DIVERGENCES.md`](../DIVERGENCES.md).
   Take `pair_id` verbatim from `GET /api/v2/amm/pairs`.
5. **There is no idempotency mechanism.** No `Idempotency-Key`, no `If-Match`, no ETag, no
   operation-level header parameters anywhere. Replay safety is per-endpoint and payload-keyed
   (`htlc-edge-01`, `htlc-edge-02`). A generic retry wrapper is not part of this API.

---

## Divergences these vectors encode

Where the platform's OpenAPI document and the delivered gateway disagree, **these vectors follow
the gateway**, because the platform surface wins. Every such place is catalogued once, with
source citations, in [`../DIVERGENCES.md`](../DIVERGENCES.md) — eleven rows for Scenario A
(`A1`–`A11`) and twelve for Scenario B (`B1`–`B12`).

Do not keep a second table here. Two catalogues of the same facts is what produced the
situation this register replaced: `mocks/README.md` published eleven rows, this file published
five, and the vectors followed the document on six of them.

The interface contracts now declare the delivered shape for all of these, so no vector asserts a
status code or a field name its contract does not declare. If one ever does again, that is a
defect in the contract, not in the vector.

---

## Provenance

Derived from the delivered CBWeb3 API Gateway v2.3.0 (`LNetNetworks/cbweb3-platform`, branch
`develop`) — both the OpenAPI documents it self-serves at `GET /openapi.yaml` and, where those
documents and the shipped handlers disagree, the handlers. Every divergence is annotated in the
`notes` field of the vector that exercises it. Nothing here is derived from "Deliverable 5" or
the pre-delivery "CBWeb3 Pilot API" baseline.
