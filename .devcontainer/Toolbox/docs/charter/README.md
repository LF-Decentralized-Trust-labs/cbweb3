# CBWeb3 Toolbox

The **CBWeb3 Toolbox** is a curated set of **integration-ready, reusable artifacts** that help implementers and contributors build, test, and validate interoperability and privacy-aware flows in the CBWeb3 ecosystem.

This is **not** a full product implementation. It is a **shared "integration kit"**: contracts, mocks, test vectors, conformance tests, and sandbox guidance that reduce ambiguity and accelerate integration across participants.

---

## What the Toolbox provides

### In scope
- **Interface contracts**: OpenAPI specs scoped per domain, with versioning, error models, and request/response examples
- **Reference mocks**: canonical simulated API responses (JSON fixtures) for each flow, enabling development without live blockchain infrastructure
- **Test vectors**: deterministic fixtures (input → expected output) with preconditions, categories, and validation guidance for compatibility and regression testing
- **Conformance tests**: executable checks that verify compliance with the contracts *(planned — Session 3)*
- **Sandbox / devnet guidance**: non-sensitive sample configurations and "golden path" tutorials *(planned — Session 4)*

### Out of scope
- Production secrets, keys, real customer data, internal endpoints
- Full business logic implementations or production deployments (these belong in their respective component repos)
- Anything that requires privileged access or sensitive runtime material

---

## Repository structure

```text
.devcontainer/Toolbox/
│
├── docs/
│   ├── charter/                    # This document — scope, principles, conventions
│   │   ├── README.md
│   │   └── CONTRIBUTING.md
│   └── onboarding/                 # Quickstart, FAQs (planned — Session 4)
│
├── contracts/                      # Interface contracts (OpenAPI specs per domain)
│   ├── pvp/                        # ✅ PvP Settlement (FX Agreement + HTLC)
│   │   ├── openapi_pvp_v0.1.0.yaml
│   │   ├── CHANGELOG.md
│   │   └── README.md
│   ├── amm/                        # AMM & Liquidity (planned)
│   └── compliance/                 # Compliance & Governance (planned)
│
├── mocks/                          # Reference mocks (static JSON fixtures)
│   └── pvp/                        # ✅ PvP Settlement mocks
│       ├── README.md
│       ├── happy-path/             # Full 6-step PvP settlement
│       │   ├── 01_create_fx_agreement.json
│       │   ├── 02_accept_fx_agreement.json
│       │   ├── 03_htlc_lock_initiator.json
│       │   ├── 04_htlc_lock_counterparty.json
│       │   ├── 05_htlc_settle_counterparty.json
│       │   └── 06_htlc_settle_initiator.json
│       └── timeout-refund/         # HTLC expiration and refund path
│           ├── 01_htlc_settle_expired.json
│           └── 02_htlc_refund.json
│
├── test-vectors/                   # Deterministic test fixtures
│   └── pvp/                        # ✅ PvP Settlement vectors
│       ├── README.md
│       ├── pvp_fx_agreement_vectors.json   # 5 vectors (2 happy, 3 error)
│       └── pvp_htlc_vectors.json           # 8 vectors (3 happy, 4 error, 1 edge)
│
├── conformance/                    # Conformance test suites (planned — Session 3)
│   ├── spec/
│   └── tests/
│
├── sandbox/                        # Sandbox/devnet guidance (planned — Session 4)
│   ├── devnet-guide/
│   └── sample-configs/             # NO secrets
│
└── tools/                          # Validation helpers (planned)
```

---

## Available artifacts

### 1. Interface contracts

Contracts are **OpenAPI 3.0.3 specifications** scoped to a single domain. Each contract lives in its own folder under `contracts/` and includes:

