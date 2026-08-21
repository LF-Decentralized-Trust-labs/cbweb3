<!-- SPDX-License-Identifier: Apache-2.0 -->

# Scenario A — where the end-to-end tests actually live

**This directory is intentionally empty of tests.** Scenario A's end-to-end suite
exists; it is not here, and this file says where it is so the empty directory stops
implying the suite is missing.

That implication was the actual defect. The directory held only a `.gitkeep`, and a
reviewer reading the tree concluded Scenario A had no E2E coverage — the finding
recorded in card R1-12.10. What it really has is a naming mismatch: the E2E suite is
called *integration*.

## The live end-to-end test

`../integration/` — Go, build tag `integration`, entry point `TestFullHappyPath`.

It runs against a **live stack**: it takes the API-gateway URLs of the banks and both
central banks plus the Besu RPCs, drives the full correspondent-banking path, and
records per-step on-chain evidence (transaction hash, block number, gas) into a
machine-readable bundle. Timeout is 30 minutes because it is a real settlement, not a
mock.

```bash
make -C ../.. scenario-a.test-integration   # brings the stack up if it is not already running
```

> **This test does not pass on a clean local bring-up, and that is a provisioning gap
> rather than a defect.** Ran on 2026-08-21: `TestFullHappyPath` fails at `Phase1_Login`
> with `{"error":"invalid credentials"}`, and phases 2–8 then fail on the dependency.
> Phase0 passes and all five gateways answer 200 on `/healthz`, so the stack is up.
>
> The reason is a deliberate design decision, not a bug. Since `f55ade5b`
> (*feat(auth): require user (password grant) login for portals*, 2026-06-30) the auth
> service accepts **only** the OIDC password grant: portal login must be a real Keycloak
> **user** — the per-role admin users provisioned from `spec.adminUsers` — precisely so
> an operator cannot log in with a realm client id/secret. `client_credentials` is
> refused on purpose.
>
> This test predates that decision. It authenticates with the `clientId`/`clientSecret`
> pair from `backend/config/.env.infra.*`, which is exactly the credential type the
> decision rejects. Compounding it, `deploy/local/keycloak/init.sh` creates the clients
> and service accounts but **no users** — realms `bank-a`, `bank-b` and `central-bank-a`
> hold zero — so there is currently no user for the supported path to authenticate as
> either.
>
> Ruled out along the way, so nobody repeats the search: the client secrets are correct
> and identical to what Keycloak holds; the `.env.infra.*` files all exist (they are
> dotfiles, so a plain `ls` hides them); the clients live in **per-entity** realms, not
> in `cbweb3`; the auth service reaches Keycloak fine at `http://keycloak:8080` and the
> issuer on the minted token matches what it expects; and audience enforcement is off.
> The 401 is the password grant refusing a client credential, nothing more.
>
> What would close it: provision the per-role users locally the way the toolkit does
> from `spec.adminUsers`, then have the test log in as one. Both are decisions for whoever
> owns the local bring-up, so this file records the state rather than guessing at a fix.

## The scripted end-to-end walkthroughs

`../../tryouts/` — shell, run by hand against a live stack. These are the scenarios
catalogued as **E2E-A-01** through **E2E-A-04** in [`../TEST-CATALOG.md`](../TEST-CATALOG.md):
participant onboarding and token lifecycle on each spoke, the cross-spoke bilateral
HTLC settlement, and the timeout-refund safety path.

`tryout-fx-agreement-e2e.sh` is the full atomic swap; `QUICKSTART-FX-E2E.md` in that
directory is the shortest route to running it.

## Why the suite was not simply moved here

Moving `../integration/` to this directory would rename a Go module, break the
`make test.e2e` and `integration_lite` targets, break the CI lane that points at
`scenario-a/tests/integration/go.mod`, and invalidate the paths in `TEST-CATALOG.md` —
all to satisfy a directory name. The card allows either moving **or** referencing; this
is the reference.

Renaming `integration` to `e2e` across the Makefile, the CI workflow and the catalogue
is a defensible cleanup, but it is a separate decision with no test-coverage gain, so it
is not bundled here.

## Contrast with Scenario B

Scenario B keeps one Go E2E inside its own `tests/e2e/` (`residue-return`), *and* an
`integration` module, *and* `tests/integration-lite`. The two scenarios organise this
differently on purpose-of-history rather than design; see
[`../../../docs/scenario-drift.md`](../../../docs/scenario-drift.md).

## Keeping this file honest

`../integration/e2e_pointer_test.go` (tag `integration_lite`, so CI runs it) asserts
every path and target named above still exists. If someone moves the suite or renames a
target, that test fails instead of this file quietly becoming fiction.

It earned its place immediately: the first draft of this README told readers to run
`make test.e2e`, a target that does not exist. The guard failed on it before anyone
could follow the instruction.
