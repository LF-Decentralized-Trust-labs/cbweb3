# Onboarding — CBWeb3 Toolbox

Welcome to the CBWeb3 Toolbox. This page connects the resources you need to get started as
a contributor or an integrator.

---

## First, decide which scenario you are in

CBWeb3 settles cross-border payments two different ways, and they share almost nothing at
the API level. Everything below branches on this choice.

| | **Scenario A** — single-ledger, spoke-to-spoke | **Scenario B** — hub-and-spoke |
|---|---|---|
| Settlement mechanism | FX agreement + a pair of dual-layer HTLCs | Escrow → bridge lock-mint → Hub AMM swap → bridge-out → residue return |
| Contract | `Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml` | `Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml` |
| Tutorial | [01 — PvP settlement](../../sandbox/tutorials/01-pvp-settlement-mock.md) | [03 — Hub swap](../../sandbox/tutorials/03-hub-swap-mock.md) |
| HTLC endpoints | 6 | **none** |
| FX agreement endpoints | 7 | **none** |

Both share one authentication surface,
`Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml`, which is byte-identical between the two
delivered gateway specifications.

---

## Start here

| Step | What | Where | Time |
|------|------|-------|------|
| 1 | **Understand the Toolbox** | [Toolbox README](../../README.md) | 10 min |
| 2 | **Learn the architecture** | [Architecture Overview](../../sandbox/devnet-guide/architecture-overview.md) | 15 min |
| 3 | **Walk through both settlement flows** | [Flow Walkthrough](../../sandbox/devnet-guide/flow-walkthrough.md) | 15 min |
| 4 | **Run the mock servers** | [Mock Server Setup](../../sandbox/devnet-guide/mock-server-setup.md) | 5 min |
| 5a | **Execute a Scenario A settlement** | [Tutorial 1: PvP against a mock](../../sandbox/tutorials/01-pvp-settlement-mock.md) | 25 min |
| 5b | **or a Scenario B settlement** | [Tutorial 3: Hub swap against a mock](../../sandbox/tutorials/03-hub-swap-mock.md) | 25 min |
| 6 | **Run the conformance tests** | [Tutorial 2: Validate an implementation](../../sandbox/tutorials/02-validate-implementation.md) | 15 min |

**Total: ~85 minutes** for one scenario end to end, or ~110 for both.

---

## The three things that most often trip people up

1. **Authentication is a cookie, not a bearer token.** The whole `/api/v1` surface is
   guarded by an `access_token` **HttpOnly cookie**. `BearerAuth` exists only on twelve
   Scenario B `/api/v2` routes, and never on the swap endpoints. Use a session that keeps a
   cookie jar. `CBWEB3_AUTH_TOKEN` no longer exists anywhere in this repository.
2. **Paths are complete.** Contract paths carry their own `/api/v1` or `/api/v2` prefix and
   the `servers:` entries are bare origins. Never append `/api/v1` to `CBWEB3_BASE_URL`.
3. **The reserve prelude is not optional.** A bank cannot lock or swap tokens it does not
   hold. Deposit → approve → exchange → escrow → approve, *then* settle. And the create half
   of that lifecycle lives on a commercial-bank gateway while the approve half lives on a
   Central Bank gateway — **no single base URL can drive the whole thing.**

---

## For contributors

| Resource | Description |
|----------|-------------|
| [CONTRIBUTING.md](../../CONTRIBUTING.md) | Contribution workflow, branch naming, PR checklist |
| [Issue Templates](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/new/choose) | Structured forms for contracts, mocks and test vectors |
| [Conformance Requirements](../../conformance/spec/conformance_requirements.md) | What "pass" means, testing levels, gateway profiles |
| [Security Checklist](../../conformance/spec/security_checklist.md) | OWASP API Top 10 adapted to CBWeb3, including the recorded finding that the platform implements **no CSRF mechanism** |
| [DIVERGENCES.md](../../DIVERGENCES.md) | Every place the platform's OpenAPI document disagrees with the gateway it ships — read this before you file a bug against a contract |

