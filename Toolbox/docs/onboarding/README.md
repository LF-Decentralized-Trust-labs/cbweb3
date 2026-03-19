# Onboarding — CBWeb3 Toolbox

Welcome to the CBWeb3 Toolbox. This page connects all the resources you need to get started as a contributor or integrator.

---

## Start here

| Step | What | Where | Time |
|------|------|-------|------|
| 1 | **Understand the Toolbox** | [Toolbox README](../../README.md) | 10 min |
| 2 | **Learn the architecture** | [Architecture Overview](../../sandbox/devnet-guide/architecture-overview.md) | 15 min |
| 3 | **Walk through the PvP flow** | [Flow Walkthrough](../../sandbox/devnet-guide/flow-walkthrough.md) | 10 min |
| 4 | **Run the mock server** | [Mock Server Setup](../../sandbox/devnet-guide/mock-server-setup.md) | 5 min |
| 5 | **Execute a PvP settlement** | [Tutorial 1: PvP against Mock](../../sandbox/tutorials/01-pvp-settlement-mock.md) | 15 min |
| 6 | **Run conformance tests** | [Tutorial 2: Validate Implementation](../../sandbox/tutorials/02-validate-implementation.md) | 10 min |

**Total: ~60 minutes** to go from zero to running the full PvP flow and understanding the Toolbox.

---

## For contributors

| Resource | Description |
|----------|-------------|
| [CONTRIBUTING.md](../../CONTRIBUTING.md) | Contribution workflow, branch naming, PR checklist |
| [Issue Templates](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/new/choose) | Structured forms for contracts, mocks, and test vectors |
| [Conformance Requirements](../../conformance/spec/conformance_requirements.md) | What "pass" means, testing levels |
| [Security Checklist](../../conformance/spec/security_checklist.md) | OWASP API Top 10 adapted to CBWeb3 |

---

## For implementers

If you are building a CBWeb3-compatible API:

1. **Start with the contract:** `Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml`
2. **Use the mocks as reference:** `Toolbox/mocks/pvp/happy-path/` (files 01-06)
3. **Validate with test vectors:** `Toolbox/test-vectors/pvp/`
4. **Run conformance tests:** Point them at your API with `CBWEB3_BASE_URL`

---

## Where to contribute

| Area | Status | Good first contribution |
|------|--------|------------------------|
| `contracts/amm/` | Vacante | Extract AMM endpoints from D5 OpenAPI spec |
| `contracts/compliance/` | Vacante | Extract compliance endpoints from D5 OpenAPI spec |
| `mocks/` | PvP complete | Add mock scenarios for AMM or compliance flows |
| `test-vectors/` | PvP complete | Add test vectors for AMM or compliance flows |
| `conformance/` | PvP complete | Add tests for new domains as contracts are created |
| `sandbox/` | New | Improve tutorials, add Besu devnet local setup |

---

## Reference documents (not in repo)

These external deliverables provide context for the Toolbox artifacts:

| Document | Content | Relevance |
|----------|---------|-----------|
| D2 - Use Cases | PvP, HTLC, AMM use cases | Defines the flows |
| D3 - Requirements | Functional/non-functional requirements | Validation criteria |
| D4 - Architecture | System architecture (Spoke/Hub topology) | Component interactions |
| D5 - Endpoints Specification | Full OpenAPI spec + Postman | Contract baseline |
| D7v2 - Design Document | Detailed technical design | Stack, interfaces, flows |
