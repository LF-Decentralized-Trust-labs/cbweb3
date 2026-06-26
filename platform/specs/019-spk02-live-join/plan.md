# Implementation Plan: SP02 — Live Spoke Join (QBFT + Paladin)

**Branch**: `019-spk02-live-join` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/019-spk02-live-join/spec.md`

---

## Summary

SP02 is a pure-research spike with no production code. It answers two questions left open by
SP01: (1) what is the exact QBFT validator-voting sequence to promote a synced non-validator to
an active validator on a live network, and (2) whether Paladin nodes discover new peers
reactively via the on-chain transport registry or require a restart of existing nodes when a
new bank joins.

The POC extends SP01's `stack-found.yml` (3-validator QBFT) with a live Paladin layer, then
exercises the full join sequence — QBFT vote + Paladin registration — against the running
stack. All results are captured in an automated verification suite (T1–T6) and distilled into
ADR-002, which becomes the design contract for the toolkit's `mode: join` implementation.

---

## Technical Context

**Language/Version**: Bash 5+, `jq`, `yq` (YAML parse), `openssl` (cert gen), `curl` (RPC)
**Primary Dependencies**: `hyperledger/besu:25.8.0` (pinned), Paladin Core (project-pinned version)
**Storage**: Filesystem only — genesis.json, Besu data volumes, Paladin SQLite + LevelDB data volumes, TLS cert files
**Testing**: Bash verification scripts (same pattern as SP01's T1–T6); `make spk02.verify` as single entry point
**Target Platform**: Linux (primary); Docker Desktop macOS (documented differences)
**Project Type**: Research spike / infrastructure POC
**Performance Goals**: Validator promotion within one QBFT epoch after final vote; Paladin peer discovery within 30 seconds of on-chain registration
**Constraints**: No modification to `scenario-a/deploy/local/` or `scenario-a/make/`; Besu must not restart during join; block gap ≤ 2× configured block time
**Scale/Scope**: Single local host, 4 Besu nodes (3 existing validators + 1 joiner), 3 Paladin nodes (CB + Bank-A existing, Bank-X joiner)

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | Notes |
|-----------|---------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | All artifacts in `scenario-a/provisioning/spikes/spk-02-live-join/`. No Scenario B files touched. No modifications to existing `scenario-a/deploy/local/` or `scenario-a/make/`. |
| **II. Privacy by Design** | ✅ N/A | No on-chain value transfers, no PII, no token contracts in the spike itself. Paladin is exercised for transport-layer connectivity only; no Zeto/Noto token operations are required to validate the join behavior. |
| **III. Atomic Settlement Guarantee** | ✅ N/A | No settlement logic. The spike validates infrastructure connectivity, not settlement atomicity. |
| **IV. Compliance Gate Before Participation** | ✅ N/A | No API gateway, no payment flow, no Keycloak. The IdentityRegistry registration tested here is the Paladin transport registry, not the participant onboarding compliance gate. |
| **V. Test-First at Every Layer** | ✅ PASS | Verification scripts T1–T6 are defined before implementation. Each test has a clear exit-code contract and a described observable output. |
| **VI. Observability and Auditability** | ✅ PASS | All verification scripts emit structured `PASS <id> <desc>` or `FAIL <id> <desc>` lines. ADR-002 requires a complete evidence table. |

**Constitution Check Post-Design**: Re-run after Phase 1 confirms no contracts, services, or
shared infra were added beyond the spike boundary.

---

## Project Structure

### Documentation (this feature)

```text
specs/019-spk02-live-join/
├── plan.md              # This file
├── research.md          # Phase 0 output — unknowns resolved
├── data-model.md        # Phase 1 output — entities + state transitions
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code

```text
scenario-a/provisioning/spikes/spk-02-live-join/
├── Makefile
├── README.md
├── compose/
│   ├── stack-found.yml      # 3-validator Besu + Paladin CB + Paladin Bank-A
│   └── stack-join.yml       # 1 new Besu (non-validator) + 1 Paladin Bank-X
├── scripts/
│   ├── env-defaults.sh          # Port + name constants (overridable by env)
│   ├── start-found.sh           # Start Besu found stack + Paladin layer
│   ├── vote-in-validator.sh     # Call qbft_proposeValidatorVote on all 3 existing validators
│   ├── join-besu.sh             # Start new Besu node (non-validator, isolated net)
│   ├── join-paladin.sh          # Start Paladin Bank-X, gen cert, register on-chain
│   ├── verify-qbft.sh           # T1: validator set; T2: block continuity
│   ├── verify-paladin-join.sh   # T3: mTLS cross-node op; T4: no existing node restart
│   ├── verify-restart-scope.sh  # T5: container start-time comparison before/after
│   └── verify-adr-evidence.sh   # T6: ADR-002 evidence table fields populated
├── data/                    # Runtime-generated, gitignored
│   ├── genesis.json
│   ├── key0 … key3          # Besu node keys (0–2 existing validators, 3 joiner)
│   └── paladin/             # Paladin config + cert output per node
├── bundles/                 # Gitignored runtime artifacts
│   └── spoke-spk02.bundle.yaml
└── docs/
    └── adr-002-live-validator-join.md
```

