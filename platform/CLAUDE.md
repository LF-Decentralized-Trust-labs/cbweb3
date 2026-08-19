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
- Go 1.26 microservices, gRPC intra-entity, REST via API Gateway externally
- Keycloak (OIDC) for auth; PKI from central bank CAs
- React + Turborepo frontend on Node 22 LTS
- Docker Compose per scenario; Postgres for service persistence

New runtime dependencies outside this stack require justification in the PR and the scenario README.

**Toolchain versions live in [`docs/TOOLCHAIN.md`](docs/TOOLCHAIN.md) and that file wins.**
The floor is the same for both scenarios: **Go 1.26** (every `go.mod`, every
`golang:1.26-alpine` builder) and **Node 22 LTS** (root `.nvmrc`, `engines`, every
`node:22-alpine` image, `@types/node ^22`). Older versions quoted in the Active
Technologies list below are historical records of individual specs, not the current floor.
Do not introduce a per-scenario or per-module version split without recording it in the
Recorded deviations table of that file.

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
- Every new source file carries an SPDX licence header — see below. Not optional.
- Branch and PR reviews must verify those headers explicitly, not rely on the CI badge.

## License headers (mandatory)

**Every new source file created in this project must carry an SPDX licence header. This is a hard requirement, not a convention** — it is a Digital Public Goods compliance obligation (`docs/DPG-COMPLIANCE.md`) enforced in CI by the `License Headers` workflow.

The identifier is always `Apache-2.0`. Place it as the **first line** of the file, followed by a blank line:

```go
// SPDX-License-Identifier: Apache-2.0

package orchestrator
```

Per language:

| Files | Header | Placement |
| --- | --- | --- |
| `.go`, `.ts`, `.tsx`, `.sol` | `// SPDX-License-Identifier: Apache-2.0` | first line (before `//go:build`, before `pragma solidity`) |
| `.sh`, `.py`, `.yml` | `# SPDX-License-Identifier: Apache-2.0` | first line, or line 2 immediately after a shebang |

Rules:

- The header must appear **within the first 5 lines** — that is the window the checker inspects. A header lower down does not count.
- In Go files with a build constraint, the SPDX line goes first, then a blank line, then `//go:build` (see `scenario-a/toolkit` for the reference form).
- Do **not** add headers to generated or vendored output. The documented exclusions are `*.pb.go`, `*/bindings/*`, `vendor`, `node_modules`, `dist`, `build`, `.next`, `.turbo`.
- Go and TypeScript are enforced by CI. Solidity, shell, Python and YAML are not yet gated, but the header is still required — write it when creating the file rather than backfilling later.

Verify before opening a PR (both must exit `0`):

```bash
bash tools/check-license-headers.test.sh   # self-test: the gate must fail on a header-less fixture
bash tools/check-license-headers.sh        # the actual scan
```

Run the checker with **bash**, never zsh. zsh does not fork the last stage of a pipeline, which masks the class of bug that once made this gate report success without verifying anything.

### Checking it in review

**Reviewing a branch or a PR includes verifying the licence headers. This check is mandatory — no branch or PR is approved without it.** A green `License Headers` badge is not sufficient evidence on its own; confirm it directly.

For every review:

1. List the source files the branch adds, and confirm each one carries the header:

   ```bash
   git diff --name-only --diff-filter=A origin/develop...HEAD \
     | grep -E '\.(go|ts|tsx|sol|sh|py)$' \
     | xargs -r -I{} sh -c 'head -5 "{}" | grep -q SPDX-License-Identifier || echo "MISSING: {}"'
   ```

2. Run the gate locally on the branch — both commands must exit `0`:

   ```bash
   bash tools/check-license-headers.test.sh
   bash tools/check-license-headers.sh
   ```

3. Cover what CI does not gate. Solidity, shell, Python and YAML additions are outside the scanned set, so they need reading by eye — the workflow going green says nothing about them.

4. Treat a missing header as a change request, not a nit. Headers are added in the branch under review; do not defer them to a follow-up PR.

