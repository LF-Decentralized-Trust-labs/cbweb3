# Paladin — pinned version, the Zeto defect, and the upgrade that closed it

> **Status: upgraded to `v1.0.0` and the defect is verified closed (2026-09-02).** The pin
> was `v0.15.0-rc.1`, a release candidate published 2026-01-22 carrying a Zeto defect that
> permanently stranded central-bank money in a fraction of private locks. On `v1.0.0`,
> `transferLocked` spends the affected states: three zero-byte locked states were settled
> three-for-three with receipts read, on a stack rebuilt from zero. See *Criterion 3b* below.
>
> Two caveats that belong with that result: the incidence measured here was **~4% of locks
> (3 in 71), not ~1 in 256** as previously documented; and the `locked_state_id.go` guard must
> stay until no deployed environment runs a pre-`v1.0.0` node.
>
> Tracked in Notion as *Upgrade Paladin off v0.15.0-rc.1*, referencing
> [PR #147](https://github.com/LNetNetworks/cbweb3-platform/pull/147).

| | |
| --- | --- |
| Pinned now | `docker.io/lfdecentralizedtrust/paladin:v1.0.0` (GA, 2026-06-25) — was `v0.15.0-rc.1` |
| First upstream release with the fix | `v1.0.0-rc.8` (2026-04-13) |
| Upgrade applied | `v1.0.0` (GA, 2026-06-25) — same image repository; the tag is the only edit, but it is not *only* a tag bump (see below) |
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

### 1. The pin is in seven places

`v0.15.0-rc.1` is hardcoded, not read from one variable. Every occurrence must move
together — a missed one silently keeps part of the estate on the defective build, which
is the failure this table exists to prevent. Verify the list before starting, with

```
grep -rniE 'paladin:(v[0-9]|latest)' --include='*.go' --include='*.y*ml' --include='*.example'
```

Two details in that pattern are load-bearing. It matches `latest` as well as a `v` tag,
and it does not name an image repository: the last two rows below live under a
*different* organisation (`lfdt-labs`, not `lfdecentralizedtrust`) on a floating tag, so
a pattern written around the Scenario A pin finds everything except the rows most likely
to be forgotten. `--include='*.example'` is what reaches the `.env.example`.

The grep also hits a doc comment in `scenario-a/toolkit/engine/orchestrator/deps.go`,
which cites the tag as an example on the `PaladinImage` field. That is not a pin — the
field carries whatever the caller passes, and the callers are the `apply.go` and
`profile.go` rows below — so it is deliberately absent from this table. Worth updating
when the pin moves, so the example does not go stale, but it changes no behaviour.

| File | Form |
| --- | --- |
| `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-found.yml` | `image:` ×2 |
| `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-join.yml` | `image:` ×1 |
| `scenario-a/toolkit/engine/orchestrator/step_start_besu_found.go` | `defaultPaladinImage` constant |
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

---

## Upgrade executed — verification log (2026-09-02)

The pin moved to `v1.0.0` in the commit that carries this section. What follows is what was
measured, and what the acceptance criteria above still leave open. Read the last part before
treating the defect as closed.

### Method

Absolute clean host, twice over: containers, volumes, **images** and build cache all removed
(`docker system prune -a --volumes`, 17.4 GB reclaimed) and the sample data directories
deleted, so nothing could come from a stale layer or a resumed state file. The toolkit CLI was
rebuilt from the branch and the pin verified **inside the binary**, not only in the source.

Topology: relay + `central-bank-brazil` (found) + `bank-itau` (join) + `central-bank-colombia`
(found) + `bank-bancolombia` (join), with `CBWEB3_HOME` and `CBWEB3_SINGLE_HOST=1` exported as
`samples/deploy-all.sh` does — see the note on that below.

### Criterion 1 — every row on one explicit tag: **met**

The grep this document prescribes returns nine hits across the seven files, all
`docker.io/lfdecentralizedtrust/paladin:v1.0.0`, no `latest` anywhere.

Two things worth recording beyond the table:

- `lfdt-labs/paladin`, which the two Scenario B rows defaulted to, **does not exist on Docker
  Hub** ("object not found"). The floating tag was pointing at nothing.
- Those two rows are also dead configuration today. Only
  `provisioning/templates/entity-besu.compose.yaml` declares a `paladin` service and it has
  **zero references outside tests**: the live paths compose `entity-besu-founder` (found-spoke)
  and `entity-besu-join` (join), neither of which runs Paladin. **Scenario B runs no Paladin at
  all.** Fixed rather than deleted — whether Scenario B should run Paladin is a separate call.
  Note also `step_join_test.go:198` asserts the join composes `entity-besu` with a substring
  match that `entity-besu-join` satisfies, so the test passes while naming the dead template.

### Criterion 2 — docs state the new pin: **met**

`docs/TOOLCHAIN.md` carries `v1.0.0` in the image list, the advisory and the pin matrix. This
section is the other half.

### Criterion 3 — the blocker was an operator error, not a defect

An earlier revision of this section claimed the cross-spoke FX proposal was broken by the
relay spreading a camelCase event into a snake_case gRPC request. **That was wrong on both
counts and is retracted.** `proposeOnCounterpart` maps every field explicitly and correctly
(`counterparty_b: event.counterpartyB`, …), and the internal listing the relay reads returns
all identities populated — checked directly against the running gateway.

The real cause was a missing field in the *test* payload. When the relay forwards a proposal
it sets `on_behalf: true`, and on that path the destination resolves its bilateral group as
`{local CB, custodian}` rather than `{originator, counterparty}` — because the originator is
remote and not a member of any group on that spoke (`server.go:1183-1190`, whose comment says
exactly this). With no `custodian` in the proposal, `EnsureFXContext` refuses:

```
ensure Pente context: EnsureFXContext: originator and counterparty identities are required
```

Setting `custodian` to the destination bank's Paladin identity fixes it: the proposal
propagated to `spoke-cop` in ~20s and the counterparty accepted it on-chain.

The lesson worth keeping is about method, not about the code: three layers were read and
blamed before the payload was checked. The internal listing and the relay mapping were both
correct all along.

### Criterion 3b — the defect itself is demonstrated CLOSED on v1.0.0

The decisive test is not that a lock succeeds; it is that `transferLocked` **spends a locked
state whose first byte is zero**. On `v0.15.0-rc.1` that call failed permanently
(`PD210134: Failed to query states by IDs. Wanted: 1, Found: 0`), retried or not.

Three such states were obtained by looping locks of amount 1 on `bank-itau` until the guard
in `locked_state_id.go` refused one, which logs the offending id. Each was then spent with a
`transferLocked` built to match the adapter's call exactly (`client.go:476-493`) — same ABI,
same `lockedInputs`/`delegate`/`transfers` shape — submitted straight to the bank's Paladin
node, and each receipt was read rather than assumed:

| locked state id | transaction | receipt |
| --- | --- | --- |
| `0x003b014366c41e58…d2ab389` | `41d2b590-57c8-4649-9acf-079d0aa75580` | `success: true`, block 1634 |
| `0x0099834d9bf6b953…15aedc` | `860e70fc-3fbf-446c-8e77-19e2ca399506` | `success: true`, block 1651 |
| `0x000659303c8dc207…b85c39` | `82508903-2435-4217-909f-eccdb8b280b6` | `success: true`, block 1651 |

Three for three. Submission acceptance was explicitly **not** treated as the result: the old
failure happened asynchronously at assemble, so `ptx_sendTransaction` returning an id proves
nothing and every receipt was polled to `success`.

This also refutes, for `v1.0.0`, the claim in `locked_state_id.go` that the tokens of a refused
lock "cannot be recovered, because the rollback path uses the same call". That was true of the
old build; on `v1.0.0` the same call releases them — these three transfers recovered the exact
tokens the guard had written off.

### Criterion 4 — the guard is removed

`unsettleableLockedStateID` guarded a defect that `v1.0.0` does not have. The removal was
gated on the rollout — with a pre-`v1.0.0` node the guard is what prevents an HTLC that can
never settle — and that gate is satisfied: **there is no production deployment**, so every
environment moves with the pin in this repository.

Gone with it: `locked_state_id.go`, its predicate tests, the `ports.ErrUnsettleableLock`
sentinel and the two `FailedPrecondition` branches in the orchestrator's gRPC layer. What
replaces them is a test that pins the opposite behaviour — `Lock` accepts the three zero-byte
ids proven settleable above, plus `0x0a`/`0x0e`, which have a zero high nibble and settled even
on the affected build.

Leaving the guard in would not have been the cautious choice. At the measured ~4% incidence it
refuses roughly one lock in twenty, and its own refusal path strands those funds — on `v1.0.0`
that is money lost to a check defending against nothing.

Two things found while proving it, worth carrying into that change:

- **The incidence is far higher than the guard's own comment assumes.** The comment tells the
  operator to retry because "state ids are effectively random, so a fresh lock has ~255/256
  odds of being usable". Observed: **3 zero-byte ids in 71 locks (~4%)**, where 1/256 predicts
  0.3. Whatever generates these ids is not uniform in the top byte. On the old build that means
  roughly one lock in twenty stranded funds — an order of magnitude worse than documented.
- **A verification bypass was written and then discarded.** An env-gated escape from the guard
  (`ZETO_ALLOW_LEADING_ZERO_LOCK`) was added to reach the settle path, then reverted unused:
  calling Paladin directly proved the same thing without a rebuild or a new flag. Recorded so
  the next person does not add the flag believing it is required.

### What the upgrade *is* verified to do

- **It comes up with our configuration, unchanged.** Four Paladin nodes on `v1.0.0` — both CBs
  and both banks — across two independent spokes. Container image digest checked, not just the
  tag.
- **`create-zeto-token` succeeds**, which this document flagged as the main risk (`v1.0.0`
  changed how Zeto contracts are installed, and the manifests declare
  `core.paladin.io/v1alpha1`).
- **The Zeto domain works.** `ptx_queryTransactions` returns a `private` transaction on the
  `zeto` domain, and a tCeBM mint returns a Paladin receipt with `success: true` at a real block
  height.
- **Pente groups are created** on both spokes by the bank joins.
- Every apply reported `rc=0`; 47 containers.

None of the changes this document warned about — package renames, the npm scope, RPC auth
plugins, privacy-group access — affected this path. That is not the same as saying nothing
changed; it is saying that what changed is not on the path we use.

### Upstream containment, verified against the source repository

The claim "the fix is in v1.0.0" no longer rests on release notes. Checked through the
GitHub API on 2026-09-02:

| tag | `compare` status vs. the fix commit | contains it |
| --- | --- | --- |
| `v1.0.0` (the new pin) | `ahead` | **yes** |
| `v1.0.0-rc.8` | `ahead` | yes — confirms it as the first release carrying it |
| `v0.15.0-rc.1` (the old pin) | `diverged` | **no** |

And the commit is unambiguously ours. `5da12da7e72fb465ba0080bcbfbea00aa0a84565`, dated
2026-03-17, message **"ensure fixed width ID string when loading states"**, changes exactly
two files:

```
domains/zeto/internal/zeto/fungible/handler_transferLocked.go       (+1/-1)
domains/zeto/internal/zeto/fungible/handler_transferLocked_test.go  (+37/-0)
```

The one-line change in `loadCoins` replaces `String()` with `HexUint256To32ByteHexString` —
the exact mechanism this document describes — and upstream added a 37-line regression test
alongside it, so the behaviour is now guarded there too.

This is containment evidence, not a functional proof on our stack. It says the version we
moved to carries the corrected code; it does not replace criterion 3, which is watching a
leading-zero-byte state settle here.

### A static shortcut that does not work — recorded so it is not retried

Checking whether `libzeto.so` contains `HexUint256To32ByteHexString` proves nothing: the helper
is present in **both** `v0.15.0-rc.1` and `v1.0.0` (2 occurrences each). The fix changed the
**call site** in `loadCoins`, not the existence of the helper. `go version -m` on the two
libraries is also inconclusive — both report the Paladin modules as `(devel)`, with no version
string. Only the functional test settles it.

### Prerequisite discovered while running this

`samples/deploy-all.sh` exports `CBWEB3_HOME` and `CBWEB3_SINGLE_HOST=1`. Invoking
`cbweb3 apply` directly without them makes the toolkit treat each manifest's `advertisedHost`
(a Docker network alias) as externally routable, which pins Paladin's peer gRPC port to the
fixed 9000 — so the second founding spoke on one host fails with
`Bind for 0.0.0.0:9000 failed: port is already allocated`. With the variables set, the ports
derive per entity (31650 / 31750). Anyone reproducing this must export them.

### To close the upgrade

Fix the relay's camelCase/snake_case forwarding, then run criterion 3: loop locks of amount 1
until a `zeto_lock_ref` beginning `0x00` appears (expect a few hundred locks at ~20s each,
refreshing the access token every 5 minutes — it expires at 300s), lock the destination leg with
the same `hash_lock`, and require HTTP 200 on settle. Then criterion 4, then the E2E suites and
performance baselines of criterion 5.
