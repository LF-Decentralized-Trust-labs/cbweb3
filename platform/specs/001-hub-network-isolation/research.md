# Phase 0 Research: International Hub Network Isolation

All technical unknowns for this feature were resolved by examining the existing, proven `spoke-besu-a`/`spoke-besu-b` deployment pattern and the `/speckit.clarify` session — no item below required external research. There are no remaining `NEEDS CLARIFICATION` markers.

## Decision 1: Mirror the spoke-besu-a/b deployment pattern for the Hub

- **Decision**: Create `scenario-b/deploy/local/hub-besu/` as a structural mirror of `scenario-b/deploy/local/spoke-besu-a/` — same file set (`.env.network`, `startBesu.sh`, `stopBesu.sh`, `config/configTemplate.json`, `config/qbftConfigFile.json`, `genesis/genesis.json`, `nodes/`), with `chainId: 1337`, `NETWORK_NAME=hub_besu_network`, `CONTAINER_PREFIX=cbweb3-hub-besu`, and a dedicated RPC/P2P/WS port range that doesn't collide with Spoke A (`86xx`) or Spoke B (`87xx`).
- **Rationale**: This pattern is already proven twice over (two independent networks follow it today). Replicating it keeps the Hub "topologically separate" (FR-004) while staying "operationally consistent" (FR-003), and it is immediately familiar to anyone — including LNet partners — who already knows how to operate a spoke.
- **Alternatives considered**:
  - *Reuse/extend Spoke B's network for the Hub* — rejected: this just relocates the same defect (Hub-on-a-domestic-chain) onto a different spoke; it does not produce a neutral layer.
  - *Adopt a different orchestration approach (e.g., a new IaC tool)* — rejected as unjustified complexity; the constitution's Technology Stack Constraints fix Docker Compose + Besu/QBFT, and introducing a new runtime dependency would require documented justification this feature has no grounds to provide.

## Decision 2: Genesis parameters mirror Spoke A/B exactly, except chainId

- **Decision**: The Hub genesis uses `blockperiodseconds: 2`, `epochlength: 30000`, `requesttimeoutseconds: 4`, `gasLimit: "0x1c9c380"`, `zeroBaseFee: true`, and the same fork-block configuration as the spokes — only `chainId` changes, to `1337`.
- **Rationale**: Verified by direct comparison of `spoke-besu-a/config/configTemplate.json` (chainId 1338) and `spoke-besu-b/config/configTemplate.json` (chainId 1339): every QBFT/gas/timing parameter is identical between the two existing networks. The feature input explicitly calls for mirroring these values "to keep the networking layer consistent across the prototype" (FR-003).
- **Alternatives considered**: Tuning Hub-specific block time or gas limit for higher AMM throughput — rejected; this would introduce exactly the cross-network inconsistency the input warns against, and no requirement calls for different Hub performance characteristics.

## Decision 3: Single validator for the sandbox Hub, explicitly documented

