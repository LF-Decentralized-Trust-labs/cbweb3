# Test Catalog — CBWeb3 Platform (Scenario B)

> **Deliverable 12** · Complete test case inventory — Scenario B: AMM Hub and Cross-Currency Liquidity
>
> Strategy document: [`scenario-b/docs/test-execution-plan.md`](../docs/test-execution-plan.md)
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29
>
> Legend: **[Implemented]** = test exists and is runnable · **[Partial]** = test partially covers the scenario · **[Planned]** = specified, not yet implemented

---

> **Work in Progress.** Smart contract unit tests are fully implemented. Backend API unit tests and integration tests are partially implemented. Several integration and E2E areas are Planned. See the [Known Test Coverage Gaps](../docs/test-execution-plan.md#known-test-coverage-gaps) section in the execution plan for details.

---

## Table of Contents

- [Section 1 — Scenario B Smart Contract Unit Tests (Foundry)](#section-1--scenario-b-smart-contract-unit-tests-foundry)
  - [AutomatedMarketMaker](#automatedmarketmaker)
  - [AutomatedMarketMakerCooperative](#automatedmarketmakercooperative)
  - [LiquidityCommitRegistry](#liquiditycommitregistry)
  - [PairRegistry](#pairregistry)
  - [ManualOracle](#manualoracle)
  - [FXAgreement (Hub)](#fxagreement-hub)
  - [SpokeBridge](#spokebridge)
  - [CurrencyRegistry](#currencyregistry)
  - [IdentityRegistry (Hub)](#identityregistry-hub)
  - [TokenizedCentralBankMoney (Hub)](#tokenizedcentralbankmoney-hub)
  - [HashTimeLockedContract (Hub)](#hashtimelockedcontract-hub)
  - [CBWeb3Hub Deployment](#cbweb3hub-deployment)
- [Section 2 — Backend API Unit Tests (Go)](#section-2--backend-api-unit-tests-go)
- [Section 3 — Integration Tests](#section-3--integration-tests)
- [Section 4 — E2E Tests](#section-4--e2e-tests)

---

## Section 1 — Scenario B Smart Contract Unit Tests (Foundry)

**Framework**: Foundry (`forge test`)
**Source**: `scenario-b/contracts/test/`
**Run**: `make scenario-b.test-contracts`

```bash
# Full suite
cd scenario-b/contracts && forge test -vv

# Individual file
forge test --match-path "test/AutomatedMarketMaker.t.sol" -vv
```

---

### AutomatedMarketMaker

**File**: `contracts/test/AutomatedMarketMaker.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-AMM-01 | `test_AddLiquidity_Success` | Add liquidity — happy path | LP tokens minted to provider; both reserve balances increase; `LiquidityAdded` event emitted | [Implemented] |
| UT-SC-B-AMM-02 | `test_Revert_AddLiquidity_ZeroAmount` | Add liquidity — zero amount | Transaction reverts; pool state unchanged | [Implemented] |
| UT-SC-B-AMM-03 | `test_Revert_AddLiquidity_UnverifiedCaller` | Add liquidity — unverified caller | Transaction reverts; IdentityRegistry check enforced | [Implemented] |
| UT-SC-B-AMM-04 | `test_RemoveLiquidity_Success` | Remove liquidity — happy path | LP tokens burned; both token balances returned proportionally; `LiquidityRemoved` event emitted | [Implemented] |
| UT-SC-B-AMM-05 | `test_GetAmountIn_Math` | getAmountIn — constant-product math | Returns correct `amountIn` for given `amountOut` using `x*y=k`; tolerance = 0 wei | [Implemented] |
| UT-SC-B-AMM-06 | `test_Revert_GetAmountIn_InsufficientLiquidity` | getAmountIn — insufficient liquidity | Reverts with `INSUFFICIENT_LIQUIDITY` when `amountOut` >= reserve | [Implemented] |
| UT-SC-B-AMM-07 | `test_SwapExactOutput_Success` | swapExactOutput — happy path | Beneficiary receives exact `amountOut`; payer debited correct `amountIn`; pool reserves updated; `Swap` event emitted | [Implemented] |
| UT-SC-B-AMM-08 | `test_SwapExactOutput_ReverseDirection` | swapExactOutput — reverse direction (tokenB → tokenA) | Correct pricing and reserve update in reverse direction | [Implemented] |
| UT-SC-B-AMM-09 | `test_Revert_Swap_SlippageExceeded` | swapExactOutput — slippage exceeded | Transaction reverts when `amountIn > maxAmountIn`; no tokens transferred | [Implemented] |
| UT-SC-B-AMM-10 | `test_Revert_Swap_ZeroAmount` | swapExactOutput — zero output amount | Transaction reverts | [Implemented] |
| UT-SC-B-AMM-11 | `test_Revert_Swap_InvalidToken` | swapExactOutput — invalid token address | Transaction reverts | [Implemented] |
| UT-SC-B-AMM-12 | `test_Revert_Swap_UnverifiedSender` | swapExactOutput — unverified sender | Transaction reverts; IdentityRegistry check enforced | [Implemented] |
| UT-SC-B-AMM-13 | `test_Revert_Swap_UnverifiedTo` | swapExactOutput — unverified recipient | Transaction reverts | [Implemented] |
| UT-SC-B-AMM-14 | `test_CircuitBreaker_Pause_Success` | Circuit breaker — governance pause | `setPause(true)` succeeds; all swaps and liquidity operations revert with `ContractPaused`; `Paused` event emitted | [Implemented] |
| UT-SC-B-AMM-15 | `test_CircuitBreaker_Unpause_Success` | Circuit breaker — governance unpause | `setPause(false)` succeeds; swaps and liquidity operations resume; `Unpaused` event emitted | [Implemented] |
| UT-SC-B-AMM-16 | `test_Revert_CircuitBreaker_Unauthorized` | Circuit breaker — unauthorized caller | `setPause` reverts for caller without `CENTRAL_BANK_ROLE` or `GOVERNANCE_ROLE` | [Implemented] |
| UT-SC-B-AMM-17 | `test_FeeManagement_CorrectDeduction` | Fee math — fee deducted from amountIn | Actual `amountIn` includes fee component; net received by pool matches expected formula | [Implemented] |
| UT-SC-B-AMM-18 | `test_Revert_Constructor_ZeroAddressTokenA` | Constructor — zero address tokenA | Deployment reverts | [Implemented] |
| UT-SC-B-AMM-19 | `test_Revert_Constructor_ZeroAddressTokenB` | Constructor — zero address tokenB | Deployment reverts | [Implemented] |
| UT-SC-B-AMM-20 | `test_Revert_Constructor_ZeroAddressIdentityRegistry` | Constructor — zero address registry | Deployment reverts | [Implemented] |
| UT-SC-B-AMM-21 | `test_ConstantProduct_InvariantFuzz` | Constant-product invariant — fuzz | After any swap, `reserveA * reserveB >= k_before`; invariant never violated across >= 10 000 runs | [Implemented] |
| UT-SC-B-AMM-22 | `test_Revert_Swap_WhenPaused` | swapExactOutput — paused state | Swap reverts while circuit breaker is active; no pool state changes | [Implemented] |

---

### AutomatedMarketMakerCooperative

**File**: `contracts/test/AutomatedMarketMakerCooperative.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-AMMC-01 | `test_MultiPairSupport` | Multi-pair AMM — distinct pairs maintain separate reserves | Two separate BRL-USD and EUR-USD pools have independent reserve states | [Implemented] |
| UT-SC-B-AMMC-02 | `test_PairRegistry_Integration` | PairRegistry integration — confirmed pair enables AMM operations | Liquidity can only be added for a pair that is `ACTIVE` in PairRegistry | [Implemented] |
| UT-SC-B-AMMC-03 | `test_Revert_AddLiquidity_UnconfirmedPair` | Multi-pair — unconfirmed pair rejects liquidity | Adding liquidity for a `PROPOSED` (not `ACTIVE`) pair reverts | [Implemented] |

---

### LiquidityCommitRegistry

**File**: `contracts/test/LiquidityCommitRegistry.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-LCR-01 | `test_RegisterCommit_Success` | registerCommit — happy path | Commit stored with correct pair, amount, provider, and expiry (`block.timestamp + 72h`); `CommitRegistered` event emitted | [Implemented] |
| UT-SC-B-LCR-02 | `test_RegisterCommit_BothSides_EmitsCommitMatched` | registerCommit — both sides trigger CommitMatched | When both counterparties register matching commits, `CommitMatched` event is emitted with correct commit IDs | [Implemented] |
| UT-SC-B-LCR-03 | `test_CancelCommit_Success` | cancelCommit — provider cancels own commit | Commit transitions to `CANCELLED`; provider cannot be charged; `CommitCancelled` event emitted | [Implemented] |
| UT-SC-B-LCR-04 | `test_Revert_CancelCommit_WrongCaller` | cancelCommit — wrong caller | Transaction reverts; only commit provider can cancel | [Implemented] |
| UT-SC-B-LCR-05 | `test_ExpireCommit_After72h` | expireCommit — after 72h TTL | Commit transitions to `EXPIRED` when called after `expiryTimestamp`; `CommitExpired` event emitted | [Implemented] |
| UT-SC-B-LCR-06 | `test_Revert_ExpireCommit_BeforeTTL` | expireCommit — before TTL | Transaction reverts when called before `expiryTimestamp` | [Implemented] |
| UT-SC-B-LCR-07 | `test_Revert_RegisterCommit_Duplicate` | registerCommit — duplicate registration | Transaction reverts when provider already has an active commit for the same pair and side | [Implemented] |
| UT-SC-B-LCR-08 | `test_Revert_RegisterCommit_UnverifiedProvider` | registerCommit — unverified provider | Transaction reverts; only verified participants can register commits | [Implemented] |

---

### PairRegistry

**File**: `contracts/test/PairRegistry.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-PAR-01 | `test_ProposePair_Success` | proposePair — happy path | Pair stored with `PROPOSED` state; unique `pairId` assigned; `PairProposed` event emitted with correct currencyA, currencyB, proposer | [Implemented] |
| UT-SC-B-PAR-02 | `test_ConfirmPair_Success` | confirmPair — bilateral approval | Counterparty calls `confirmPair`; pair transitions to `ACTIVE`; `PairConfirmed` event emitted | [Implemented] |
| UT-SC-B-PAR-03 | `test_Revert_ConfirmPair_WrongCaller` | confirmPair — wrong counterparty | Transaction reverts; only designated counterparty can confirm | [Implemented] |
| UT-SC-B-PAR-04 | `test_Revert_ConfirmPair_Unilateral` | confirmPair — proposer self-confirms | Transaction reverts; proposer cannot confirm their own proposal | [Implemented] |
| UT-SC-B-PAR-05 | `test_GetAllActivePairs_ReturnsOnlyActive` | getAllActivePairs — filters correctly | Returns only pairs in `ACTIVE` state; `PROPOSED` and `CANCELLED` pairs excluded | [Implemented] |
| UT-SC-B-PAR-06 | `test_Revert_ProposePair_DuplicateCurrencyPair` | proposePair — duplicate active pair | Transaction reverts when an `ACTIVE` pair for the same currency combination already exists | [Implemented] |

---

### ManualOracle

**File**: `contracts/test/ManualOracle.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-ORC-01 | `test_SetRate_Success` | setRate — authorized operator | Rate stored for currency pair; `RateUpdated` event emitted with pair, rate, and timestamp | [Implemented] |
| UT-SC-B-ORC-02 | `test_GetRate_Success` | getRate — returns current rate | Returns correct rate previously set; includes timestamp | [Implemented] |
| UT-SC-B-ORC-03 | `test_Revert_SetRate_Unauthorized` | setRate — unauthorized caller | Transaction reverts; only authorized oracle operator role can set rates | [Implemented] |
| UT-SC-B-ORC-04 | `test_Revert_GetRate_UnknownPair` | getRate — unknown pair | Reverts or returns zero with appropriate error for a pair that has never been set | [Implemented] |

---

### FXAgreement (Hub)

**File**: `contracts/test/FXAgreement.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-FXA-01 | `test_Propose_Success` | propose — happy path | Agreement created with `PROPOSED` state; unique `tradeId` assigned; `AgreementProposed` event emitted | [Implemented] |
| UT-SC-B-FXA-02 | `test_Revert_Propose_DuplicateTradeId` | propose — duplicate tradeId | Transaction reverts | [Implemented] |
| UT-SC-B-FXA-03 | `test_Revert_Propose_UnverifiedSender` | propose — unverified sender | Transaction reverts; hub IdentityRegistry gatekeeper enforced | [Implemented] |
| UT-SC-B-FXA-04 | `test_Revert_Propose_ZeroAmount` | propose — zero origin or counter amount | Transaction reverts | [Implemented] |
| UT-SC-B-FXA-05 | `test_Accept_Success` | accept — correct counterparty | Agreement transitions `PROPOSED → ACCEPTED`; `AgreementAccepted` event emitted | [Implemented] |
| UT-SC-B-FXA-06 | `test_Revert_Accept_WrongCaller` | accept — wrong counterparty | Transaction reverts; 403 equivalent | [Implemented] |
| UT-SC-B-FXA-07 | `test_Reject_Success` | reject — counterparty rejects | Agreement transitions `PROPOSED → REJECTED` (terminal); `AgreementRejected` event emitted | [Implemented] |
| UT-SC-B-FXA-08 | `test_Cancel_Success` | cancel — initiator cancels PROPOSED | Agreement transitions `PROPOSED → CANCELLED` (terminal) | [Implemented] |
| UT-SC-B-FXA-09 | `test_Settle_Success` | settle — governance caller on ACCEPTED | Agreement transitions `ACCEPTED → SETTLED` (terminal); `AgreementSettled` event emitted | [Implemented] |
| UT-SC-B-FXA-10 | `test_Revert_Settle_NotGovernance` | settle — non-governance caller | Transaction reverts | [Implemented] |
| UT-SC-B-FXA-11 | `test_ProposeOnBehalf_Success` | proposeOnBehalf — governance acting on behalf | Agreement created with correct initiator and counterparty | [Implemented] |
| UT-SC-B-FXA-12 | `test_Revert_ProposeOnBehalf_NotGovernance` | proposeOnBehalf — unauthorized | Transaction reverts | [Implemented] |
| UT-SC-B-FXA-13 | `test_AcceptOnBehalf_Success` | acceptOnBehalf — governance acting on behalf | Agreement transitions to `ACCEPTED` | [Implemented] |
| UT-SC-B-FXA-14 | `test_Revert_AcceptOnBehalf_NotGovernance` | acceptOnBehalf — unauthorized | Transaction reverts | [Implemented] |

---

### SpokeBridge

**File**: `contracts/test/SpokeBridge.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-BRG-01 | `test_LockAndMint_Success` | lockAndMint — happy path | Spoke tCeBM locked (transferred from sender to bridge escrow); hub W-tCeBM minted to recipient; `Locked` and `Minted` events emitted | [Implemented] |
| UT-SC-B-BRG-02 | `test_BurnAndUnlock_Success` | burnAndUnlock — happy path | Hub W-tCeBM burned; spoke tCeBM unlocked (transferred from bridge escrow to recipient); `Burned` and `Unlocked` events emitted | [Implemented] |
| UT-SC-B-BRG-03 | `test_Revert_LockAndMint_UnverifiedSender` | lockAndMint — unverified sender | Transaction reverts; only verified participants can lock | [Implemented] |
| UT-SC-B-BRG-04 | `test_Revert_BurnAndUnlock_UnverifiedSender` | burnAndUnlock — unverified sender | Transaction reverts | [Implemented] |
| UT-SC-B-BRG-05 | `test_Revert_LockAndMint_ZeroAmount` | lockAndMint — zero amount | Transaction reverts | [Implemented] |
| UT-SC-B-BRG-06 | `test_Revert_BurnAndUnlock_InsufficientHubBalance` | burnAndUnlock — insufficient W-tCeBM | Transaction reverts; no spoke tokens unlocked | [Implemented] |

---

### CurrencyRegistry

**File**: `contracts/test/CurrencyRegistry.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-CUR-01 | `test_RegisterCurrency_Success` | registerCurrency — happy path | Currency stored with code, name, and issuing CB address; `CurrencyRegistered` event emitted | [Implemented] |
| UT-SC-B-CUR-02 | `test_Revert_RegisterCurrency_Duplicate` | registerCurrency — duplicate code | Transaction reverts; currency code must be unique | [Implemented] |
| UT-SC-B-CUR-03 | `test_RemoveCurrency_Success` | removeCurrency — authorized governance | Currency deactivated; `CurrencyRemoved` event emitted | [Implemented] |
| UT-SC-B-CUR-04 | `test_GetAllCurrencies_ReturnsActiveOnly` | getAllCurrencies — filters inactive | Returns only active currencies; removed entries excluded | [Implemented] |

---

### IdentityRegistry (Hub)

**File**: `contracts/test/IdentityRegistry.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-IDR-01 | `test_RegisterParticipant_Success` | registerParticipant — governance | Participant stored with correct role and `Pending` status; `ParticipantRegistered` event emitted | [Implemented] |
| UT-SC-B-IDR-02 | `test_CanTransact_VerifiedParticipant` | canTransact — Verified status | Returns `true` for participant with `Verified` status and transactable role | [Implemented] |
| UT-SC-B-IDR-03 | `test_CanTransact_UnverifiedParticipant` | canTransact — non-Verified status | Returns `false` for `Pending`, `Suspended`, or `Expired` status | [Implemented] |
| UT-SC-B-IDR-04 | `test_CanGovern_GovernanceRoles` | canGovern — governance roles | Only `GOVERNANCE_ROLE` and `CENTRAL_BANK_ROLE` return `true` | [Implemented] |
| UT-SC-B-IDR-05 | `test_IsLiquidityProvider_MLPRole` | isLiquidityProvider — MLP role | Returns `true` only for participants registered with `LIQUIDITY_PROVIDER` role and `Verified` status | [Implemented] |
| UT-SC-B-IDR-06 | `test_GetCentralBankOf_CommercialBank` | getCentralBankOf — commercial bank lookup | Returns correct Central Bank address for a registered commercial bank participant | [Implemented] |

---

### TokenizedCentralBankMoney (Hub)

**File**: `contracts/test/TokenizedCentralBankMoney.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-TCB-01 | `test_Mint_AuthorizedCentralBank` | mint — CENTRAL_BANK_ROLE | `totalSupply` and recipient balance increase; `Transfer(0x0, recipient, amount)` emitted | [Implemented] |
| UT-SC-B-TCB-02 | `test_Revert_Mint_Unauthorized` | mint — unauthorized caller | Transaction reverts; only `CENTRAL_BANK_ROLE` holder can mint | [Implemented] |
| UT-SC-B-TCB-03 | `test_Burn_AuthorizedCentralBank` | burn — CENTRAL_BANK_ROLE | `totalSupply` and sender balance decrease; `Transfer(sender, 0x0, amount)` emitted | [Implemented] |
| UT-SC-B-TCB-04 | `test_Revert_Burn_Unauthorized` | burn — unauthorized caller | Transaction reverts | [Implemented] |

---

### HashTimeLockedContract (Hub)

**File**: `contracts/test/HashTimeLockedContract.t.sol`

> Hub-side HTLC used for hub-native payment settlement. Shares the LOCKED/SETTLED/REFUNDED FSM with the Scenario A spoke HTLC.

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-HTC-01 | `test_Lock_Success` | Lock — happy path on hub | HTLC transitions to `LOCKED`; sender balance deducted; `HTLCLocked` event emitted | [Implemented] |
| UT-SC-B-HTC-02 | `test_Settle_Success` | Settle — correct pre-image | HTLC transitions to `SETTLED`; recipient credited; `HTLCSettled` event emitted | [Implemented] |
| UT-SC-B-HTC-03 | `test_Refund_AfterExpiry` | Refund — after timeLock expiry | HTLC transitions to `REFUNDED`; sender balance restored | [Implemented] |
| UT-SC-B-HTC-04 | `test_Revert_Settle_InvalidSecret` | Settle — wrong pre-image | Transaction reverts; funds remain in escrow | [Implemented] |
| UT-SC-B-HTC-05 | `test_Revert_Refund_BeforeExpiry` | Refund — before timeLock expiry | Transaction reverts; time-lock requirement enforced | [Implemented] |

---

### CBWeb3Hub Deployment

**File**: `contracts/test/CBWeb3Hub.t.sol`

| Test ID | Function | Title | Expected Result | Status |
|---------|----------|-------|-----------------|--------|
| UT-SC-B-HUB-01 | `test_AllContractsDeployed` | Hub deployment — all contract addresses non-zero | AMM, PairRegistry, LiquidityCommitRegistry, ManualOracle, FXAgreement, SpokeBridge, CurrencyRegistry, IdentityRegistry, hub tCeBM addresses are all non-zero after deployment script executes | [Implemented] |
| UT-SC-B-HUB-02 | `test_RolesAssignedCorrectly` | Hub deployment — roles assigned | `GOVERNANCE_ROLE`, `CENTRAL_BANK_ROLE`, and `LIQUIDITY_PROVIDER` roles are correctly assigned to the expected addresses in hub IdentityRegistry | [Implemented] |
| UT-SC-B-HUB-03 | `test_HubTokensDeployed` | Hub deployment — W-tCeBM tokens deployed | Hub-wrapped token contracts (W-tCeBMa, W-tCeBMb) are deployed and have correct `name`, `symbol`, and `CENTRAL_BANK_ROLE` assignment | [Implemented] |

---

## Section 2 — Backend API Unit Tests (Go)

**Framework**: Go `testing` package
**Source**: `scenario-b/backend/services/*/internal/`
**Run**: `make scenario-b.test-backend` (`cd scenario-b/backend && go test ./...`)

| Test ID | File / Function | Title | Expected Result | Status |
|---------|----------------|-------|-----------------|--------|
| UT-API-B-01 | `handlers_test.go / TestHealth` | Health check endpoint | Returns 200 OK with service status | [Implemented] |
| UT-API-B-02 | `handlers_test.go / TestAMMQuoteHandler_Success` | AMM quote endpoint — valid pair and amount | Returns 200 with `amount_in`, `price_impact`, `fee` fields; values are positive numbers | [Implemented] |
| UT-API-B-03 | `handlers_test.go / TestAMMQuoteHandler_UnknownPair` | AMM quote endpoint — unknown pair | Returns 404 Not Found with structured error | [Implemented] |
| UT-API-B-04 | `handlers_test.go / TestAMMQuoteHandler_ZeroAmount` | AMM quote endpoint — zero amountOut | Returns 400 Bad Request | [Implemented] |
| UT-API-B-05 | `handlers_test.go / TestAMMSwapHandler_Success` | AMM swap endpoint — valid payload | Returns 202 Accepted with `txHash` and status `pending`; contract call mocked to return success | [Implemented] |
| UT-API-B-06 | `handlers_test.go / TestAMMSwapHandler_SlippageExceeded` | AMM swap endpoint — slippage revert from contract | Returns 422 Unprocessable Entity with slippage error message | [Implemented] |
| UT-API-B-07 | `handlers_test.go / TestAMMSwapHandler_Unauthorized` | AMM swap endpoint — missing JWT | Returns 401 Unauthorized | [Implemented] |
| UT-API-B-08 | `handlers_test.go / TestAMMSwapHandler_InsufficientRole` | AMM swap endpoint — insufficient role | Returns 403 Forbidden when caller role is below required minimum | [Implemented] |
| UT-API-B-09 | `handlers_test.go / TestBridgeHandler_LockAndMint_Success` | Bridge lock-and-mint endpoint — valid payload | Returns 202 Accepted with `txHash`; lockAndMint call mocked to succeed | [Implemented] |
| UT-API-B-10 | `handlers_test.go / TestBridgeHandler_BurnAndUnlock_Success` | Bridge burn-and-unlock endpoint — valid payload | Returns 202 Accepted with `txHash` | [Implemented] |
| UT-API-B-11 | `handlers_test.go / TestBridgeHandler_ZeroAmount` | Bridge endpoint — zero amount | Returns 400 Bad Request | [Implemented] |
| UT-API-B-12 | `handlers_test.go / TestLiquidityHandler_CommitSuccess` | Liquidity commit endpoint — valid commit | Returns 201 Created with commit ID | [Implemented] |
| UT-API-B-13 | `handlers_test.go / TestLiquidityHandler_CommitDuplicate` | Liquidity commit endpoint — duplicate commit | Returns 409 Conflict | [Implemented] |
| UT-API-B-14 | `handlers_test.go / TestLiquidityHandler_CancelCommit` | Liquidity cancel-commit endpoint | Returns 200 OK; commit status updated to `CANCELLED` | [Implemented] |
| UT-API-B-15 | `handlers_test.go / TestFXAgreementHandler_Create` | FX agreement creation — valid payload | Returns 201 Created with `tradeId` and `status = PROPOSED` | [Implemented] |
| UT-API-B-16 | `handlers_test.go / TestFXAgreementHandler_Accept` | FX agreement acceptance — correct counterparty | Returns 200 OK; status transitions to `ACCEPTED` | [Implemented] |
| UT-API-B-17 | `handlers_test.go / TestFXAgreementHandler_Accept_WrongCaller` | FX agreement acceptance — wrong caller | Returns 403 Forbidden | [Implemented] |
| UT-API-B-18 | `handlers_test.go / TestFXAgreementHandler_NotFound` | FX agreement GET — non-existent ID | Returns 404 Not Found | [Implemented] |
| UT-API-B-19 | `middleware/auth_test.go / TestJWTMiddleware_MissingToken` | Auth middleware — missing JWT | Returns 401 Unauthorized | [Implemented] |
| UT-API-B-20 | `middleware/auth_test.go / TestJWTMiddleware_ExpiredToken` | Auth middleware — expired JWT | Returns 401 Unauthorized | [Implemented] |
| UT-API-B-21 | `middleware/auth_test.go / TestRBACMiddleware_CommercialBankBlockedOnGovernance` | RBAC middleware — COMMERCIAL_BANK on governance endpoint | Returns 403 Forbidden | [Implemented] |
| UT-API-B-22 | `middleware/correlation_test.go / TestCorrelationIDMiddleware` | Correlation ID middleware | `X-Correlation-Id` injected/propagated on every request; UUID format validated | [Implemented] |
| UT-API-B-23 | `payment-orchestrator/internal/amm_client_test.go` | AMM client adapter — quote deserialization | Quote response correctly deserialized from AMM contract return data | [Implemented] |
| UT-API-B-24 | `payment-orchestrator/internal/amm_client_test.go` | AMM client adapter — swap error mapping | AMM contract revert reason correctly mapped to API error response | [Implemented] |
| UT-API-B-25 | `compliance/internal/domain/status_test.go` | Compliance status domain | Status transitions conform to allowed FSM for hub participants | [Implemented] |
| UT-API-B-26 | `handlers_test.go / TestPoolStatusHandler` | Pool status endpoint — valid pair | Returns 200 with `reserveA`, `reserveB`, `k`, `pair`, `status` fields | [Implemented] |
| UT-API-B-27 | `handlers_test.go / TestPoolStatusHandler_UnknownPair` | Pool status endpoint — unknown pair | Returns 404 Not Found | [Planned] |
| UT-API-B-28 | `handlers_test.go / TestAMMSwapHandler_ContractPaused` | AMM swap endpoint — paused contract | Returns 503 Service Unavailable with circuit-breaker message when AMM is paused | [Planned] |

---

## Section 3 — Integration Tests

**Framework**: Go `testing` with live local hub Devnet infrastructure
**Prerequisites**: `make scenario-b.up-infra` + `make scenario-b.deploy-contracts` completed; all readiness criteria met (see execution plan)

| Test ID | Title | Preconditions | Steps | Expected Result | Status |
|---------|-------|---------------|-------|-----------------|--------|
| INT-API-B-01 | AMM quote — live hub contract | Hub devnet running; AMM pool seeded with BRL-USD liquidity | `GET /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000` | Returns 200; `amount_in` matches constant-product formula against live reserves; response within 300ms p95 | [Implemented] |
| INT-API-B-02 | AMM swap execution — on-chain confirmation | Hub devnet running; bank-a has sufficient tCeBMa allowance on AMM | `POST /api/v2/amm/swap/exact-output` with valid payload | Returns 202; `txHash` present; on-chain `Swap` event confirmed; pool reserves updated; finality within 10s | [Implemented] |
| INT-API-B-03 | LiquidityCommitRegistry commit-reveal — CommitMatched event | Hub devnet running; BRL-USD pair active in PairRegistry | Register commit from CB-A side; register matching commit from CB-B side | `CommitMatched` event emitted on hub; event captured via `eth_getLogs` | [Partial] |
| INT-API-B-04 | SpokeBridge lockAndMint — on-chain W-tCeBM minted | Hub and Spoke-A devnet running; bank-a has tCeBMa balance on spoke | `POST /api/v2/hub/bridge/lock-and-mint` | Returns 202; spoke tCeBMa balance decreases; hub W-tCeBMa balance increases by same amount; `Locked` and `Minted` events confirmed | [Partial] |
| INT-API-B-05 | FX Agreement hub settlement — on-chain | Hub devnet running; FXAgreement deployed | Create FX Agreement → accept → settle via governance call | `AgreementSettled` event confirmed on hub; API returns `status = SETTLED` | [Implemented] |
| INT-API-B-06 | AML blocking — hub participant | Hub API gateway + compliance service running | Request FX Agreement with a participant flagged as sanctioned in compliance registry | Returns 403 or 422 before any on-chain call; no `agreementId` created | [Implemented] |
| INT-API-B-07 | Circuit breaker — AMM pause enforced on API | Hub devnet running; AMM deployed | Governance call pauses AMM; then attempt swap via API | Swap returns 503; contract call blocked before submission | [Partial] |
| INT-API-B-08 | Cacti watcher — CommitMatched event detection | Hub devnet + Cacti relay running with watcher active | Submit matching commits to LiquidityCommitRegistry | Cacti relay logs show CommitMatched event detected within 15s of block finality | [Planned] |
| INT-API-B-09 | AMM insufficient liquidity — API error mapping | Hub devnet running; AMM pool with minimal reserves | `POST /api/v2/amm/swap/exact-output` with `amount_out` > reserves | Returns 400 with user-friendly `INSUFFICIENT_LIQUIDITY` message; not a raw contract error | [Planned] |
| INT-API-B-10 | PairRegistry confirmation — bilateral approval on-chain | Hub devnet running | CB-A proposes pair → CB-B confirms pair → pair status verified via contract read | Pair status = `ACTIVE` in PairRegistry; `getAllActivePairs` includes new pair | [Planned] |

---

## Section 4 — E2E Tests

**Framework**: Bash scripts (`scenario-b/tryouts/`)
**Prerequisites**: `make scenario-b.up` completed; all readiness criteria met; all 6+ API gateways return 200 on `/healthz`
**On-chain verification**: `eth_getTransactionReceipt` polling + contract read functions

---

### E2E-B-01 — Cooperative Liquidity Pair Formation (US1)

- **Test ID**: E2E-B-01
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us1`
- **Status**: [Implemented]
- **User Story**: US1 — Central Banks cooperatively form a BRL-USD liquidity pair on the hub through the PairRegistry and LiquidityCommitRegistry commit-reveal mechanism
- **Preconditions**: Hub devnet running; central-bank-a and central-bank-b registered with `Verified` status and `CENTRAL_BANK_ROLE` on hub; BRL and USD registered in CurrencyRegistry

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | central-bank-a and central-bank-b login via `/api/v1/auth/login` |
| 2 | Propose pair | central-bank-a calls `POST /api/v2/hub/pair-registry/propose` (BRL-USD, counterparty = CB-B) |
| 3 | Assert proposal | `PairProposed` event emitted; `pairId` assigned; pair status = `PROPOSED` |
| 4 | Confirm pair | central-bank-b calls `POST /api/v2/hub/pair-registry/confirm/{pairId}` |
| 5 | Assert active | `PairConfirmed` event emitted; pair status = `ACTIVE` in registry |
| 6 | Register commits | central-bank-a registers BRL-side commit; central-bank-b registers USD-side commit via `POST /api/v2/hub/liquidity/commit` |
| 7 | Assert CommitMatched | `CommitMatched` event emitted on hub; Cacti relay detects event |
| 8 | Verify AMM pool | `GET /api/v2/amm/pool/BRL-USD/status` returns non-zero `reserveA` and `reserveB` |

- **Expected Outcome**: BRL-USD pair is active; AMM pool has non-zero reserves; LP shares credited to CB-A and CB-B
- **Evidence**: `PairProposed`, `PairConfirmed`, `CommitRegistered` (x2), `CommitMatched` events captured in `aggregated_traces.log`

---

### E2E-B-02 — MLP Bilateral Liquidity Provisioning (US2)

- **Test ID**: E2E-B-02
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us2`
- **Status**: [Implemented]
- **User Story**: US2 — MLP registers matched commits on both sides of a pair; Cacti relay detects CommitMatched and executes dual-sided liquidity provisioning
- **Preconditions**: Hub devnet running; MLP registered with `LIQUIDITY_PROVIDER` role; BRL-USD pair active in PairRegistry

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | MLP login via `/api/v1/auth/login` |
| 2 | Register BRL-side commit | MLP calls `POST /api/v2/hub/liquidity/commit` (pair = BRL-USD, side = BRL, amount = X) |
| 3 | Register USD-side commit | MLP calls `POST /api/v2/hub/liquidity/commit` (pair = BRL-USD, side = USD, amount = Y) |
| 4 | Assert CommitMatched | `CommitMatched` event emitted; Cacti relay detects and processes |
| 5 | Verify LP shares | MLP LP token balance increases on hub AMM; pool reserves updated |

- **Expected Outcome**: MLP has LP shares in AMM pool; AMM pool reserves reflect both-sided provisioning
- **Evidence**: `CommitRegistered` (x2), `CommitMatched`, AMM `Sync` events captured

---

### E2E-B-03 — Commercial Bank FX Swap via AMM (US3)

- **Test ID**: E2E-B-03
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us3`; `bash tryouts/tryout-commercial-swap-e2e.sh`
- **Status**: [Partial] — governance path works; commercial bank direct swap path is in progress
- **User Story**: US3 — Commercial bank performs a cross-currency swap via the hub AMM: obtains a quote, submits an exact-output swap, and bridges back to their spoke
- **Preconditions**: Hub devnet running; BRL-USD pair active; AMM pool seeded; bank-a registered as commercial bank with sufficient tCeBMa on spoke

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | bank-a login via `/api/v1/auth/login` |
| 2 | Obtain quote | `GET /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000` |
| 3 | Assert quote | Response includes `amount_in`, `price_impact`, `fee`; values consistent with constant-product formula |
| 4 | Bridge in | bank-a calls `POST /api/v2/hub/bridge/lock-and-mint` to lock spoke tCeBMa and receive hub W-tCeBMa |
| 5 | Execute swap | bank-a calls `POST /api/v2/amm/swap/exact-output` with `amount_out=1000`, `max_amount_in` from quote |
| 6 | Assert swap | `Swap` event emitted; pool reserves updated; invariant `reserveA * reserveB >= k_before` holds |
| 7 | Bridge out | bank-a calls `POST /api/v2/hub/bridge/burn-and-unlock` to convert W-tCeBMb → spoke tCeBMb |
| 8 | Assert settlement | bank-a has received tCeBMb on spoke; no orphaned tokens on hub |

- **Expected Outcome**: bank-a successfully exchanges tCeBMa for tCeBMb via hub AMM; full bridge round-trip completes; AMM invariant preserved
- **Known Gap**: commercial bank direct authentication path for swap still in progress; governance path executes correctly

---

### E2E-B-04 — FX Agreement Lifecycle on Hub (US4)

- **Test ID**: E2E-B-04
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us4`
- **Status**: [Implemented]
- **User Story**: US4 — Two banks execute a complete FX Agreement lifecycle on the hub FXAgreement contract
- **Preconditions**: Hub devnet running; bank-a and bank-b registered with `Verified` status on hub

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | bank-a and bank-b login via `/api/v1/auth/login` |
| 2 | Propose agreement | bank-a calls `POST /api/v2/hub/fx-agreements` with valid pair, amounts, and counterparty = bank-b |
| 3 | Assert proposal | `tradeId` assigned; `AgreementProposed` event emitted; status = `PROPOSED` |
| 4 | Accept agreement | bank-b calls `POST /api/v2/hub/fx-agreements/{tradeId}/accept` |
| 5 | Assert acceptance | `AgreementAccepted` event emitted; status = `ACCEPTED` |
| 6 | Settle agreement | Governance calls `POST /api/v2/hub/fx-agreements/{tradeId}/settle` |
| 7 | Assert settlement | `AgreementSettled` event emitted; status = `SETTLED` (terminal) |

- **Expected Outcome**: Full FSM traversal (PROPOSED → ACCEPTED → SETTLED) confirmed on-chain; all events captured
- **Evidence**: `AgreementProposed`, `AgreementAccepted`, `AgreementSettled` events in `aggregated_traces.log`

---

### E2E-B-05 — PairRegistry Bilateral Pair Approval (US5)

- **Test ID**: E2E-B-05
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us5`
- **Status**: [Implemented]
- **User Story**: US5 — Two Central Banks cooperatively activate a new currency pair on the hub through bilateral PairRegistry approval
- **Preconditions**: Hub devnet running; central-bank-a and central-bank-b registered with `CENTRAL_BANK_ROLE`; both currencies registered in CurrencyRegistry

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | central-bank-a and central-bank-b login |
| 2 | Propose pair | CB-A calls `POST /api/v2/hub/pair-registry/propose` |
| 3 | Assert proposal | Pair status = `PROPOSED`; only CB-A recorded as proposer |
| 4 | Confirm pair | CB-B calls `POST /api/v2/hub/pair-registry/confirm/{pairId}` |
| 5 | Assert active | Pair status = `ACTIVE`; `getAllActivePairs` returns updated list |
| 6 | Verify unilateral rejection | Attempt to confirm using CB-A credentials (proposer self-confirm); must revert |

- **Expected Outcome**: Pair transitions PROPOSED → ACTIVE only after bilateral confirmation; unilateral self-confirmation reverts

---

### E2E-B-06 — SpokeBridge Lock&Mint / Burn&Unlock (US6)

- **Test ID**: E2E-B-06
- **Script / Command**: `bash tryouts/tryout-scenario-b-e2e.sh us6`
- **Status**: [Implemented]
- **User Story**: US6 — Commercial bank bridges spoke tCeBM to the hub (lockAndMint), receives hub W-tCeBM, then bridges back (burnAndUnlock) and receives spoke tCeBM
- **Preconditions**: Hub and Spoke-A devnet running; SpokeBridge deployed on spoke; bank-a has tCeBMa balance on Spoke-A

| Step | Action | System Interaction |
|------|--------|--------------------|
| 1 | Authenticate | bank-a login via `/api/v1/auth/login` |
| 2 | Lock and mint | bank-a calls `POST /api/v2/hub/bridge/lock-and-mint` (amount = 5000 tCeBMa) |
| 3 | Assert mint | `Locked` event on Spoke-A; `Minted` event on hub; W-tCeBMa balance = 5000 |
| 4 | Assert spoke balance | Spoke-A tCeBMa balance decreased by 5000 |
| 5 | Burn and unlock | bank-a calls `POST /api/v2/hub/bridge/burn-and-unlock` (amount = 5000 W-tCeBMa) |
| 6 | Assert unlock | `Burned` event on hub; `Unlocked` event on Spoke-A; spoke tCeBMa balance restored |
| 7 | Assert symmetry | Hub W-tCeBMa balance = 0; spoke tCeBMa balance = original amount; no tokens created or destroyed |

- **Expected Outcome**: Symmetric bridge round-trip; total token supply across hub + spoke is conserved; no orphaned balances
- **Evidence**: `Locked`, `Minted`, `Burned`, `Unlocked` events captured in `aggregated_traces.log` with network labels
