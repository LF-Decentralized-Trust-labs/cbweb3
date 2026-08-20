# Paladin — pinned version, known defect and required upgrade

> **Status: upgrade required, not yet scheduled.** Both scenarios run Paladin
> `v0.15.0-rc.1`, a release candidate published 2026-01-22. It carries a Zeto defect
> that permanently strands central-bank money in ~1 of every 256 private locks. The
> defect is fixed upstream. Nothing in this repository can fix it; only the upgrade can.
>
> Tracked in Notion as *Upgrade Paladin off v0.15.0-rc.1*, referencing
> [PR #147](https://github.com/LNetNetworks/cbweb3-platform/pull/147).

| | |
| --- | --- |
| Pinned now | `docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1` (2026-01-22, prerelease) |
| First upstream release with the fix | `v1.0.0-rc.8` (2026-04-13) |
| Recommended target | `v1.0.0` (GA, 2026-06-25) — same image repository, tag change only |
| Scope | Scenario A **and** Scenario B (both run Paladin; only A has a Paladin adapter in the backend today) |
| Containment already merged | [PR #147](https://github.com/LNetNetworks/cbweb3-platform/pull/147) — refuses the poisoned lock; does **not** cure the defect |

---

## The defect

`transferLocked` — the call that settles a Zeto lock, and the same call the rollback
path uses — fails permanently when the locked-state id's **first byte** is `0x00`:

```
PD011814: Domain reverted transaction on assemble:
PD210134: Failed to query states by IDs. Wanted: 1, Found: 0
```

State ids are effectively random, so this is a **1-in-256 event per locked state**
(a lock consuming several states is proportionally more exposed). A zero high
*nibble* is harmless; only a whole zero *byte* is fatal.

### Root cause (upstream, confirmed)

The id travels in the `uint256[] lockedInputs` parameter. Inside the Zeto domain,
`transferLockedHandler.loadCoins` built the state-store query from
`input.String()` — the minimal-width hex rendering of a `HexUint256`, which drops a
leading zero byte — while the state store keys on the full 32-byte width. A 31-byte
rendering therefore matches nothing, and assembly reverts.

Upstream references:

| | |
| --- | --- |
| Issue | [LFDT-Paladin/paladin#668](https://github.com/LFDT-Paladin/paladin/issues/668) — *bug: Zeto integration test failed to find locked state* (closed 2026-08-13; the maintainer's closing comment names the truncated `00` explicitly) |
| Fix | commit [`5da12da`](https://github.com/LFDT-Paladin/paladin/commit/5da12da7e72fb465ba0080bcbfbea00aa0a84565) in PR [#1083](https://github.com/LFDT-Paladin/paladin/pull/1083), merged 2026-03-18 into `v1-develop`: `input.String()` → `common.HexUint256To32ByteHexString(input)`, with a regression test over a leading-zero-byte id |
| Present in | `v1.0.0-rc.8` and later, including `v1.0.0`. **Absent** from `v0.15.0-rc.1`, `v0.15.0` and `v0.16.0-rc.0` |

### Why it cannot be fixed from this repository

The obvious candidate — carrying `lockedInputs` as `bytes32[]` so the width is
explicit — was implemented and run against the live domain during PR #147. Zeto
validates the ABI signature and rejects it outright:

```
PD210016: Unexpected signature for function 'transferLocked':
expected='function transferLocked(uint256[] memory lockedInputs, string memory delegate, ...
```

That change converts a 1-in-256 failure into a 100% failure. The parameter type is
not the bug; the domain's internal query is, and that code ships inside the Paladin
image.

---

## What is contained today, and what is not

PR #147 added a guard in the Scenario A payment-orchestrator: the Paladin adapter's
`Lock()` refuses a lock whose returned state id begins with a zero byte, before the
public cross-spoke HTLC record exists.

| | Contained by PR #147 |
| --- | --- |
| One poisoned lock freezing the relay's event cursor, and with it every settlement queued behind it | **yes** — the relay never sees an event for a refused lock |
| The operator learning which amount became unrecoverable | **yes** — one `Error` line carrying amount and delegate, a deliberate, commented exception to the no-amounts-in-logs rule |
| A client being told to retry (`FailedPrecondition`, not `Internal`) | **yes** |
| The tokens of the refused lock | **no** — they are already locked on-chain when the id becomes known, and the rollback path calls the same `transferLocked` |
| Locks created *before* the guard was deployed, whose id begins with a zero byte | **no** — those records still exist and still cannot settle |
| Any lock whose id Paladin returns in a shorter-than-32-byte rendering | **no, by design** — the guard judges only full-width ids rather than guessing |

Only the upgrade closes the remaining rows.

---

## The upgrade work

### 1. The pin is in eight places

`v0.15.0-rc.1` is hardcoded, not read from one variable. Every occurrence must move
together — a missed one silently keeps part of the estate on the defective build, which
is the failure this table exists to prevent. Verify the list before starting, with
`grep -rn 'lfdecentralizedtrust/paladin:v' --include='*.go' --include='*.y*ml'`:

| File | Form |
| --- | --- |
| `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-found.yml` | `image:` ×2 |
| `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-join.yml` | `image:` ×1 |
| `scenario-a/toolkit/engine/orchestrator/step_start_besu_found.go` | `defaultPaladinImage` constant |
| `scenario-a/toolkit/engine/orchestrator/deps.go` | image constant |
| `scenario-a/toolkit/engine/apply/profile.go` | `CBWEB3_PALADIN_IMAGE` default |
| `scenario-a/toolkit/engine/apply/apply.go` | `CBWEB3_PALADIN_IMAGE` default |
| `scenario-b/provisioning/templates/entity-besu.compose.yaml` | `${PALADIN_IMAGE:-lfdt-labs/paladin:latest}` — **unpinned and a different repository** |
| `scenario-b/provisioning/templates/vars/entity-besu.env.example` | `PALADIN_IMAGE=lfdt-labs/paladin:latest` — same |

The last two rows are a second problem the upgrade should close: Scenario B's
provisioning templates default to a floating `latest` tag on a different image
repository than the one Scenario A pins. A floating tag means two hosts provisioned a
week apart can run different Paladin builds, which is exactly the class of difference
this document exists to prevent.

### 2. Why it is not a tag bump

`v0.15.0-rc.1` → `v1.0.0` spans the project's move to GA and a change of GitHub
organisation (`LF-Decentralized-Trust-labs` → `LFDT-Paladin`). Release notes for
`v1.0.0` include package renames after the org move, a new npm scope for the SDK and
examples, RPC auth plugins, the distributed-sequencer integration, privacy-group
access changes, and operator/CRD charts at `1.0.0`. Before scheduling, budget for:

- **Config surface** — Paladin node config keys and defaults across two minor versions.
- **Zeto contract deployment manifests** — `scenario-{a,b}/provisioning/paladin/contracts/*.yaml` declare `core.paladin.io/v1alpha1` `SmartContractDeployment` resources; confirm the API version and the Zeto contract set still match (`v1.0.0` moved to installing Zeto contracts from the npm registry).
- **Both scenarios at once** — the constitution requires explicit justification for a PR touching `scenario-a/` and `scenario-b/`; this is one of the legitimate cases, and the justification is this document.
- **Full revalidation** — the private-token paths are the ones most likely to shift: deposit/mint, lock, `transferLocked` settle, and the cross-spoke HTLC flow end to end.

### 3. Acceptance criteria

1. Every row in the pin table above resolves to the same explicit tag; no `latest` anywhere.
2. `docs/TOOLCHAIN.md` states the new pin, and this document is updated to say the defect is closed.
3. A leading-zero-byte locked state settles successfully. Because ids are random, the practical test is a loop: lock repeatedly until a state id beginning `0x00` appears (expected within a few hundred locks), then settle it and require HTTP 200. Without that evidence, the upgrade is unverified.
4. The guard in `scenario-a/backend/services/payment-orchestrator/internal/adapters/paladin/locked_state_id.go` is revisited: either kept as a safety net with its comment updated to say the defect is fixed upstream and the guard is belt-and-braces, or removed together with `ports.ErrUnsettleableLock`, the `FailedPrecondition` mapping in `internal/grpc/server/server.go` and the two test files. Leaving a workaround in place with a comment that describes a defect that no longer exists is how the next reader is misled.
5. Scenario A and Scenario B E2E suites green, plus the performance baselines, since the sequencer changed upstream.

---

## Open follow-ups that the upgrade does *not* close

These are separate from the version pin and stay on the board:

1. **Relay cursor containment.** `scenario-a/.../adapters/cacti/relay.go` breaks its handler loop on the first handler error, so any permanently failing event freezes the cursor for every event behind it. The upgrade removes today's known cause, not the fragility. A fix needs a quarantine-and-alert design, because blindly advancing past a failed settle risks abandoning the destination leg of an HTLC whose secret is already public — partial settlement, which the constitution forbids.
2. **A durable record for an orphaned lock.** The payment-orchestrator has no audit sink (no compliance client, no event table), so the only trace of an unrecoverable amount is a log line. The pre-existing rollback-failure path in `internal/grpc/server/server.go` has the same gap.

---

## Related

- [`docs/TOOLCHAIN.md`](TOOLCHAIN.md) — authoritative version floors; pinned images are listed there
- [`docs/scenario-drift.md`](scenario-drift.md) — where the Scenario A / Scenario B pin divergence belongs once classified
- [`.specify/memory/constitution.md`](../.specify/memory/constitution.md) — privacy, atomicity and scenario-isolation rules cited above
- [PR #147](https://github.com/LNetNetworks/cbweb3-platform/pull/147) — the containment guard and the live reproduction it is based on
