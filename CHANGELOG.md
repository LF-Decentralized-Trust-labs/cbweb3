# Changelog

All notable changes to this repository are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Component-level detail for the platform is kept in `platform/CHANGELOG.md`.

## [Unreleased]

> **Toolbox realignment to CBWeb3 API Gateway v2.3.0.** The Toolbox's published interface
> contract described an API that **was never implemented on any CBWeb3 gateway**. All seven
> of its paths, its authentication model, its error model and its field casing were wrong;
> it had been drafted from a pre-delivery pilot sketch (deliverable D5) rather than from the
> delivered platform. The delivered gateway serves prefixed paths
> (`/api/v1/payments/fx/agreements`, `/api/v1/htlc/lock`, …) behind an `access_token`
> HttpOnly cookie, across **two structurally different settlement scenarios**. Everything
> downstream of that contract — mocks, test vectors, conformance tests, sandbox tutorials,
> CI — inherited the same fiction and shipped green for four sessions, because nothing in CI
> cross-checked an artifact against the specification it claimed to implement.
>
> The realignment replaces the contract layer wholesale and rebuilds every artifact on top
> of it. **The platform surface wins**: the delivered gateway is in production use by
> central banks and commercial banks, and no change was requested of the vendor. All work is
> Toolbox-side.
>
> **There is no backward compatibility, and none was attempted.** The Toolbox had no
> consumers: no downstream project, client or deployment referenced the deleted contract. No
> deprecation shims, no path aliases and no dual-publication of the old paths were added.
> Anything built against the old contract could not have worked against a real gateway in
> the first place.

### Added

- `SECURITY.md` — responsible disclosure policy and coordinated disclosure timeline
- `MAINTAINERS.md` — authoritative maintainer list referenced by `CONTRIBUTING.md`
- `NOTICE` — Apache-2.0 attribution notice
- `AGENTS.md` — project conventions for AI coding tools, per the draft LFDT AI Guidelines
- `CHANGELOG.md` — this file
- Pull request template and Dependabot configuration
- Working documentation site: the `Documentation` workflow builds `mkdocs --strict` on
  every pull request and publishes to GitHub Pages from `main`
- `docs/research-papers/index.md` — index of the commissioned research
- `Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml` — the shared authentication and session
  surface (8 paths / 8 operations, 4 public). It is published once because the whole auth
  block is byte-identical between the two delivered gateway specifications, which removes
  the only drift risk on the thing every integration starts with
- `Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml` — Scenario B, the hub-and-spoke
  International Hub surface (52 paths / 59 operations, 12 public): reserve lifecycle, AMM
  quote and swap, bridge lock-mint / burn-unlock, liquidity provisioning, hub registries,
  and the v2 circuit breaker and oversight surfaces. Scenario B was previously undocumented
  by the Toolbox entirely — half the delivered platform had no published interface
- `Toolbox/sandbox/tutorials/03-hub-swap-mock.md` — a Scenario B walkthrough, including the
  residue and hub-reconciliation assertions that distinguish a settlement test from an API
  test
- `Toolbox/tools/validate_artifact_paths.py` — CI gate: every mock and test-vector `path`
  must resolve to a real path **and method** in one of the three contracts
- `Toolbox/tools/verify_hashlocks.py` — CI gate: every documented secret / hash-lock pair is
  recomputed
- A manual `workflow_dispatch` drift check that fetches a live gateway's self-served
  `GET /openapi.yaml` and diffs its path set against the published contracts
- `Toolbox/DIVERGENCES.md` — the canonical register of every place the platform's published
  OpenAPI document disagrees with the gateway it ships, with source citations. The mocks, the
  vectors, the conformance suite and the contracts all reference it instead of each keeping a
  private table that drifts

### Fixed

- The `Documentation` workflow was an empty file; the site had never been published
- `overrides/main.html` was empty, which suppressed the Material theme and would have
  rendered blank pages
- Broken theme asset references in `mkdocs.yml`: the logo pointed at
  `assets/CBWeb3Negros.png` while the file is `assets/CBweb3Negros.png`, and the favicon
  pointed at `assets/project-icon.png`, which does not exist
