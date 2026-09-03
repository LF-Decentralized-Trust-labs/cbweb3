<!--
SYNC IMPACT REPORT
==================
Version change: [template] → 1.0.0 (initial ratification — all placeholders resolved)

Modified principles: N/A (initial fill)

Added sections:
  - Core Principles (6 principles)
  - Technology Stack Constraints
  - Development Workflow
  - Governance

Removed sections: N/A

Templates checked:
  - .specify/templates/plan-template.md         ✅ aligned (Constitution Check section present)
  - .specify/templates/spec-template.md         ✅ aligned (user stories, requirements, success criteria)
  - .specify/templates/tasks-template.md        ✅ aligned (phased delivery, test-first discipline)
  - .specify/templates/constitution-template.md ✅ source template

Follow-up TODOs: None — all fields resolved from repo context.

---

AMENDMENT 1.0.4 — 2026-06-06
=============================
Version change: 1.0.3 → 1.0.4 (PATCH — resolve IBFT 2.0 vs QBFT ambiguity in Technology Stack
Constraints; QBFT is the sole consensus protocol, IBFT 2.0 is explicitly excluded)

Modified sections:
  - Technology Stack Constraints / Blockchain: expanded the Besu line to state that IBFT 2.0 is
    NOT used and that any "IBFT 2.0/QBFT" references in external documents (e.g., D7 §2.1) are
    resolved in favour of QBFT. No change to runtime behaviour — QBFT was already the stated
    consensus algorithm.

Trigger: editorial review noted that D7 §2.1 lists "IBFT 2.0/QBFT" ambiguously, which had been
a prior source of inconsistency. The constitution is the authoritative source; the disambiguation
prevents future confusion when cross-referencing D7.

---

AMENDMENT 1.0.3 — 2026-06-06
=============================
Version change: 1.0.2 → 1.0.3 (PATCH — technical accuracy in Principle III: add explicit Scenario B
relay verification clause; "lock and reveal" semantics already scoped to Scenario A, but no
corresponding relay verification rule existed for Scenario B)

Modified principles:
  - III. Atomic Settlement Guarantee: split the single relay-verification sentence into two
    scenario-scoped clauses. Scenario A retains "Spoke lock event + Hub secret-reveal event".
    Scenario B gains "Spoke lock event → Hub mint, Hub burn event → Spoke unlock" plus the
    existing circuit-breaker validation requirement. No change to atomicity constraints or
    timeout/refund requirements.

Trigger: editorial review noted that the relay verification rule described HTLC lock+reveal
semantics that apply only to Scenario A's dual-layer HTLC bridge. Scenario B's relay operates on
lock→mint / burn→unlock events; grouping both under the same verification clause was
technically incorrect and left Scenario B's relay requirements underspecified.

---

AMENDMENT 1.0.2 — 2026-06-06
=============================
Version change: 1.0.1 → 1.0.2 (PATCH — technical accuracy in Principle II: distinguish ZKP-based
ZetoToken from notary-based NotoToken; remove incorrect "ZKP-based" label applied to NotoToken)

Modified principles:
  - II. Privacy by Design: replaced "ZKP-based privacy tokens (Paladin/Zeto domain)" with
    "privacy-preserving tokens from the Paladin domain"; expanded the permitted-mechanisms sentence
    to name each contract with its correct privacy model — `ZetoToken` (Zeto domain, ZKP-based)
    and `NotoToken` (Noto domain, notarized/notary-enforced confidentiality). No change to which
    contracts are permitted or to any other constraint.

Trigger: editorial review identified that grouping NotoToken under the "ZKP-based" banner is
factually wrong — Noto's confidentiality is enforced by a designated notary, not by zero-knowledge
proofs. The distinction carries material privacy-model implications.

---

AMENDMENT 1.0.1 — 2026-06-06
=============================
Version change: 1.0.0 → 1.0.1 (PATCH — wording clarification, no semantic change to governance)

Modified principles:
  - III. Atomic Settlement Guarantee: replaced imprecise "AMM-based settlement via `FXAgreement`
    contracts" with a factually accurate two-layer description of Scenario B's settlement model:
    `FXAgreement` (bilateral pre-trade registry) + `AutomatedMarketMaker` (constant-product AMM) +
    `LiquidityCommitRegistry` (commit-reveal LP provisioning). Relay auditability requirements
    now distinguish Scenario A (HTLC events) from Scenario B (AMM/circuit-breaker events).
  - VI. Observability and Auditability: generalised "full HTLC lifecycle" to cover both scenario
    settlement models explicitly.

