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

> **Known broken as of 2026-08-21.** On a clean local bring-up this test does not get
> past `Phase1_Login`, and the bring-up itself aborts earlier at `noc.setup-agents`.
> Both are the same defect and neither is in the test: the gateway is given
> `KC_BASE_PATH=http://localhost:8081` — a host URL — while running inside a container,
> so it cannot reach Keycloak and every credential exchange returns 401. Verified from
> inside the container: `localhost:8081` refuses the connection, `cbweb3-keycloak:8080`
> answers. Tracked as a P0 in Notion — "Scenario A: local bring-up cannot authenticate".
>
> This is recorded here rather than left for the next person to rediscover: a pointer
> that sends someone to a command which fails is worse than no pointer. The E2E suite
> itself is real and is where this file says it is; what is broken is the environment it
> needs.

The same module carries a hermetic lane under the tag `integration_lite`, which needs no
stack and is the one CI runs on every pull request:

```bash
cd ../integration && go test -tags integration_lite ./...
```

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
