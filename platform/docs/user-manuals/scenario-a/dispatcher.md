# Dispatcher Portal — User Manual (Scenario A)

**Audience:** Any operator or end-user who needs to reach a CBWeb3 institutional portal and does not know its direct URL.
**Data source:** No backend calls. Routing is performed entirely in the browser using a static, pre-configured mapping of Client ID prefixes to portal URLs.

---

## 1. Overview

The Dispatcher Portal is the **single, shared entry point** for all institutions participating in Scenario A — Enhanced Correspondent Banking. Instead of distributing a different URL for each institution, every user starts at the same Dispatcher address and enters their **Client ID**. The portal resolves the Client ID to the correct institutional portal and redirects the browser there, pre-filling the username field on the destination login page.

This means:

- Administrators publish **one URL** for the entire platform.
- Users do not need to know which bank-specific portal URL to open.
- Routing is deterministic: the Client ID prefix uniquely identifies the institution.

The Dispatcher is a **single-screen application**. There is no sidebar, no navigation menu, and no authenticated session of its own. Its only job is the redirect.

<!-- TODO: screenshot — full Dispatcher landing page on a 1280 px wide viewport -->

---

## 2. Supported institutions and Client ID prefixes

The Dispatcher recognises the following prefixes. A Client ID that starts with any of these strings is routed to the corresponding portal. Matching is case-insensitive.

| Institution | Client ID prefix | Default portal URL (local dev) |
|---|---|---|
| Bank A | `bank-a-` | `http://localhost:5173` |
| Bank B | `bank-b-` | `http://localhost:5174` |
| Bank C | `bank-c-` | `http://localhost:5175` |
| Bank D | `bank-d-` | `http://localhost:5176` |
| Central Bank A | `central-bank-a-` | `http://localhost:5177` |
| Central Bank B | `central-bank-b-` | `http://localhost:5178` |

