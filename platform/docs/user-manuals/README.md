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

Every portal below is wired to a live backend through the API Gateway. The
**Scenario B** Governance portal additionally carries a mock toggle for local UI
work; the toolkit switches it off when it builds the portal, so a deployed portal
is live. The manuals only describe features that exist in
the shipped product, and each manual states at the top exactly where its data
comes from. A portal's screens are populated only when the backend services it
calls are reachable; in an offline or demo environment with the backends down,
screens show empty tables or zero counters rather than fabricated data.

| Portal | Scenario A | Scenario B | Data source today |
|---|---|---|---|
| **Bank** | Live backend | Live backend | Real API (API Gateway); no mock toggle is read in either scenario |
| **Governance** | Live backend (Dashboard + Registry exposed) | Live backend; a mock toggle exists for local UI work but is off in deployed stacks | Scenario A has no working mock path — its toggle is inert. **Scenario B** consumes `VITE_USE_MOCKS`, read as `!== "false"`; the toolkit passes `false`, so only a hand-run dev server sees mocks |
| **Treasury** | Live backend (approvals exposed) | Live backend | Real API (API Gateway) |
| **Supervisor** | Live backend (connectivity-dependent) | Live backend (connectivity-dependent) | Real API; every screen calls live endpoints. See the note below for auth/data caveats |
| **NOC** | Live backend | Live backend | Real API — connects to the live NOC backend service |

> **Important:** The **Supervisor** portal's screens call real backend
> endpoints through the API Gateway — it is **not** a hardcoded mock-data or
> fake-login build, and its frontend holds no sample data. Two caveats remain,
> documented per screen in each manual:
>
> - **Data freshness.** When the backend services are offline (as in most
>   local/demo setups) screens show empty tables or zero counters, and in a
>   development environment the endpoints may return demonstration data rather
>   than a live production data set.
> - **Authentication.** Scenario B authenticates against the platform's OIDC
>   provider (Keycloak). In Scenario A, production Keycloak enforcement is not yet
>   active, so the login is not a production security boundary.
>
> Do not treat figures shown in an offline or development environment as
> authoritative for any real supervisory decision.

---

## Scenario A — Enhanced Correspondent Banking

| Portal | Who uses it | Manual |
|---|---|---|
| Bank | Commercial bank operators | [scenario-a/bank.md](./scenario-a/bank.md) |
| Governance | Central bank governance team | [scenario-a/governance.md](./scenario-a/governance.md) |
| Treasury | Central bank treasury / issuance team | [scenario-a/treasury.md](./scenario-a/treasury.md) |
| Supervisor | Supervisors / regulators (preview) | [scenario-a/supervisor.md](./scenario-a/supervisor.md) |
| NOC | Network operations engineers | [scenario-a/noc.md](./scenario-a/noc.md) |

For the cross-portal end-to-end golden path (who does what, in which portal, in
what order) see **[scenario-a/end-to-end-walkthrough.md](./scenario-a/end-to-end-walkthrough.md)**.

## Scenario B — International Hub

| Portal | Who uses it | Manual |
|---|---|---|
| Bank | Commercial bank operators | [scenario-b/bank.md](./scenario-b/bank.md) |
| Governance | Central bank governance team | [scenario-b/governance.md](./scenario-b/governance.md) |
| Treasury | Central bank treasury / issuance team | [scenario-b/treasury.md](./scenario-b/treasury.md) |
| Supervisor | Supervisors / regulators (preview) | [scenario-b/supervisor.md](./scenario-b/supervisor.md) |
| NOC | Network operations engineers | [scenario-b/noc.md](./scenario-b/noc.md) |

For the cross-portal end-to-end golden path (who does what, in which portal, in
what order) see **[scenario-b/end-to-end-walkthrough.md](./scenario-b/end-to-end-walkthrough.md)**.

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
