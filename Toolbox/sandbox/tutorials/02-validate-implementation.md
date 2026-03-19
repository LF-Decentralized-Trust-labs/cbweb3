# Tutorial 2: Validate Your Implementation with Conformance Tests

Run the CBWeb3 conformance test suite against any API implementation to verify it conforms to the PvP interface contract.

**Time:** ~10 minutes
**Prerequisites:** Python 3.9+, pip (see [prerequisites.md](../devnet-guide/prerequisites.md))

---

## Overview

The conformance tests are executable checks that verify:
- Correct HTTP status codes for each operation
- Required fields in response bodies
- Proper error handling (missing fields, invalid secrets, expired locks)

The tests can run against:
- **Prism mock server** (Level 2 — validates test correctness)
- **Your real API implementation** (Level 3 — validates conformance)

---

## Setup

### Install Python dependencies

```bash
pip install pytest requests
```

### Option A: Run against mock server

Start Prism in a separate terminal:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010
```

The tests will use `http://localhost:4010` by default.

### Option B: Run against your implementation

Set the environment variable to point to your API:

```bash
export CBWEB3_BASE_URL=https://your-api.example.com
export CBWEB3_AUTH_TOKEN=your-real-jwt-token
```

---

## Run the tests

### All tests

```bash
cd Toolbox/conformance
pytest -v
```

### Only happy-path tests

```bash
pytest -m happy_path -v
```

### Only error tests

```bash
pytest -m error -v
```

### Only FX Agreement tests

```bash
pytest -m fx_agreement -v
```

### Only HTLC tests

```bash
pytest -m htlc -v
```

### List tests without running them

```bash
pytest --co -v
```

---

## Interpreting results

### All passing

```
test_pvp_fx_agreement.py::TestCreateFxAgreement::test_happy_path PASSED
test_pvp_fx_agreement.py::TestAcceptFxAgreement::test_happy_path PASSED
test_pvp_htlc.py::TestHtlcLock::test_happy_path PASSED
...
========================= 16 passed in 2.34s =========================
```

Your implementation conforms to the PvP contract v0.1.0.

### Some failing

```
test_pvp_htlc.py::TestHtlcSettleInvalidSecret::test_error FAILED

    assert resp.status_code == 400
    AssertionError: assert 500 == 400
```

This means your implementation returned `500` instead of the expected `400` for an invalid secret. Check the test vector `htlc-err-01` in `test-vectors/pvp/pvp_htlc_vectors.json` for the exact specification.

### Note on Prism limitations

When running against Prism (Level 2), some error and edge-case tests may fail because Prism always returns the first `2xx` example by default. This is expected behavior — those tests are designed for Level 3 (real implementations).

To see which tests are expected to pass against Prism:

```bash
pytest -m happy_path -v
```

---

## Test coverage

| Domain | Test methods | Happy path | Error | Edge case |
|--------|-------------|------------|-------|-----------|
| FX Agreement | 7 | 2 | 3 | 0 |
| HTLC | 9 | 3 | 4 | 1 |
| **Total** | **16** | **5** | **7** | **1** |

---

## Configuration reference

| Variable | Default | Description |
|----------|---------|-------------|
| `CBWEB3_BASE_URL` | `http://localhost:4010` | Target API base URL |
| `CBWEB3_AUTH_TOKEN` | Synthetic token | Bearer token for authentication |

You can also pass `--base-url` as a CLI option:

```bash
pytest --base-url=http://localhost:8080 -v
```

---

## Contributing new test vectors

If you identify a missing test case:

1. Add a new vector to the appropriate file in `test-vectors/pvp/`
2. Write a corresponding test method in `conformance/tests/pvp/`
3. Follow the format documented in the [Toolbox README](../../README.md#3-test-vectors)
4. Submit a PR using the [test-vector issue template](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/new?template=test-vector.yml)

---

## Next steps

- **Explore the test code:** `Toolbox/conformance/tests/pvp/`
- **Read the conformance spec:** `Toolbox/conformance/spec/conformance_requirements.md`
- **Review the security checklist:** `Toolbox/conformance/spec/security_checklist.md`