Trigger: code audit confirmed `FXAgreement` is a deal registry, not an AMM; AMM logic lives in
`AutomatedMarketMaker.sol` and `LiquidityCommitRegistry.sol`.
-->

# CBWeb3 Platform Constitution

## Core Principles

### I. Scenario-Scoped Independence

Each scenario (e.g., Scenario A — Enhanced Correspondent Banking, Scenario B — International Hub)
MUST be entirely self-contained: independent smart contracts, microservices, infrastructure
definitions, and deployment tooling housed under its own top-level directory (e.g., `scenario-a/`,
`scenario-b/`). Cross-scenario code sharing is PROHIBITED except through an explicitly versioned
shared library with its own release lifecycle. A scenario MUST be startable and demonstrable
without any other scenario being present or running.

**Rationale**: Scenario isolation prevents regressions across independent research contexts,
enables parallel development tracks, and ensures each scenario can be archived or deprecated
without affecting others.

### II. Privacy by Design

All inter-bank value transfers MUST use privacy-preserving tokens from the Paladin domain. PII,
transaction amounts, and counterparty identities MUST NOT appear in plaintext on-chain. Two
mechanisms are permitted and are the ONLY permitted on-chain value representations in production
flows: `ZetoToken` (Paladin/Zeto domain — ZKP-based, cryptographic privacy enforced on-chain) and
`NotoToken` (Paladin/Noto domain — notarized, confidentiality enforced by a designated notary).
Central bank minting via `tCeBM` (ERC-20 CBDC) is acceptable for reserve-layer issuance only and
MUST be isolated from retail settlement paths.

**Rationale**: The platform models sovereign CBDC infrastructure. Plaintext on-chain data would
violate the confidentiality requirements of any real-world central bank deployment.

### III. Atomic Settlement Guarantee

Cross-network transfers MUST be atomic. HTLC (dual-layer Hash Time-Locked Contract) is the
canonical mechanism for Scenario A. For Scenario B, atomicity is achieved through a two-layer
model: `FXAgreement` (bilateral pre-trade registry: propose → accept → settle state machine,
governance-mediated) coordinates deal terms, while `AutomatedMarketMaker` (constant-product AMM,
x*y=k) executes the actual token swap; `LiquidityCommitRegistry` handles sovereign CB liquidity
provisioning via a commit-reveal protocol. Partial settlement states — where one spoke has settled
but the other has not — are FORBIDDEN in production code paths. For Scenario A, the relay MUST
verify the Spoke lock event and the Hub secret-reveal event before settlement is declared complete.
For Scenario B, the relay MUST verify the Spoke lock event before triggering Hub mint, and the Hub
burn event before authorizing Spoke unlock; additionally, the circuit breaker (1-of-N pause, 2-of-N
resume) MUST be validated before any swap executes.
Timeout and refund paths MUST be tested for both mechanisms.

**Rationale**: A failed atomic swap that leaves one party's funds locked without recourse
constitutes a critical financial defect. Atomicity is the core safety property of the platform.

### IV. Compliance Gate Before Participation

No entity (commercial bank, central bank, or counterparty) MAY submit or receive a payment
transaction without first passing: (1) on-chain `IdentityRegistry` registration, (2) Onboarding/AML
compliance approval recorded in the `Compliance` service, and (3) a valid Keycloak OIDC session
for all API interactions. These three gates MUST be enforced at the API gateway layer and MUST NOT
be bypassable by any internal service-to-service call. The compliance state MUST be re-checked at
payment initiation, not only at onboarding time.

**Rationale**: The platform models regulated financial infrastructure. Bypassing identity or
AML checks, even in test flows, normalises patterns that would constitute regulatory violations
in production.

### V. Test-First at Every Layer

Testing discipline is NON-NEGOTIABLE and applies to all layers:

- **Smart contracts**: Foundry tests MUST exist and pass before any contract is deployed to a
  spoke network. New functions require new tests.
- **Backend services**: Unit tests and integration tests MUST be written before the feature
  implementation is considered complete. The test MUST fail before the implementation is written.
- **End-to-end**: Each scenario MUST have an E2E test suite (in `tests/`) that exercises the
  full cross-spoke flow before the scenario can be declared "runnable" in documentation.
- **Performance**: Scenarios advertised as production-grade MUST have a passing performance
  baseline test (in `tests/performance/`).

The Red-Green-Refactor cycle is strictly enforced: write failing test → implement to pass →
refactor without breaking.