Before you open a PR, run the two gates CI runs:

```bash
python Toolbox/tools/validate_artifact_paths.py   # every mock/vector path must exist in a contract
python Toolbox/tools/verify_hashlocks.py          # every documented SHA-256 pair recomputes
```

---

## For implementers

If you are building a CBWeb3-compatible API or client:

1. **Start with the session surface:** `Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml`.
   Every integration begins here, and there are two login flows — direct, and the two-step
   PKI nonce + wallet-bind dance that commercial banks actually perform.
2. **Then your scenario's contract.** Read its `README.md` *before* the YAML: it lists the
   divergences from the platform's own document and the open questions that are not
   resolvable by guesswork.
   - Scenario A: `Toolbox/contracts/pvp/` — 28 paths, 32 operations
   - Scenario B: `Toolbox/contracts/amm/` — 52 paths, 59 operations
3. **Use the mocks as reference:** `Toolbox/mocks/{auth,pvp,amm}/`, numbered in flow order.
4. **Validate with the test vectors:** `Toolbox/test-vectors/{auth,pvp,amm}/`.
5. **Run the conformance suite** against your gateway. Set `CBWEB3_BASE_URL` to a bare
   origin, pick a `CBWEB3_AUTH_MODE` (`direct`, `pki` or `bearer`) and declare your
   `CBWEB3_PROFILE` so that routes your deployment never registered **skip** instead of
   failing.

**Assert on status codes, never on error text.** The `/api/v1` error model is
`{"error": "<free-form string>"}` with no machine-readable code.

---

## Where to contribute

| Area | Status | Good first contribution |
|------|--------|------------------------|
| `contracts/auth/`, `contracts/pvp/`, `contracts/amm/` | Published at 2.3.0 | Resolve one of the open questions listed in a contract README against a live gateway, and record the answer |
| `contracts/compliance/` | **Deferred, not cancelled** | The Scenario A compliance / governance / supervisor / oversight / onboarding surface (~42 paths). Large; coordinate with maintainers first |
| `mocks/` | auth, pvp, amm covered | Add fixtures for a flow branch that is not yet represented |
| `test-vectors/` | auth, pvp, amm covered | Add error and edge-case vectors, especially for Scenario B governance and oversight |
| `conformance/` | auth, pvp, amm covered | Add `live_only` assertions that verify value actually moved, not just that a call returned 200 |
| `sandbox/` | Three tutorials | Add a Besu devnet local setup, or a client-library walkthrough |
| `tools/` | Two CI gates | Add a gate that validates a mock's request body against its operation's `requestBody` schema |

> **`contracts/ccip/` and `contracts/privacy/` are not planned artifacts.** Earlier versions
> of this page listed them as contribution targets; no such contract was ever specified and
> neither directory has ever existed. They are not in the roadmap.

---

## Reference documents (not in this repository)

These external deliverables provide historical context.

| Document | Content | Relevance |
|----------|---------|-----------|
| D2 — Use Cases | PvP, HTLC, cross-chain interoperability use cases | Defines the flows |
| D3 — Requirements | Functional / non-functional requirements | Validation criteria |
| D4 — Architecture | System architecture (spoke / hub topology) | Component interactions |
| D5 — Endpoints Specification | Pre-delivery OpenAPI draft + Postman collection | **Historical only.** It describes a pre-delivery pilot sketch, not the delivered gateway. The Toolbox's contracts are transcribed from the delivered API Gateway v2.3.0 specifications instead |
| D7v2 — Design Document | Detailed technical design | Stack, interfaces, flows |

> **Why D5 is marked historical.** The Toolbox's first contract was extracted from D5 and
> described seven endpoints that no CBWeb3 gateway has ever served. It was deleted in the
> 2026-08 realignment. If you have a D5-derived client or Postman collection, it does not
> talk to the delivered platform.
