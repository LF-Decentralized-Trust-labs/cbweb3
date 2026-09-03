# N-Spoke Scalability — Design & Implementation Plan

**Status:** Draft for team review
**Owner:** Samuel Venzi (also reviewing PRs + participating in implementation)
**Source:** Report 1 §7.1 (Cross-Cutting, P1) · Report 2 A-ARCH-1 (P1) · internal dailies 2026-06-24
**Priority:** P1 — **the** gating item. This is what unblocks central-bank pilot testing; every other P1 (AMM, relay, JWT) is workaround-able, this is not.
**Scope:** Scenario A (Enhanced Correspondent Banking) only.

---

## 1. Goal

Make the platform deployable for an arbitrary number of participants — stand up a central bank, stand up a commercial bank, connect it to a spoke — **by configuration, not by editing code**, and with **the same toolset working locally and on staging/prod**. Today everything is hardwired to exactly two spokes and deployed statically on one machine.

Two layers, kept distinct:

- **(A) Provisioning scalability** — an operator can bring up a new participant from a declarative config, on any environment. *This is the first and largest track.*
- **(B) Code/data-model scalability** — the system can *represent and route* N spokes (relay + settlement data model). *Required for >2 spokes to actually interoperate.*

---

## 2. Topology & LNET alignment

### Confirmed (2026-06-24)

- **Scenario A = pairwise settlement among N spokes.** Each spoke is **founded by its own central bank**; any two spokes can transact via the relay. There is no hub in A. Reject any "two fixed spokes" assumption.
- **The Cacti relay is operated centrally by LNET** — it is the network's centralized deployment component and **cannot be operated by an individual bank**. A new spoke *registers with* the LNET-operated relay; it never ships its own relay.

### PKI / identity — current state (verified in code)

Each participant carries **two identities**:
- **TLS/PKI** — X.509 cert (ECDSA P-256), used for the API-gateway PKI nonce challenge-response auth and (scaffolded, currently off) mTLS via `CA_CERT_FILE`.
- **Blockchain** — a KMS-managed secp256k1 key whose EVM address is registered on-chain in **IdentityRegistry** (`PARTICIPANT_REGISTRY_ADDRESS`) — the approved-participant whitelist.

The real onboarding flow (`onboarding_proxy.go` smart mode) is **CSR → central-bank-signs**:
1. Commercial bank generates its **own keypair + CSR** (`OU=ROLE_COMMERCIAL_BANK`); it holds its private key.
2. Gateway adds the CSR + a KMS blockchain pubkey, POSTs `credential-request` to the central bank.
3. **The central bank holds the spoke CA** (`central-bank-<x>-ca.{crt,key}`, "required to sign CSRs and governance bootstrap") and **signs the CSR**, issuing the participant cert.
4. Proof-of-possession (signed nonce) → on-chain IdentityRegistry registration with the EVM address.

**Trust root is per-spoke: the founding central bank is the CA for its spoke.** (The `make/05-pki.mk` path that signs `bank-x.crt` with a per-bank self-signed CA is a **local bootstrap shortcut**, not the production flow.)

### PKI — decided (2026-06-24): single-tier, CB-issued

The spoke's **central bank CA signs every commercial-bank CSR**. The bank holds its own keypair; the CB issues the leaf cert. There is no bank-side CA. This model applies to both local and prod. The **central bank CA cert is the spoke trust anchor and ships in the join bundle**. (Local rooting is self-signed; prod roots the CB CA in real PKI — same model, different root source.)

---

## 3. Scenario A — coupling audit (ground truth, verified in code)

The reports are right that it's hardwired to two spokes, but the coupling is **uneven**. Concentrating effort matters. Ranked:

### Tier 1 — Hard architectural coupling (the "2 = bilateral" assumption) → track (B)

1. **The relay is the core problem.** `interop/hub-and-spoke/cacti/src/config.ts:36-53` hardcodes `spokeA`/`spokeB` objects and **cross-wires `counterpartGrpc`** (A's counterpart = B, B's = A). `htlc-relay.ts` (`resolveCounterpart`, ~line 485-525) assumes **each spoke has exactly one counterpart**, matching locks by `hashLock` in a shared ring buffer. With >2 spokes there is no "the counterpart" — the relay must route each trade to its *destination* spoke. Real redesign, not parametrization.
2. **Positional two-leg data model.** `apis/proto/payment_orchestrator/v1/payment_orchestrator.proto:48-49,71-72` carries fixed `spoke_a_receiver`/`spoke_b_receiver`. The trade is still two legs, but they're keyed by *position* ("A/B"), not by **spoke id**.

