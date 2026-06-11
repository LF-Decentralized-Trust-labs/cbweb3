# Interface Contract: Service → Hub Configuration Resolution

This document specifies the configuration-resolution contract every Scenario B service MUST satisfy when determining "which network is the Hub." This is the contract User Story 3 and FR-006 through FR-009 exist to guarantee — it is what actually closes the defect described in the feature input (services silently resolving to `1338`).

## 1. Resolution Source

| Rule | Contract |
|---|---|
| Source of truth | The Hub's network identity (`HUB_CHAIN_ID`) and endpoint (`HUB_BESU_RPC_URL` / `HUB_RPC_URL`) MUST be read exclusively from runtime configuration (environment variables, mounted config files) — never from a literal embedded in source code or compiled into a binary (FR-006) |
| Applies to | API Gateway, Payment Orchestrator, and all four entity backend stacks (`central-bank-a`, `central-bank-b`, `bank-a`, `bank-b`) |

## 2. Default Value

| Rule | Contract |
|---|---|
| Default | Every default-supplying location (Go `getEnv` call sites, `.env.infra.*.example` templates, `docker-compose-backend.*.yaml` environment blocks) MUST default `HUB_CHAIN_ID` to `1337` (FR-007) |
| Prohibition | No default-supplying location may resolve to `1338` for the Hub's identity, whether as a literal default or as a fallback to a Spoke A-specific variable (FR-008) — this is the exact defect currently present in `make/30-contracts.mk`'s `contracts.deploy-hub` target and `identity_bootstrap.go`'s `int64(1338) // Hub default` |
| Verification | SC-002: a repository-wide search for the Hub's default value MUST return `1337` at 100% of locations and `1338` at 0% |

## 3. Fallback Behavior (the "non-silent default" contract)

This is the most behaviorally significant clause — it is what distinguishes "configurable with a documented default" (acceptable) from "hardcoded" (prohibited), per the spec's Assumptions section.

| Condition | Required behavior |
|---|---|
| `HUB_CHAIN_ID` present and valid | Service uses the configured value; no warning emitted |
| `HUB_CHAIN_ID` absent | Service (a) MUST NOT fail to start, (b) MUST fall back to `1337`, (c) MUST emit a structured-log warning that names the variable and states the default applied (FR-009) — satisfying Constitution Principle VI's "no silent failures" rule |
| `HUB_CHAIN_ID` present but pointing at an unreachable or mismatched network | System MUST surface the mismatch clearly (e.g., comparing configured vs. actual `eth_chainId` from the endpoint) rather than silently operating against the wrong ledger (Edge Cases) |

**Log shape contract** (Constitution Principle VI — structured JSON, correlation ID, service name, severity, ISO-8601 timestamp): the warning MUST be emitted through the same structured logger already used for other startup diagnostics — e.g.
```json
{"level":"warn","service":"payment-orchestrator","msg":"HUB_CHAIN_ID not set; defaulting to 1337","ts":"2026-06-08T12:00:00Z"}
```

## 4. End-to-End Observability Contract

| Rule | Contract |
|---|---|
| On-chain activity location | After configuration changes are applied, any end-to-end flow that exercises the Hub (e.g., a cross-currency AMM swap) MUST produce 100% of its Hub-side on-chain activity on chain `1337` and 0% on chain `1338` (FR-014, SC-005) |
| Verification method | Compare relay/event logs' `chainId` field (or the RPC endpoint they were captured from) against `1337` for every Hub-attributed event in the flow |

## 5. Call-Site Inventory (for test coverage — Constitution Principle V)

Each row below is a location this contract binds; each MUST have a test that asserts both the default-value and the fallback-warning behavior (red→green, written before the implementing change):

| File | Current state | Required end state |
|---|---|---|
| `payment-orchestrator/cmd/payment-orchestrator/main.go:205` | `getEnv("HUB_CHAIN_ID", "1338")`, no warning on default | `getEnv("HUB_CHAIN_ID", "1337")` + warning emitted when default applied |
| `api-gateway/internal/app/app.go:234` | reads `HUB_CHAIN_ID` directly via `os.Getenv`, no documented default/warning | resolves via the same default-plus-warning pattern, defaulting to `1337` |
| `api-gateway/internal/app/identity_bootstrap.go` | `chainID := int64(1338) // Hub default` (hardcoded literal) | sourced from configuration, defaulting to `1337` with a warning when absent |
