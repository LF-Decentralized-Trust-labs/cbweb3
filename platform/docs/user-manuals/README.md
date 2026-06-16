# CBWeb3 Platform — Per-Portal User Manuals

This directory contains end-user manuals for every web portal shipped with the
CBWeb3 platform, organised by scenario. The manuals are written for the people
who will actually operate the portals at participating institutions — bank
operators, central-bank governance teams, treasury teams, supervisors and
network operations engineers — not for developers.

Two independent products live side by side:

- **Scenario A — Enhanced Correspondent Banking** (dual-layer HTLC PvP settlement)
- **Scenario B — International Hub** (FXAgreement + AMM + bridge, with relay and circuit breaker)

Treat them as separate products. A portal may exist in both scenarios but expose
a different set of screens in each; each manual documents the portal **as it
behaves in that scenario**.

---

## How to use these manuals

- Find your scenario, then your portal, in the table below.
- Each manual walks through every screen that is currently available in that
  portal, with a status reference and a troubleshooting section.
- Screenshots are referenced as placeholders (`<!-- TODO: capture screenshot -->`)
  pointing at `img/<scenario>/…`. They will be captured against the running test
  environment and dropped into those folders.

---

## Implementation status — read this first

Not every portal is wired to a live backend yet. The manuals only describe
features that exist in the shipped product, and each manual states clearly at the
top where it currently runs on **mock (sample) data** rather than live data.

| Portal | Scenario A | Scenario B | Data source today |
|---|---|---|---|
| **Dispatcher** | Static routing only | N/A | No backend — all routing is client-side |
| **Bank** | Live backend | Live backend | Real API (API Gateway) |
| **Governance** | Live backend (Dashboard + Registry exposed) | Live backend | Real API, with a development mock toggle |
| **Treasury** | Live backend (approvals exposed) | Mock data | See note in the manual |
| **Supervisor** | **Mock data only** | **Mock data only** | Sample data + demo login — **not production** (see R2-CR-8) |
| **NOC** | Mock data only | Mock data only | Sample data — monitoring views are illustrative |

> **Important (R2-CR-8):** The **Supervisor** portal currently runs entirely on
> mock data and a demonstration login. It is a UI preview, not a live regulatory
> tool. Its manual documents only the screens that exist and flags every place
> where the data is illustrative. Do not rely on Supervisor figures for any real
> decision.

---

## Scenario A — Enhanced Correspondent Banking

| Portal | Who uses it | Manual |
|---|---|---|
| Dispatcher | All users — unified entry point that routes to the correct institutional portal | [scenario-a/dispatcher.md](./scenario-a/dispatcher.md) |
| Bank | Commercial bank operators | [scenario-a/bank.md](./scenario-a/bank.md) |
| Governance | Central bank governance team | [scenario-a/governance.md](./scenario-a/governance.md) |
| Treasury | Central bank treasury / issuance team | [scenario-a/treasury.md](./scenario-a/treasury.md) |
| Supervisor | Supervisors / regulators (preview) | [scenario-a/supervisor.md](./scenario-a/supervisor.md) |
| NOC | Network operations engineers | [scenario-a/noc.md](./scenario-a/noc.md) |

## Scenario B — International Hub

| Portal | Who uses it | Manual |
|---|---|---|
| Bank | Commercial bank operators | [scenario-b/bank.md](./scenario-b/bank.md) |
| Governance | Central bank governance team | [scenario-b/governance.md](./scenario-b/governance.md) |
| Treasury | Central bank treasury / issuance team | [scenario-b/treasury.md](./scenario-b/treasury.md) |
| Supervisor | Supervisors / regulators (preview) | [scenario-b/supervisor.md](./scenario-b/supervisor.md) |
| NOC | Network operations engineers | [scenario-b/noc.md](./scenario-b/noc.md) |

---

## Related operator documentation

- **Scenario A Bank Portal — detailed operator guide:**
  [`scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md`](../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md)
  is the reference-quality walkthrough of the Bank Portal and is the template
  these manuals follow. The Scenario A Bank manual in this directory links to it.
- **Onboarding:** [`scenario-a/docs/runbooks/ONBOARDING-DOCS.md`](../../scenario-a/docs/runbooks/ONBOARDING-DOCS.md)

---

## Conventions used in every manual

- **Route** — the in-app URL path (the part after the portal's base address).
- **Sidebar label** — the name as it appears in the portal's left navigation.
- **Status / state tables** — the exact status values you will see in the UI and
  what each one means.
- **Planned / not yet available** — a feature that exists in the design but is not
  exposed in the current build. These are called out explicitly and never
  documented as if they work.
