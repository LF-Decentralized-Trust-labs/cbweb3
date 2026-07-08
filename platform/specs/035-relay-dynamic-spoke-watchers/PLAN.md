# FX cross-spoke on-chain + dynamic relay — plan

Branch: `035-relay-dynamic-spoke-watchers`. Scenario A.

## Goal
Make cross-spoke FX agreements real: settle them **on-chain in private Pente groups** (not
DB rows), let the **central bank aggregate** its banks' agreements, and make the **relay watch
spokes dynamically** (present and future) instead of static config.

## What's broken today (evidence)
- **Relay watchers are static.** Built once from `SPOKE_A/SPOKE_B` env / `CACTI_SPOKES_CONFIG`;
  the dynamic `/api/v1/spokes` registry is passive (registering ≠ watching). On `deploy-all`
  the relay booted on stale defaults, WS never reconnected → `connection not open` forever.
  (Interim fix in place via `spokes.local.yaml` — to be removed.)
- **FX agreements are off-chain by deliberate choice.** `ProposeFXAgreement` wraps the on-chain
  path in `if false // future optional on-chain audit path` (server.go) and only writes
  Postgres. DB rows have empty `group_id`/`contract_address`; the CB's PO list is empty.
- **`register-relay` under-registers.** Sends `{spokeId, besuRpc, htlcAddress}` where
  `htlcAddress` is a placeholder (`RegistryContractAddress`); missing `besuWs`,
  `internalApiUrl`, `grpcEndpoint`. Real HTLC is `DeployedAddrs.HTLCAddress`.

## Decisions
- **On-chain, private (Pente).** FX agreement is the on-chain source of truth in a bilateral
  **CB↔bank** Pente group; the DB becomes a projection. (Constitution: privacy II, atomicity III.)
- **Coordinator = the central bank**, one per spoke. Cross-spoke propose/accept/settle go
  `on_behalf` → `proposeOnBehalf`/`settle`, gated by `canGovern()` = CENTRAL_BANK only; the CB
  PO signs. `internalApiUrl` = CB api-gateway, `grpcEndpoint` = CB payment-orchestrator.
- **Relay observes via the CB, not on-chain events.** The relay isn't a Pente member, so it
  can't see private `AgreementProposed`. Instead the **CB PO indexes the groups it belongs to**
  and exposes the aggregate; the relay polls the CB. (Verified: CB Paladin is a member of every
  bilateral group on its spoke.)
- **Group discovery is dynamic; contract address is bank-pushed (A6).** The CB re-enumerates its
  groups via `pgroup_queryGroups` each cycle (new banks appear automatically). But a
  non-submitting member **cannot** discover a group's FXAgreement address from chain
  (probe-verified: submitter-scoped tx records; no `pgroup_queryTransactions`). So the joining
  **bank registers `{spokeId, bankIdentity, groupId, fxAgreementAddress}` with the CB** at
  runtime (it has the address from `deploy-fxa-pente`).
- **Destination group = CB↔custodian.** On each spoke the agreement lives in the CB↔(active
  local bank) group: originator on source, **custodian** on destination. Dest CB
  `proposeOnBehalf` into `CB ↔ custodian` (keyed on the `custodian` identity).
- **`trade_id` reused end-to-end** so `bytes32 = sha256(uuid)` matches source↔dest →
  idempotent relay redelivery.

## Two surprises that dominate the work
1. **The Pente proxy the backend calls does not exist.** `pente_client.go` POSTs to
   `PENTE_BASE_URL/api/v1/pente/fx/...` — nothing in the repo serves it. → **rewrite
   `pente_client.go` to call Paladin `pgroup_*` directly** (as `toolkit/.../pente.go` does).
2. **No IdentityRegistry inside the Pente groups.** `propose` reverts empty `0x` from
   `IDENTITY_REGISTRY.canTransact()` (no in-group registry code). → toolkit must **deploy +
   verify a registry inside each bilateral group**.