**Structure Decision**: Self-contained spike under `provisioning/spikes/spk-02-live-join/`;
mirrors SP01's layout exactly. No shared Makefile includes; the spike Makefile is standalone.
SP01's `stack-found.yml` is NOT imported — SP02 ships its own compose files that add the
Paladin layer. This avoids coupling SP02's lifecycle to SP01's.

---

## Phase 0: Research

**Output**: `specs/019-spk02-live-join/research.md`

### Unknowns to resolve

#### U1 — QBFT epoch boundary and vote activation timing

**Question**: In Besu 25.8.0 with QBFT block-header voting, what is the default epoch length
(in blocks)? When exactly does a voted validator become active — at the epoch boundary
immediately after threshold is reached, or at the next epoch boundary after the epoch
containing the threshold block?

**Research tasks**:
- Read Besu 25.8.0 QBFT source / docs for `--qbft-fork-blocks`, `blockperiodseconds`,
  and epoch calculation
- Run `qbft_getValidatorsByBlockNumber` at every block for 2 epochs on a live 3-validator
  network after all votes are cast; record the exact block at which the new validator appears
- Confirm whether `qbft_getValidatorsByBlockNumber("pending")` shows the candidate before
  activation

**Decision format**:
```
Epoch length: N blocks (default or configured)
Activation rule: [next epoch boundary / immediate after threshold]
Verification method: qbft_getValidatorsByBlockNumber("latest") vs ("pending")
```

#### U2 — Paladin TLS trust model (the core unknown)

