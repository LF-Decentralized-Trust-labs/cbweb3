# Quickstart: Validating an Independent International Hub

This is a condensed, runnable verification sequence — distinct from the full deployment runbook (FR-010, which a partner uses to *build* the environment from scratch). Use this quickstart to confirm, after the implementation lands, that the Hub genuinely satisfies the independence and correctness claims in the spec. Each step maps directly to a User Story's "Independent Test" and its acceptance scenarios — run them in order; each is a gate for the next.

## Prerequisites

- Branch `001-hub-network-isolation` checked out, implementation applied
- Docker Compose and `forge` available locally
- No assumption of Spoke A / Spoke B / backend services running yet (that's the point of Step 1)

## Step 1 — Hub starts and runs independently (validates US1 / FR-001, FR-002, FR-004)

```bash
cd scenario-b
make deploy.up-hub-besu        # brings up ONLY the Hub network — no spokes, no backend
```

Then verify:
```bash
curl -s -X POST http://<HUB_RPC_URL> -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
# Expect: {"result":"0x539", ...}   (0x539 == 1337)

curl -s -X POST http://<HUB_RPC_URL> -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
# Re-run after a few seconds — block number MUST advance (validator producing blocks)
```

**Pass condition**: chain ID is `1337`, blocks are advancing, and Spoke A / Spoke B remain stopped throughout — proving the Hub is not a hidden dependent of either (Edge Cases, US1 Scenario 2).

## Step 2 — Shared contracts deploy to and live on the Hub (validates US2 / FR-005)

```bash
make contracts.deploy-hub      # MUST target the new hub-besu RPC, not Spoke A's
```

Then verify each contract from the suite (AMM, Liquidity Pool/pair/currency registries, identity registry, FX agreement, shared tokens — see `data-model.md` §2 for the full list):
```bash
cast code <AMM_ADDRESS> --rpc-url <HUB_RPC_URL>
# Expect: non-empty bytecode matching the compiled artifact — "live and operable" (US2 Scenario 2)

cast call <TCEBM_ADDRESS> "totalSupply()(uint256)" --rpc-url <HUB_RPC_URL>
# Expect: 0 on a fresh deployment — the TVL reconciliation baseline (US2 Scenario 3, FR-012, SC-007)
```

**Pass condition**: every contract returns a distinct, non-zero address with matching deployed bytecode on chain `1337`, and Hub-minted token supply is `0` before any cross-chain activity.

## Step 3 — Services resolve the Hub from configuration, defaulting to 1337 (validates US3 / FR-006–FR-009)

```bash
# 3a. Confirm no stale defaults remain anywhere in the repo
grep -rn "HUB_CHAIN_ID" scenario-b/backend scenario-b/contracts scenario-b/make | grep -c "1338"
# Expect: 0   (SC-002 — zero locations default to 1338)

grep -rn "HUB_CHAIN_ID" scenario-b/backend scenario-b/contracts scenario-b/make | grep -c "1337"
# Expect: matches the count of all default-supplying locations (100% default to 1337)
```

```bash
# 3b. Start a service WITHOUT HUB_CHAIN_ID set, confirm non-silent fallback
unset HUB_CHAIN_ID
go run ./scenario-b/backend/services/payment-orchestrator/cmd/payment-orchestrator
# Expect in stdout (structured JSON log): a "warn" entry naming HUB_CHAIN_ID and stating
# the 1337 default was applied — service MUST still start successfully (FR-009)
```

```bash
# 3c. Start the full stack with standard config (no overrides), confirm resolution
make scenario-b.up-infra && make scenario-b.deploy-contracts
# Then inspect a running service's resolved configuration / logs:
# Expect HUB_CHAIN_ID resolves to 1337, sourced from configuration (US3 Scenario 1)
```

**Pass condition**: zero `1338` defaults remain for the Hub identity; absence of the variable produces a warning + safe default, never a crash or a silent `1338` assumption.

## Step 4 — End-to-end activity actually lands on chain 1337 (validates US3 Scenario 4 / FR-014, SC-005, and Constitution Principle III)

```bash
# Exercise a cross-currency swap through the AMM (per the existing E2E/smoke suite)
make scenario-b.smoke-test     # or the equivalent documented E2E entrypoint
```

Then cross-check the relay/event logs:
```bash
grep -n "chainId" <relay-log-output> | sort | uniq -c
# Expect: 100% of Hub-attributed events show chainId 1337; 0% show 1338 (SC-005)
```

This step doubles as the first real exercise of the Constitution's Scenario B relay-verification clause (Principle III: "Spoke lock event → Hub mint, Hub burn event → Spoke unlock") across two genuinely distinct chains — something that was structurally impossible to test meaningfully while "Hub" and "Spoke A" were the same chain.

**Pass condition**: 100% of Hub-side on-chain activity for the exercised flow occurs on `1337`; the lock→mint / burn→unlock relay verification and circuit-breaker checks complete successfully across the Hub ↔ Spoke A and Hub ↔ Spoke B boundaries.

## Step 5 — Scenario A is unaffected (validates US5 / FR-013, SC-006)

```bash
make scenario-a.test           # or the equivalent documented Scenario A check entrypoint
```

**Pass condition**: Scenario A's checks pass at the same rate as on `main` — no new failures attributable to this branch.

## Step 6 — Runbook reproducibility spot-check (validates US4 / FR-010–FR-012, FR-015)

Hand `scenario-b/docs/runbooks/deployment-runbook.md` to someone who hasn't touched this branch and have them execute Steps 1–4 above using *only* the runbook's instructions (not this quickstart). Confirm they can do so without asking a clarifying question, and that the runbook explicitly states (and they can locate):
- the single-validator sandbox caveat and what production would additionally require (FR-011),
- the "zero until first lock" TVL baseline and how to confirm it (FR-012),
- that the old Hub-on-Spoke-A deployment is retired/non-authoritative, with its addresses listed (FR-015).

**Pass condition**: a full, unaided run-through (SC-003) — this is the basis for the LNet sign-off referenced in the spec's Assumptions.