**Rationale**: The platform is research infrastructure relied upon to validate CBDC
interoperability designs. A broken scenario with passing tests is a documentation failure;
a passing scenario with no tests is an untested claim.

### VI. Observability and Auditability

All backend microservices MUST emit structured JSON logs to stdout. Log entries MUST include:
request/correlation ID, service name, severity, and timestamp in ISO-8601 format. gRPC services
MUST propagate distributed trace context (OpenTelemetry) across service boundaries. The interop
relay MUST log every cross-spoke event with enough detail to reconstruct the full settlement
lifecycle from logs alone: for Scenario A this means lock detected, secret revealed, and
settlement submitted; for Scenario B this means commit registered, pair matched, swap executed,
and circuit-breaker state transitions. Silent failures —
where an error is swallowed without a log entry — are PROHIBITED.

**Rationale**: Cross-spoke flows span multiple independent networks and services. Without
end-to-end auditability, debugging settlement failures or compliance incidents is impractical.

## Technology Stack Constraints

The following technology choices are fixed for all scenarios unless explicitly superseded by a
constitution amendment with a migration plan:

- **Blockchain**: Hyperledger Besu with **QBFT** consensus, one network per spoke. IBFT 2.0 is
  explicitly NOT used; any reference to "IBFT 2.0/QBFT" in external documents (e.g., D7) is
  resolved in favour of QBFT.
- **Smart contracts**: Solidity, compiled and tested with Foundry (`forge`).
- **Privacy layer**: Paladin Core with Zeto Domain (ZKP) and Noto Domain.
- **Backend**: Go microservices communicating intra-entity via gRPC, inter-entity via on-chain
  contracts and the relay. The REST API Gateway is the sole external-facing entry point per entity.
- **Identity and auth**: Keycloak (OIDC/OAuth2) for API authentication. On-chain `IdentityRegistry`
  for participant identity. PKI certificates issued by central bank CAs.
- **Frontend**: React monorepo (Turborepo). Apps: `bank`, `governance`, `supervisor`, `treasury`,
  `noc`, `dispatcher`.
- **Infrastructure**: Docker Compose per scenario. Postgres for service persistence.
- **Testing**: Foundry (contracts), `go test` (services), Playwright or equivalent (E2E frontend).

Introducing a new runtime dependency outside this stack requires a documented justification
added to the implementing PR and referenced in the affected scenario's README.

## Development Workflow

- **Scenario-first**: All work MUST be scoped to a specific scenario directory. A PR that touches
  multiple scenarios simultaneously MUST justify the cross-scenario change explicitly.
- **Branch strategy**: Feature branches target `develop` (or a scenario-specific integration
  branch). Production scenarios merge to `main` via PR only.
- **PR requirements**: Every PR MUST include a description of the change, a test plan, and a
  reference to any affected scenario documentation. PRs that remove compliance or security checks
  require explicit approval from the project lead.
- **Constitution Check**: Every implementation plan (`plan.md`) MUST include a Constitution Check
  section that validates compliance with all six Core Principles before Phase 1 begins.
- **Complexity justification**: Any deviation from established architectural patterns (e.g.,
  adding a fourth project layer, introducing a shared DB across entities) MUST be recorded in the
  plan's Complexity Tracking table with rationale and rejected simpler alternatives.
- **Documentation currency**: The scenario's `README.md` MUST accurately reflect implementation
  status (Fully implemented / In progress / Planned) at time of PR merge.

## Governance

This constitution supersedes all other project practices, guidelines, and README-level conventions.
Where a conflict exists, the constitution takes precedence.

**Amendments** require: (1) a written proposal describing the change, motivation, and impact on
existing scenarios; (2) a version increment per the policy below; (3) updates to any affected
templates in `.specify/templates/`; (4) a migration plan if the amendment introduces a
backward-incompatible constraint.

**Versioning policy**:
- MAJOR: Removal or redefinition of a Core Principle; backward-incompatible governance change.
- MINOR: New principle, new mandatory section, or material expansion of existing guidance.
- PATCH: Wording clarification, typo fix, or non-semantic refinement.

**Compliance review**: All PRs and implementation plans MUST verify adherence to Core Principles
I–VI before merging. The Constitution Check section in `plan-template.md` is the primary
enforcement mechanism.

**Runtime guidance**: See `scenario-a/README.md` (and equivalents per scenario) for
day-to-day operational guidance. This constitution governs architecture and process; scenario
READMEs govern setup and execution.

**Version**: 1.0.4 | **Ratified**: 2026-06-05 | **Last Amended**: 2026-06-06