| File | Purpose |
|------|---------|
| `openapi_<domain>_v<version>.yaml` | The OpenAPI specification with paths, schemas, error model, and examples |
| `CHANGELOG.md` | Version history following [Keep a Changelog](https://keepachangelog.com/) |
| `README.md` | Flow diagram, file index, and validation instructions |

**Currently available:**

| Domain | Folder | Endpoints | Description |
|--------|--------|-----------|-------------|
| **PvP Settlement** | `contracts/pvp/` | 7 | FX Agreement (create, accept, get) + HTLC (lock, settle, refund, status) |

#### How to use a contract

**Validate the spec:**
```bash
# Using Spectral (recommended)
npx @stoplight/spectral-cli lint contracts/pvp/openapi_pvp_v0.1.0.yaml

# Using swagger-cli
npx @apidevtools/swagger-cli validate contracts/pvp/openapi_pvp_v0.1.0.yaml
```

**Generate a mock server:**
```bash
# Prism serves the OpenAPI examples as deterministic responses
npx @stoplight/prism-cli mock contracts/pvp/openapi_pvp_v0.1.0.yaml
```

**Generate client SDKs:**
```bash
# Example: generate a Python client
npx @openapitools/openapi-generator-cli generate \
  -i contracts/pvp/openapi_pvp_v0.1.0.yaml \
  -g python \
  -o ./generated/python-pvp-client
```

---

### 2. Reference mocks

Mocks are **static JSON fixtures** representing canonical API request/response pairs. They allow integrators to understand the expected behavior of each endpoint without running any infrastructure.

Each mock file contains:

```json
{
  "id": "mock-pvp-hp-01",
  "title": "Step 1: Create FX Agreement",
  "description": "Human-readable explanation of this step",
  "request": {
    "method": "POST",
    "path": "/api/v1/fx/agreement",
    "headers": { "..." },
    "body": { "..." }
  },
  "response": {
    "status": 201,
    "headers": { "..." },
    "body": { "..." }
  }
}
```

**Currently available:**

| Scenario | Folder | Files | Description |
|----------|--------|-------|-------------|
| **PvP happy path** | `mocks/pvp/happy-path/` | 6 | Complete settlement: propose → accept → lock A → lock B → settle B → settle A |
| **PvP timeout/refund** | `mocks/pvp/timeout-refund/` | 2 | Failed settlement after HTLC expiry + refund |

#### PvP happy path flow

The 6-step happy path simulates a full PvP settlement between two central banks:

```
Step  Endpoint                            Actor           Action
───── ─────────────────────────────────── ─────────────── ──────────────────────────────
  1   POST /fx/agreement                  CB Costa Rica   Propose FX deal (1M CRC @ 0.058)
  2   POST /fx/agreement/{id}/accept      CB Dom. Rep.    Accept terms → READY_FOR_SETTLEMENT
  3   POST /htlc/lock                     CB Costa Rica   Lock 1,000,000 tCeBM-CRC (hashLock)
  4   POST /htlc/lock                     CB Dom. Rep.    Lock 58,000 tCeBM-DOP (same hashLock)
  5   POST /htlc/settle                   CB Costa Rica   Reveal secret → claim tCeBM-DOP
  6   POST /htlc/settle                   CB Dom. Rep.    Use revealed secret → claim tCeBM-CRC
```

#### How to use mocks

**As a reference:** Read the JSON files sequentially (01 → 06) to understand the full PvP flow, including exact request bodies and expected responses.

**With Postman:** Import the mock files as examples in your Postman collection, or use the Postman collection from the D5 deliverable (`CBWeb3_Pilot_API.postman_collection.json`) and configure mock server responses using these fixtures.

**With a mock server (Prism):**
```bash
npx @stoplight/prism-cli mock contracts/pvp/openapi_pvp_v0.1.0.yaml
# Prism will serve the examples embedded in the OpenAPI spec
# The static JSON fixtures provide additional scenarios (timeout, refund)
```

**Programmatically:** Parse the JSON files in your test harness to feed requests and assert responses.

---

### 3. Test vectors

Test vectors are **deterministic test fixtures** for validating that an implementation conforms to the contract. Unlike mocks (which show what the API looks like), vectors define **what MUST happen** under specific conditions.

Each vector file contains an array with this structure:

```json
{
  "id": "htlc-err-01",
  "title": "Settle HTLC — invalid secret (hash mismatch)",
  "description": "Providing a secret that does not hash to the hashLock returns 400.",
  "category": "error",
  "preconditions": {
    "htlcExists": true,
    "htlcStatus": "LOCKED",
    "timeLockNotExpired": true
  },
  "input": {
    "method": "POST",
    "path": "/api/v1/htlc/settle",
    "body": {
      "contractId": "{{VALID_CONTRACT_ID}}",
      "secret": "wrong-secret"
    }
  },
  "expectedOutput": {
    "status": 400,
    "body": {
      "code": "HTLC_HASH_MISMATCH",
      "message": "{{NON_EMPTY_STRING}}"
    }
  },
  "notes": "SHA-256('wrong-secret') != hashLock. Contract MUST reject."
}
```

**Field reference:**

| Field | Description |
|-------|-------------|
| `id` | Unique identifier (format: `<domain>-<category>-<number>`) |
| `title` | Short human-readable name |
| `description` | What is being tested and why |
| `category` | `happy-path`, `error`, or `edge-case` |
| `preconditions` | State that MUST exist before the test runs |
| `input` | HTTP request to send |
| `expectedOutput` | Expected HTTP response (status + key fields) |
| `notes` | Implementation guidance or rationale |

**Placeholder conventions:**
- `{{NON_EMPTY_STRING}}` — assert the field is a non-empty string (exact value is implementation-specific)
- `{{POSITIVE_INTEGER}}` — assert the field is a positive integer
- `{{SAME_AS_INPUT}}` — assert the field matches the corresponding input value
- `{{VALID_AGREEMENT_ID}}` / `{{VALID_CONTRACT_ID}}` — use a real ID from a previous step in the test sequence

**Currently available:**

| File | Domain | Vectors | Categories |
|------|--------|---------|------------|
| `pvp_fx_agreement_vectors.json` | FX Agreement | 5 | 2 happy-path, 3 error |
| `pvp_htlc_vectors.json` | HTLC | 8 | 3 happy-path, 4 error, 1 edge-case |

#### How to validate an implementation

1. Deploy your implementation of the PvP contract
2. For each vector file, iterate through the `vectors` array:
   - Set up the `preconditions` (create agreements, lock funds, etc.)
   - Send the `input` request
   - Assert that the response matches `expectedOutput`
3. **All vectors MUST pass** for an implementation to be considered conformant with the PvP contract v0.1.0

#### Cryptographic correctness

The HTLC vectors use real SHA-256 hashes. The synthetic secret and its hash are:
- **Secret:** `cbweb3-test-secret-2026`
- **SHA-256:** `0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069`

Implementations can independently verify this hash to validate their SHA-256 logic.

---

## Synthetic data policy

All artifacts use **synthetic data** exclusively. The reference corridor is:

| Entity | Address | Currency |
|--------|---------|----------|
| Central Bank of Costa Rica (initiator) | `0xCB001_CostaRica` | tCeBM-CRC |
| Central Bank of Dominican Republic (counterparty) | `0xCB002_DominicanRepublic` | tCeBM-DOP |

- **Exchange rate:** 0.058 (synthetic)
- **Amount:** 1,000,000 tCeBM-CRC → 58,000 tCeBM-DOP
- **JWT tokens:** Prefixed with `SYNTHETIC_TOKEN_` — not valid for any real system
- **Transaction hashes and block numbers:** Fabricated

**No real financial data, credentials, endpoints, or participant information is included.**

---

## Design principles (quality bar)

- **Contract-first**: specs define behavior before implementation
- **Reproducible**: artifacts must be verifiable locally (and in CI where applicable)
- **Non-sensitive by default**: samples must be safe to publish
- **Versioned**: artifacts follow semantic versioning and include changelogs
- **Security-by-default**: OWASP-minded design (input validation, authn/z boundaries, safe defaults, logging hygiene)
- **Operational clarity**: clear prerequisites, assumptions, and error semantics

---

## Artifact conventions

### Contracts
- MUST include: **version**, **breaking change notes** (CHANGELOG.md), **error model**, and **examples**
- MUST use stable, language-neutral formats (OpenAPI 3.0.3 / JSON Schema)
- MUST be scoped to a single domain (e.g., `pvp/`, `amm/`, `compliance/`)
- SHOULD be validatable with standard tools (Spectral, swagger-cli)

### Reference mocks
- MUST be **canonical**: deterministic responses aligned with the contract
- MUST include: `id`, `title`, `description`, `request`, `response`
- MUST use only synthetic data
- SHOULD be organized by scenario (`happy-path/`, `timeout-refund/`, etc.)

### Test vectors
- MUST be deterministic and minimal
- MUST include: `id`, `title`, `description`, `category`, `preconditions`, `input`, `expectedOutput`
- MUST cover: happy paths, error cases, and edge cases
- SHOULD use placeholder conventions (`{{NON_EMPTY_STRING}}`, etc.) for implementation-specific values

### Conformance tests *(planned — Session 3)*
- MUST state: what "pass" means and what is explicitly out of scope
- SHOULD be runnable in CI and locally (where feasible)

---

## How work is tracked

All work is tracked using the **GitHub Project** in this repository, with issues labeled by:

- `area:contracts`, `area:mocks`, `area:test-vectors`, `area:conformance`, `area:sandbox`, `area:docs`
- `prio:P0`, `prio:P1`, `prio:P2`
- `needs-owner` (when maintainers are seeking a contributor to lead the work)

---

## Roadmap

| Session | Topic | Status |
|---------|-------|--------|
| 1 | Toolbox charter, structure, contribution guide | ✅ Complete |
| 2 | Interface contracts + reference mocks + test vectors (PvP) | ✅ Initial artifacts ready |
| 3 | Conformance tests + CI gates (security + compatibility) | Planned |
| 4 | Sandbox/devnet guidance + sample configs + onboarding | Planned |

---

## Getting started

1. Read this charter to understand the Toolbox scope and conventions
2. Explore `contracts/pvp/` — start with the `README.md` and the OpenAPI spec
3. Walk through `mocks/pvp/happy-path/` (files 01–06) to see a complete PvP settlement
4. Review `test-vectors/pvp/` to understand the validation criteria
5. Pick an issue from the Project board, ideally tagged `good-first-issue` or `needs-owner`
6. Follow `CONTRIBUTING.md` for the contribution workflow

---

## License

Unless explicitly stated otherwise, the Toolbox follows the repository's license (Apache-2.0). Contributions must be compatible with that license.

---

## Security notes

- Apply OWASP-style thinking (e.g., input validation, least privilege, secure defaults, safe logging).
- If you discover a security vulnerability, **do not open a public issue**. Follow the repository's security reporting process (see `SECURITY.md` if present) or contact maintainers privately.
