# Sandbox — CBWeb3 Toolbox

Get started with the CBWeb3 Toolbox in **30-60 minutes** without touching real infrastructure or sensitive data.

This sandbox provides everything you need to understand, explore, and validate the CBWeb3 PvP Settlement flow using only synthetic data and a local mock server.

---

## What you'll be able to do

1. **Understand the architecture** — Learn how the Spoke/Hub topology, HTLC settlement, and interoperability layers work
2. **Run a mock API server** — Serve the PvP interface contract locally using Prism
3. **Walk through a complete PvP settlement** — Execute all 6 steps of a cross-border FX settlement using curl
4. **Validate an implementation** — Run the conformance test suite against any API endpoint

---

## Quick start

### 1. Prerequisites

See [devnet-guide/prerequisites.md](devnet-guide/prerequisites.md) for detailed requirements. In short:

- **Node.js 18+** (for Prism mock server)
- **Python 3.9+** (for conformance tests)
- **curl** or **httpie** (for API calls)

### 2. Start the mock server

```bash
# From the repository root
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010
```

See [devnet-guide/mock-server-setup.md](devnet-guide/mock-server-setup.md) for configuration options.

### 3. Run the PvP tutorial

Follow [tutorials/01-pvp-settlement-mock.md](tutorials/01-pvp-settlement-mock.md) to execute a complete PvP settlement against the mock server — all 6 steps with copy-paste curl commands.

### 4. Run conformance tests

Follow [tutorials/02-validate-implementation.md](tutorials/02-validate-implementation.md) to run the automated test suite.

```bash
cd Toolbox/conformance
pip install pytest requests
pytest -v
```

---

## Sandbox structure

```text
sandbox/
├── README.md                          ← You are here
├── devnet-guide/
│   ├── prerequisites.md               ← Software requirements
│   ├── mock-server-setup.md           ← How to run Prism mock server
│   ├── architecture-overview.md       ← Simplified system architecture
│   └── flow-walkthrough.md            ← PvP settlement flow explained
├── sample-configs/
│   ├── .env.example                   ← Environment variables (synthetic)
│   └── prism-config.md                ← Prism mock server configuration
└── tutorials/
    ├── 01-pvp-settlement-mock.md      ← Tutorial: PvP against mock server
    └── 02-validate-implementation.md  ← Tutorial: Conformance tests
```

---

## What is NOT included

This sandbox intentionally excludes:
- Private keys or wallet credentials
- Real network endpoints or node addresses
- Production configurations or deployment scripts
- Sensitive institutional data

All data is **synthetic**. See the [Toolbox README](../README.md#synthetic-data-policy) for the full synthetic data policy.

---

## Related resources

| Resource | Location | Description |
|----------|----------|-------------|
| PvP interface contract | `Toolbox/contracts/pvp/` | OpenAPI spec with 7 endpoints |
| Reference mocks | `Toolbox/mocks/pvp/` | 8 JSON fixtures (happy-path + timeout) |
| Test vectors | `Toolbox/test-vectors/pvp/` | 13 deterministic test cases |
| Conformance tests | `Toolbox/conformance/` | 16 pytest test methods |
| Contribution guide | `Toolbox/CONTRIBUTING.md` | How to contribute artifacts |
