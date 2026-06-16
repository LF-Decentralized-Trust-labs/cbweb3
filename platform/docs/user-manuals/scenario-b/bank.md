# Bank Portal — User Manual (Scenario B)

**Audience:** Commercial bank operators participating in the International Hub.
**Scenario:** Scenario B — International Hub (FXAgreement + AMM + Bridge).
**Data source:** Live backend (real API Gateway) for all operational screens.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Access and Login](#2-access-and-login)
3. [Navigation](#3-navigation)
4. [Screens](#4-screens)
   - 4.1 [Dashboard](#41-dashboard)
   - 4.2 [Transfer (`/transfer`)](#42-transfer-transfer)
   - 4.3 [Issuance Requests / Deposits (`/deposits`)](#43-issuance-requests--deposits-deposits)
   - 4.4 [Reserve Tokenisation / Escrows (`/escrows`)](#44-reserve-tokenisation--escrows-escrows)
   - 4.5 [Approve AMM (`/approve-amm`)](#45-approve-amm-approve-amm)
   - 4.6 [Redeems (`/redeems`)](#46-redeems-redeems)
   - 4.7 [Bridge (`/bridge`)](#47-bridge-bridge)
   - 4.8 [Compliance (`/compliance`)](#48-compliance-compliance)
   - 4.9 [Onboarding (`/onboarding`)](#49-onboarding-onboarding)
   - 4.10 [Settings (`/settings`)](#410-settings-settings)
   - 4.11 [AMM Trading (`/amm`) — advanced, not in sidebar](#411-amm-trading-amm--advanced-not-in-sidebar)
5. [Typical Workflows](#5-typical-workflows)
   - 5.1 [Getting started: obtain tCeBM](#51-getting-started-obtain-tcebm)
   - 5.2 [Initiate a cross-border transfer](#52-initiate-a-cross-border-transfer)
   - 5.3 [Bridge tCeBM to the Hub and back](#53-bridge-tcebm-to-the-hub-and-back)
   - 5.4 [Redeem tCeBM for fiat](#54-redeem-tcebm-for-fiat)
6. [Status Reference](#6-status-reference)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. Overview

The Bank Portal is the day-to-day operational interface for a commercial bank operator in **Scenario B — International Hub**. Unlike Scenario A (which settles value bilaterally between two spokes using HTLC), Scenario B routes cross-border value through a shared **Hub** equipped with an **Automated Market Maker (AMM)** for FX. Value moves between a spoke and the Hub via a **bridge** that locks native CBDC (tCeBM) on the spoke and mints a mirrored position at the Hub.

The end-to-end operational lifecycle is:

1. **Onboard** your institution with the Central Bank (one-time).
2. **Request tCeBM issuance** by submitting a fiat collateral proof (Deposits page).
3. **Tokenise** the approved fiat reserve into tCeBM (Escrows page).
4. **Approve the AMM** to spend your tCeBM (Approve AMM page).
5. **Transfer** value cross-border via the Hub AMM (Transfer page), or **Bridge** tCeBM between the spoke and the Hub (Bridge page).
6. **Redeem** tCeBM back to fiat when needed (Redeems page).

| Actor | Portal | Role |
|---|---|---|
| Commercial Bank Operator | Bank Portal (this portal) | Manages tCeBM balances, issuance, tokenisation, redemption, AMM approvals, cross-border transfers, and bridge operations |
| Central Bank Governance Operator | Governance Portal | Approves onboarding KYC, issuance, tokenisation, and redeem requests |

---

## 2. Access and Login

**Route:** `/login`

Open the Bank Portal URL provided by your administrator. You will land on the **Sign in to Bank Portal** screen.

<!-- TODO: screenshot -->

Fill in both fields and click **Sign in**:

| Field | Description |
|---|---|
| **Client ID** | The institutional client identifier provided by the Central Bank (minimum 3 characters). |
| **Client Secret** | The corresponding credential secret (minimum 6 characters). |

Authentication is managed via backend HTTP-only session cookies. On success you are redirected to the Dashboard.

> If sign-in fails immediately with correct credentials, confirm your institution's onboarding status is `ACTIVE`. New institutions must complete onboarding before they can access operational screens.

---

## 3. Navigation

The left-hand sidebar lists all screens available in Scenario B:

| Sidebar label | Route | Purpose |
|---|---|---|
| **Dashboard** | `/` | Real-time overview of balances, AMM pool status, and recent activity |
| **Transfer** | `/transfer` | Cross-border transfer via Hub AMM |
| **Issuance Requests** | `/deposits` | Request the Central Bank to issue tCeBM |
| **Reserve Tokenisation** | `/escrows` | Convert approved fiat reserve (fCeBM) into tCeBM |
| **Approve AMM** | `/approve-amm` | Authorise the AMM to spend your tCeBM |
| **Redeems** | `/redeems` | Convert tCeBM back to fiat |
| **Bridge** | `/bridge` | Move tCeBM between the spoke and the Hub |
| **Compliance** | `/compliance` | Attach ZK credentials to pending operations |
| **Onboarding** | `/onboarding` | Institution registration wizard |
| **Settings** | `/settings` | Portal environment information |

> The **AMM Trading** page (`/amm`) is accessible by direct URL but is not shown in the sidebar. It is an advanced interface for direct pool swaps and is described in section 4.11.

---

## 4. Screens

### 4.1 Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

<!-- TODO: screenshot -->

The Dashboard provides a real-time summary of your institution's position. It refreshes pool and circuit-breaker status every 30 seconds automatically.

**Identity / context card**

Displays your bank ID, country, wallet address (truncated), roles, and a compliance status badge derived from your onboarding state.

**KPI row**

| Card | What it shows |
|---|---|
| **tCeBM Balance** | Your current on-chain tCeBM token balance. |
| **Fiat Reserve (fCeBM)** | Your current fCeBM (fiat-backed reserve token) balance. |
| **Pending Actions** | Combined count of deposits, tokenisations, and redeems in PENDING status. |
| **Bridged to Hub** | Total tCeBM currently mirrored at the Hub across all active bridge positions. |

**Status strip**

| Card | What it shows |
|---|---|
| **Circuit Breaker** | `LIVE` (swaps operational) or `HALTED` (cross-border swaps paused by Central Bank). |
| **Hub Pool (pair)** | Current pool status (e.g. `ACTIVE`, `INACTIVE`) and whether the pool has an imbalance flag. |
| **FX Proposals Awaiting You** | Count of FX trade agreements in `PROPOSED` state that require your action. |

**Quick actions**

Shortcut buttons to: New Deposit, Redeem, Cross-border Transfer, Bridge to Hub, AMM Trade.

**Recent Activity table**

Chronological list (up to 8 records) of deposits, tokenisations, redeems, and bridge events, with type, amount, status badge, and timestamp.

**Bridge Positions panel**

Shows up to 6 bridge positions with their mirrored amount, asset, and current bridge state badge.

**Charts**

Three bar charts summarising: Tokenised vs Reserve balance comparison, request breakdown by status (Pending/Approved/Rejected) across deposits/tokenisations/redeems, and daily request volume over the past 7 days.

---

### 4.2 Transfer (`/transfer`)

**Route:** `/transfer` · **Sidebar label:** Transfer

<!-- TODO: screenshot -->

This is the primary screen for initiating a **cross-border payment via the Hub AMM**. The transfer uses the cross-currency swap flow: your tCeBM on the source spoke is bridged in, swapped at the Hub for the target currency, and bridged out to the beneficiary's spoke. The page shows the current pool status for pair **W-BRL-ARS** at the top.

> Before you can execute a transfer, the AMM must be approved to spend your tCeBM (see [Approve AMM](#45-approve-amm-approve-amm)).

**Step 1 — Get Quote**

| Field | Description |
|---|---|
| **Source Currency** | The currency you are sending (e.g. `BRL`). |
| **Target Currency** | The currency the beneficiary will receive (e.g. `ARS`). |
| **Amount Out** | The amount the beneficiary should receive (in target currency units). |

Click **Get Quote**. The Quote Details card appears showing:

| Field | Description |
|---|---|
| **Effective Rate** | The implied exchange rate for this swap. |
| **Amount In** | How much of the source currency you will spend. |
| **Amount Out** | The target amount you entered. |
| **TTL** | Seconds remaining before the quote expires. |

> Quotes expire. If the TTL reaches zero before you execute, the Execute Transfer button becomes disabled. You must re-submit the Get Quote form to obtain a fresh quote. If the quote was auto-refreshed in the background, a warning banner appears — review the new quote before confirming.

**Step 2 — Execute Transfer**

Once a valid quote is shown:

| Field | Description |
|---|---|
| **Max Amount In** | Your slippage ceiling — the maximum source currency you will allow the swap to consume. Enter a value manually or click **Use Suggested Max** to fill in the quote amount plus slippage buffer. |
| **Beneficiary Bank ID** | The bank identifier of the recipient institution. |

Click **Execute Transfer**. The button is disabled if the pool is not `ACTIVE`, if no quote is loaded, or if any required field is empty.

**Transfer Progress**

While the transfer is in flight, a four-step progress indicator is shown:

| Step | Meaning |
|---|---|
| **BRIDGE IN PROGRESS** | Your tCeBM is being locked on your spoke. |
| **SWAP IN PROGRESS** | The AMM swap is executing on the Hub. |
| **BRIDGE OUT PROGRESS** | The swapped tokens are being unlocked on the beneficiary's spoke. |
| **COMPLETED** | The transfer has settled end-to-end. |

On completion, the screen shows the swap transaction hash, amounts in and out, bridge position IDs, and a correlation ID. Click **Reset** to start a new transfer.

**Transfer errors**

| Error | What it means | Action |
|---|---|---|
| Pool not ACTIVE | The Hub pool is not accepting swaps. | Wait for Central Bank to activate the pool. |
| Circuit Breaker HALTED | Swaps are suspended. | Wait for governance to resume. |
| Slippage limit exceeded | Price moved beyond your Max Amount In. | Re-quote and retry. |
| Insufficient pool liquidity | Pool reserves too low for this amount. | Try a smaller amount or contact your Central Bank. |
| Daily transfer limit exceeded | Your Central Bank has set a volume cap for today. | Contact your Central Bank to adjust the limit. |
| Quote expired | Quote expired during submission; a new quote was auto-fetched. | Review the refreshed quote and confirm again. |
| BRIDGE OUT FAILED (critical) | Source funds were debited but the bridge-out failed. | Click **Acknowledge** and immediately contact your Central Bank with the displayed Swap ID. Do not retry without guidance. |

---

### 4.3 Issuance Requests / Deposits (`/deposits`)

**Route:** `/deposits` · **Sidebar label:** Issuance Requests

<!-- TODO: screenshot -->

Use this screen to request the Central Bank to **issue tCeBM** against your fiat collateral. The top of the screen shows your current tCeBM balance, fiat reserve (fCeBM) balance, and count of pending issuance requests.

**Submitting a request**

1. Enter the **Amount** in fiat currency units.
2. Click **Review Issuance Request**.
3. A confirmation card appears with the formatted amount. Click **Confirm Request** to submit, or **Cancel** to go back.
4. On success a toast notification shows the assigned request ID.

**Issuance Requests table**

| Column | Description |
|---|---|
| **ID** | Unique request identifier. |
| **Fiat Amount** | The amount submitted. |
| **Status** | Current status badge (see [Status Reference](#6-status-reference)). |
| **Issuance Reference** | On-chain transaction hash once approved and minted (truncated). |
| **Rejection Reason** | Populated when the Central Bank rejects the request. |
| **Created At** | Submission timestamp. |

> tCeBM is minted only after the Central Bank approves the request from the Governance Portal. The page does not auto-refresh — click **Refresh** to update balances and statuses.

---

### 4.4 Reserve Tokenisation / Escrows (`/escrows`)

**Route:** `/escrows` · **Sidebar label:** Reserve Tokenisation

<!-- TODO: screenshot -->

After a deposit (issuance request) is approved, use this screen to **convert the approved fiat reserve (fCeBM) into tCeBM**. The top of the screen shows your fCeBM balance and count of pending tokenisation requests.

**Submitting a tokenisation request**

1. Enter or select the **Deposit ID** of an approved deposit (a drop-down datalist shows approved IDs with their amounts).
2. Enter the **Amount** to tokenise (in fiat currency units).
3. Click **Review Tokenisation Request**.
4. A confirmation card shows the fCeBM amount being converted and the equivalent tCeBM. Click **Confirm Request** to submit, or **Cancel** to go back.

**Tokenisation Requests table**

| Column | Description |
|---|---|
| **ID** | Unique tokenisation request identifier. |
| **Deposit ID** | The parent deposit this request is linked to. |
| **Amount** | The fCeBM amount being converted. |
| **Status** | Current status badge. |
| **Burn Tx (fCeBM)** | Transaction hash of the fCeBM burn (truncated). |
| **Mint Tx (tCeBM)** | Transaction hash of the tCeBM mint (truncated). |
| **Created At** | Submission timestamp. |

> The Central Bank must approve the tokenisation request before the on-chain burn/mint executes. Click **Refresh** to update.

---

### 4.5 Approve AMM (`/approve-amm`)

**Route:** `/approve-amm` · **Sidebar label:** Approve AMM

<!-- TODO: screenshot -->

Before the AMM can use your tCeBM in a swap, you must grant it a spending allowance. This step is required once per session (or whenever you need to increase the allowance). The screen shows your current tCeBM balance for reference.

**Granting approval**

1. Enter the **Amount** of tCeBM (in base units) you want to approve for AMM usage.
2. Click **Review Approval**. A warning badge appears if the amount exceeds your current balance (this is non-blocking — you may still submit if you expect the balance to increase).
3. A confirmation card appears: *"You are about to approve N tCeBM for AMM usage."* Click **Confirm Approval** to submit, or **Cancel** to go back.
4. On success a toast notification confirms the approval. A green badge **AMM approval confirmed in current session** is shown.

> If a Transfer or AMM swap fails with an approval error, return here and approve a sufficient amount before retrying.

---

### 4.6 Redeems (`/redeems`)

**Route:** `/redeems` · **Sidebar label:** Redeems

<!-- TODO: screenshot -->

Use this screen to **convert tCeBM back to fiat**. The top of the screen shows your tCeBM balance and count of pending redeem requests.

**Submitting a redeem request**

1. Enter the **Amount** of tCeBM to redeem.
2. Click **Review Redeem Request**.
3. A confirmation card shows the tCeBM amount being submitted for fiat reserve release. Click **Confirm Request** to submit, or **Cancel** to go back.
4. On success a toast notification shows the assigned redeem request ID.

**Redeem Requests table**

| Column | Description |
|---|---|
| **ID** | Unique redeem request identifier. |
| **Amount** | The tCeBM amount submitted. |
| **Status** | Current status badge. |
| **Redemption Tx** | On-chain transaction hash of the redemption (truncated). |
| **Rejection Reason** | Populated when the Central Bank rejects the request. |
| **Created At** | Submission timestamp. |

> The Central Bank must approve the redeem from the Governance Portal before fiat is released. Click **Refresh** to update.

---

### 4.7 Bridge (`/bridge`)

**Route:** `/bridge` · **Sidebar label:** Bridge

<!-- TODO: screenshot -->

The Bridge moves tCeBM between your spoke and the Hub using a **lock/mint** (spoke → Hub) and **burn/unlock** (Hub → spoke) model. Positions are tracked in the Bridge Positions table, which polls automatically every 5 seconds while any position is in a non-terminal state.

**Lock & Mint (spoke → Hub)**

1. Enter the **Amount** of tCeBM to bridge (integer, in base units).
2. Click **Submit Lock & Mint**.
3. The backend derives your bank identity and asset context from your authenticated session. A new bridge position appears in the table with state `LOCKING`.

**Burn & Unlock (Hub → spoke)**

1. Find the **Position ID** of an `ACTIVE` bridge position in the table below, and copy it.
2. Paste it into the **Position ID** field.
3. The form validates that the selected position is in `ACTIVE` state. If the position is not `ACTIVE`, the **Submit Burn & Unlock** button remains disabled.
4. Click **Submit Burn & Unlock**.

> Burn & Unlock is only available for positions in `ACTIVE` state. If the position is in any other state, wait for it to reach `ACTIVE` before proceeding.

**Bridge Positions table**

Use the **Filter by state** dropdown to narrow the list. Click **Manual Refresh** at any time to force a poll.

| Column | Description |
|---|---|
| **Position ID** | Unique identifier for this bridge position. |
| **Owner** | The bank ID that initiated this position. |
| **Spoke** | The spoke network where native funds are held. |
| **Native Asset** | The asset locked on the spoke. |
| **State** | Current bridge lifecycle state (see state table below). |
| **Mirrored Amount** | Amount of the mirrored asset at the Hub. |
| **Relayer Retries** | Number of times the relay has retried this operation. |

If a position remains in `LOCKING` or `BURNING` for more than 2 minutes, the table displays *"Polling timeout reached. Refresh to continue."* — click **Manual Refresh** and check the relay status in the NOC Portal if it remains stuck.

---

### 4.8 Compliance (`/compliance`)

**Route:** `/compliance` · **Sidebar label:** Compliance

<!-- TODO: screenshot -->

The Compliance Center allows you to view your institution's ZK-compliance credentials and attach them to pending on-chain operations.

**Credential Library**

A table listing all credentials associated with your institution:

| Column | Description |
|---|---|
| **Select** | Checkbox to include this credential in an attachment. |
| **Credential** | Unique credential identifier. |
| **Type** | Credential type badge (e.g. KYC, AML). |
| **Issuer** | The entity that issued this credential. |

**Attach Credentials**

1. Select one or more credentials using the checkboxes.
2. Enter the **Transaction ID** of the pending operation you want to attach them to.
3. Click **Attach Selected**.

> Credentials are issued by the Central Bank after KYC approval. Contact your Central Bank if the credential library is empty.

---

### 4.9 Onboarding (`/onboarding`)

**Route:** `/onboarding` · **Sidebar label:** Onboarding

<!-- TODO: screenshot -->

The Onboarding Wizard registers a new commercial bank with the Central Bank and provisions its on-chain wallet. This is a one-time process. Until onboarding completes and the institution status reaches `ACTIVE`, the operational screens are inaccessible.

For the full step-by-step onboarding procedure, see the [ONBOARDING-DOCS runbook](../../../scenario-b/docs/runbooks/ONBOARDING-DOCS.md).

**Wizard stages**

```
1. Operator  →  2. Institution  →  3. KYC  →  4. Complete
```

| Stage | Description |
|---|---|
| **1. Operator Confirmation** | Confirms the signed-in operator identity before proceeding. Click **Start Onboarding**. |
| **2. Institution Data** | Fill in Institution Name, Bank Code, Country, Role, Username, and Email. Click **Submit Onboarding Request**. |
| **3. KYC Approval** | Waiting state. The portal polls for Central Bank approval every 5 seconds. Displays Request ID, Wallet, and elapsed time. Click **Refresh status** to force an immediate check. |
| **4. Complete** | Finalization: PKI credential exchange and on-chain activation. Shows Wallet Address, Transaction Hash, and PKI login badge. Click **Go to Dashboard**. |

---

### 4.10 Settings (`/settings`)

**Route:** `/settings` · **Sidebar label:** Settings

<!-- TODO: screenshot -->

Displays read-only environment information for the current portal session:

- API mode (mock services enabled/disabled).
- Authentication mechanism (backend-managed HTTP-only cookies).
- WebSocket mode (simulated relay events or live).

> This screen contains no configurable options. It is informational only.

---

### 4.11 AMM Trading (`/amm`) — advanced, not in sidebar

**Route:** `/amm` · Not in sidebar navigation (access via direct URL or Dashboard quick action **AMM Trade**)

<!-- TODO: screenshot -->

This is an advanced interface that provides direct access to the Hub AMM pool. It offers two trading modes via tabs:

**Exact Output tab**

A two-panel interface for manual pool swaps:

- **Quote Exact Output** — Enter a pair (e.g. `BRL-ARS`) and an Amount Out to get the required input amount and price impact. Quotes go stale after 10 seconds; a **Refresh Quote** button appears when stale.
- **Swap Exact Output** — Execute the swap with explicit control over Max Amount In, Payer ID (pre-filled from session), and Beneficiary ID. The Execute Swap button is disabled if the circuit breaker is `HALTED`, the pool is not `ACTIVE`, or AMM approval is missing.
- **Pool Status** panel — Shows reserves for both sides, current ratio, and last update time.
- **Approve AMM** panel — A collapsible helper to grant AMM approval without leaving the page (equivalent to the dedicated Approve AMM page).

**Cross-Currency tab**

A guided cross-currency swap flow (BRL → ARS via pool `W-BRL-ARS`) with the same quote → execute pattern as the Transfer page. Includes a four-step progress indicator (BRIDGE IN PROGRESS → SWAP IN PROGRESS → BRIDGE OUT PROGRESS → COMPLETED) and the same error handling, including the critical `BRIDGE_OUT_FAILED` case.

> For day-to-day cross-border payments, use the **Transfer** page instead. The AMM Trading page is intended for operators who need direct pool visibility or are troubleshooting swap parameters.

---

## 5. Typical Workflows

### 5.1 Getting started: obtain tCeBM

Before you can make cross-border transfers you need tCeBM in your wallet. Follow these steps in order:

1. **Onboard** (if not already done): navigate to **Onboarding** and complete all four wizard stages.
2. **Request issuance**: go to **Issuance Requests**, enter a fiat amount, and confirm. Wait for the Central Bank to approve the request (status changes from `PENDING` to `APPROVED`).
3. **Tokenise the reserve**: go to **Reserve Tokenisation**, select the approved Deposit ID, enter an amount, and confirm. Wait for Central Bank approval (status changes to `APPROVED`). Your tCeBM balance increases.
4. **Approve the AMM**: go to **Approve AMM**, enter at least the amount you plan to transfer, and confirm. You must do this before the AMM can execute swaps on your behalf.

---

### 5.2 Initiate a cross-border transfer

Prerequisites: tCeBM balance available, AMM approved.

1. Navigate to **Transfer**.
2. Confirm the pool status banner shows **Pool W-BRL-ARS - ACTIVE**. If it shows another status, contact your Central Bank.
3. Enter the **Source Currency**, **Target Currency**, and **Amount Out** (the amount the recipient should receive).
4. Click **Get Quote** and review the effective rate and Amount In shown in the Quote Details card.
5. Check the TTL — if it is low, proceed quickly to Step 6 or re-quote.
6. Click **Use Suggested Max** to fill the Max Amount In field, or enter a custom ceiling.
7. Enter the **Beneficiary Bank ID**.
8. Click **Execute Transfer**.
9. Monitor the four-step progress bar until the status reaches **COMPLETED**.
10. Note the Swap Tx Hash and Correlation ID for your records. Click **Reset** to prepare for the next transfer.

---

### 5.3 Bridge tCeBM to the Hub and back

**Lock & Mint (move tCeBM from your spoke to the Hub)**

1. Navigate to **Bridge**.
2. In the **Lock & Mint** card, enter the amount to bridge (in base units).
3. Click **Submit Lock & Mint**.
4. A new position appears in the Bridge Positions table with state `LOCKING`. Wait for it to reach `ACTIVE` (auto-polls every 5 seconds).

**Burn & Unlock (retrieve tCeBM from the Hub back to your spoke)**

1. Navigate to **Bridge** and locate the target position in the Bridge Positions table.
2. Confirm the position state is `ACTIVE`.
3. Copy the **Position ID** and paste it into the **Burn & Unlock** card.
4. Click **Submit Burn & Unlock**.
5. The position state transitions to `BURNING` and eventually `BURNED` / `RELEASED`.

---

### 5.4 Redeem tCeBM for fiat

1. Navigate to **Redeems**.
2. Confirm your tCeBM balance is sufficient.
3. Enter the tCeBM amount to redeem.
4. Click **Review Redeem Request**, then **Confirm Request**.
5. The request appears as `PENDING`. The Central Bank approves it from the Governance Portal; status changes to `APPROVED` once the fiat reserve is released.

---

## 6. Status Reference

### Issuance (Deposit) request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Submitted; awaiting Central Bank approval. |
| `APPROVED` | Approved by the Central Bank; tCeBM has been minted. |
| `REJECTED` | Declined by the Central Bank; check the Rejection Reason column. |
| `MINT_FAILED` | Approved but the on-chain mint failed — contact your Central Bank. |

### Tokenisation (Escrow) request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Submitted; awaiting Central Bank approval. |
| `APPROVED` | Approved; fCeBM burned and tCeBM minted on-chain. |
| `REJECTED` | Declined by the Central Bank. |

### Redeem request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Submitted; awaiting Central Bank approval. |
| `APPROVED` | Approved; fiat reserve released. |
| `REJECTED` | Declined by the Central Bank; check the Rejection Reason column. |
| `MINT_FAILED` | Error during on-chain processing — contact your Central Bank. |

### Transfer progress states

| State | Meaning |
|---|---|
| `BRIDGE_IN_PROGRESS` | Your tCeBM is being locked on your spoke. |
| `SWAP_IN_PROGRESS` | The AMM swap is executing on the Hub. |
| `BRIDGE_OUT_PROGRESS` | Swapped tokens are being unlocked on the beneficiary's spoke. |
| `COMPLETED` | Transfer fully settled. |

### Bridge position states

| State | Meaning |
|---|---|
| `LOCKING` | Lock & Mint initiated; waiting for the relay to confirm the lock on the spoke. |
| `ACTIVE` | Funds are locked on the spoke and mirrored at the Hub. Burn & Unlock is now available. |
| `BURNING` | Burn & Unlock initiated; waiting for the relay to confirm the burn at the Hub. |
| `BURNED` | Mirrored position burned at the Hub; native CBDC unlock in progress on the spoke. |
| `RELEASED` | Native CBDC successfully unlocked on the spoke. Terminal state. |
| `RECONCILIATION_REQUIRED` | The relay encountered an error that requires manual intervention. Contact your Central Bank. |

### FX Agreement states (visible in Dashboard counter and `/agreements` — Scenario A routes only)

| State | Displayed as | Meaning |
|---|---|---|
| `FX_STATE_PROPOSED` | Proposed | Agreement has been proposed; counterparty must accept or reject. |
| `FX_STATE_ACCEPTED` | Accepted | Counterparty accepted; agreement proceeds to settlement. |
| `FX_STATE_REJECTED` | Rejected | Counterparty rejected the agreement. |
| `FX_STATE_CANCELLED` | Cancelled | Originator cancelled the agreement. |
| `FX_STATE_SETTLED` | Settled | Agreement has settled. |

### Onboarding statuses

| Status | Meaning |
|---|---|
| `NONE` | No onboarding request exists. Start the wizard. |
| `PENDING` | Onboarding request submitted; awaiting Central Bank KYC review. |
| `CREDENTIAL_REQUESTED` | System is internally requesting the PKI credential (shows as PENDING to you). |
| `KYC_APPROVED` | Central Bank approved KYC; final on-chain activation in progress. |
| `ACTIVE` | Institution fully onboarded; all operational screens are accessible. |
| `REJECTED` | Central Bank rejected the onboarding request. Contact governance support. |
| `REVOKED` | Institution access was revoked after activation. |
| `FROZEN` | Institution account was frozen. |

---

## 7. Troubleshooting

### Sign-in fails with "unauthorized"

- Verify your **Client ID** and **Client Secret** are correct.
- Confirm the spoke backend services are running and reachable (check with your infrastructure team).
- Confirm your institution's status is `ACTIVE` — if it is not, complete onboarding first.

### Operational screens are inaccessible after login

- Your institution is not yet `ACTIVE`. Navigate to **Onboarding** to check the wizard state and continue from where it left off.

### Issuance, tokenisation, or redeem request is stuck in PENDING

- The Central Bank must approve it from the Governance Portal. Contact your Central Bank governance operator.
- Click **Refresh** on the relevant page to check for status updates.

### AMM approval error on Transfer or AMM Trading

- Navigate to **Approve AMM** and grant a sufficient allowance. The approved amount must cover the Amount In of the swap.

### "Pool not ACTIVE" — Transfer button disabled

- The Hub AMM pool is not yet activated for your currency pair. Contact your Central Bank to activate the pool.

### Circuit Breaker HALTED — swaps disabled

- Cross-border swaps have been suspended by the Central Bank (1-of-N pause governance). Wait for governance to resume (2-of-N resume). No action is required from the bank operator.

### Quote expired — Execute Transfer is disabled

- Click **Get Quote** again to obtain a fresh quote. If the pool is `HALTED` or `INACTIVE`, swaps are not possible until it is operational again.

### A transfer completed with BRIDGE OUT FAILED (critical)

- Your source funds were debited but the bridge-out to the beneficiary failed. Click **Acknowledge** in the critical error card. Do not retry. Contact your Central Bank immediately with the **Swap ID** shown on screen. Manual reconciliation is required.

### A bridge position is stuck in LOCKING or BURNING

- The relay carries events between the spoke and the Hub. If a position stays in `LOCKING` or `BURNING` for more than 2 minutes, click **Manual Refresh** on the Bridge page.
- If it remains stuck, check relay status in the **NOC Portal** before assuming a contract problem.
- If the position state transitions to `RECONCILIATION_REQUIRED`, contact your Central Bank.

### Onboarding wizard appears stuck at Step 3 (KYC Pending)

- Click **Refresh status** on the Step 3 screen to force an immediate status poll.
- If your institution already shows `ACTIVE` in the Governance Portal but the wizard still shows Step 3, log out and log back in, then navigate to Onboarding again.

### Compliance credential library is empty

- Credentials are issued by the Central Bank as part of the onboarding process. If the library is empty after your institution is `ACTIVE`, contact your Central Bank governance operator.
