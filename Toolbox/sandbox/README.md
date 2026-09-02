# Sandbox — CBWeb3 Toolbox

Get from zero to a working CBWeb3 integration in **30–60 minutes**, without touching real
infrastructure, real credentials or real data.

Everything here runs against a local [Prism](https://stoplight.io/open-source/prism) mock
of the Toolbox interface contracts. The contracts mirror **CBWeb3 API Gateway v2.3.0** as
delivered, so the paths, field names and status codes you exercise here are the ones a real
gateway serves.

---

## Two scenarios, three contracts

CBWeb3 settles cross-border payments two different ways, and they share almost nothing at
the API level. Pick the one you are integrating with before you start.

| | **Scenario A** — single-ledger, spoke-to-spoke | **Scenario B** — hub-and-spoke |
|---|---|---|
| Settlement mechanism | FX agreement + a pair of dual-layer HTLCs | Escrow → bridge lock-mint → Hub AMM swap → bridge-out → residue return |
| Contract | `Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml` | `Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml` |
| Tutorial | [01 — PvP settlement](tutorials/01-pvp-settlement-mock.md) | [03 — Hub swap](tutorials/03-hub-swap-mock.md) |
| HTLC endpoints | 6 | **none — Scenario B has no HTLC surface at all** |
| FX agreement endpoints | 7 | **none** |

Both scenarios share one authentication surface,
`Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml`, which is byte-identical between the two
delivered gateway specifications.

---

## What you'll be able to do

1. **Understand the architecture** — spoke/hub topology, the two settlement mechanisms, and
   where the Toolbox artifacts sit relative to the delivered platform
2. **Run mock API servers** — one Prism instance per contract, on ports 4010 / 4011 / 4012
3. **Walk a complete Scenario A settlement** — reserve prelude, FX agreement, both HTLC
   legs, settlement and the refund branch, with copy-paste `curl`
4. **Walk a complete Scenario B settlement** — discovery, reserve prelude, quote, swap, and
   the residue check that turns an API test into a settlement test
5. **Validate an implementation** — run the conformance suite against a mock or against your
   own gateway

---

## Quick start

### 1. Prerequisites

See [devnet-guide/prerequisites.md](devnet-guide/prerequisites.md). In short:

- **Node.js 18+** (Prism mock server, Spectral linter)
- **Python 3.9+** (conformance tests)
- **curl** or **httpie**, and **jq** for readable output

### 2. Start the mock servers

One instance per contract, from the repository root:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &
```

Start only the one you need — the tutorials say which. See
[devnet-guide/mock-server-setup.md](devnet-guide/mock-server-setup.md) for options.

> **Paths are complete.** Contract paths carry their own `/api/v1` or `/api/v2` prefix and
> the `servers:` entries are bare origins, so Prism serves exactly what a gateway serves.
> `http://localhost:4010/api/v1/htlc/lock` — nothing is stripped, nothing is re-prefixed,
> and `CBWEB3_BASE_URL` must **not** carry a path suffix.

### 3. Authenticate

Every settlement call needs an `access_token` **HttpOnly cookie**. There is no bearer token
on the `/api/v1` surface. Against Prism, any cookie value works:

```bash
curl -s http://localhost:4010/api/v1/token/balance \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' | jq .
```

Against a real gateway you obtain the cookie from `POST /api/v1/auth/login` — and if you are
a commercial bank, from the two-step PKI nonce flow that follows it. See
[tutorials/01](tutorials/01-pvp-settlement-mock.md) step 0.

### 4. Run a tutorial

| Tutorial | Scenario | Time |
|---|---|---|
| [01 — PvP settlement against a mock](tutorials/01-pvp-settlement-mock.md) | A | ~20 min |
| [02 — Validate an implementation](tutorials/02-validate-implementation.md) | both | ~10 min |
| [03 — Hub cross-currency swap against a mock](tutorials/03-hub-swap-mock.md) | B | ~20 min |

---

## Sandbox structure

```text
sandbox/
├── README.md                          ← You are here
├── devnet-guide/
│   ├── prerequisites.md               ← Software requirements
│   ├── mock-server-setup.md           ← How to run the Prism mock servers
│   ├── architecture-overview.md       ← System architecture, both scenarios
│   └── flow-walkthrough.md            ← Both settlement flows, explained
├── sample-configs/
│   ├── .env.example                   ← Environment variables (synthetic)
│   └── prism-config.md                ← Prism mock server configuration
└── tutorials/
    ├── 01-pvp-settlement-mock.md      ← Scenario A: FX agreement + HTLC
    ├── 02-validate-implementation.md  ← Conformance tests
    └── 03-hub-swap-mock.md            ← Scenario B: bridge + Hub AMM swap
```

---

## What is NOT included

This sandbox intentionally excludes:

- Private keys, participant certificates or wallet credentials
- Real network endpoints or node addresses
- Production configurations or deployment scripts
- Sensitive institutional data
- Any `/internal/*` endpoint. The relay-facing surface is authenticated by a relay
  credential, is never client-callable, and is deliberately absent from every contract.

All data is **synthetic**. See the [Toolbox README](../README.md#synthetic-data-policy) for
the full policy.

---

## Related resources

| Resource | Location | Description |
|----------|----------|-------------|
| Auth contract (shared) | `Toolbox/contracts/auth/` | 8 paths / 8 operations — login, PKI wallet bind, refresh, logout, session introspection |
| PvP contract (Scenario A) | `Toolbox/contracts/pvp/` | 28 paths / 32 operations — token, reserve lifecycle, FX agreement, HTLC |
| AMM contract (Scenario B) | `Toolbox/contracts/amm/` | 52 paths / 59 operations — token, reserve lifecycle, AMM, bridge, hub registries, governance, oversight |
| Reference mocks | `Toolbox/mocks/` | Canonical request/response fixtures, one directory per domain |
| Test vectors | `Toolbox/test-vectors/` | Deterministic input → expected output fixtures |
| Conformance tests | `Toolbox/conformance/` | Executable compliance checks |
| Contribution guide | `Toolbox/CONTRIBUTING.md` | How to contribute artifacts |