## Phase 0 — DONE (spike, proven on live env)
The bank-deployed FXAgreement (`0xb9a219…`, addr via `ptx_getDomainReceipt` on the bank node)
**replicates to the CB node and executes there** — CB `pgroup_call getAgreement` returned the
contract's own `FXA__TradeNotFound`, not "no contract". Foundation holds. Blocker surfaced:
in-group IdentityRegistry (→ Phase 1a).

## Work

### Phase 1 — Backend on-chain FX (critical path; start here)
- **1a. In-group IdentityRegistry** — `toolkit/engine/orchestrator/pente.go`,
  `step_create_pente*.go`, `step_deploy_fxa*.go`: deploy `IdentityRegistry` into each group,
  verify CB + bank, deploy FXAgreement pointing at the in-group address; persist
  `FX_AGREEMENT_ADDRESS` on the found side too.
- **1b. Rewrite `pente_client.go`** (`backend/.../adapters/paladin/pente_client.go`) → Paladin
  `pgroup_sendTransaction` (propose/accept/settle), `pgroup_call getAgreement` (read),
  `pgroup_queryGroups` (discover), `ptx_getDomainReceipt` (resolve address). No HTTP proxy.
- **1c. Turn on propose** (`backend/.../grpc/server/server.go`): remove `if false`,
  `EnsureFXContext` at propose, persist `group_id`/`contract_address`; verify accept/reject/
  cancel/settle submit; atomicity + idempotency (persist intermediate state, guard retries).
- `cmd/payment-orchestrator/main.go`: construct the client when `PENTE_ENABLED`.
- **Checkpoint:** propose→accept→settle round-trip green in tests + on Docker.

### Phase 2 — CB aggregation + bank registration
- **A4 indexer** — new `backend/.../payment-orchestrator/internal/indexer/fx_indexer.go`:
  loop → `pgroup_queryGroups` → per-group `getAgreement` (address from the A6 registry) →
  project to `fx_agreements`.
- **A6 registration** — new store + handler; route in `api-gateway/.../router.go`; bank posts
  `{spokeId, bankIdentity, groupId, fxAgreementAddress}` on startup/first propose.
- Repo: `fx_agreement_gorm.go` + `_models.go` — new table(s) for the registry + projections.
- **Checkpoint:** CB `/internal/v1/payments/fx/agreements` lists a bank's on-chain agreement
  (the empty-`18645` bug is gone).

### Phase 3 — Toolkit provisioning
- `step_render_cb_env.go` / `step_render_bank_env.go`: `PENTE_ENABLED`, `PENTE_BASE_URL`,
  `FX_AGREEMENT_PENTE_CONTRACT_ADDRESS`.
- `step_register_relay.go` + `deps.go` (`SpokeInfo`): six fields, real HTLC, CB coordinator.
- **Checkpoint:** `deploy-all` yields a stack where Phase 2 works end-to-end.

### Phase 4 — Relay dynamic registry
- `interop/hub-and-spoke/cacti/src/index.ts`: registry-driven connector/watcher/gRPC lifecycle
  (create/reconcile/remove on register/deregister; reconcile from persisted registry at start).
- `htlc-relay.ts`: runtime add/remove spoke; WS reconnect + backoff.
- `spoke-registry.ts`: emit lifecycle events; enforce six-field validation.
- `spokes-config.ts`: legacy env/YAML → bootstrap seeder into the registry.
- **Checkpoint:** runtime spoke registration starts a watcher with no restart; reconnect after
  a Besu bounce.

### Phase 5 — E2E
- `deploy-all`: proposal on Itaú observed via the CB aggregate, mirrored to dest CB↔custodian.
- Tests: Foundry (in-group registry + FXAgreement), Go (`pente_client`, `server`, indexer,
  `register-relay`, env-render), relay vitest (dynamic watcher, reconnect, reject-incomplete).

## Open questions (recommendations)
- **Indexing (Q1):** poll `pgroup_queryGroups` + read each per cycle first; Paladin event
  subscription later as a latency optimization.