### Tier 2 — Mechanical duplication (parametrizable, low risk) → informs track (A)

3. **`make/40-paladin.mk`** — every target exists twice (`-spoke-a`/`-spoke-b`) with hardcoded `BESU_RPC_URL_A/B`, `PALADIN_CB_URL` (31648/31748), literal volume names. **But the underlying Go/bash scripts already accept `SPOKE=` / `BESU_RPC_URL=`.** The duplication is only in the Makefile.
4. **`deploy/local/spoke-besu-{a,b}/startBesu.sh`** — two parallel copies with fully hardcoded network name, container prefix, node names, RPC/WS/P2P ports — *and it regenerates genesis on every run* (chain-destroying outside local).
5. **`make/10-deploy.mk:4-5`** — `SPOKE_A_ENTITIES`/`SPOKE_B_ENTITIES` hardcoded; `deploy.up-spoke-a/-b`. (Backend service targets are *already* pattern rules: `deploy.up-backend-%`.)

### Tier 3 — Already well-isolated (good news; minimal work)

6. **Backend per-entity config is clean.** `backend/docker-compose-backend.<entity>.yaml` + `backend/config/.env.infra.<entity>` are per-entity, each holding only its own `CB_PRIVATE_KEY`/`BESU_RPC_URL`. **No cross-spoke references.**
7. **PKI is already generic** (`make/05-pki.mk` loops over `COMMERCIAL_BANK_IDS`, per-bank CA/cert, idempotent).
8. **Keys are per-entity, not pooled.** Scenario A already splits keys per-entity — no shared multi-bank `.env` to untangle.

**Net:** Scenario A's provisioning layer is mostly mechanical duplication sitting on scripts that are *already* parametrized by spoke/entity. The genuinely hard parts are the relay and the positional proto (Tier 1).

---

## 4. Design principle — independent, declarative, environment-agnostic

**The independent deployment is a NEW, standalone toolkit. It does not modify or depend on the existing Makefile/`startBesu.sh`.** The current `deploy/local` + `make/*.mk` stay **untouched as the sample / reference network** (think `fabric-samples` / `test-network`) — we keep them green to diff against. We deliberately do **not** refactor them into pattern rules.

Three rules make "same toolset, local and prod" true:

1. **Declarative over imperative.** A participant's deployment is described by a **YAML manifest** (desired state). A single `apply` command reconciles to it. The YAML is the canonical source of truth — no interactive prompting.
2. **Environment is a parameter, never a fork.** Local vs prod is a different set of values + a couple of pluggable *strategy* selections — not a different code path. Two categories, kept distinct:
   - **Parameterized with a local-friendly default** (a value/strategy you pass): addressing values (local = container DNS name, prod = real host), cert rooting (`self-signed` local → CA prod), key provider (local KMS emulator → real KMS), image source (`build` → registry ref).
   - **Forbidden outright, in every environment** (never a parameter, never a "local default"): inferring addressing from co-location or leaking container IPs into the model; regenerating genesis on an existing spoke; private keys in any file or env (all keys go through the key provider, even locally).
