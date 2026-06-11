# Phase 1 Data Model: International Hub Network Isolation

This feature is infrastructure/configuration in nature — it has no application database schema or migrations (Storage: N/A in `plan.md`). The "entities" below are the configuration, network, and operational concepts the spec's functional requirements operate on. They are documented here to give the implementation and its tests a precise, shared vocabulary, mirroring the **Key Entities** section of `spec.md`.

## 1. Hub Network Profile

Describes the Hub Besu/QBFT network as a deployable unit — the configuration analog of `spoke-besu-a`'s `.env.network` + `config/configTemplate.json` pair.

| Field | Type / Format | Source of Truth | Validation Rule |
|---|---|---|---|
| `chainId` | integer | `hub-besu/config/configTemplate.json` | MUST be `1337` (FR-002); MUST differ from Spoke A (`1338`) and Spoke B (`1339`) |
| `networkName` | string | `hub-besu/.env.network` (`NETWORK_NAME`) | MUST be unique among the three networks (e.g., `hub_besu_network`); no shared Docker network with either spoke (FR-004) |
| `containerPrefix` | string | `hub-besu/.env.network` (`CONTAINER_PREFIX`) | MUST be unique (e.g., `cbweb3-hub-besu`); no shared container namespace with either spoke |
| `rpcPort` / `p2pPort` / `wsPort` | integer | `hub-besu/.env.network` | MUST NOT collide with Spoke A's (`86xx`) or Spoke B's (`87xx`) port ranges |
| `consensus` | enum | `hub-besu/config/configTemplate.json` | MUST be `qbft` (Constitution Technology Stack Constraints; IBFT 2.0 explicitly excluded) |
| `blockPeriodSeconds` | integer | genesis config | MUST equal Spoke A/B's value (`2`) — operational consistency (FR-003) |
| `epochLength` | integer | genesis config | MUST equal Spoke A/B's value (`30000`) |
| `requestTimeoutSeconds` | integer | genesis config | MUST equal Spoke A/B's value (`4`) |
| `gasLimit` | hex string | genesis config | MUST equal Spoke A/B's value (`"0x1c9c380"`) |
| `zeroBaseFee` | boolean | genesis config | MUST equal Spoke A/B's value (`true`) |
| `validatorCount` | integer | `hub-besu/genesis/genesis.json` (extraData), `nodes/` | Sandbox: `1` is acceptable and MUST be documented as such (FR-011); production: MUST be documented separately as requiring a multi-validator (`n ≥ 3f+1`) QBFT set |

**Relationships**: A Hub Network Profile is the deployment target for exactly one **Shared Contract Suite** instance (1:1, post-cutover) and is referenced by every **Hub Network Identity (configuration value)** instance across services (1:N).

**Lifecycle / state transitions**: `stopped → starting (genesis + validator bootstrap) → running (producing blocks, RPC live)`. Per US1 Acceptance Scenario 2, transitions of Spoke A/B (`running ↔ stopped`) MUST NOT cause a Hub Network Profile state transition — independence is the defining invariant.

## 2. Shared Contract Suite (on the Hub)

The set of contracts that must be deployed to, and addressable on, the Hub Network Profile. Mirrors the spec's "Shared Contract Suite" entity, instantiated specifically at chain `1337`.

| Field | Type / Format | Validation Rule |
|---|---|---|
| `contractName` | enum: `IdentityRegistry`, `CurrencyRegistry`, `PairRegistry`, `TokenizedCentralBankMoney (tCeBM-BRL)`, `TokenizedCentralBankMoney (tCeBM-EUR)`, `ManualOracle`, `AutomatedMarketMaker`, `LiquidityCommitRegistry`, `FXAgreement`, … (full suite per `CBWeb3Hub.s.sol:DeployCBWeb3Hub`) | MUST match the deployment script's known set — "moving only some of them would leave the Hub partially neutral and partially domestic" (per spec Assumptions) |
| `address` | 20-byte hex (checksummed) | MUST be non-zero, distinct per contract, and resolvable via RPC on chain `1337` (FR-005, SC-004) |
| `chainId` | integer | MUST equal `1337` for every entry — this is the field that makes "deployed to the Hub" verifiable (SC-004) |
| `deployedCodeHash` | bytes32 | MUST match the compiled artifact's hash — "live and operable, not merely an address record" (US2 Acceptance Scenario 2) |

**Relationships**: Each entry belongs to exactly one Hub Network Profile (N:1). `IdentityRegistry` entries are also referenced by the **Compliance Gate** flow (Constitution Principle IV) — unchanged by this feature, only its host network changes.

