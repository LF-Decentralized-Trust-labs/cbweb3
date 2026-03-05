# Conformance Requirements — CBWeb3 Toolbox

> Version: 0.1.0 | Effective: 2026-03-05

---

## 1. Purpose

This document defines what it means for a CBWeb3 implementation to be **conformant** with the Toolbox interface contracts. It establishes the testing levels, pass/fail criteria, and scope boundaries.

---

## 2. Scope

### In scope (v0.1.0)
- **PvP Settlement domain** only: FX Agreement + HTLC endpoints
- Contract: `contracts/pvp/openapi_pvp_v0.1.0.yaml`
- Test vectors: `test-vectors/pvp/pvp_fx_agreement_vectors.json`, `test-vectors/pvp/pvp_htlc_vectors.json`

### Out of scope (v0.1.0)
- Authentication & Access endpoints
- Token & Lifecycle (Zeto) endpoints
- AMM & Liquidity endpoints
- Compliance & Governance endpoints
- Performance, load, and stress testing
- Network-level security (TLS configuration, firewall rules)
- Smart contract bytecode verification

---

## 3. Testing levels

### Level 1 — Schema validation (static)

**What it checks:** Toolbox artifacts themselves are structurally correct.

| Check | Tool | Runs in CI? |
|-------|------|-------------|
| OpenAPI specs are valid and lint-clean | Spectral | Yes |
| Mock JSON files conform to `schemas/mock.schema.json` | ajv-cli | Yes |
| Test vector JSON files conform to `schemas/vector.schema.json` | ajv-cli | Yes |

**Pass criteria:** Zero errors from Spectral and ajv. Warnings are acceptable.

### Level 2 — Behavioral conformance (against mock server)

**What it checks:** The conformance test suite passes against a Prism mock server running the OpenAPI spec. This validates that the tests themselves are correct and that the spec examples are consistent.

| Check | Tool | Runs in CI? |
|-------|------|-------------|
| FX Agreement happy-path tests pass | pytest + Prism | Yes |
| HTLC happy-path tests pass | pytest + Prism | Yes |

**Pass criteria:** All pytest tests pass with exit code 0.

**Limitations:** Prism returns examples from the OpenAPI spec, so error-path and edge-case tests cannot be validated at this level. They require a real implementation (Level 3).

### Level 3 — Implementation conformance (against real deployment)

**What it checks:** A real CBWeb3 implementation passes all test vectors (happy paths, errors, edge cases).

| Check | Tool | Runs in CI? |
|-------|------|-------------|
| All FX Agreement vectors pass | pytest | No (manual) |
| All HTLC vectors pass | pytest | No (manual) |

**Pass criteria:** All test vectors pass. An implementation that fails any vector is **not conformant** with the contract version.

**How to run:**
```bash
CBWEB3_BASE_URL=https://your-deployment.example.com pytest Toolbox/conformance/tests/
```

---

## 4. What "pass" means

An implementation is **conformant with PvP contract v0.1.0** if and only if:

1. For every test vector in `test-vectors/pvp/`:
   - The preconditions can be set up (or are met by the system state)
   - The `input` request returns a response matching `expectedOutput`
   - The HTTP status code matches exactly
   - All fields listed in `expectedOutput.body` are present with matching values
   - Placeholder values (`{{NON_EMPTY_STRING}}`, `{{POSITIVE_INTEGER}}`, etc.) match their constraints

2. All response bodies conform to the schemas defined in the OpenAPI spec

3. The error model (`code` + `message`) is present on all 4xx/410 responses

### Placeholder conventions

| Placeholder | Validation rule |
|-------------|----------------|
| `{{NON_EMPTY_STRING}}` | Field exists, is a string, length > 0 |
| `{{POSITIVE_INTEGER}}` | Field exists, is an integer, value > 0 |
| `{{SAME_AS_INPUT}}` | Field value equals the corresponding input value |
| `{{SAME_AS_PRECONDITION}}` | Field value equals the corresponding precondition value |
| `{{ERROR_CODE}}` | Field exists, is a non-empty string (exact code is implementation-specific) |
| `{{VALID_AGREEMENT_ID}}` | Use a real agreement ID from a previous test step |
| `{{VALID_CONTRACT_ID}}` | Use a real HTLC contract ID from a previous test step |
| `{{PROPOSED_AGREEMENT_ID}}` | Use an agreement ID with status PROPOSED |

---

## 5. What is explicitly NOT tested

- **Performance:** No latency or throughput requirements
- **Ordering guarantees:** Tests do not validate event ordering across nodes
- **Privacy verification:** Zeto privacy proofs are not verified by conformance tests
- **Multi-node coordination:** The two-leg HTLC coordination (both parties locking) is documented but not enforced by automated tests
- **Authentication:** Tests use a mock auth token; real JWT validation is not tested
- **Idempotency:** Tests do not verify idempotent behavior of endpoints

---

## 6. Test execution requirements

### Environment
- Python 3.9+
- `pip install pytest requests`
- Network access to the target deployment (or localhost for mock server)

### Configuration
- `CBWEB3_BASE_URL`: Target implementation URL (default: `http://localhost:4010`)
- `CBWEB3_AUTH_TOKEN`: Bearer token for authentication (default: synthetic token)

### Running against Prism (Level 2)
```bash
# Terminal 1: Start mock server
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010

# Terminal 2: Run tests
pytest Toolbox/conformance/tests/ -v
```

### Running against a real implementation (Level 3)
```bash
export CBWEB3_BASE_URL=https://your-deployment.example.com
export CBWEB3_AUTH_TOKEN=your-real-jwt-token
pytest Toolbox/conformance/tests/ -v
```

---

## 7. Versioning

- Conformance requirements are versioned alongside the interface contracts
- A new contract version (e.g., v0.2.0) will produce a new set of test vectors and conformance tests
- Implementations must specify which contract version they target
- Backward compatibility: passing v0.2.0 tests does NOT imply passing v0.1.0 tests (unless explicitly stated)

---

## 8. Reporting conformance

Implementations that pass all Level 3 tests can report conformance by:
1. Opening a PR to add their name to a future `conformance/results/` directory
2. Including test output logs as evidence
3. Specifying the contract version and test vector commit hash
