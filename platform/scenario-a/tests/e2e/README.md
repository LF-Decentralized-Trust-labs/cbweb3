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

It does **not** provision. Bring a stack up with the toolkit first, then run the suite
against it — endpoints, operator logins and Paladin identities are derived from the same
manifests the toolkit was applied with, so nothing has to be passed by hand:

```bash
cd ../../samples && ./deploy-all.sh          # the single provisioning path
make -C ../.. scenario-a.test-integration    # run against it
make -C ../.. scenario-a.test-integration-env  # show what it would use, without running
```

> **It passes, from scratch, in a single pass.** Measured 2026-08-22 against a stack
> torn down and rebuilt with `samples/deploy-all.sh --clean` (Brazil = spoke-brl,
> Colombia = spoke-cop): all nine phases green, exit 0, 92s. `Phase2_Onboard` took 15.8s
> doing real PKI onboarding of all four banks — no inherited state — and settlement
> completed in the same run.
>
> Five EVM transactions land in the evidence bundle, on a chain new enough for the block
> numbers to show it: `fx_propose` (spoke-brl, block 505), `fx_accept` (spoke-cop, block
> 361), both HTLC locks and the origin settle. Both legs reach `SETTLED`, so the atomic
> cross-spoke path is exercised for real. Zeto/Paladin operations appear as privacy-layer
> ids rather than EVM txs, which is correct.
>
> Independently corroborated by `samples/sample-tryout.sh` on the same stack: 26 steps,
> 44 assertions, exit 0, no warnings — cross-spoke PvP settled *and* reserve redeemed,
> with the balance deltas checked (tCeBM −5000, fCeBM +5000).
>
> Two things had to change to get here, and both were configuration rather than logic:
>
> 1. **The 401 is gone.** The suite authenticates with the toolkit's per-role operator
>    accounts (`spec.adminUsers`) instead of realm client credentials. Since `f55ade5b`
>    (*require user (password grant) login for portals*) the auth service accepts only
>    the OIDC password grant, and the login endpoint's `clientId`/`clientSecret` JSON
>    fields are the wire contract's names, not the credential type — a username and
>    password is exactly what belongs in them. The old `deploy/local` bring-up provisioned
>    clients but no users, which is why this could not work there.
> 2. **Party identities are Paladin identities.** The FX propose sent bank codes
>    (`bank-a`, `bank-b`, `bank-d`), which the api-gateway rejected with HTTP 400
>    *"not members of the Paladin roster"*. The roster is an exact-match list of
>    `funded_operator@<spoke>-<bank>` strings, and the orchestrator resolves the bilateral
>    Pente group by `{originator, counterparty}` membership, so a bank code matches
>    neither. Spoke ids and currencies came from the same place: `spoke-a`/`spoke-b` and
>    `USD`/`BRL` name nothing in a toolkit topology.

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
