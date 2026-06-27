# cbweb3-platform

Research platform for CBDC interoperability. Two independent scenarios live side by side:

- `scenario-a/` — Enhanced Correspondent Banking (dual-layer HTLC)
- `scenario-b/` — International Hub (FXAgreement + AMM + LiquidityCommitRegistry, with relay + circuit breaker)

Treat them as separate products. Do not share code across them except via an explicitly versioned shared library.

## Authoritative architecture rules

Architectural and process rules live in `.specify/memory/constitution.md` (currently v1.0.4). Read it before non-trivial changes. Short version of the load-bearing rules:

- **Scenario isolation.** Never reach across `scenario-a/` ↔ `scenario-b/`. A PR touching both needs explicit justification.
- **Privacy.** Inter-bank value transfers use `ZetoToken` (Paladin/Zeto, ZKP) or `NotoToken` (Paladin/Noto, notary). `tCeBM` is reserve-layer only. No plaintext PII or amounts on-chain.
- **Atomicity.** Scenario A = HTLC lock + secret reveal. Scenario B = lock → mint, burn → unlock, with circuit-breaker (1-of-N pause, 2-of-N resume) validated before swaps. Partial settlement is forbidden in production paths; timeout/refund paths must be tested.
- **Compliance gate at the API gateway.** IdentityRegistry + Compliance service + Keycloak OIDC. Never bypass in service-to-service calls. Re-check at payment initiation, not just onboarding.
- **Test-first, every layer.** Foundry for contracts, `go test` for services, E2E suite per scenario. Failing test before implementation.
- **Observability.** Structured JSON logs to stdout (request ID, service, severity, ISO-8601 ts). OTel trace context across gRPC. Silent error swallowing is prohibited. The relay must log the full settlement lifecycle.

## Stack

- Hyperledger Besu 25.8.0 with **QBFT** consensus (not IBFT 2.0), one network per spoke + hub
- Solidity contracts compiled and tested with Foundry (`forge`)
- Paladin Core (Zeto + Noto domains) for privacy tokens
- Go 1.26+ microservices, gRPC intra-entity, REST via API Gateway externally
- Keycloak (OIDC) for auth; PKI from central bank CAs
- React + Turborepo frontend
- Docker Compose per scenario; Postgres for service persistence

New runtime dependencies outside this stack require justification in the PR and the scenario README.

## Scenario B layout (most active)

```
scenario-b/
  contracts/             Foundry project (FXAgreement, AutomatedMarketMaker, LiquidityCommitRegistry, HTLC, tCeBM, ZetoToken, NotoToken, IdentityRegistry)
  backend/services/      api-gateway, auth, compliance, fx, ledger-gateway, payment-orchestrator, payments
  frontend/apps/         bank, governance, supervisor, treasury, noc
  interop/hub-and-spoke/ Cacti-based relay
  deploy/local/          Docker Compose stacks + tooling
  make/                  Modular makefiles (included by scenario-b/Makefile)
  tests/                 E2E + performance baselines
  specs/                 Feature specs and plans
```

## Common commands (run from `scenario-b/`)

Infra and stack:
- `make scenario-b.up` / `make scenario-b.down` — full stack lifecycle
- `make scenario-b.nuke` — wipe state
- `make scenario-b.restart`
- `make deploy.up-besu` / `deploy.up-infra` / `deploy.up-backend` — bring up layers individually
- `make dev.up-bank-a` / `dev.up-central-bank-a` / etc. — per-entity dev stacks

Contracts:
- `make contracts.build` / `contracts.test` / `contracts.fmt` / `contracts.lint` / `contracts.slither`
- `make contracts.deploy-all-with-sync` — deploy + propagate addresses
- `make contracts.sync-addresses` — refresh address files for backends/frontends

PKI:
- `make pki.gen-all` — generate cert material for all entities
- `make pki.check`

Tests:
- `make scenario-b.test` — full scenario suite
- `make scenario-b.test-contracts` / `scenario-b.test-backend`
- `make test.api-gateway` / `test.auth` / `test.compliance`
- `make scenario-b.perf-baseline` — performance baseline (required for production-grade scenarios)
- `make scenario-b.tryout-us1` / `tryout-us2` / `tryout-us3` — user-story walkthroughs