Also flag, in review, any change that weakens the gate itself: a counter moved back inside a pipe, a new exclusion path, `|| true` on the check step, or the self-test removed from the workflow. Those need explicit justification in the PR description.

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
- Go 1.26+; YAML 1.2 (schema and manifest files) + `gopkg.in/yaml.v3` (new; only addition to `toolkit/go.mod`) (023-manifest-schema-validation)
- None — stateless; reads one file, validates, returns result (023-manifest-schema-validation)
- Go 1.26+ + `github.com/ethereum/go-ethereum v1.17.1` (já presente em múltiplos módulos do Scenario A; adicionar ao `toolkit/go.mod`) (024-tk2-keyprovider-interface)
- Nenhum — implementação local usa in-memory map; sem persistência (024-tk2-keyprovider-interface)
- Go 1.26+ + Go stdlib apenas (`crypto/x509`, `crypto/ecdsa`, `crypto/elliptic`, `crypto/rand`, `encoding/pem`, `sync`, `context`) (025-tk3-certsource-interface)
- Nenhum — implementação local usa `map[string]*spokeCert` em memória; a chave privada da CA do spoke nunca é serializada para disco (025-tk3-certsource-interface)
- Docker Compose v2 (YAML 3.8+); Bash 5+ (script de guarda do genesis) + `hyperledger/besu:25.8.0` (imagem parametrizada via `BESU_IMAGE`); `docker compose` plugin v2 (026-tk4-compose-central-bank)
- Bind mounts via `SPOKE_DATA_DIR`; sem named volumes — garante portabilidade entre hosts (026-tk4-compose-central-bank)
- Filesystem — `<SPOKE_DATA_DIR>/.provisioning-state.yaml`, `<SPOKE_DATA_DIR>/.deployed-addrs.env` (027-tk5-orchestration-engine)
- Go 1.26+ + `gopkg.in/yaml.v3` (serialização do bundle), `encoding/pem` (validação CA cert), `crypto/sha256` (hash genesis), `encoding/base64` (embedding genesis), `net/http` (JSON-RPC admin_nodeInfo) — todos stdlib ou já em `toolkit/go.mod` (028-tk6-join-bundle-emitter)
- Filesystem — `<SPOKE_DATA_DIR>/.deployed-addrs.env`, `<SPOKE_DATA_DIR>/genesis/genesis.json`, `<SPOKE_DATA_DIR>/tls/central-bank.crt`; saída: `<outputDir>/bundles/<spoke-id>.bundle.yaml` (028-tk6-join-bundle-emitter)
- Go 1.26+ CLI binary + `flag` stdlib (flag parsing), `encoding/json` + `gopkg.in/yaml.v3` (structured output), `os/signal` + `syscall` (signal handling) — zero new external deps (029-tk7-apply-command)
- Filesystem — lê `<dataDir>/.provisioning-state.yaml` (dry-run); escreve nenhum arquivo diretamente (delegado ao engine e bundle emitter) (029-tk7-apply-command)
- TypeScript 5.4+ (Node.js 20 LTS — conforme devDependencies `@types/node ^20`) + `js-yaml ^4.1.0` (novo — parsing YAML), `@hyperledger/cactus-plugin-ledger-connector-besu ^2.0.0`, `@grpc/grpc-js ^1.10.0`, `express ^4.18.0` (031-relay-spoke-registry)
- Nenhum novo. `RelayStore` (arquivo JSON existente) é preservado. (031-relay-spoke-registry)
- Go 1.26+ (orchestrator, bundle, manifest, pki packages); Docker Compose v2 + YAML 3.8+ (TK-8 template) + `gopkg.in/yaml.v3` (bundle/manifest parsing), `net/http` (CSR HTTP POST + polling), `github.com/ethereum/go-ethereum v1.17.1` (QBFT JSON-RPC, IdentityRegistry), `os/signal`+`syscall` (file lock), `flag` (CLI) — todos presentes em `toolkit/go.mod`; Docker Compose v2 plugin; `hyperledger/besu:25.8.0` (pinned) (032-commercial-bank-join)
- Filesystem — `SPOKE_DATA_DIR/genesis/genesis.json` (leitura do bundle; nunca regenerado), `SPOKE_DATA_DIR/tls/commercial-bank.{crt,key}`, `SPOKE_DATA_DIR/.provisioning-state.yaml`, `SPOKE_DATA_DIR/.provisioning.lock` (032-commercial-bank-join)
- Go 1.26+ + `gopkg.in/yaml.v3` (YAML parse); Go-side validation is source of truth, JSON-Schema for editor/CI (033-tk-b1-manifest-schema)
- Filesystem/none — stateless validation (reads one manifest or a set; writes no state) (033-tk-b1-manifest-schema)
- Go 1.26+ + `github.com/ethereum/go-ethereum` (secp256k1 + EVM address), stdlib `crypto/{ecdsa,elliptic,x509,rand}` + `encoding/pem` (CertSource + CSR) (034-tk-b2-keyprovider-certsource)
- None — KeyProvider keys and per-spoke CA live in memory only; nothing persisted (no-secrets invariant) (034-tk-b2-keyprovider-certsource)
- Go 1.26 (validation/tests) + `gopkg.in/yaml.v3` (already in toolkit/go.mod); compose templates (YAML) under scenario-b/provisioning/; Docker Compose v2 optional (test-only) (035-tk-b4-parametrized)
- None — compose templates are files under provisioning/; validation is stateless (reads template + example env, returns result) (035-tk-b4-parametrized)
- TypeScript 5.4 (relay Cacti, Node 20) + Go 1.26 (`RelayRegistrar` in toolkit); relay reuses express/ethers v6/cactus-besu (no new runtime deps); tests via node:test + ts-node; Go stdlib only (036-tk-b5-generalized)
- Relay RelayStore = persisted JSON file on the relay volume (loaded at boot, saved per registration); Go local registrar is in-memory (036-tk-b5-generalized)
- Go 1.26 (toolkit) — no new Go deps (go-ethereum RPC probes + yaml.v3 already present); os/exec CommandRunner (real/fake) for docker compose + forge + Keycloak; E2E needs Docker/Foundry/Besu (skip-with-warning) (037-tk-b6-orchestration)
- State per step in `<dataDir>/.provisioning-state.yaml` + flock lock; hub bundle at `<outDir>/bundles/hub.bundle.yaml` (public, no secrets); contract addresses from Foundry broadcast JSON (037-tk-b6-orchestration)
- Go 1.26 (toolkit) — no new Go deps; found-spoke mode reuses the TK-B6 engine/exec/addrs/bundle + KeyProvider/CertSource/RelayRegistrar; enode via admin_nodeInfo; soft steps (add-noc-agent) (038-tk-b7-found)
- Spoke bundle at `<outDir>/bundles/spoke-<id>.bundle.yaml` includes genesis + enode + chainId + spoke contract addresses (public, no secrets); consumed by join (TK-B8) (038-tk-b7-found)
- Go 1.26 (toolkit) — no new Go deps; join mode reuses the TK-B6/B7 engine/exec/addrs/bundle (LoadSpoke) + pki.GenerateBankCSR; new wait-sync gate via eth_syncing; canonical flow has no relay/noc step (039-tk-b8-join)
- Bank joins as a non-validating full node (CB is the sole QBFT validator): write-genesis copies the spoke bundle genesis (non-destructive + sha256 guard), wait-sync blocks on eth_syncing, gen-csr is the only PKI step (key 0600, OU=ROLE_COMMERCIAL_BANK, zero CA material; signing/registration are runtime) (039-tk-b8-join)
- Go 1.26 (toolkit) — **SUPERSEDED**: the sovereign-pair tail (open-sovereign-pair/commit-liquidity/seed-oracle) and the `spec.pair` manifest field were REMOVED from found-spoke in c90de691. There is no provisioning step for corridors; `TestApplyFoundSpokeHasNoSovereignTail` asserts apply never plans them (040-tk-b9-sovereign-pair)
- Strict sovereignty (still the governing rule, now enforced at runtime): opening a corridor is two independent sovereign acts, each signed by its own CB — one proposes via the governance portal (proposePair), the counterparty confirms (confirmPair), then each commits its own liquidity. No run holds the counterparty key, so SeedNewSovereignPair.s.sol (needs both) is NOT reused (040-tk-b9-sovereign-pair)
- Go 1.26 (toolkit e2e/perf tests) — no new Go deps; TK-B10 is a verification phase (tests + docs): a full-pipeline E2E (found-hub→found-spoke×2→join via apply.Apply; the corridor is opened separately at runtime) exercising swap/breaker/SpokeBridge, plus a toolkit-native Go perf baseline (p95 quote/swap) and E2E-STATUS.md; all skip-with-warning (041-tk-b10-e2e-baseline)
- SpokeBridge reality: only lock(token,amount,txId) + release(txId) (GOVERNANCE) + getLock — no on-chain mint/burn/unlock/timeout; mint is relay-mediated, refund is release; AMM swap is swapTokensForExactTokens; breaker via pause/signResume (quorum 2)/isPaused (041-tk-b10-e2e-baseline)

## Recent Changes
- 014-supervisor-portal: Added Go 1.26+ (backend), TypeScript / React 18 (frontend) + Fiber v2 (HTTP), GORM + Postgres (persistence), Keycloak OIDC (auth), Zustand (frontend state), TanStack Query (data fetching), shadcn/ui components