- **Decision**: The Hub runs with one validator/bootnode in local/sandbox environments (mirroring the `central-bank-a` bootnode role in `spoke-besu-a`). The runbook documents this explicitly (FR-011), alongside what a multi-validator (≥4 nodes, to tolerate `f` Byzantine failures under QBFT's `n ≥ 3f+1` rule) production configuration would additionally require.
- **Rationale**: Matches the feature input's explicit guidance that "a single node is acceptable for prototype purposes, as long as it's documented." Avoids over-engineering a sandbox network whose purpose is to prove topological independence, not production resilience.
- **Alternatives considered**: Mirror the spokes' 3-node topology for the Hub — rejected for the sandbox; it adds setup and resource cost without advancing the validation goal (independence), though the runbook must make the gap explicit so the sandbox setup is never mistaken for a production-ready one.

## Decision 4: Non-silent configuration-fallback pattern in Go services

- **Decision**: `payment-orchestrator/cmd/payment-orchestrator/main.go:205` and `api-gateway/internal/app/app.go:234` continue to read `HUB_CHAIN_ID` from the environment, defaulting to `1337` when absent — but now also emit a warning naming the variable and the default applied. `identity_bootstrap.go`'s inline `chainID := int64(1338) // Hub default` is replaced with the same default-plus-warning treatment, sourced from configuration rather than hardcoded.
- **Rationale**: This satisfies FR-009 ("must not fail to start … must emit a clearly visible warning") using a pattern the codebase already mostly has — `main.go` already uses a `getEnv(key, default)` helper for other variables (e.g., `SPOKE_CHAIN_ID`); the only gap is that it doesn't currently log when a default is applied. Adding the missing log line at the three call sites is the minimal correct fix and matches Constitution Principle VI's "no silent failures" rule, using the existing structured-JSON logger.
- **Alternatives considered**: Centralizing all chain-ID resolution into a new shared config package — rejected as overreach for three call sites in one scenario; Constitution Principle I prohibits new *cross*-scenario shared libraries, and even a same-scenario package would be a premature abstraction here ("three similar lines is better than a premature abstraction").

## Decision 5: Contract redeployment via existing Foundry scripts, retargeted only

- **Decision**: `contracts/script/CBWeb3Hub.s.sol:DeployCBWeb3Hub` is reused unchanged. Only the deployment *target* changes: `make/30-contracts.mk`'s `contracts.deploy-hub` target stops defaulting `BESU_HUB_RPC` to `$(SPOKE_A_RPC_URL)` (currently at line 59, with an accompanying "WARNING: BESU_HUB_RPC not set, defaulting to Spoke-A RPC" message that exists specifically because this defect was already known) and instead targets the new `hub-besu` RPC endpoint; `contracts/.env.example` gets `HUB_RPC_URL` updated to point at it and `HUB_CHAIN_ID=1337`.
- **Rationale**: The contract source is already correct and already covered by Foundry tests (Constitution Principle V) — the defect is purely *where* they get deployed, not what gets deployed. `make/30-contracts.mk:59` is the literal, smoking-gun mechanism that bakes "Hub == Spoke A" into the deployment layer; removing that fallback (rather than papering over it) is the minimal correct fix and directly satisfies FR-008 ("No part of the system may assume the Hub's network identity is `1338`… as a fallback").
- **Alternatives considered**: Writing new Hub-specific deployment scripts — rejected; it would duplicate proven, tested logic for no functional gain, and would itself need new Foundry tests under Principle V, adding risk without benefit.

## Decision 6: Decommissioning approach for the existing Hub-on-Spoke-A deployment

- **Decision**: No state-migration tooling is built. The runbook instructs operators to perform a full clean rebuild (stop everything, bring the new Hub network up first, redeploy the shared contract suite to it, then bring up the spokes and relay) and explicitly documents the prior chain-1338 "hub" contract addresses as retired/non-authoritative. The existing `contracts.sync-addresses` target — already chained into `scenario-b.deploy-contracts` — naturally overwrites the stale addresses in `.env.infra.*` and the Cacti relay's `.env` with the new chain-1337 addresses, so no new address-propagation mechanism is required.
- **Rationale**: This directly implements the `/speckit.clarify` resolution (clean rebuild, no migration — FR-015). `contracts.sync-addresses` already performs exactly the address-propagation step needed; building anything new here would duplicate working infrastructure.
- **Alternatives considered**: Building a state-export/import tool for the legacy "hub" deployment — explicitly rejected by the clarification; there is no production data on this prototype worth preserving, and migration tooling for a known-misconfigured deployment would add risk and maintenance burden for zero lasting value.

## Outstanding NEEDS CLARIFICATION

None. Every technical unknown was resolved either against an existing, working pattern in the codebase (spoke genesis configs, the `getEnv` helper, the Foundry deploy scripts, `contracts.sync-addresses`) or via the single `/speckit.clarify` question already answered (clean-rebuild cutover).
