# Contract — Manifest Admission Operator (both scenarios)

The Admission operator login is a **required** `adminUsers[]` entry on every central-bank entity
declaration. This contract anchors the manifest-validation tests and the sample-manifest updates.

## Required entry

Every central-bank manifest MUST declare an admin user for the admission role (validation requires **at
least one**; a second entry for the same role is not meaningful and is not tested for):

```yaml
# Scenario A — ROLE_-prefixed
adminUsers:
  - role: ROLE_ADMISSION
    username: admin@<entity>.admission.gov
    password: <entity>-admission-local   # local profile only — secret store in staging/prod

# Scenario B — bare role name
adminUsers:
  - role: ADMISSION
    username: admin@<entity>.admission.gov
    password: <entity>-admission-local
```

## Validation contract

| # | Assertion | Scenario A location | Scenario B location |
|---|---|---|---|
| V1 | A central-bank manifest WITHOUT an admission admin user **fails** validation | `toolkit/engine/manifest/validate.go:286` — `requiredAdminRolesByEntity["central-bank"]` += `ROLE_ADMISSION` (already keyed off `spec.role`, checked at `:341`) | `toolkit/engine/manifest/validate.go:222` `validateAdminUsers` — **requires a signature change**, see below |
| V2 | A central-bank manifest WITH the admission user **passes** validation | same | same |
| V3 | The admission user maps to the central-bank realm and yields a `ROLE_ADMISSION` JWT claim | `toolkit/engine/orchestrator/keycloak.go:144` — add `ROLE_ADMISSION` to the `adminUsersForRealmRoles(...)` call inside `centralBankRealmPlans`; `renderRealmJSON` (`:171`) emits it | `toolkit/engine/orchestrator/admin_users.go:19` `realmRolesForAdminRole`: `ADMISSION → {central_bank, ROLE_ADMISSION}` |
| V4 | Non-central-bank entities are unaffected | required set keyed by `spec.role`; `commercial-bank` and `noc` untouched | keyed by `spec.topology.role`; `hub` and `join`-mode bank manifests untouched |

### Scenario B — why V1 needs a signature change

`validateAdminUsers(users []AdminUser, r *Result)` (`validate.go:222`) receives only the slice and has no
entity context, so it cannot express "required for central banks". Scenario B has no
`requiredAdminRolesByEntity` equivalent. The keying value does exist: `spec.topology.role` —
`central-bank` in `samples/brazil/central-bank-brazil.yaml:10`, `hub` in `samples/hub/hub-cbweb3.yaml`.
Pass it through (or add a sibling validator invoked from `validate.go:146` alongside the existing call) and
require `ADMISSION` only for `topology.role == "central-bank"`.

### Entity scope of the requirement

| Entity | Declares an Admission operator? | Why |
|---|---|---|
| A `spec.role: central-bank` / B `spec.topology.role: central-bank` | **Required** | Onboards commercial banks in its jurisdiction |
| B `spec.topology.role: hub` (`samples/hub/hub-cbweb3.yaml`) | **Exempt** | Commercial banks join spokes, not the hub. The hub declares `GOVERNANCE` + `NOC_ADMIN` only |
| A `spec.role: commercial-bank` / B `mode: join` + `topology.role: commercial-bank` (e.g. `samples/brazil/bank-itau.yaml:8,10`) | Exempt | Not an onboarding authority |
| A `spec.role: noc` | Exempt | Operations, not admission |

Without this table the "required immediately" clarification would fail hub provisioning.

## Keycloak realm-role provisioning (local/dev)

Both provisioning paths must be covered — they are independent.

| # | Assertion | Location |
|---|---|---|
| K1 | The `ROLE_ADMISSION` realm role exists for the central bank (docker-compose path) | `scenario-a/deploy/local/keycloak/init.sh:332` (`CENTRAL_BANK_ROLES`); `scenario-b/deploy/local/keycloak/init.sh:370` (`CENTRAL_BANK_ROLES`) |
| K2 | (B only) realm-role list includes admission if enumerated there | **Resolved: not required.** `scenario-b/deploy/local/keycloak/realms/scenario-b-realm.json` enumerates only the bare Scenario B roles (`central_bank`, `commercial_bank`, `hub_operator`); the `ROLE_*` family is created by `init.sh` |
| K3 | The realm role exists on the **toolkit** path | No separate task. Scenario B: `appendKeycloakUsers` (`orchestrator/step_found_spoke.go:385`) creates each mapped realm role idempotently, so V3 covers it. Scenario A: `renderRealmJSON` emits the realm's role list, so V3 covers it |

## Sample manifests to update (required)

All committed central-bank sample manifests gain the admission `adminUsers[]` entry:

- Scenario A: `scenario-a/samples/{argentina,colombia,brazil,proxy-smoke}/central-bank-*.yaml`
- Scenario B: `scenario-b/samples/{argentina,colombia,brazil,proxy-smoke}/central-bank-*.yaml`

Verified: four central-bank manifests per scenario, no others. `scenario-b/samples/hub/hub-cbweb3.yaml` is
**excluded** per the entity-scope table above. Scenario A's samples already declare all four roles currently
required (`ROLE_GOVERNANCE`, `ROLE_TREASURY`, `ROLE_SUPERVISOR`, `ROLE_NOC_ADMIN`), so the Admission entry is
a fifth; Scenario B's declare three (`GOVERNANCE`, `TREASURY`, `SUPERVISOR`), so it is a fourth.

No rendered `keycloak-import/*.json` exists to regenerate — verified absent under both `samples/` trees.

## Local/pilot dual-grant

For local/pilot bootstrap, the single bootstrap operator holds **both** governance and admission (so
existing single-operator E2E/tryout flows pass). This is a bootstrap convenience, not a manifest schema
change; production/staging assign the two roles to distinct identities.