3. **Behavior is driven by `mode`, not by environment.** `mode: found` (a central bank creating a new spoke; its Besu node is the **bootnode**; it **deploys its own spoke's contracts**) vs `mode: join` (a participant attaching to an existing spoke via a join bundle). The found-vs-join difference exists identically in local and prod.

---

## 5. The provisioning toolkit (track A)

### 5.1 Manifest — inputs (spec)

One self-contained YAML per participant (kills the mixed-config sin by construction). Example — a central bank founding a spoke, local profile:

```yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata: { name: central-bank-brazil }
spec:
  scenario: a
  environment: local            # preset bundle of defaults; any field below overrides it
  role: central-bank            # central-bank | commercial-bank
  mode: found                   # found = new spoke (this node is the bootnode) | join
  spoke: { id: spoke-brl, chainId: 1337, currency: BRL }
  node:                         # explicit addressing — NEVER inferred from co-location
    rpc: { port: 8645 }
    ws:  { port: 8655 }
    p2p: { port: 31303 }
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil   # explicit value (local = container DNS); never inferred
  image: build                       # build | <registry-ref>  (value-only switch, no code)
  keyProvider: kms://local-emulator  # local KMS emulator | kms://... (prod) — keys never in files
  certSource: self-signed            # self-signed (local rooting) | ca://... (prod)
  relay: { endpoint: http://cbweb3-cacti:4000 }
```

A commercial bank is the same shape with `role: commercial-bank`, `mode: join`, and `joinBundleRef: ./bundles/spoke-brl.bundle.yaml`. Promoting to prod = the same file with `environment: prod`, `image:` → registry ref, `keyProvider:` → `kms://…`, `certSource:` → `ca://…`, real `advertisedHost`.

- **Versioned schema** (`apiVersion`) so the format can evolve; validate up-front, fail fast with clear errors.
- **Secrets are referenced, never inlined.** The manifest is committable/reviewable; it contains no private keys.

### 5.2 Join bundle — outputs (status)

Some values don't exist until after deployment — the **enode**, **genesis**, **deployed contract addresses**. The toolkit **emits** these as a **join bundle** artifact. A founding CB's `apply` produces `bundles/<spoke-id>.bundle.yaml`; a joining bank's `mode: join` manifest *consumes* it. Manifests chain: found → emit bundle → join.

### 5.3 Compose templates + orchestration engine

- **Two compose templates** — `central-bank` and `commercial-bank` — parametrized over substrate variables (image, addressing, TLS source, key provider, persistence). The template provides container topology only.
- **The engine owns the orchestration sequence**, which is ~80% of the work and is environment-agnostic. It reproduces what `setup-spoke-*` does today: deploy spoke contracts (IdentityRegistry, ZetoFactory, PenteFactory, FXAgreement) → generate/obtain TLS → render Paladin configs → register Paladin nodes → start/verify Paladin → create Zeto token → create Pente context → deploy FXAgreement-in-Pente → real onboarding (IdentityRegistry) → register spoke with the relay. (The existing scripts already take `SPOKE=`, so they are reusable as building blocks — invoked by the engine, not the Makefile.)
- **Idempotent and non-destructive.** `apply` converges; it must **never regenerate genesis** on a running spoke.

### 5.4 Profiles

A profile is just a **named bundle of default parameter values + strategy selections**: `local` (container-network addressing, self-signed cert rooting, keys via a **local KMS emulator**, build-from-source, ephemeral volumes) vs `staging`/`prod` (real DNS, CA-rooted certs, real KMS, registry images, persistent volumes). Only **two** parameters need coded implementations — `keyProvider` (KMS) and `certSource` (CA); `image` (build vs registry ref) is a value-only switch with no code.

**Local-first, prod-shaped:** design the full parameter surface and interfaces now; implement and test the **`local` strategies** first (KMS emulator, self-signed rooting); the prod `keyProvider` (real KMS) and `certSource` (real CA) are the **two implementations slotted behind the same interfaces** later. Promoting to prod = new values + those two implementations — *no change to the engine's flow*.

---

## 6. Code/data-model track (track B — required for >2 spokes to transact)

- **Proto / data model (front-loaded to Phase 1 — data-model first):** replace positional `spoke_a_receiver`/`spoke_b_receiver` with spoke-identified legs — `source_spoke_id` + `dest_spoke_id` + per-leg receivers. Keep two legs (correspondent banking is pairwise), but make them spoke-keyed. Update all consumers (relay leg parsing, orchestrator). Test-first; migrate + backfill existing rows.
- **Relay routing (Phase 2):** `config.ts` → a **spoke registry** (`spokes[]` keyed by spoke id) loaded from config; `resolveCounterpart` routes by the trade's `dest_spoke_id` instead of a single static counterpart.
- **Relay sequencing — two steps:**
  - *Quick path first:* with legs already spoke-keyed (Phase 1), run the **still-bilateral relay routing** against two independently-provisioned spokes (`SPOKE_A_*`/`SPOKE_B_*` config). Validates **provisioning end-to-end only — not N-routing**; routing stays bilateral until the registry replaces it.
  - *Proper path:* generalize to the spoke registry for true N.

---

## 7. Reusing PR #60 (Docker images for pilot)

PR #60 publishes backend images to **GoLedger's ECR** and brings the stack up via Compose pulling from ECR.

- **Cannot adopt as-is:** the repo belongs to LNET; their deployment can't depend on GoLedger's private registry.
- **Pivot:** the `image:` manifest parameter covers this — `build` locally, an **LNET registry ref** in prod. Build-once-then-pull beats build-from-source.
- **Mine it regardless:** it's a concrete **per-entity env-variable mapping** — cross-reference against the manifest schema. Gabi notes it was pilot-oriented; treat it as a map, not a merge.

---

## 8. Milestones & sequencing

> Strategy: Scenario A first. Prove the `local` profile end-to-end (KMS emulator + self-signed rooting) without building the **prod** KMS/CA implementations, but never hardcode a local-only assumption.

**Phase 0 — Spikes to derisk (do first)**
1. **Cross-stack / enode addressing** across independent stacks (separate compose projects, eventually separate hosts): how the advertised enode + relay reach nodes without a shared docker network.
2. **Live-spoke join** = QBFT validator vote + **dynamic Paladin node registration** on a *running* spoke, ideally without restart. (Flagged earlier as "hard to change"; this is the riskiest unknown for `mode: join`.)

**Phase 1 — Data model + CB-spoke manifest (`mode: found`, `local` profile)**
3. **Proto/DB migration (data-model first):** positional `spoke_a_receiver`/`spoke_b_receiver` → `source_spoke_id`/`dest_spoke_id` + per-leg receivers; update consumers (relay leg parsing, orchestrator); migrate + backfill; test-first.
4. Manifest schema + validation; `central-bank` compose template.
5. Orchestration engine reproducing the spoke-setup sequence (§5.3), idempotent, genesis-once.
6. Emits the join bundle (enode, genesis, deployed addresses, relay endpoint, trust anchor).
7. **Proof:** adding a CB spoke (`spoke-brl`) needs only a manifest — no per-spoke code edits, and the sample's `startBesu.sh`/Makefile untouched. (The one-time proto/DB migration in step 3 is foundational, not per-spoke.)

**Phase 2 — Connect CB spokes via the relay**
8. Quick path: run the still-bilateral relay (now reading spoke-keyed legs) against two manifest-provisioned spokes; settlement end-to-end. Validates provisioning, not N-routing.
9. Replace bilateral routing with the track-B spoke registry (§6) for >2.

**Phase 3 — Commercial-bank manifest (`mode: join`)**
10. `commercial-bank` template; consume join bundle; live join (Phase 0 spike); **real onboarding** (not the auto-register shortcut from the D12 finding).

**Phase 4 — staging/prod profile**
11. Slot in the prod `keyProvider` (real KMS) + `certSource` (real CA) implementations; switch `image` to the LNET registry ref (value-only); validate on staging.

---

## 9. Acceptance criteria

- A participant is added **from a YAML manifest alone**, with **no code edits and no hand-edited shared config**.
- Demonstrated in Scenario A: a CB spoke via `mode: found`, two spokes settling via the relay, a commercial bank via `mode: join`.
- The **same manifest** deploys to `local` and (with profile/value changes only) to `prod` — no engine changes.
- No participant private keys live in any file (manifest, config, or keystore); all keys go through a key provider (local KMS emulator locally, real KMS in prod), per actor.
- Bring-up is idempotent; genesis is never regenerated on a running spoke.
- Settlement legs are spoke-keyed, not positional; relay routes by destination spoke.
- The existing sample network (`deploy/local` + Makefile) is unchanged and still green.

---

## 10. Risks / watch-items

- **Enode/addressing across hosts** — Phase 0 spike; the biggest design driver. Container IPs must never leak into the model.
- **Live-spoke join (QBFT + Paladin)** — Phase 0 spike; may force a restart and change the `mode: join` UX.
- **Genesis regeneration** — the sample's `startBesu.sh` regenerates genesis every run; the toolkit must be the opposite (non-destructive). Don't copy that behavior.
- **Breaking proto/DB change (track B)** — coordinate migration; backfill positional → spoke-keyed legs in one pass; failing test first (constitution).
- **Scenario isolation (constitution)** — keep the toolkit's artifacts self-contained within Scenario A; **no shared code** beyond an explicitly versioned shared library.
- **Onboarding-not-tested smell** — provisioning must exercise the real onboarding path (IdentityRegistry), not the auto-register helper found during D12 work.

---

## 11. Process

- Samuel added as reviewer on all P1 PRs; critical feedback comes from inside the team, not from LNET.
- Manifests being committable YAML makes per-participant deployments reviewable in PRs.
- Break this plan into board subtasks/checklist after team review.
- Each implementation PR carries a Constitution Check section (per workflow rules).