Frontend:
- `make frontend-scenario-b` / `frontend-scenario-b-down` / `frontend-scenario-b-logs`

Proto:
- `make proto-gen` / `proto-lint` / `proto-breaking`

## Workflow

- Feature branches target `develop` or a scenario integration branch; `main` is merge-only via PR.
- Every implementation plan (`plan.md`) needs a Constitution Check section before Phase 1.
- Deviations from established patterns go in the plan's Complexity Tracking with rejected alternatives.
- Scenario `README.md` status (Fully implemented / In progress / Planned) must be accurate at merge.
- PRs that weaken compliance or security checks need project-lead approval.

## Gotchas

- Contracts are not edited in feature branches that "redeploy contracts" — those targets only redeploy and resync addresses. Contract source changes are a separate concern.
- After `contracts.deploy-*`, run `contracts.sync-addresses` (or use `-with-sync` variants) so backends/frontends pick up new addresses.
- The relay (`interop/hub-and-spoke/cacti`) must observe events on both networks; if a settlement appears stuck, check relay logs before assuming a contract bug.
- `tCeBM` is reserve-layer only — do not introduce it into retail/settlement code paths.

## Active Technologies
- Go 1.26+ (backend), TypeScript / React 18 (frontend) + Fiber v2 (HTTP), GORM + Postgres (persistence), Keycloak OIDC (auth), Zustand (frontend state), TanStack Query (data fetching), shadcn/ui components (014-supervisor-portal)
- Postgres — existing `audit_log` table (both scenarios); new `disclosure_requests` + `disclosure_signatures` tables (Scenario A only); existing tables in Scenario B (014-supervisor-portal)
- Go 1.26+ (toolkit PKI step), GNU Make + Bash (Makefile fix) (016-fix-pki-local-ca)
- Filesystem only — `{bankCode}.key`, `{bankCode}.csr`, `{bankCode}.crt` (leaf cert from CB) (016-fix-pki-local-ca)
- Bash 5+ scripts, Docker Compose v2, YAML + `hyperledger/besu:25.8.0` (pinned), `curlimages/curl:latest` (smoke test), `jq`, `sha256sum` (018-spk01-cross-stack-enode)
- Filesystem only — genesis.json, node keypairs, bundle YAML (`bundles/<spoke-id>.bundle.yaml`) (018-spk01-cross-stack-enode)
- Bash 5+, `jq`, `yq` (YAML parse), `openssl` (cert gen), `curl` (RPC) + `hyperledger/besu:25.8.0` (pinned), Paladin Core (project-pinned version) (019-spk02-live-join)
- Filesystem only — genesis.json, Besu data volumes, Paladin SQLite + LevelDB data volumes, TLS cert files (019-spk02-live-join)
- Go 1.26+ (payment-orchestrator), TypeScript (relay, frontend) + protoc + protoc-gen-go (code gen), GORM v2 (ORM + AutoMigrate), gRPC (020-fx-spoke-keyed-legs)
- PostgreSQL — `fx_agreements` table; column additions + backfill + column drops via startup SQL (020-fx-spoke-keyed-legs)
- Go 1.26+ + GORM v2 (`gorm.io/gorm`, `gorm.io/driver/postgres`), `gorm.io/driver/sqlite` (testes), `log/slog` (stdlib) (021-db-migration-spoke-keyed)
- PostgreSQL (produção), SQLite (testes de repositório) (021-db-migration-spoke-keyed)
- Go 1.26+ (backend), TypeScript 5.x (relay + frontend) + `gorm.io/gorm` v2, `google.golang.org/grpc`, `google.golang.org/protobuf`, `buf` (proto codegen), Cacti HTLC relay (TypeScript) (022-update-proto-consumers)
- PostgreSQL (produção, tabelas `fx_agreements`), SQLite (testes de repositório) (022-update-proto-consumers)

## Recent Changes
- 014-supervisor-portal: Added Go 1.26+ (backend), TypeScript / React 18 (frontend) + Fiber v2 (HTTP), GORM + Postgres (persistence), Keycloak OIDC (auth), Zustand (frontend state), TanStack Query (data fetching), shadcn/ui components