- Site navigation was almost entirely commented out; only the landing page was reachable
- `docs/index.md` referred to LACChain/LACNet where the project now uses LNet
- `.gitignore` did not exclude Python bytecode caches, and listed `localdocs/` twice
- Toolbox CI ran only on pull requests, so `main` could drift without verification
- **A false SHA-256 digest published as a verifiable reference value.** Every Toolbox
  artifact claimed `SHA-256("cbweb3-test-secret-2026")` was
  `0x7f83b165…9069` and instructed implementers to verify it. It is not the SHA-256 of that
  string, and could not be identified as the digest of any obvious candidate; the string is
  not valid hex either. Any integrator following the instruction got a mismatch. Replaced
  with a pair anyone can recompute in one line (`printf 'hello' | sha256sum` →
  `2cf24dba…9824`), chosen by the Toolbox rather than taken from the platform, and now
  enforced by a CI gate
- Toolbox CI started Prism against the deleted contract filename and smoke-tested
  `POST /fx/agreement` with an `Authorization: Bearer` header — a path and a credential
  neither gateway has ever served. Because Prism was serving the same fiction the contract
  described, the gate passed. It now starts one Prism instance per contract (4010 pvp /
  4011 amm / 4012 auth), probes real paths with a `Cookie:` credential, and asserts that a
  request **without** the cookie is rejected with `401`
- The Toolbox artifact table in this repository's README claimed 16 conformance test
  methods; the real count at the time was 17. Counts are now derived from the shipped files

### Changed

- `mkdocs.yml` no longer loads the `mike` versioning plugin. No versioned documentation
  has ever been deployed, so the version selector would have resolved to nothing.
  Reinstate it together with a versioned deployment when the first release is cut.
- **`Toolbox/contracts/pvp/` now describes Scenario A as delivered** — 28 paths / 32
  operations, none of them public: the token surface, the full reserve lifecycle
  (deposit → approve → fiat-exchange → escrow → approve), the FX agreement lifecycle and
  the dual-layer HTLC. The reserve lifecycle is included because a bank cannot HTLC-lock
  tokens it does not hold, and is **deliberately duplicated** into the Scenario B contract
  because the two gateways genuinely differ (`deposits/fiat-exchange` versus
  `deposits/exchange`, and neither serves both). A single shared document would have had to
  declare both and would therefore have described a gateway that does not exist
- **Authentication is now `CookieAuth`** (`access_token`, HttpOnly, `SameSite=Strict`)
  across the whole `/api/v1` surface, replacing the assumed `Authorization: Bearer`.
  `BearerAuth` is declared only in the Scenario B contract, and only as an alternative on
  the twelve `/api/v2` `RequireAnyAuth` routes; the two cross-currency swap endpoints are
  cookie-only. **No CSRF mechanism is published, because the platform implements none** —
  `SameSite=Strict` is the sole mitigation, and that absence is recorded as a finding in the
  security checklist rather than papered over
- Conformance fixtures use a session-scoped `requests.Session` cookie jar with four
  authentication modes (`mock`, `direct`, `pki`, `bearer`), plus two orthogonal marker tiers
  (`mock_safe` versus `live_only`) and a gateway-profile fixture, so a `404` from a route a
  deployment never registered **skips** rather than fails
- Every mock, test vector, sandbox tutorial, devnet guide, sample config and onboarding page
  was rewritten against the new contracts. Endpoint paths now carry their own `/api/v1` or
  `/api/v2` prefix, which removes the three-way spec/mock/Prism path mismatch the old docs
  papered over with warning notes
- Contract versions now track **the gateway version they describe** (2.3.0), in both the
  filename and `info.version`, instead of an independent `0.x` series

### Removed

- **`Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml` is deleted, not deprecated.** It
  described seven flat paths (`/fx/agreement`, `/htlc/lock`, …) that no CBWeb3 gateway has
  ever served. It is not reinstated under any alias
- `CBWEB3_AUTH_TOKEN` and every synthetic bearer token, repository-wide. The credential is a
  cookie
- The invented error codes `HTLC_HASH_MISMATCH` and `HTLC_EXPIRED`, asserted by exact string
  in the old conformance tests, vectors and mocks. The `/api/v1` error model is
  `{"error": "<free-form string>"}` with **no machine-readable code**; conformance now
  asserts on status codes only
- References to `contracts/ccip/` and `contracts/privacy/` as contribution targets. Neither
  contract was ever specified and neither directory has ever existed
