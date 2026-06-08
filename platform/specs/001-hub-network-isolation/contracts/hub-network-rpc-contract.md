# Interface Contract: Hub Network RPC / Identity

This document specifies the externally observable contract that the new International Hub Besu network MUST satisfy. "External" here means: from the point of view of any Scenario B service, deployment script, the Cacti relay, or an LNet operator following the runbook — i.e., everything *outside* the `hub-besu/` directory itself. This is the contract User Story 1 and FR-001/002/003/004 exist to guarantee.

## 1. Network Identity

| Property | Contract |
|---|---|
| `eth_chainId` (JSON-RPC) | MUST return `1337` (`0x539`) |
| Consensus algorithm | MUST be QBFT (verifiable via `qbft_getValidatorsByBlockNumber` responding successfully; IBFT 2.0 endpoints MUST NOT be the active consensus path) |
| Genesis hash | MUST be distinct from Spoke A's and Spoke B's genesis hashes — proof of "no shared genesis state" (FR-004) |
| Peer set | MUST contain zero peers in common with Spoke A's or Spoke B's `admin_peers` response — proof of "no shared validator nodes / peer connections" (FR-004) |

## 2. Availability / Independence

| Scenario | Contract |
|---|---|
| Hub started; Spoke A and Spoke B stopped | `eth_blockNumber` MUST advance over time (block production continues) and `eth_chainId` MUST keep returning `1337` (US1 Scenario 1 & 2) |
| Spoke A or Spoke B started before Hub is reachable | Spokes MUST start (or fail) on their own merits — Hub reachability MUST NOT be a precondition for spoke startup (Edge Cases) |
| Operator queries network identity | Response MUST unambiguously distinguish the Hub (`1337`) from Spoke A (`1338`) and Spoke B (`1339`) — no overlapping identifiers (US1 Scenario 3) |

## 3. Operational Parameters (parity contract with Spoke A/B)

These values are NOT independently chosen — they are a **parity contract**: the Hub's genesis MUST report the same values Spoke A and Spoke B already report, so that "operationally consistent" (FR-003) is verifiable by direct comparison rather than by trust.

| Parameter | Required value (matches Spoke A & B) |
|---|---|
| `blockperiodseconds` | `2` |
| `epochlength` | `30000` |
| `requesttimeoutseconds` | `4` |
| `gasLimit` | `0x1c9c380` |
| `zeroBaseFee` | `true` |

**Verification**: `curl -s -X POST <HUB_RPC_URL> -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'` MUST return `{"result":"0x539", ...}`; equivalent calls against Spoke A (`0x53a`) and Spoke B (`0x53b`) MUST return distinct results from the same request shape — this triad of calls is the basis of the runbook's and quickstart's independence check.

## 4. Startup Time Budget

| Property | Contract |
|---|---|
| Cold start to "operational" (validator producing blocks + RPC answering `eth_chainId`) | MUST complete in under 10 minutes using only the documented procedure (SC-001) |
