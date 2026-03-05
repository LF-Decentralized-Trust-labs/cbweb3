# Conformance Tests — CBWeb3 Toolbox

Executable tests that verify an implementation conforms to the CBWeb3 Toolbox interface contracts.

## Quick start

### Prerequisites
- Python 3.9+
- Node.js 18+ (for Prism mock server)

### Install dependencies
```bash
pip install pytest requests
```

### Run against Prism mock server (Level 2)
```bash
# Terminal 1: start mock server
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010

# Terminal 2: run tests
cd Toolbox/conformance
pytest -v
```

### Run against a real implementation (Level 3)
```bash
export CBWEB3_BASE_URL=https://your-deployment.example.com
export CBWEB3_AUTH_TOKEN=your-jwt-token
cd Toolbox/conformance
pytest -v
```

## Test structure

```
conformance/
├── spec/
│   ├── conformance_requirements.md   ← What "pass" means, scope, levels
│   └── security_checklist.md         ← OWASP API Top 10 adapted to CBWeb3
├── tests/
│   ├── conftest.py                   ← Shared fixtures (base_url, auth_headers)
│   └── pvp/
│       ├── test_pvp_fx_agreement.py  ← FX Agreement tests (5 vectors)
│       └── test_pvp_htlc.py         ← HTLC tests (8 vectors)
├── pytest.ini                        ← pytest configuration
└── README.md                         ← This file
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CBWEB3_BASE_URL` | `http://localhost:4010` | Target API base URL |
| `CBWEB3_AUTH_TOKEN` | Synthetic token | Bearer token for authentication |

You can also use `--base-url` as a pytest CLI option:
```bash
pytest --base-url=http://localhost:8080 -v
```

## Running specific test categories

```bash
# Only happy-path tests
pytest -m happy_path -v

# Only error tests
pytest -m error -v

# Only FX Agreement tests
pytest -m fx_agreement -v

# Only HTLC tests
pytest -m htlc -v
```

## Coverage

| Domain | Vectors | Happy path | Error | Edge case |
|--------|---------|------------|-------|-----------|
| FX Agreement | 5 | 2 | 3 | 0 |
| HTLC | 8 | 3 | 4 | 1 |
| **Total** | **13** | **5** | **7** | **1** |

## Limitations

- **Level 2 (Prism):** Prism returns OpenAPI examples for all requests. Error-path and edge-case tests may not produce the expected error status codes against Prism. These tests are designed for Level 3 (real implementation).
- **Authentication:** Tests use a synthetic Bearer token. Real JWT validation is not tested.
- **State isolation:** Tests create their own preconditions (agreements, locks) but do not clean up. Running against a stateful implementation may accumulate test data.
