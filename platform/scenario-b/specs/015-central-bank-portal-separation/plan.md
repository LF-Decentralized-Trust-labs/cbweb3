# Plan: Central Bank Portal Separation — Governance vs Treasury

**Status:** Planning
**Scope:** `scenario-b/frontend/apps/governance/` and `scenario-b/frontend/apps/treasury/`
**Branch:** `refactor/central-bank-portal-separation`
**Reference:** Scenario A portal separation (established pattern)

---

## 0. Goal

Enforce a clear responsibility boundary between the two Central Bank portals:

| Portal | Primary Responsibility |
|---|---|
| **Governance Portal** | Regulatory and supervisory functions — KYC/onboarding approvals, circuit breaker, AML oversight |
| **Treasury Portal** | Financial and operational functions — token issuance/minting, operational approvals, **liquidity injection** (Scenario B–specific) |

The Scenario A layout is the reference model. Scenario B currently misplaces the liquidity management feature inside the Governance Portal, and its Treasury Portal is missing the approval flows that are present in Scenario A's Treasury Portal.

---

## 1. Constitution Check

| Rule | Assessment |
|---|---|
| Scenario isolation | ✅ Changes touch only `scenario-b/frontend/` |
| Privacy (ZKP/Noto) | ✅ No token implementation change |
| Atomicity | ✅ UI routing only; no settlement logic touched |
| Compliance gate | ✅ No auth bypass; Keycloak flows unchanged |
| Test-first | ⚠️ E2E tests reference specific portal routes — update after route changes |
| Observability | ✅ Not affected |

---

## 2. Current State

### Governance Portal (Scenario B — `scenarioBChildren` and `scenarioBNavItems`)

| Route | Label | Category |
|---|---|---|
| `/` | Dashboard | governance |
| `/registry` | Registry | governance ✅ |
| `/liquidity` | Liquidity Management | **treasury** ← misplaced |
| `/deposits-approval` | Issuance Approvals | treasury (also in Scenario A Governance) |
| `/escrows-approval` | Tokenisation Approvals | treasury (also in Scenario A Governance) |
| `/redeems-approval` | Redeem Approvals | treasury (also in Scenario A Governance) |
| `/swap-monitor` | Swap Monitor | governance ✅ |
| `/circuit-breaker` | Circuit Breaker | governance ✅ |
| `/oversight` | Oversight | governance ✅ |
| `/transfer-limits` | Transfer Limits | treasury (already in Treasury) |
| `/audit` | Audit | governance ✅ |
| `/settings` | Settings | — |

### Treasury Portal (Scenario B — fixed routing)

| Route | Label | Gap vs Scenario A |
|---|---|---|
| `/` | Dashboard | — |
| `/funding-requests` | Funding Requests | — |
| `/issuance` | Issuance | — |
| `/redemption` | Redemption | — |
| `/reconciliation` | Reconciliation | — |
| `/audit` | Audit | — |
| `/kyc` | KYC | — |
| `/transfer-limits` | Transfer Limits | — |
| `/settings` | Settings | — |
| ~~`/deposits-approval`~~ | ~~Issuance Approvals~~ | **MISSING** |
| ~~`/escrows-approval`~~ | ~~Tokenisation Approvals~~ | **MISSING** |
| ~~`/redeems-approval`~~ | ~~Redeem Approvals~~ | **MISSING** |
| ~~`/liquidity`~~ | ~~Liquidity Management~~ | **MISSING** (Scenario B specific) |

---

## 3. Target State

### Governance Portal (Scenario B — after refactoring)

Focused on regulatory/supervisory functions only:

| Route | Label | Note |
|---|---|---|
| `/` | Dashboard | |
| `/registry` | Registry | KYC/onboarding approvals |
| `/swap-monitor` | Swap Monitor | Scenario B specific |
| `/circuit-breaker` | Circuit Breaker | |
| `/oversight` | Oversight | AML disclosure quorum |
| `/deposits-approval` | Issuance Approvals | Kept for governance visibility (matches Scenario A pattern) |
| `/escrows-approval` | Tokenisation Approvals | Kept for governance visibility |
| `/redeems-approval` | Redeem Approvals | Kept for governance visibility |
| `/audit` | Audit | |
| `/settings` | Settings | |

**Removed:** `/liquidity` (moves to Treasury), `/transfer-limits` (already in Treasury, no duplication needed)

### Treasury Portal (Scenario B — after refactoring)

Full financial/operational surface, including Scenario B–specific liquidity injection:

| Route | Label | Note |
|---|---|---|
| `/` | Dashboard | |
| `/funding-requests` | Funding Requests | |
| `/issuance` | Issuance | Token minting |
| `/redemption` | Redemption | Token burning/redemption |
| `/reconciliation` | Reconciliation | |
| `/deposits-approval` | Issuance Approvals | **Added** — mirrors Scenario A Treasury |
| `/escrows-approval` | Tokenisation Approvals | **Added** — mirrors Scenario A Treasury |
| `/redeems-approval` | Redeem Approvals | **Added** — mirrors Scenario A Treasury |
| `/liquidity` | Liquidity Management | **Added** — moved from Governance |
| `/transfer-limits` | Transfer Limits | Already present |
| `/audit` | Audit | |
| `/settings` | Settings | |

**Removed:** `/kyc` — KYC approval is exclusively a Governance Portal responsibility via `/registry`; having a separate KYC page in Treasury creates a confusing split of the same concern. Evaluate whether the KycPage content is fully redundant before deleting the file.

---

## 4. Phases

### Phase 1 — Move Liquidity Management to Treasury

**Files changed:**