- **Auth (Q5):** authenticate `POST /api/v1/spokes` and the CB internal FX API — near-term
  hardening, not blocking.

## Cleanup
- Remove interim `interop/hub-and-spoke/cacti/spokes.local.yaml` +
  `docker-compose.override.local.yml` once Phase 3/4 land.
- `docker-compose.yaml`: stop relying on stale `SPOKE_A/B` defaults.

## Effort
Concentrated in the backend (rewrite `pente_client.go`; indexer + A6) and the toolkit
(in-group registry). Relay work is contained. Start with 1a/1b — highest surprise risk.

## Implementation status (2026-07-01, pre-deploy)

Done + unit-tested (builds green; `go test` passing):
- **1b** — `pente_client.go` rewritten to Paladin `pgroup_*` direct (propose/accept/settle →
  `pgroup_sendTransaction` + receipt-wait; getAgreement → `pgroup_call`; EnsureFXContext →
  `pgroup_queryGroups` by members). 7 hermetic tests.
- **1a** — `setupBilateralFXAContext` in `pente.go` (deploy in-group IdentityRegistry, register
  CB=CENTRAL_BANK + bank=COMMERCIAL_BANK, deploy FXAgreement wired to it) + wired into
  `step_deploy_fxa_join.go` (persists `FX_PENTE_REGISTRY_ADDRESS` + `FX_AGREEMENT_ADDRESS`). Test.
- **1c** — `ProposeFXAgreement` on-chain path enabled (removed `if false`, mirrors the accept
  path: EnsureFXContext → submit → persist context; atomicity: no PROPOSED row unless on-chain
  succeeds). Decimal `Rate` → 1e18 fixed-point. `partyAddress` fills the zero-address bug.
  Existing Pente tests updated to the new behavior.
- **3 (env)** — bank + CB backend `.env` now render `PENTE_ENABLED=true` + `PENTE_BASE_URL`.

- **A6 (bank side)** — file-based FX-context transport (chosen over proto/gRPC: PO is
  gRPC-only, buf regen risky, soft-tail reorder unsafe). `deploy-fxa-join` writes
  `<dataDir>/pki/fx-contexts.json` (bind-mounted to `/workspace/backend/config/pki`); the PO's
  `fxContextStore` re-reads it per propose and resolves group+address by bank identity
  (`fxcontext.go`), wired into `ProposeFXAgreement` (file-first, Pente-client fallback);
  `FX_CONTEXTS_FILE` env in `main.go`. Unblocks the **bank's own on-chain propose**. Tests.

Remaining (not yet implemented):
- **A6 (CB side)** — `deploy-fxa-join` also writes the context into the **CB's** `dataDir/pki`
  (needs the CB dataDir threaded into the join step) so the CB PO can resolve for
  `proposeOnBehalf` + the indexer. Same file format; small extension.
- **2 (CB indexer / A4)** — CB PO reads its `fx-contexts` + `pgroup_call getAgreement` per group,
  projects to `fx_agreements` so `/internal/v1/payments/fx/agreements` is populated for the relay.
- **3 (register-relay)** — `step_register_relay.go` + `deps.SpokeInfo`: six fields, real HTLC,
  CB coordinator endpoints.
- **4 (relay)** — registry-driven dynamic watcher lifecycle + WS reconnect.

Known coupling to resolve during live testing:
- **FXAgreement address ordering.** `deploy-fxa` runs in the join soft tail, *after* the backend
  starts, so `FX_AGREEMENT_PENTE_CONTRACT_ADDRESS` cannot be embedded in the env at render time.
  It renders empty; the on-chain propose needs it supplied at runtime. For the **bank's own**
  propose the bank node can resolve it (it submitted the deploy — `ptx_getDomainReceipt`); for
  the **CB** it needs A6. Until resolved, `EnsureFXContext` returns the "pending A6" error. This
  is the first thing to close when wiring Phase 2.
- **On-chain party-address & rate-scale semantics** (`partyAddress`, 1e18 rate) are documented
  TODOs to confirm against a live deploy.
