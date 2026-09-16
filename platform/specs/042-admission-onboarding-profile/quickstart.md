# Quickstart — Verify the Admission Profile (per scenario)

Goal: bring up a scenario and confirm five operator checks — **Admission signs in and sees only its own
surface**, **Admission mutates**, **Governance reads but cannot mutate onboarding**, **Governance keeps
everything else (including certificate issuance)**, **the central bank signs** — plus Scenario A's A2
assertions. Run each scenario independently (Scenario-Scoped Independence).

## Prerequisites

- Docker Compose v2, Go 1.26+, Foundry (contracts are unchanged, but the stack builds them).
- **Three accounts**, because the checks below are as much about refusals as about successes:
  1. an **Admission-only** operator (checks 1, 2, and the negative half of 4);
  2. a **governance-only** operator (checks 3 and 4);
  3. a **dual-grant** bootstrap operator holding both roles (SC-005 / the existing E2E and tryout flows).
  The dual-grant operator alone cannot detect a broken authorization boundary — it passes either way.

## Scenario B (cleaner — approve already off-chain)

```bash
cd scenario-b
make scenario-b.up
# unit/authz regression + invariant tests
make scenario-b.test-backend
# user-story walkthroughs
make scenario-b.tryout-us1
make scenario-b.test            # full suite
make frontend-scenario-b        # governance portal
```

Manual checks:
1. **Admission signs in, and sees only its own surface**: with an Admission-only account, log in to the
   governance portal — it must authenticate (this catches the `stores/auth.store.ts` login gate, R12). Then
   confirm the sidebar shows only Dashboard + Registry, and that typing a governance-only URL directly
   (`/accounts`, `/circuit-breaker`, `/audit`, `/settings`) redirects rather than loading a page full of 403s
   (this catches a missing T016a/T032a — the widened portal gate without per-page authorization).
2. **Admission mutates**: with an Admission token, `POST /api/v1/compliance/approve-kyc` → participant
   reaches `KYC_APPROVED`; `GET /api/v1/compliance/participants` and `GET /api/v1/governance/registry`
   both load (the registry read is the check that catches the prefix-scoped `govGroup` guard, R8/INV-4).
3. **Governance reads-only**: with a governance-only token, `GET /api/v1/compliance/participants` → 200,
   `GET /api/v1/governance/registry` → 200; `POST /api/v1/compliance/approve-kyc` and
   `POST /api/v1/governance/participants` → 403.
4. **Governance keeps everything else**: with the same governance-only token,
   `POST /api/v1/governance/registry/csr`, `POST /api/v1/governance/accounts/freeze`,
   `POST /api/v1/governance/circuit-breaker/toggle` and `PUT /api/v1/governance/parameters` → still 2xx
   (FR-004 / INV-5). With an Admission-only token, the CSR route → 403 (FR-002a).
5. **Central bank signs**: after onboarding completion, the on-chain register+verify signer is the
   central-bank key (auth-service `CompleteOnboarding`), never the Admission operator.

## Scenario A (includes the A2 split)

```bash
cd scenario-a
# bring up per this scenario's README / make targets, then:
go test ./backend/services/...      # authz regression + invariant + approve-flow attribution
# E2E / tryout equivalents for scenario A
```

**Scenario A's portal-critical routes differ from B's.** A's governance portal reads the registry via
`GET /api/v1/governance/registry` and approves via `POST /api/v1/governance/approve-kyc`
(`frontend/apps/governance/src/services/api/registry.api.ts:14,19,39`) — **both inside `govGroup`**. So in
Scenario A the group relaxation is what makes the Admission profile usable at all; substitute these two
routes for B's `/compliance/*` pair in checks 2 and 3 below.

Manual checks (the same five as Scenario B, with the route substitution above) plus the **A2 assertions**:
- `ApproveKYC` (compliance) **no longer** calls `RegisterParticipant`/`VerifyParticipant` inline; the
  on-chain register+verify happens in the auth service's existing `CompleteOnboarding`.
- Invariant test: no `ROLE_ADMISSION`-authorized request produces an `IdentityRegistry` transaction
  signed by anything other than the central-bank key — covering **both** of Scenario A's inline paths
  (compliance `ApproveKYC` and auth `OnboardParticipant`, see contracts/authorization-matrix.md INV-2).
- ~~**New semantics** (FR-017): a participant at `KYC_APPROVED` is no longer necessarily on-chain
  `Verified`.~~ **Amendment 1 (2026-09-01): no longer applies.** Approval still registers and verifies
  on-chain in the same action, so there is no window to test and no tryout script to revisit.

## Cross-cutting checks (both)

- **Manifest validation**: a central-bank sample manifest with the admission `adminUsers[]` entry passes;
  remove the entry → validation fails (required). A **non**-central-bank declaration without the entry must
  still pass — check `scenario-b/samples/hub/hub-cbweb3.yaml` and a `join`-mode bank manifest, plus
  Scenario A's `commercial-bank` and `noc` samples.
- **Both Keycloak paths**: the `ROLE_ADMISSION` realm role must exist after a `deploy/local` bring-up
  (`init.sh`) *and* after a toolkit `apply` (`realmRolesForAdminRole` / `renderRealmJSON`). Verify the JWT
  carries `ROLE_ADMISSION`, not a bare `ADMISSION`.
- **Dual-grant migration**: the bootstrap operator holding both roles completes the full onboarding E2E
  with no regression (SC-005).
- **No contract redeploy**: `contracts.sync-addresses` is NOT required; `IdentityRegistry` unchanged.

## Definition of done (maps to Success Criteria)

- **SC-001 / SC-002a**: an Admission-only operator signs in and completes every onboarding-record action
  (read participant/pending/registry, approve KYC, register the participant record) with no governance grant.
- **SC-002**: a governance-only operator is refused approve-KYC and participant-record registration, while
  still succeeding on the read views **and** on freeze/unfreeze, circuit-breaker, parameters,
  **certificate issuance** and **central-bank operator provisioning**.
- **SC-003 / SC-004**: 100% of on-chain register+verify are CB-signed; the audit trail distinguishes the
  off-chain approver from the on-chain signer.
- **SC-005**: existing E2E/tryout flows pass with the dual-grant bootstrap operator, no regression.
- **SC-006**: zero contract redeployments and zero new runtime dependencies.
- **SC-007**: **not establishable from this quickstart alone.** Running it in one scenario says nothing about
  parity with the other. Once both PRs have landed, walk the same operator journey in each portal and compare
  what the operator can do, is refused, and can see (task T043a). Wiring differences — which route serves a
  view, which provisioning mechanism is used, the manifest role spelling — are expected and do not violate
  SC-007; differences in operator-visible capability do.
