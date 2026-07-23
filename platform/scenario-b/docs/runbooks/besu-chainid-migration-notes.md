# Migration Notes — Besu 25.8.0 and Chain ID Rebase · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-07-23
>
> **Scope:** Network parameter changes between deliverable **D6 v2** and **D12** that affect the network operated by LNET.

> **Status: Informational.** This note documents the rationale and compatibility checks for two parameter changes that were already applied to the D12 codebase. It does not change any code or configuration. The final values for a live network must be agreed with LNET as the network operator (see [Coordination with LNET](#coordination-with-lnet)).

This note records the *delta* between D6 v2 and D12 for two network-level parameters. The end state is already reflected throughout the D12 sources; what was missing was an explicit record of the change, the rationale, and the compatibility verification. All D12 values below are verifiable in the repository at the paths cited. The D6 v2 values are recorded for historical reference.

---

## Summary of changes

| Parameter | D6 v2 (previous) | D12 (current) | Where defined in D12 |
|-----------|------------------|---------------|----------------------|
| Besu image | `hyperledger/besu:24.x` | `hyperledger/besu:25.8.0` (pinned) | genesis tooling, provisioning templates, toolkit defaults, docs |
| Hub chain ID | `80000` | `1337` | `deploy/local/hub-besu/config/configTemplate.json` |
| Spoke-A chain ID | `80001` | `1338` | `deploy/local/spoke-besu-a/config/configTemplate.json` |
| Spoke-B chain ID | `80002` | `1339` | `deploy/local/spoke-besu-b/config/configTemplate.json` |

---

## 1. Besu 24.x → 25.8.0

### Rationale

- **Single pinned version across the platform.** The constitution (`.specify/memory/constitution.md`) requires a pinned Besu image and forbids `latest`. D12 standardizes on `hyperledger/besu:25.8.0` so that every spoke and the hub run an identical, reproducible client build. The version is pinned (not a floating tag) so genesis semantics and consensus behavior are deterministic across entities and across CI runs.
- **Alignment with the D12 stack.** The 25.8.0 line is the version referenced by the project stack description in `CLAUDE.md` and the architecture documentation, keeping the runtime consistent with the documented and tested configuration.

### Compatibility verification

- **Consensus (QBFT) unchanged.** The migration is client-version only; the consensus engine remains **QBFT** (not IBFT 2.0). All Scenario B genesis templates keep the same `qbft` block — `blockperiodseconds: 2`, `epochlength: 30000`, `requesttimeoutseconds: 4` — in `deploy/local/hub-besu/config/configTemplate.json`, `deploy/local/spoke-besu-a/config/configTemplate.json`, and `deploy/local/spoke-besu-b/config/configTemplate.json`. QBFT genesis configuration is supported by Besu 25.8.0, so existing validator sets and voting flows carry over without a genesis rebuild attributable to the version bump.
- **Genesis config compatible.** The fork schedule in the genesis templates (all EIP/fork blocks at `0`, `shanghaiTime`/`cancunTime` at `0`, `zeroBaseFee: true`) is honored by 25.8.0. No new mandatory fork or config key was introduced by the upgrade in these templates.
- **Image pinned and confirmed.** The 25.8.0 pin appears in the Scenario A provisioning templates (`scenario-a/provisioning/templates/central-bank/docker-compose.yaml`, `.../commercial-bank/docker-compose.yaml`, parameterized via `BESU_IMAGE`) and the toolkit defaults (`scenario-a/toolkit/engine/orchestrator/step_start_besu_found.go`, `.../engine/apply/profile.go`, `.../engine/apply/apply.go`, default `hyperledger/besu:25.8.0`; env override `CBWEB3_BESU_IMAGE`). Architecture and test-plan docs reference `v25.8.0` (`scenario-b/docs/architecture/architecture-overview.md`, `scenario-b/docs/test-execution-plan.md`). No `24.x` Besu reference remains in the sources.

### Outstanding cleanup (compatibility caveat)

The Scenario B local bring-up scripts download the `besu-25.8.0` distribution but still invoke `docker run hyperledger/besu:latest` at runtime (`scenario-b/deploy/local/hub-besu/startBesu.sh`, `.../spoke-besu-a/startBesu.sh`, `.../spoke-besu-b/startBesu.sh`, and the `addNewNode.sh` helpers). The chart doc `scenario-b/docs/charts/scenario-b/architecture.md` also still shows `hyperledger/besu:latest` for the spokes. To fully realize the pinning intent of this migration, those `latest` references should be pinned to `25.8.0`. This is a follow-up item and is called out here for traceability; it is not changed by this note.

---

## 2. Chain IDs 80000 / 80001 / 80002 → 1337 / 1338 / 1339

### Rationale

- **Development-standard IDs.** D12 rebases the three networks onto the conventional local/development chain IDs `1337` (hub), `1338` (spoke-A), and `1339` (spoke-B). These are contiguous and easy to reason about in tooling, wallets, and local RPC clients.
- **Consistency across every layer.** The new IDs are wired uniformly through genesis, backend compose, environment examples, contract deployment, and the makefiles, so a single set of values is used end to end.

### Where the D12 values are defined

- **Genesis (source templates; runtime `genesis.json` is generated and gitignored):**
  - `deploy/local/hub-besu/config/configTemplate.json` → `"chainId": 1337`
  - `deploy/local/spoke-besu-a/config/configTemplate.json` → `"chainId": 1338`
  - `deploy/local/spoke-besu-b/config/configTemplate.json` → `"chainId": 1339`
- **Backend compose (defaults):** `backend/docker-compose-backend.bank-a.yaml` (`SPOKE_CHAIN_ID` default `1338`, `HUB_CHAIN_ID` default `1337`), `.bank-b.yaml` (`1339` / `1337`), `.central-bank-a.yaml` (`1338` / `1337`), `.central-bank-b.yaml` (`1339` / `1337`).
- **Environment examples:** `backend/config/.env.infra.bank-a.example` (`BESU_CHAIN_ID=1338`, `HUB_CHAIN_ID=1337`), `.bank-b.example` (`1339` / `1337`), `.central-bank-a.example`, `.central-bank-b.example`.
- **Contracts and makefiles:** `contracts/.env.example` (`HUB_CHAIN_ID=1337`), `make/30-contracts.mk`, `make/10-deploy.mk`, `make/60-scenario-b.mk` (hub `1337` / spoke-A `1338` / spoke-B `1339`).

The previous D6 v2 values `80000` / `80001` / `80002` do not appear anywhere in the D12 sources; the rebase is complete.

### Impact for the LNET network

- **Chain ID is a genesis-level identity.** Changing it produces a distinct network — nodes on the old IDs cannot peer or reach consensus with nodes on the new IDs. For a running network this is not an in-place upgrade; it implies a coordinated re-genesis (or a deliberate hard cutover) across all validators and full nodes.
- **Transaction replay / signing.** Chain ID participates in EIP-155 transaction signatures. Clients, wallets, relayers, and any pre-signed transaction material must be reconfigured to the new IDs; transactions signed for the old chain IDs are not valid on the new networks.
- **Downstream configuration.** Every backend service, the relay, and the frontends read the chain IDs from the environment/compose values above. A live rebase requires these to be updated in lockstep with the genesis, and contract addresses to be re-synced (`make contracts.sync-addresses`) after redeployment on the new networks.

### Observed label inconsistency (for follow-up, not changed here)

`backend/config/.env.infra.mlp.example` labels chain `1338` as the "Hub" (`BESU_CHAIN_ID=1338` with a "Hub Besu node (chain 1338)" comment), whereas the hub elsewhere is `1337`. This should be reconciled when the values are finalized.

---

## Coordination with LNET

The values recorded here are the current D12 development defaults. **LNET operates the network**, and the final Besu image version and the production chain IDs for the hub and each spoke must be agreed with LNET before any live deployment. The relay is centrally operated by LNET (`scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml`), so any chain ID or client-version change also has to be coordinated with LNET's relay and node operations. Treat the local values (`25.8.0`; `1337` / `1338` / `1339`) as the reference configuration for development and testing, not as a commitment for the operated network.

---

## References

- Constitution (image pinning rule): `.specify/memory/constitution.md`
- Stack description: `CLAUDE.md`
- Genesis templates: `scenario-b/deploy/local/{hub-besu,spoke-besu-a,spoke-besu-b}/config/configTemplate.json`
- Backend compose: `scenario-b/backend/docker-compose-backend.*.yaml`
- Environment examples: `scenario-b/backend/config/.env.infra.*.example`
- Makefiles: `scenario-b/make/{10-deploy,30-contracts,60-scenario-b}.mk`
- Architecture overview: `scenario-b/docs/architecture/architecture-overview.md`