1. `scenario-b/frontend/apps/treasury/src/routes/index.tsx`
   - Import `LiquidityManagementPage` from Governance (or create a re-export/shared wrapper)
   - Add route `{ path: "liquidity", element: <LiquidityManagementPage /> }`

2. `scenario-b/frontend/apps/treasury/src/components/layout/Sidebar.tsx`
   - Add nav item `{ to: "/liquidity", label: "Liquidity Management", icon: Landmark }`

3. `scenario-b/frontend/apps/governance/src/routes/index.tsx`
   - Remove `{ path: "liquidity", element: <LiquidityManagementPage /> }` from `scenarioBChildren`
   - Remove import of `LiquidityManagementPage`

4. `scenario-b/frontend/apps/governance/src/components/layout/Sidebar.tsx`
   - Remove `{ to: "/liquidity", label: "Liquidity Management", icon: Landmark }` from `scenarioBNavItems`

**Decision on LiquidityManagementPage location:**
- If the page and its store/services have no Governance-specific dependencies, move the file tree
  (`features/liquidity/`, `services/api/liquidity.api.ts`, `stores/liquidity.store.ts`, `types/liquidity.types.ts`)
  to the Treasury app.
- If moving introduces significant refactor risk, keep the source in Governance but export the page
  via a shared package. Prefer moving unless it breaks the build.

### Phase 2 — Add Approval Pages to Treasury Portal

The approval page implementations already exist in the Governance app. Two options:

**Option A — Shared component (preferred if pages are identical)**
- Extract `DepositsApprovalPage`, `EscrowsApprovalPage`, `RedeemsApprovalPage` to `packages/ui` or a
  new `packages/shared-pages` workspace package.
- Import in both Governance and Treasury.

**Option B — Copy with scenario context (preferred if pages need treasury-specific layout/API)**
- Copy the three page files to `scenario-b/frontend/apps/treasury/src/pages/`.
- Adjust API service paths if the treasury backend uses different prefixes.
- Wire routes and sidebar entries.

Start with Option B (lower risk, faster); refactor to Option A in a follow-up if the duplication
becomes a maintenance problem.

**Files changed (Option B):**

1. Copy/create in `treasury/src/pages/`:
   - `DepositsApprovalPage.tsx`
   - `EscrowsApprovalPage.tsx`
   - `RedeemsApprovalPage.tsx`

2. `scenario-b/frontend/apps/treasury/src/routes/index.tsx`
   - Add three routes: `deposits-approval`, `escrows-approval`, `redeems-approval`

3. `scenario-b/frontend/apps/treasury/src/components/layout/Sidebar.tsx`
   - Add nav items for the three approval pages (after Issuance/Redemption group)

### Phase 3 — Remove Transfer Limits Duplication from Governance

Transfer Limits is already present in Treasury. Having it in Governance (Scenario B only) adds noise.

**Files changed:**

1. `scenario-b/frontend/apps/governance/src/routes/index.tsx`
   - Remove `{ path: "transfer-limits", element: <TransferLimitsPage /> }` from `scenarioBChildren`

2. `scenario-b/frontend/apps/governance/src/components/layout/Sidebar.tsx`
   - Remove Transfer Limits entry from `scenarioBNavItems`

3. Evaluate whether the `TransferLimitsPage` file in Governance can be deleted or whether
   it is used by the `scenarioAChildren` path (Scenario A Governance doesn't have it — safe to delete
   from governance if unused by Scenario A routes).

### Phase 4 — Evaluate and Remove KYC Page from Treasury

1. Read `scenario-b/frontend/apps/treasury/src/pages/KycPage.tsx` and compare with
   Governance's `RegistryPage.tsx`.
2. If fully redundant → remove route, sidebar entry, and the page file.
3. If the KYC page serves a distinct Treasury function (e.g., viewing compliance status without
   approval authority) → keep but scope-limit the label to avoid confusion with the Governance registry.

### Phase 5 — E2E and Integration Test Updates

1. Grep for hardcoded `/liquidity` paths in `scenario-b/tests/` and update to the new portal URL
   (treasury hostname + `/liquidity`).
2. Grep for approval page paths (`/deposits-approval`, etc.) in treasury E2E tests; if missing, add
   smoke tests covering navigation to the newly added routes.
3. Run `make scenario-b.test` to verify no broken references.

---

## 5. Complexity Tracking

| Decision | Chosen | Rejected | Reason |
|---|---|---|---|
| Liquidity page location | Move to Treasury app | Keep in Governance | Liquidity injection is a treasury operation (sovereign fund management); placing it in Governance conflates regulatory and financial roles |
| Transfer Limits in Governance | Remove from Governance | Keep in both | Duplication; the parameter is treasury-owned and doesn't require governance authority in Scenario B |
| Approval pages in Treasury | Add (copy) | Leave in Governance only | Mirrors Scenario A pattern; treasury operator should have full approval surface without switching portals |
| Approval pages in Governance | Keep | Remove | Matches Scenario A where both portals surface the approvals; governance monitoring role justifies visibility |
| KYC page in Treasury | Remove (after audit) | Keep | Registry/KYC authority belongs exclusively to Governance; treasury operators should not have a parallel KYC approval surface |
| Code sharing strategy | Copy first (Option B) | Extract to shared package (Option A) | Lower immediate risk; refactor if maintenance cost warrants it |

---

## 6. Non-goals

- No changes to backend API contracts or service implementations.
- No changes to Scenario A portals (they are the reference; do not regress them).
- No changes to the Bank, NOC, or Supervisor portals.
- No redesign of the approval page UX or data models — routing/navigation only in Phases 1–3.
