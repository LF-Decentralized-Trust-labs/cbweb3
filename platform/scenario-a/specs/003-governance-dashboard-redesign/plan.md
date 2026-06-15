# Plan: Scenario A — Governance Portal Dashboard Redesign

**Status:** Planning
**Scope:** `scenario-a/frontend/apps/governance/src/pages/DashboardPage.tsx` only
**Branch:** `refactor/central-bank-portal-separation`
**Backend changes:** None — all data sources already exist

---

## 0. Problem

The current dashboard renders one Card (Circuit Breaker badge) and one Quick Actions button. Every
other data block is commented out. The hooks and stores are already wired; the API calls are already
being made on mount — the data just never reaches the UI.

This is a pure frontend presentation task. No new endpoints, no new stores, no new API calls.

---

## 1. Available Data (existing hooks, no changes needed)

| Hook | Store method | Data used on dashboard |
|---|---|---|
| `useRegistry()` | `fetch()` | `participants[]`, `pendingKyc[]` |
| `useAccounts()` | `fetch()` | `accounts[]` |
| `useCircuitBreaker()` | `fetchState()` | `circuitBreaker` |
| `useAuditLogs()` | `fetch()` | `logs[]` |
| `useParameters()` | `fetch()` | `parameters` — **currently not called from dashboard** |

`useParameters` is the only addition: one extra `fetch()` call in the existing `useEffect`.

---

## 2. Derived values

All computed from existing data, no backend changes:

```ts
const activeParticipants   = participants.filter(p => p.status === "ACTIVE").length;
const pendingOnboardings   = pendingKyc.length;                   // CREDENTIAL_REQUESTED
const frozenAccounts       = accounts.filter(a => a.frozen).length;
const recentEvents         = logs.slice(0, 5);                    // latest 5 audit entries
const isHalted             = circuitBreaker?.state === "HALTED";
```

---

