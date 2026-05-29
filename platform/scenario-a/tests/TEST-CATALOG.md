# Test Catalog — CBWeb3 Platform

> **Deliverable 12** · Complete test case inventory
>
> Strategy document: [`scenario-a/docs/test-execution-plan.md`](../docs/test-execution-plan.md)
>
> Legend: **[Implemented]** = test exists and is runnable · **[Planned]** = specified, not yet implemented

---

## Table of Contents

- [Scenario A — Smart Contract Unit Tests](#scenario-a--smart-contract-unit-tests)
  - [HTLC — HashTimeLockedContract](#htlc--hashtimelockedcontract)
  - [tCeBM — TokenizedCentralBankMoney](#tcebm--tokenizedcentralbankmoney)
  - [fCeBM — FiatCentralBankMoney](#fcebm--fiatcentralbankmoney)
  - [IdentityRegistry](#identityregistry)
  - [FXAgreement](#fxagreement)
  - [SpokeBridge](#spokebridge)
- [Scenario A — API Unit Tests](#scenario-a--api-unit-tests)
- [Scenario A — API Integration Tests](#scenario-a--api-integration-tests)
- [Scenario A — E2E Tests](#scenario-a--e2e-tests)
- [Scenario B — Smart Contract Unit Tests (Implemented, Hub Deployment Pending)](#scenario-b--smart-contract-unit-tests)
- [Scenario B — API and E2E Tests (Planned)](#scenario-b--api-and-e2e-tests-planned)

---

## Scenario A — Smart Contract Unit Tests

**Framework**: Foundry (`forge test`)
**Source**: `scenario-a/contracts/test/`
**Run**: `cd scenario-a/contracts && forge test --match-path "test/HashTimeLockedContract.t.sol" -v`

---

### HTLC — HashTimeLockedContract

**File**: `contracts/test/HashTimeLockedContract.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-A-01 | `test_Lock_Success` | Lock — happy path | EVM state updates; HTLC transitions to `LOCKED`; sender balance deducted | [Implemented] |
| UT-SC-A-02 | `test_Lock_EmitsEvent` | Lock — event emission | `HTLCLocked` event emitted with correct `contractId`, `amount`, `hashLock`, `timeLock` | [Implemented] |
| UT-SC-A-03 | `test_Settle_Success` | Settle — correct pre-image | HTLC transitions to `SETTLED`; recipient balance credited | [Implemented] |
| UT-SC-A-04 | `test_Settle_EmitsEvent` | Settle — event emission | `HTLCSettled` event emitted with `contractId` and revealed `secret` | [Implemented] |
| UT-SC-A-05 | `test_Refund_Success` | Refund — after timeLock expiry | HTLC transitions to `REFUNDED`; sender balance restored | [Implemented] |
| UT-SC-A-06 | `test_Refund_EmitsEvent` | Refund — event emission | `HTLCRefunded` event emitted with correct `contractId` | [Implemented] |
| UT-SC-A-07 | `test_Revert_Lock_AlreadyExists` | Lock — duplicate contractId | Transaction reverts; EVM state unchanged | [Implemented] |
| UT-SC-A-08 | `test_Revert_Lock_TimeLockExpired` | Lock — past timeLock | Transaction reverts; `timeLock` must be `> block.timestamp` | [Implemented] |
| UT-SC-A-09 | `test_Revert_Settle_InvalidSecret` | Settle — wrong pre-image | Transaction reverts; funds remain in escrow | [Implemented] |
| UT-SC-A-10 | `test_Revert_Settle_ContractNotLocked` | Settle — non-LOCKED state | Transaction reverts | [Implemented] |
| UT-SC-A-11 | `test_Revert_Refund_TimeLockNotExpired` | Refund — called before expiry | Transaction reverts; time-lock requirement enforced | [Implemented] |
| UT-SC-A-12 | `test_Revert_Refund_ContractNotLocked` | Refund — non-LOCKED state | Transaction reverts | [Implemented] |
| UT-SC-A-13 | `test_Revert_Lock_UnverifiedSender` | Lock — unverified sender | Transaction reverts; IdentityRegistry gatekeeper enforced | [Implemented] |
| UT-SC-A-14 | `test_Revert_Lock_UnverifiedReceiver` | Lock — unverified receiver | Transaction reverts | [Implemented] |
| UT-SC-A-15 | `test_GetLockDetails_NonExisting` | Get — non-existent contractId | Returns zero/default struct; no revert | [Implemented] |
| UT-SC-A-16 | `test_Lock_WithAcceptedAgreement_Success` | Lock — with accepted FX Agreement | Lock succeeds when linked `agreementId` is in `ACCEPTED` state | [Implemented] |
| UT-SC-A-17 | `test_Revert_Lock_AgreementNotAccepted` | Lock — agreement not accepted | Transaction reverts if linked agreement is in `PROPOSED` or `SETTLED` state | [Implemented] |
| UT-SC-A-18 | `test_Revert_Lock_AgreementExpired` | Lock — expired agreement | Transaction reverts if agreement expiry timestamp has passed | [Implemented] |

---

### tCeBM — TokenizedCentralBankMoney

**File**: `contracts/test/TokenizedCentralBankMoney.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-A-19 | `test_InitialState` | Initial state validation | `name`, `symbol`, `admin`, `centralBank` match constructor args; supply = 0 | [Implemented] |
| UT-SC-A-20 | `test_Mint_Success` | Mint — authorized Central Bank | `totalSupply` and recipient balance increase by minted amount; `Transfer(0x0, recipient, amount)` emitted | [Implemented] |
| UT-SC-A-21 | `test_Mint_RevertUnauthorized` | Mint — unauthorized caller | Transaction reverts; only `CENTRAL_BANK_ROLE` holder can mint | [Implemented] |
| UT-SC-A-22 | `test_Burn_Success` | Burn — authorized Central Bank | `totalSupply` and sender balance decrease by burned amount; `Transfer(sender, 0x0, amount)` emitted | [Implemented] |
| UT-SC-A-23 | `test_Burn_RevertUnauthorized` | Burn — unauthorized caller | Transaction reverts | [Implemented] |

---

### fCeBM — FiatCentralBankMoney

**File**: `contracts/test/FiatCentralBankMoney.t.sol`

Shares the same ERC-20 + `CENTRAL_BANK_ROLE` pattern as tCeBM. The test suite validates mint/burn authorization symmetrically. See `FiatCentralBankMoney.t.sol` for function-level test list.

---

### IdentityRegistry

**File**: `contracts/test/IdentityRegistry.t.sol` + `contracts/test/IdentityRegistryCert.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-A-24 | `test_RegisterParticipant_Success` | Register participant — governance | Participant stored with correct role and `NONE` status | [Implemented] |
| UT-SC-A-25 | `test_RegisterParticipant_RevertIf_NotAdmin` | Register — unauthorized caller | Transaction reverts; only `GOVERNANCE_ROLE` can register | [Implemented] |
| UT-SC-A-26 | `test_UpdateStatus_Suspension` | Update status — Verified → Suspended | Status updated; `ParticipantStatusUpdated` event emitted | [Implemented] |
| UT-SC-A-27 | `test_CanTransact_RoleValidation` | canTransact — role check | Returns `true` only for `CENTRAL_BANK` and `COMMERCIAL_BANK` roles with `Verified` status | [Implemented] |
| UT-SC-A-28 | `test_Register_RevertIf_AddressZero` | Register — zero address | Transaction reverts | [Implemented] |
| UT-SC-A-29 | `test_UpdateStatus_RevertIf_NotGovernance` | Update status — unauthorized | Transaction reverts | [Implemented] |
| UT-SC-A-30 | `test_UpdateStatus_Reactivation` | Update status — Suspended → Verified | Status restored; participant can transact again | [Implemented] |
| UT-SC-A-31 | `test_CanTransact_PendingStatus` | canTransact — Pending status | Returns `false`; pending participants cannot transact | [Implemented] |
| UT-SC-A-32 | `test_CanTransact_ExpiredStatus` | canTransact — Expired status | Returns `false` | [Implemented] |
| UT-SC-A-33 | `test_UnregisteredAddress_CannotTransact` | canTransact — unregistered address | Returns `false` | [Implemented] |
| UT-SC-A-34 | `test_GetParticipant_DefaultValues` | Get participant — non-existent | Returns zero/default struct | [Implemented] |
| UT-SC-A-35 | `test_CanTransact_AllTransactableRoles` | canTransact — all Verified roles | `GOVERNANCE`, `CENTRAL_BANK`, `COMMERCIAL_BANK` all return `true` when Verified | [Implemented] |
| UT-SC-A-36 | `test_RegisterParticipant_EmitsEvent` | Register — event emission | `ParticipantRegistered` event emitted with correct address and role | [Implemented] |
| UT-SC-A-37 | `test_UpdateStatus_EmitsEvent` | Update status — event emission | `ParticipantStatusUpdated` event emitted | [Implemented] |
| UT-SC-A-38 | `test_RegisterParticipant_Overwrite` | Register — overwrite existing | Existing participant data replaced by new registration | [Implemented] |
| UT-SC-A-39 | `test_RegisterParticipant_SetsLastUpdateTimestamp` | Register — timestamp set | `lastUpdated` matches `block.timestamp` | [Implemented] |
| UT-SC-A-40 | `test_UpdateStatus_RefreshesLastUpdateTimestamp` | Update status — timestamp refresh | `lastUpdated` updated on status change | [Implemented] |
| UT-SC-A-41 | `test_CanGovern_GovernanceRoles` | canGovern — governance roles | Only `GOVERNANCE` and `CENTRAL_BANK` return `true` | [Implemented] |

---

### FXAgreement

**File**: `contracts/test/FXAgreement.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-A-42 | `test_Propose_Success` | Propose — happy path | Agreement created with `PROPOSED` state; unique `tradeId` assigned | [Implemented] |
| UT-SC-A-43 | `test_Revert_Propose_DuplicateTradeID` | Propose — duplicate tradeId | Transaction reverts | [Implemented] |
| UT-SC-A-44 | `test_Revert_Propose_UnverifiedSender` | Propose — unverified sender | Transaction reverts; IdentityRegistry gatekeeper enforced | [Implemented] |
| UT-SC-A-45 | `test_Revert_Propose_ZeroOriginAmount` | Propose — zero origin amount | Transaction reverts | [Implemented] |
| UT-SC-A-46 | `test_Revert_Propose_ZeroCounterAmount` | Propose — zero counter amount | Transaction reverts | [Implemented] |
| UT-SC-A-47 | `test_Revert_Propose_ExpiredDate` | Propose — past expiry timestamp | Transaction reverts | [Implemented] |
| UT-SC-A-48 | `test_Accept_Success` | Accept — correct counterparty | Agreement transitions `PROPOSED → ACCEPTED` | [Implemented] |
| UT-SC-A-49 | `test_Revert_Accept_WrongCaller` | Accept — wrong counterparty | Transaction reverts; 403 equivalent | [Implemented] |
| UT-SC-A-50 | `test_Revert_Accept_NotProposed` | Accept — non-PROPOSED state | Transaction reverts | [Implemented] |
| UT-SC-A-51 | `test_Revert_Accept_Expired` | Accept — past expiry | Transaction reverts | [Implemented] |
| UT-SC-A-52 | `test_Reject_Success` | Reject — correct counterparty | Agreement transitions `PROPOSED → REJECTED` (terminal) | [Implemented] |
| UT-SC-A-53 | `test_Revert_Reject_WrongCaller` | Reject — wrong caller | Transaction reverts | [Implemented] |
| UT-SC-A-54 | `test_Cancel_Success` | Cancel — initiator cancels PROPOSED | Agreement transitions `PROPOSED → CANCELLED` (terminal) | [Implemented] |
| UT-SC-A-55 | `test_Revert_Cancel_WrongCaller` | Cancel — wrong caller | Transaction reverts | [Implemented] |
| UT-SC-A-56 | `test_Revert_Cancel_NotProposed` | Cancel — non-PROPOSED state | Transaction reverts | [Implemented] |
| UT-SC-A-57 | `test_Settle_Success` | Settle — governance caller on ACCEPTED agreement | Agreement transitions `ACCEPTED → SETTLED` (terminal) | [Implemented] |
| UT-SC-A-58 | `test_Revert_Settle_NotGovernance` | Settle — non-governance caller | Transaction reverts | [Implemented] |
| UT-SC-A-59 | `test_Revert_Settle_NotAccepted` | Settle — non-ACCEPTED state | Transaction reverts | [Implemented] |
| UT-SC-A-60 | `test_ProposeOnBehalf_Success` | ProposeOnBehalf — governance acting on behalf | Agreement created with correct parties | [Implemented] |
| UT-SC-A-61 | `test_Revert_ProposeOnBehalf_NotGovernance` | ProposeOnBehalf — unauthorized | Transaction reverts | [Implemented] |
| UT-SC-A-62 | `test_AcceptOnBehalf_Success` | AcceptOnBehalf — governance acting on behalf | Agreement transitions to `ACCEPTED` | [Implemented] |
| UT-SC-A-63 | `test_Revert_AcceptOnBehalf_NotGovernance` | AcceptOnBehalf — unauthorized | Transaction reverts | [Implemented] |
| UT-SC-A-64 | `test_Revert_AcceptOnBehalf_NotProposed` | AcceptOnBehalf — non-PROPOSED state | Transaction reverts | [Implemented] |
| UT-SC-A-65 | `test_Revert_AcceptOnBehalf_Expired` | AcceptOnBehalf — past expiry | Transaction reverts | [Implemented] |
| UT-SC-A-66 | `test_RejectOnBehalf_Success` | RejectOnBehalf — governance | Agreement transitions to `REJECTED` | [Implemented] |

---

### SpokeBridge

**File**: `contracts/test/SpokeBridge.t.sol`

See `SpokeBridge.t.sol` for function-level test list. Tests cover cross-spoke message authorization, IdentityRegistry verification of sender/recipient.

---

## Scenario A — API Unit Tests

**Framework**: Go `testing` package
**Source**: `scenario-a/backend/services/*/internal/`
**Run**: `cd scenario-a/backend && go test ./...`

| Test ID | File / Function | Title | Expected Result | Status |
|---------|----------------|-------|-----------------|--------|
| UT-API-A-01 | `handlers_test.go / TestHealth` | Health check endpoint | Returns 200 OK with service status | [Implemented] |
| UT-API-A-02 | `handlers_test.go / TestAuthHandlerLogin` | Auth login — valid credentials | Returns 200 with JWT access token and refresh token | [Implemented] |
| UT-API-A-03 | `handlers_test.go / TestAuthHandlerLoginInvalidCases` | Auth login — invalid cases | Returns 400 or 401 for missing/malformed credentials | [Implemented] |
| UT-API-A-04 | `handlers_test.go / TestComplianceHandlerStatus` | Compliance status handler | Returns correct compliance status for known participant | [Implemented] |
| UT-API-A-05 | `handlers_test.go / TestComplianceHandlerMissingSubject` | Compliance status — missing subject | Returns 400 Bad Request | [Implemented] |
| UT-API-A-06 | `handlers_test.go / TestAuthHandlerRefreshSuccess` | Refresh token — valid token | Returns new access token | [Implemented] |
| UT-API-A-07 | `handlers_test.go / TestAuthHandlerRefreshInvalid` | Refresh token — revoked/invalid | Returns 401 Unauthorized | [Implemented] |
| UT-API-A-08 | `handlers_test.go / TestAuthHandlerRefreshMissingBody` | Refresh token — missing body | Returns 400 Bad Request | [Implemented] |
| UT-API-A-09 | `handlers_test.go / TestAuthHandlerRefreshViaCookie` | Refresh token — via cookie | Returns new access token from cookie-based refresh token | [Implemented] |
| UT-API-A-10 | `handlers_test.go / TestAMLScreenApproved` | AML screen — approved participant | Returns `APPROVED` status | [Implemented] |
| UT-API-A-11 | `handlers_test.go / TestAMLScreenFrozenSanctioned` | AML screen — frozen/sanctioned | Returns `BLOCKED` or `SANCTIONED` status | [Implemented] |
| UT-API-A-12 | `handlers_test.go / TestAMLScreenRevokedSanctioned` | AML screen — revoked/sanctioned | Returns blocked status | [Implemented] |
| UT-API-A-13 | `handlers_test.go / TestAMLScreenMissingSubject` | AML screen — missing subject | Returns 400 Bad Request | [Implemented] |
| UT-API-A-14 | `handlers_test.go / TestProvisionParticipantSuccess` | Provision participant — success | Returns 201 Created; participant registered | [Implemented] |
| UT-API-A-15 | `handlers_test.go / TestProvisionParticipantMissingFields` | Provision participant — missing fields | Returns 400 Bad Request | [Implemented] |
| UT-API-A-16 | `handlers_test.go / TestFreezeAccountSuccess` | Freeze account — authorized | Returns 200; participant status updated to Suspended | [Implemented] |
| UT-API-A-17 | `handlers_test.go / TestUnfreezeAccountSuccess` | Unfreeze account — authorized | Returns 200; participant status restored to Verified | [Implemented] |
| UT-API-A-18 | `handlers_test.go / TestOpenAPIWalletBindEndpointIsActive` | Wallet bind endpoint — registered | Endpoint resolves and returns expected response shape | [Implemented] |
| UT-API-A-19 | `auth/internal/domain/roles_test.go` | Role domain — role validation logic | Correct role precedence and permission mapping | [Implemented] |
| UT-API-A-20 | `auth/internal/grpc/server/validate_token_enrich_test.go` | JWT validation — token enrichment | Valid JWT enriched with roles and subject claim | [Implemented] |
| UT-API-A-21 | `auth/internal/grpc/server/login_user_resolve_test.go` | Login — user resolution | User identity correctly resolved from Keycloak response | [Implemented] |
| UT-API-A-22 | `compliance/internal/domain/status_test.go` | Compliance status domain | Status transitions conform to allowed FSM | [Implemented] |
| UT-API-A-23 | `middleware/correlation_test.go` | Correlation ID middleware | `X-Correlation-Id` injected/propagated on every request | [Implemented] |
| UT-API-A-24 | `middleware/auth_test.go` | Auth middleware — missing JWT | Returns 401 Unauthorized | [Implemented] |
| UT-API-A-25 | `governance_test.go / TestGovernance*` | Governance handler | Approve-KYC requires governance role; returns 403 for insufficient role | [Implemented] |
| UT-API-A-26 | `payment-orchestrator/internal/identity/spoke_test.go` | Spoke identity mapping | Correct spoke routing based on participant registry | [Implemented] |
| UT-API-A-27 | FX Agreement creation — initial state | `POST /api/v1/payments/fx/agreements` with valid payload returns 201 with status `PROPOSED` | Returns 201; `status = PROPOSED`; unique `tradeId` generated | [Planned] |
| UT-API-A-28 | FX Agreement acceptance — wrong counterparty | Non-counterparty calls `POST .../accept` | Returns 403 Forbidden | [Planned] |
| UT-API-A-29 | FX Agreement — 404 handling | `GET /api/v1/payments/fx/agreements/{nonExistentId}` | Returns 404 Not Found (not null or 500) | [Planned] |
| UT-API-A-30 | HTLC lock — SHA-256 hash generation | HTLC utility generates correct SHA-256 of secret string | Hash matches expected cryptographic output (REQ-PAY-006) | [Planned] |
| UT-API-A-31 | Token mint — ordering (fiat burn first) | `/api/v1/payments/deposits` records fiat burn before DLT transaction | Fiat burn record created before Zeto mint call | [Planned] |
| UT-API-A-32 | Token mint — rollback on DLT failure | Besu node mocked to revert; fiat burn record must not be persisted | System state consistent between DB and ledger | [Planned] |

---

## Scenario A — API Integration Tests

**Framework**: Go `testing` with live local Devnet infrastructure
**Prerequisites**: `make spoke-all` running; `make pki.check` passes

| Test ID | Title | Preconditions | Steps | Expected Result | Status |
|---------|-------|---------------|-------|-----------------|--------|
| INT-API-A-01 | ZKP Verification — valid proof | Paladin ZKP service running | Call `verifyZKP` via Compliance Service | Returns `Cleared`; proof validated against privacy layer (REQ-COM-002) | [Planned] |
| INT-API-A-02 | ZKP Verification — invalid proof | Paladin ZKP service running | Call `verifyZKP` with invalid proof string | Returns `Blocked`; no downstream DLT calls triggered | [Planned] |
| INT-API-A-03 | SHA-256 hash verification | Hash utility connected | Generate HTLC payload; verify hash consistency | SHA-256 output is deterministic and matches contract expectation | [Planned] |
| INT-API-A-04 | HTLC lock broadcast — Spoke-A | API connected to Spoke-A Besu node | `POST /api/v1/htlc/lock` with valid `agreementId` | Transaction broadcast; API returns `txHash`; on-chain state = `LOCKED` | [Planned] |
| INT-API-A-05 | HTLC internal state sync | Spoke node reachable | Lock → wait 1-block confirmation → check internal DB | DB record status changes to `LOCKED` only after on-chain confirmation | [Planned] |
| INT-API-A-06 | Refresh token revocation | Revoked token in blacklist | `POST /api/v1/auth/refresh` with revoked token | Returns 401 Unauthorized | [Implemented] |
| INT-API-A-07 | AML local screen blocking | Sanctions registry populated with flagged entity | `POST /api/v1/compliance/aml/screen` with flagged counterparty | Returns `Blocked` immediately; no on-chain query made | [Planned] |
| INT-API-A-08 | Account freeze broadcast | Central Bank credentials; compliance service running | `POST /api/v1/compliance/accounts/freeze` | `setBlacklist` transaction broadcast; API returns receipt | [Planned] |
| INT-API-A-09 | Wallet bind — 1:1 mapping conflict | Address `0xABC` already bound to bank-a | Attempt to bind `0xABC` to bank-b | Returns 409 Conflict | [Planned] |
| INT-API-A-10 | FX Agreement creation on-chain | API connected to Besu node | `POST /api/v1/payments/fx/agreements` with valid pair and counterparty | Unique `agreementId` generated from contract event; status = `PROPOSED` | [Planned] |
| INT-API-A-11 | FX Agreement acceptance authorization | Agreement between bank-a and bank-b | Unauthorized user calls `.../accept` | Returns 403 Forbidden | [Planned] |
| INT-API-A-12 | FX Agreement state transition to ACCEPTED | Agreement in `PROPOSED` state | Counterparty bank-b calls `.../accept` | Status transitions to `ACCEPTED`; on-chain state matches | [Planned] |
| INT-API-A-13 | FX Agreement — non-existent ID | DB connected | `GET /api/v1/payments/fx/agreements/invalid-id` | Returns 404 Not Found | [Planned] |
| INT-API-A-14 | Token minting atomic sequence | User has sufficient fiat balance | `POST /api/v1/payments/deposits` → trigger fCeBM mint | Fiat burn completes before Zeto Mint call; collateral locked before issuance | [Planned] |
| INT-API-A-15 | Token minting rollback on DLT failure | Besu node configured to revert | Trigger minting; simulate Zeto Mint failure | Fiat burn record not persisted in DB; system state consistent | [Planned] |
| INT-API-A-16 | ZK-Proof WASM verification | WASM verifier module loaded in Paladin | `POST /api/v1/compliance/kyc/verify-proof` with valid proof | Proof validated by WASM module; user registry updated with Verified flag | [Planned] |
| INT-API-A-17 | JWT signature integrity | Token transfer service running | `POST /api/v1/payments/fx/agreements` with JWT signed by unauthorized key | Returns 401 Unauthorized; transfer logic aborted | [Planned] |
| INT-API-A-18 | FX Agreement oracle timeout handling | FX price oracle simulated unresponsive | `POST /api/v1/payments/fx/agreements` | Returns 503 Service Unavailable; no DB record created | [Planned] |

---

## Scenario A — E2E Tests

**Framework**: Bash scripts (`scenario-a/tryouts/`)
**Prerequisites**: `make spoke-all` completed; all 6 API gateways return 200 on `/healthz`
**On-chain verification**: `eth_getTransactionReceipt` polling + contract read functions

---

### E2E-A-01 — Participant Onboarding and Token Lifecycle (Spoke-A, bank-a)

- **Script**: `tryouts/tryout-spoke-a-bank-a.sh`
- **Status**: [Implemented]
- **Preconditions**: bank-a registered in IdentityRegistry; central-bank-a running on Spoke-A
- **Flow**: Login → onboarding (KYC submission) → compliance approval → wallet bind → mint tCeBM → transfer → balance verification
- **Success Criteria**: participant status = `Verified`; tCeBM balance matches minted amount; audit log records all state transitions

### E2E-A-02 — Participant Onboarding and Token Lifecycle (Spoke-B, bank-b)

- **Script**: `tryouts/tryout-spoke-b-bank-b.sh`
- **Status**: [Implemented]
- **Flow**: Symmetric to E2E-A-01 on Spoke-B (chain 1339); bank-b ↔ central-bank-b

### E2E-A-03 — Cross-Spoke Bilateral HTLC Settlement (Full Atomic Swap)

- **Script**: `tryouts/tryout-fx-agreement-e2e.sh`
- **ID**: E2E-UC01-01
- **Status**: [Implemented]
- **Preconditions**: bank-a and bank-b onboarded and Verified; central-bank-a and central-bank-b running; Cacti relay operational
- **Test Data**: `FX_ORIGIN_AMOUNT=100000`, `FX_COUNTER_AMOUNT=520000`

| Step | Action | System Interaction |
|------|--------|-------------------|
| 1 | Authenticate | bank-a and bank-b login via `/api/v1/auth/login` |
| 2 | Propose FX Agreement | bank-a calls `POST /api/v1/payments/fx/agreements`; status = `PROPOSED` |
| 3 | Accept FX Agreement | bank-b calls `POST .../accept`; status = `ACCEPTED` |
| 4 | Lock Funds (Spoke-A) | bank-a calls `POST /api/v1/htlc/lock`; SHA-256 hash generated; 100000 tCeBMa escrowed |
| 5 | Relay Propagation | Cacti detects `HTLCLocked` event on Spoke-A; notifies bank-b |
| 6 | Lock with Hash (Spoke-B) | bank-b calls `POST /api/v1/htlc/lock-with-hash` with same `hashLock`; 520000 tCeBMb escrowed |
| 7 | Settle | bank-a calls `POST /api/v1/htlc/settle` revealing pre-image; Cacti relay extracts secret and submits to Spoke-A automatically |
| 8 | Verify Settlement | FX Agreement status = `SETTLED`; both balances updated; no funds in escrow |

- **Success Criteria**: both ledgers reflect updated balances; agreement status = `SETTLED`; full lifecycle within 60 seconds

### E2E-A-04 — HTLC Timeout Refund (Safety Mechanism)

- **Script**: `tryouts/tryout-fx-agreement-e2e.sh` (timeout path)
- **ID**: E2E-UC01-02
- **Status**: [Implemented]
- **Preconditions**: bank-a locks funds; bank-b simulated as offline; `timeLock` set to 600 seconds (10 min test)
- **Flow**: Lock → early refund attempt (must revert) → wait for expiry → refund (must succeed)
- **Success Criteria**: early refund reverts; after expiry, full amount returned to bank-a; agreement status = terminal error/expired state

### E2E-A-05 — Escrow Flow (Deposit → Escrow → Redeem)

- **Script**: `tryouts/tryout-escrow-flow.sh`
- **Status**: [Implemented]
- **Preconditions**: bank-a and central-bank-a on Spoke-A; fiat collateral initialized

| Step | Action | System Interaction |
|------|--------|-------------------|
| 1 | Login | bank-a operator login; central-bank-a governance login |
| 2 | Register deposit | bank-a → `POST /internal/v1/payments/deposits` |
| 3 | Approve deposit | CB governance approves; fCeBM minted for bank-a |
| 4 | Request escrow | bank-a → `POST /internal/v1/payments/escrows`; fCeBM burned, tCeBM minted via Zeto |
| 5 | Approve escrow | CB governance approves; Zeto ZKP proof generated |
| 6 | Request redeem | bank-a → `POST /internal/v1/payments/redeems`; Zeto tCeBM transfer + fCeBM re-mint |
| 7 | Approve redeem | CB governance approves |
| 8 | Verify balances | fCeBM restored to original amount; tCeBM = 0 |

- **Success Criteria**: full 15-step lifecycle completes; final fCeBM balance = initial deposit amount; tCeBM balance = 0

### E2E-A-06 — Compliance Participant Screening

- **Script**: `tryouts/tryout-compliance-participants.sh`
- **Status**: [Implemented]
- **Flow**: Register sanctioned participant → attempt FX Agreement with sanctioned counterparty → verify block at pre-agreement stage
- **Success Criteria**: AML block triggered before any `agreementId` is created; no DLT interaction occurs

### E2E-A-07 — Cacti Relay Interoperability Validation

- **Script**: `tryouts/tryout-cacti-interop.sh`
- **Status**: [Implemented]
- **Flow**: Relay health check → `PluginLedgerConnectorBesu` endpoint registration → cross-chain proof store/verify → Socket.IO transport availability
- **Success Criteria**: Relay returns 200 OK health; Cacti plugin endpoints respond correctly; WebSocket subscriptions active on both spokes

### E2E-A-08 — Internal Relay Authentication

- **Script**: `tryouts/tryout-internal-relay-auth.sh`
- **Status**: [Implemented]
- **Flow**: Validate `X-Relay-Auth` header enforcement on internal endpoints (`/internal/v1/payments/*`); unauthenticated calls must return 401
- **Success Criteria**: internal endpoints inaccessible without valid relay auth token

### E2E-A-09 — Emergency Account Freeze (Governance)

- **Script**: Manual / planned automated runner
- **ID**: E2E-UC01-04
- **Status**: [Planned]
- **Preconditions**: bank-a has active HTLC in LOCKED state; central-bank-a admin credentials available
- **Flow**: CB admin issues freeze on bank-a → bank-a attempts new transfer (must be blocked) → CB admin issues unfreeze → verify operations restored
- **Success Criteria**: freeze enforced immediately; post-unfreeze, operations resume without state loss for existing locked funds

---

## Scenario B — Smart Contract Unit Tests

> **Status**: Tests implemented; hub deployment pending. Run with `forge test --match-path "test/AutomatedMarketMaker.t.sol"`.

**File**: `contracts/test/AutomatedMarketMaker.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-01 | `test_AddLiquidity_Success` | Add liquidity — success | LP tokens minted to provider; reserves updated | [Implemented] |
| UT-SC-B-02 | `test_Revert_AddLiquidity_ZeroAmount` | Add liquidity — zero amount | Transaction reverts | [Implemented] |
| UT-SC-B-03 | `test_GetAmountIn_Math` | getAmountIn — constant product math | Returns correct input for given output based on `x·y=k` formula | [Implemented] |
| UT-SC-B-04 | `test_Revert_GetAmountIn_InsufficientLiquidity` | getAmountIn — insufficient liquidity | Reverts with `INSUFFICIENT_LIQUIDITY` when output exceeds reserves | [Implemented] |
| UT-SC-B-05 | `test_SwapTokensForExactTokens_Success` | Exact-output swap — happy path | Beneficiary receives exact requested output; payer debited correct input; pool reserves updated | [Implemented] |
| UT-SC-B-06 | `test_Revert_Swap_SlippageExceeded` | Exact-output swap — slippage exceeded | Transaction reverts; no tokens transferred | [Implemented] |
| UT-SC-B-07 | `test_Revert_Swap_InvalidToken` | Swap — invalid token address | Transaction reverts | [Implemented] |
| UT-SC-B-08 | `test_CircuitBreaker_Pause_Success` | Circuit breaker — governance pause | Contract state = `PAUSED`; all swaps revert | [Implemented] |
| UT-SC-B-09 | `test_CircuitBreaker_Unpause_Success` | Circuit breaker — governance unpause | Contract state = `ACTIVE`; swaps resume | [Implemented] |
| UT-SC-B-10 | `test_Revert_CircuitBreaker_Unauthorized` | Circuit breaker — unauthorized caller | Transaction reverts; only `CENTRAL_BANK` or `GOVERNANCE` can pause | [Implemented] |
| UT-SC-B-11 | `test_Revert_Swap_ZeroAmount` | Swap — zero amount | Transaction reverts | [Implemented] |
| UT-SC-B-12 | `test_SwapTokensForExactTokens_ReverseDirection` | Exact-output swap — reverse direction (tCeBMb→tCeBMa) | Correct pricing in reverse direction | [Implemented] |
| UT-SC-B-13 | `test_Revert_Constructor_ZeroAddressTokenA` | Constructor — zero address tokenA | Deployment reverts | [Implemented] |
| UT-SC-B-14 | `test_Revert_Constructor_ZeroAddressTokenB` | Constructor — zero address tokenB | Deployment reverts | [Implemented] |
| UT-SC-B-15 | `test_Revert_Constructor_ZeroAddressIdentityRegistry` | Constructor — zero address registry | Deployment reverts | [Implemented] |
| UT-SC-B-16 | `test_Revert_AddLiquidity_UnverifiedCaller` | Add liquidity — unverified caller | Transaction reverts; IdentityRegistry check enforced | [Implemented] |
| UT-SC-B-17 | `test_Revert_Swap_UnverifiedSender` | Swap — unverified sender | Transaction reverts | [Implemented] |
| UT-SC-B-18 | `test_Revert_Swap_UnverifiedTo` | Swap — unverified recipient | Transaction reverts | [Implemented] |

---

## Scenario B — API and E2E Tests (Planned)

> These tests will be implemented when the hub network (chain 1337) is deployed. Smart contracts are ready; API endpoints and integration layer are pending.

### API Integration Tests — AMM (Planned)

| Test ID | Title | Status |
|---------|-------|--------|
| INT-API-B-01 | AMM Agreement Price Quoting — live AMM contract | [Planned] |
| INT-API-B-02 | HTLC Lock Payload Metadata Integrity — Target Chain ID + Beneficiary Address in calldata | [Planned] |
| INT-API-B-03 | Exact Output Quote Calculation — `GET /amm/quote/exact-output` | [Planned] |
| INT-API-B-04 | Insufficient Liquidity Error Mapping — contract revert → user-friendly JSON error | [Planned] |
| INT-API-B-05 | Swap Execution with ZK-Pointers — Paladin privacy layer active | [Planned] |
| INT-API-B-06 | Slippage Revert Error Handling — API returns 422 with slippage message | [Planned] |
| INT-API-B-07 | Pool Imbalance Status Detection — `GET /amm/pool/{pair}/status` | [Planned] |
| INT-API-B-08 | Liquidity Addition Approval Orchestration — multi-step ERC-20 allowance | [Planned] |
| INT-API-B-09 | Circuit Breaker RBAC Enforcement — `COMMERCIAL_BANK` role receives 403 | [Planned] |
| INT-API-B-10 | Circuit Breaker Maintenance Mode Transition — `CENTRAL_BANK_ADMIN` pauses AMM | [Planned] |

### E2E Tests — Scenario B (Planned)

| Test ID | Title | Flow | Status |
|---------|-------|------|--------|
| E2E-B-01 | Exact-Output Swap (Happy Path) | CommA bridges tCeBMa to hub → QuoteService returns required input → swap executes atomically → CommB receives exact tCeBMb | [Planned] |
| E2E-B-02 | Slippage Protection (Volatility Test) | Quote obtained → secondary actor depletes pool → original swap submitted → contract reverts; no tokens transferred | [Planned] |
| E2E-B-03 | Governance Circuit Breaker (Emergency Stop) | CB Admin calls `setPause(true)` → all swaps and LP operations revert → `setPause(false)` → operations resume without state loss | [Planned] |
| E2E-B-04 | Liquidity Imbalance Alert (Monitoring) | Large swap skews pool past 70/30 threshold → `ImbalanceAlert` event emitted → ImbalanceMonitor API captures event and sends notification to MLP → MLP adds liquidity to rebalance | [Planned] |