**Question**: When Paladin node A establishes an mTLS connection to a new Paladin node B
(whose self-signed cert was not present at node A's startup), does it:
(a) Succeed — because Paladin fetches the peer's cert from the on-chain registry before
    handshake (TOFU / dynamic trust)
(b) Fail with `x509: certificate signed by unknown authority` — because the trust store is
    loaded only at startup and requires a config update + restart
(c) Succeed because Paladin uses `InsecureSkipVerify=true` for inter-node transport

**Research tasks**:
- Inspect Paladin Core gRPC transport plugin source (`libgrpc.so` is closed; check
  Paladin docs and any published config schema for `tls.caFile`, `insecureSkipVerify`,
  or `trustAllCertificates` fields)
- Start existing Paladin nodes, then start a new one with a fresh self-signed cert; attempt
  a cross-node JSON-RPC call and observe whether the TLS handshake succeeds or fails
- If it fails: check whether adding the new cert to an `extraCACerts` list in the existing
  config (without restart) resolves it vs. requires restart
- Check Paladin release notes / changelog for any mention of dynamic cert trust behavior

**Decision format**:
```
TLS outcome: [success-no-restart / fail-requires-restart / insecure-skip]
Evidence: [log excerpt or error message observed]
Restart scope (if required): [which containers, estimated duration]
```

#### U3 — Paladin on-chain registry event subscription (reactive vs. startup-only)

**Question**: After an existing Paladin node is running, does it subscribe to events from the
on-chain transport registry contract and pick up new `PaladinRegisterTransport` events in real
time, or does it only query the registry at startup?

**Research tasks**:
- Monitor existing Paladin node logs after a new node registers on-chain; look for a log line
  referencing the new node's `nodeName` or endpoint
- If no log appears: query the Paladin REST API for known peers before and after registration
- Cross-reference with Paladin Core's `libevm.so` registry plugin documentation

**Decision format**:
```
Discovery mode: [reactive-event-driven / startup-only]
Evidence: [log line or API response showing new peer appeared]
Cooldown: [time from on-chain registration to peer appears in existing node's peer list]
```

---

## Phase 1: Design

**Prerequisites**: `research.md` complete with U1–U3 resolved

**Output**: `data-model.md`, `adr-002-live-validator-join.md`

### Step sequence (to validate in POC)

Based on research outcomes, the POC implements and validates this sequence:

```
QBFT Validator Promotion
────────────────────────
1. Start found stack (3 validators + bootnode, SP02 stack-found.yml)
2. Deploy IdentityRegistry contract on spoke (reuse paladin.deploy-registry-spoke-a pattern)
3. Generate Paladin TLS certs for CB + Bank-A + Bank-X nodes
4. Render Paladin configs for CB and Bank-A; start both Paladin nodes
5. Register CB + Bank-A Paladin nodes on-chain (TestRegisterPaladinNodes pattern)
6. Record container start times for all existing containers (baseline for T5)

7. Start Besu joiner (stack-join.yml) — non-validator, syncs to chain
8. Wait for joiner block height to converge with found stack (T_sync)
9. For each existing validator (0, 1, 2):
     curl -X POST http://localhost:<rpcN> \
       -d '{"method":"qbft_proposeValidatorVote","params":["<JOINER_ADDR>",true]}'
10. Poll qbft_getValidatorsByBlockNumber("latest") until joiner appears (T_activate)
11. Verify block heights increased monotonically throughout steps 9–10 (no gap > 2× block time)

Paladin Live Join
─────────────────
12. Start Paladin Bank-X container
13. Generate self-signed TLS cert for Bank-X (openssl req -x509)
14. Register Bank-X on-chain (TestRegisterPaladinNodes with Bank-X nodeName + endpoint)
15. Record attempt: cross-node ptx call from Bank-X to CB node
    → Observe: success (no restart) OR x509 error (restart required)
16. If restart required:
      a. Render updated configs for CB + Bank-A with Bank-X cert in trust list
      b. Rolling restart: stop CB paladin → start → healthy; then Bank-A
      c. Retry cross-node ptx call → verify success
17. Record container start times for CB + Bank-A Paladin after Bank-X join
    → Compare to baseline from step 6 (T5 assertion)
```

### ADR-002 structure

The ADR is the primary deliverable. It must record:

1. **Context** — what SP01 left unresolved (validator voting, Paladin TLS trust)
2. **Decisions**:
   - D1: QBFT vote threshold and activation timing (from U1)
   - D2: Paladin TLS trust verdict (from U2) — restart required / not required
   - D3: Paladin registry discovery mode (from U3) — reactive / startup-only
   - D4: Validated step sequence for `mode: join` (from Phase 1 POC)
   - D5 (if restart): rolling restart procedure, scope, and operator UX contract
3. **Evidence table**: T1–T6 results
4. **Rejected alternatives**: any alternatives tested and why rejected
5. **Downstream impact**: TK-x toolkit items that depend on these decisions
6. **Constitution Check**: post-design re-check

### data-model.md scope

For a spike with no persistent data structures, the data-model records:
- State transitions: `non-validator → voted (N votes) → active validator`
- State transitions: `unregistered Paladin → registered on-chain → discoverable → mTLS established`
- Artifacts produced: genesis, join bundle, ADR-002 (their schema and fields)

### Agent context update

Run after ADR-002 is drafted:

```bash
bash .specify/scripts/bash/update-agent-context.sh claude
```

Add to active technologies section of CLAUDE.md:
- `Bash 5+ scripts, Docker Compose v2, QBFT validator voting API (Besu 25.8.0), Paladin Core gRPC transport (018-spk02-live-join)`

---

## Complexity Tracking

No constitution violations. All artifacts are within `scenario-a/provisioning/spikes/spk-02-live-join/`.

The only architectural complexity is the conditional branch in the step sequence (step 15–16):
restart required vs. not required. Both branches are explicitly documented; the toolkit will
implement whichever branch the POC validates.

---

## Verification Suite (T1–T6)

| Test | Script | Observable | Pass condition |
|------|--------|-----------|----------------|
| T1 | `verify-qbft.sh` | `qbft_getValidatorsByBlockNumber("latest")` | 4 addresses returned; joiner address present |
| T2 | `verify-qbft.sh` | Block timestamps throughout voting window | No gap > 2× `blockperiodseconds` |
| T3 | `verify-paladin-join.sh` | Cross-node ptx call (Bank-X → CB) | Returns success (not x509 error) |
| T4 | `verify-restart-scope.sh` | Container start times (CB + Bank-A Paladin) | Start times identical before and after join |
| T5 | `verify-restart-scope.sh` | Container start times (Besu validators 0–2) | Start times identical throughout entire sequence |
| T6 | `verify-adr-evidence.sh` | ADR-002 evidence table | All 6 test IDs present with PASS/FAIL and observed values |

All scripts exit 0 on pass, non-zero on fail, and emit `PASS <id>` or `FAIL <id> <reason>`.
`make spk02.verify` runs T1–T6 in order and prints `ALL TESTS PASSED` only if all exit 0.