## 3. Proposed Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  SECTION 1 — Status KPIs (4 cards, 1 row on xl, 2×2 on md)     │
│                                                                  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────┐ │
│  │ Circuit       │ │ Pending KYC  │ │ Active        │ │ Frozen │ │
│  │ Breaker       │ │ Approvals    │ │ Participants  │ │ Accts  │ │
│  │               │ │              │ │               │ │        │ │
│  │  [LIVE]       │ │      3       │ │      12       │ │   1    │ │
│  │  HALTED badge │ │  → /registry │ │  → /accounts  │ │→/accts │ │
│  │  if halted    │ │  warn if > 0 │ │               │ │warn>0  │ │
│  └──────────────┘ └──────────────┘ └──────────────┘ └────────┘ │
└─────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────┐ ┌─────────────────────────────┐
│  SECTION 2A — Recent Events      │ │  SECTION 2B — Sys. Params   │
│  (last 5 audit log entries)      │ │  (read-only snapshot)       │
│                                  │ │                             │
│  Time      Action   Cat  Sev     │ │  Tx Min          R$ 100    │
│  ────────  ───────  ───  ────    │ │  Tx Max          R$ 50k    │
│  14:02:11  KYC...   CRED INFO    │ │  Slippage        0.5%      │
│  13:55:04  Freeze   FREZ WARN    │ │  Settlement      300s      │
│  13:20:00  Param..  PARA INFO    │ │                             │
│  ...                             │ │  [Edit Parameters →]        │
│                                  │ │                             │
│  [View all audit logs →]         │ │                             │
└──────────────────────────────────┘ └─────────────────────────────┘
```

---

## 4. Card Specifications

### KPI Card 1 — Circuit Breaker

- **Data:** `circuitBreaker.state`, `circuitBreaker.updatedAt`, `circuitBreaker.updatedBy`
- **Visual:** Large badge (`LIVE` = green default, `HALTED` = red destructive)
- **Sub-text:** "Last changed by `updatedBy` on `updatedAt`" (only when not loading)
- **CTA:** `<Link to="/circuit-breaker">` wraps the card or a small "Manage →" link
- **Alert:** When `HALTED`, apply a destructive/warning ring or border color to the card

### KPI Card 2 — Pending KYC Approvals

- **Data:** `pendingOnboardings` (= `pendingKyc.length`)
- **Visual:** Count as large number; badge `"NEEDS ATTENTION"` when `> 0`
- **CTA:** `<Link to="/registry">Review KYC →</Link>` always visible
- **Alert:** When `> 0`, card description text turns amber/warning color

### KPI Card 3 — Active Participants

- **Data:** `activeParticipants` / `participants.length` — show as `12 / 14`
- **Visual:** `active / total` format; no alert state needed
- **CTA:** `<Link to="/accounts">View all →</Link>`

### KPI Card 4 — Frozen Accounts

- **Data:** `frozenAccounts`
- **Visual:** Count; when `> 0`, badge `"FROZEN"` with destructive variant
- **CTA:** `<Link to="/accounts">Manage →</Link>`
- **Alert:** When `> 0`, card border turns destructive color

---

### Table — Recent Governance Events

- **Data:** `logs.slice(0, 5)` from `useAuditLogs()`
- **Columns:**
  | Column | Source field | Notes |
  |---|---|---|
  | Time | `createdAt` | `toLocaleTimeString()` — short format |
  | Action | `action` | Truncate at 40 chars |
  | Category | `category` | Small badge: CREDENTIAL, FREEZE, CIRCUIT_BREAKER, PARAMETER |
  | Severity | `severity` | Badge: INFO=secondary, WARNING=warning, CRITICAL=destructive |
  | Outcome | `outcome` | Badge: SUCCESS=default, FAILED=destructive |
- **Empty state:** "No governance events recorded yet."
- **Footer:** `<Link to="/audit">View all audit logs →</Link>`

---

### Card — System Parameters

- **Data:** `parameters` from `useParameters()` — can be `null` while loading
- **Rows (label → value):**
  | Label | Field | Format |
  |---|---|---|
  | Min Transaction | `txLimitMin` | currency-formatted number |
  | Max Transaction | `txLimitMax` | currency-formatted number |
  | Slippage Tolerance | `slippageTolerance` | `N%` |
  | Settlement Window | `settlementWindowSeconds` | `Ns` |
- **Loading state:** skeleton or "—" placeholders
- **CTA:** `<Link to="/parameters">Edit parameters →</Link>`

---

## 5. Implementation Steps

### Step 1 — Update `DashboardPage.tsx`

**Hooks to add:**
```ts
const { parameters, fetch: fetchParameters } = useParameters();
```

**useEffect — add `fetchParameters`:**
```ts
useEffect(() => {
  void fetchRegistry();
  void fetchAccounts();
  void fetchAudit();
  void fetchState();
  void fetchParameters();    // add
}, [fetchRegistry, fetchAccounts, fetchAudit, fetchState, fetchParameters]);
```

**Derived values (uncomment + add):**
```ts
const activeParticipants = participants.filter(p => p.status === "ACTIVE").length;
const frozenAccounts     = accounts.filter(a => a.frozen).length;
const pendingKycCount    = pendingKyc.length;
const recentEvents       = logs.slice(0, 5);
const isHalted           = circuitBreaker?.state === "HALTED";
```

**JSX:** Replace the current sparse layout with the 2-section layout described in §3–4.

### Step 2 — Imports

Add to the `@cbweb3/ui` import block whatever components are missing. Expected additions:
`Table`, `TableBody`, `TableCell`, `TableHead`, `TableHeader`, `TableRow`
(these are already imported but commented out — just uncomment).

Add `useParameters` to the hooks import line.

### Step 3 — No other files change

- No new stores, no new hooks, no new API files.
- No changes to routes or sidebar.
- No backend changes.

---

## 6. Non-goals

- No charts, graphs, or time-series visualizations — this dashboard is operational, not analytical.
- No real-time polling on the dashboard itself — the existing Sidebar already polls Circuit Breaker
  state; the dashboard fetches once on mount (consistent with all other pages).
- No pagination on the events table — 5 rows is the right density for a dashboard widget; full
  history is on `/audit`.
- No token/balance data — that belongs to the Treasury Portal.
- No HTLC/escrow data — same reason.

---

## 7. Complexity Tracking

| Decision | Chosen | Rejected | Reason |
|---|---|---|---|
| Parameters on dashboard | Read-only snapshot | Full inline edit | Dashboard is overview-only; editing belongs on /parameters |
| Audit log filter | No filter (show latest 5) | Show only WARNING/CRITICAL | Latest 5 gives a truthful activity feed; governance operator needs to see INFO events too (e.g. KYC approvals) |
| Participant count | active / total format | Active count alone | Total context makes the ratio meaningful |
| Frozen accounts alert | Card-level visual alert | Modal/toast | Non-blocking; operator can click through to /accounts |
| useParameters in dashboard | Add fetch() call | Leave unloaded | Parameters are fast/lightweight and are governance-relevant context |