In production or staging deployments, the administrator overrides these URLs with environment variables (see [Section 6 — Configuration](#6-configuration)). The prefix logic itself does not change.

---

## 3. Access

The Dispatcher Portal requires no login of its own. It is publicly accessible at whatever address your administrator has assigned.

**Typical addresses**

| Environment | Example URL |
|---|---|
| Local development | `http://localhost:5180` (confirm with your administrator) |
| Staging / test | Provided by the platform administrator |
| Production | Provided by the platform administrator |

No Keycloak account, no credentials, and no VPN access is required to reach the Dispatcher itself. Authentication happens on the institutional portal you are redirected to.

---

## 4. Screen — Portal Dispatcher (`/`)

The application consists of a single page with two visual regions.

### 4.1 Information panel (left column, desktop only)

Visible only on screens wider than approximately 1024 px. This panel describes the purpose of the Dispatcher and lists three properties of the platform:

| Label | Meaning |
|---|---|
| **Single entry point** | One URL for all institutions — automatically routed to your portal. |
| **Institution-bound access** | Credentials are verified against your institution's auth service. |
| **Secure by design** | Authentication happens on your institution's dedicated portal. |

This panel is informational only. No interaction is required.

### 4.2 Access form (right column / full width on mobile)

<!-- TODO: screenshot — close-up of the access form card -->

| Element | Description |
|---|---|
| **Client ID field** | Text input. Enter the Client ID issued to you by your institution. Minimum 3 characters. The placeholder shows `e.g. bank-a-client` as a format hint. |
| **Continue button** | Submits the form. The Dispatcher immediately resolves the Client ID and redirects your browser. |
| **Inline error message** | Appears below the Client ID field if the value is too short or if the prefix does not match any known institution. |

---

## 5. Typical workflow

### 5.1 Reaching your institutional portal

1. Open the Dispatcher URL in your browser.
2. In the **Client ID** field, type your Client ID exactly as it was issued to you by your Central Bank (for example, `bank-a-operator`).
3. Click **Continue**.
4. The Dispatcher checks the prefix of your Client ID against the known institution list.
   - If the prefix is recognised, your browser is redirected to your institution's portal login page. The username field on that login page is pre-filled with your Client ID.
   - If the prefix is not recognised, an error appears: *"Institution not identified. Please check your Client ID."* Verify your Client ID with your administrator and try again.
5. On the destination login page, enter your password and complete the Keycloak login flow.

### 5.2 What happens after the redirect

Once redirected, you are on your institution's dedicated portal — Bank Portal, Treasury Portal, Governance Portal, or another — and the Dispatcher is no longer involved. All subsequent actions take place within that portal and are authenticated by Keycloak.

---

## 6. Configuration

The Dispatcher does not expose a configuration UI. Portal target URLs are set at build time or via environment variables injected at container startup. The following variables are read by the application:

| Variable | Corresponding institution | Default (local dev) |
|---|---|---|
| `VITE_BANK_A_PORTAL_URL` | Bank A | `http://localhost:5173` |
| `VITE_BANK_B_PORTAL_URL` | Bank B | `http://localhost:5174` |
| `VITE_BANK_C_PORTAL_URL` | Bank C | `http://localhost:5175` |
| `VITE_BANK_D_PORTAL_URL` | Bank D | `http://localhost:5176` |
| `VITE_CENTRAL_BANK_A_PORTAL_URL` | Central Bank A | `http://localhost:5177` |
| `VITE_CENTRAL_BANK_B_PORTAL_URL` | Central Bank B | `http://localhost:5178` |

These are `VITE_` prefixed variables, which means they are embedded in the compiled JavaScript bundle. If you change them you must rebuild the Dispatcher frontend. Refer to the [configuration reference](../../../scenario-a/docs/runbooks/configuration-reference.md) and [deployment runbook](../../../scenario-a/docs/runbooks/deployment-runbook.md) for details.

---

## 7. Troubleshooting

### "Institution not identified. Please check your Client ID."

The Client ID you entered does not begin with any of the recognised prefixes. Common causes:

- **Typo in the prefix.** The prefix is case-insensitive but must otherwise match exactly (for example, `bank-a-` not `banka-`).
- **Wrong Client ID.** Confirm your Client ID with your Central Bank administrator.
- **New institution not yet added.** If your institution was recently onboarded, the Dispatcher may not yet have a routing entry for it. Contact your platform administrator.

### The redirect goes to the wrong portal

Verify that your Client ID prefix is correct. If it starts with `bank-b-` you will be sent to the Bank B portal regardless of the rest of the ID. If you believe the routing is misconfigured, contact your platform administrator to check the `VITE_BANK_*_PORTAL_URL` environment variables deployed for your environment.

### The destination portal login page does not load after the redirect

The Dispatcher redirected you successfully but the target portal is unreachable. Possible causes:

- The institutional portal service is not running.
- The portal URL environment variable points to the wrong address.
- A network or firewall policy is blocking access.

Check with your platform administrator or network operations team. The Dispatcher itself is functioning correctly if you saw no error on the Dispatcher page.

### The username field is empty on the destination login page

The redirect URL includes your Client ID as a `username` query parameter. Some portals or Single Sign-On configurations may not consume this parameter. Enter your Client ID manually in the username field and continue with the normal login flow.

### The Dispatcher page fails to load at all

This is a static React application with no backend dependency. If the page does not load, the problem is at the web server or network layer — not in the application. Confirm the Dispatcher service is running (`make frontend-spoke-all` or equivalent) and that the port is reachable from your browser.

---

## 8. Related documentation

| Document | Location |
|---|---|
| Configuration reference (all environment variables) | [`scenario-a/docs/runbooks/configuration-reference.md`](../../../scenario-a/docs/runbooks/configuration-reference.md) |
| Deployment runbook | [`scenario-a/docs/runbooks/deployment-runbook.md`](../../../scenario-a/docs/runbooks/deployment-runbook.md) |
| Bank Portal manual | [`scenario-a/bank.md`](./bank.md) |
| Governance Portal manual | [`scenario-a/governance.md`](./governance.md) |
| Treasury Portal manual | [`scenario-a/treasury.md`](./treasury.md) |
| Supervisor Portal manual | [`scenario-a/supervisor.md`](./supervisor.md) |
| NOC Dashboard manual | [`scenario-a/noc.md`](./noc.md) |