**State transitions**: `not deployed → deploying (forge script broadcasting) → deployed (address assigned) → verified (code hash + RPC query confirmed)`.

## 3. Hub Network Identity (Configuration Value)

The configuration value — read by every Scenario B service — that determines which network a service treats as "the Hub." This is the entity at the center of US3 / FR-006 through FR-009.

| Field | Type / Format | Source of Truth | Validation Rule |
|---|---|---|---|
| `variableName` | string constant | `HUB_CHAIN_ID` (and, where applicable, `HUB_BESU_RPC_URL` / `HUB_RPC_URL`) | MUST be sourced exclusively from runtime configuration — never embedded in source or compiled into a binary (FR-006) |
| `defaultValue` | integer | service `getEnv(key, default)` call sites; `.env.*.example` templates; `docker-compose-backend.*.yaml` | MUST be `1337` everywhere it appears (FR-007, SC-002); MUST NOT be `1338` anywhere (FR-008) |
| `resolvedValue` | integer | runtime environment | When explicitly set, MUST match the actual reachable network's chain ID — a mismatch MUST be surfaced, not silently tolerated (Edge Cases) |
| `fallbackBehavior` | enum: `{warn-and-default, fail-fast, silent-default}` | service startup logic | MUST be `warn-and-default` — service starts, defaults to `1337`, and emits a clearly visible structured-log warning naming the variable and the applied default (FR-009); `silent-default` is explicitly PROHIBITED |

**Relationships**: Each service instance (API gateway, payment orchestrator, each of the four entity backend stacks) holds exactly one resolved Hub Network Identity value at runtime (1:1 per process), all of which SHOULD resolve to the same Hub Network Profile (`1337`) for the system to be coherent (US3 Acceptance Scenario 4 / SC-005).

**Validation / consistency rule**: Across all configuration locations enumerated in SC-002 (service defaults, environment templates, compose files), the set of distinct `defaultValue`s MUST be the singleton `{1337}`.

## 4. Deployment Runbook (Document Entity)

The authoritative artifact for US4 — modeled here not as code but as a checklist of required sections, each directly traceable to a functional requirement, so its completeness is verifiable the same way code completeness is.

| Section | Required Content | Traces to |
|---|---|---|
| Startup sequence | Hub network up → shared contracts deployed to Hub → spoke networks up → cross-chain relay configured, in that exact dependency order | FR-010 |
| Validator configuration note | Explicit statement that the sandbox uses a single validator, PLUS a description of multi-validator production requirements | FR-011, US4 Scenario 2 |
| TVL reconciliation baseline | "Zero until first lock" explanation + how to confirm it after a fresh startup, AND how to distinguish "fresh start" from "restart with existing state" | FR-012, US4 Scenario 3, Edge Cases |
| Decommissioning / cutover section | Explicit statement that the prior Hub-on-Spoke-A deployment is retired, its addresses listed as non-authoritative, and the new deployment designated as sole source of truth | FR-015, Edge Cases |
| Stale-configuration guidance | How to detect and correct a locally cached `1338` default | Edge Cases |

**Validation rule**: A reader unfamiliar with the change MUST be able to execute every section without asking a clarifying question (SC-003) — i.e., no section may reference undocumented prerequisite knowledge.

## 5. TVL Reconciliation Baseline (Operational Invariant)

A point-in-time invariant check, not a stored record — modeled here because FR-012 / SC-007 require it to be *verifiable*, which means it needs a precise definition of "before" and "after" states.

| Field | Type / Format | Validation Rule |
|---|---|---|
| `checkpoint` | enum: `{fresh-startup, post-lock-event, restart-with-state}` | Determines which invariant applies — conflating `fresh-startup` with `restart-with-state` is the exact ambiguity the Edge Cases section calls out |
| `expectedHubMintedValue` | numeric (token units) | MUST be `0` when `checkpoint = fresh-startup`; MAY be non-zero when `checkpoint = post-lock-event` or `restart-with-state` |
| `verificationMethod` | description | MUST be documented in the runbook in a way an independent operator can execute (e.g., a specific contract query or CLI invocation) — not just asserted |

**Relationships**: Anchored to a specific Shared Contract Suite instance (the `tCeBM` / Hub-minted token contracts) on a specific Hub Network Profile. Its `fresh-startup` checkpoint is, by definition, the state immediately following the clean-rebuild cutover described in FR-015.
