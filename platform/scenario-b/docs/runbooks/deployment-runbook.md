# Deployment Runbook — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29
>
> **Deliverable 9** · CBDC System Deployment

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. The commercial bank swap flow (US3) is partially implemented — use a governance account for full end-to-end testing. Sovereign liquidity reconciliation lacks automatic recovery for failed matched commits. Paladin/Zeto privacy is available on spokes but not on the hub. See [Known limitations](#known-limitations) below.

This runbook describes the complete procedure for bringing up the CBWeb3 Scenario B environment (International Hub with AMM/FX), from prerequisites through operational endpoint verification and smoke tests.

---

## Table of Contents

- [Scope](#scope)
- [Prerequisites](#prerequisites)
- [Command overview](#command-overview)
- [Upgrade safety — irreversible steps](#upgrade-safety--irreversible-steps)
- [Step-by-step deployment](#step-by-step-deployment)
  - [Phase 1 — PKI certificate generation](#phase-1--pki-certificate-generation)
  - [Phase 2 — Shared infrastructure](#phase-2--shared-infrastructure)
  - [Phase 3 — Besu networks (Spoke-A and Spoke-B)](#phase-3--besu-networks-spoke-a-and-spoke-b)
  - [Phase 4 — Smart contracts (hub + spokes)](#phase-4--smart-contracts-hub--spokes)
  - [Phase 5 — Sovereign pair seed (optional)](#phase-5--sovereign-pair-seed-optional)
  - [Phase 6 — Backend services](#phase-6--backend-services)
  - [Phase 7 — MLP stack (optional)](#phase-7--mlp-stack-optional)
  - [Phase 8 — Cacti relay](#phase-8--cacti-relay)
- [Health verification](#health-verification)
- [Smoke tests](#smoke-tests)
  - [HTLC cross-spoke smoke test](#htlc-cross-spoke-smoke-test)
  - [AMM smoke test (quote + swap)](#amm-smoke-test-quote--swap)
- [End-to-end tryouts](#end-to-end-tryouts)
- [Teardown](#teardown)
- [Troubleshooting](#troubleshooting)
- [Known limitations](#known-limitations)
- [Changelog](#changelog)

---

## Scope

| Scenario | Status | Description |
|----------|--------|-------------|
| **Scenario B** | Implemented (partially) | International Hub with AMM/FX, cooperative sovereign liquidity, PairRegistry, cross-spoke bridge via Cacti |

Scenario B adds an independent International Hub network on top of the two domestic spokes from Scenario A. The Hub runs on its own Besu/QBFT network (chain **1337**, RPC port **8845**) that is topologically isolated from both spokes. Spoke-A runs on chain 1338 (port 8645) and Spoke-B on chain 1339 (port 8745). All shared contracts (AMM, registries, FX agreement, tokens) are deployed exclusively to the Hub.

---

## Prerequisites

Install all tools before proceeding. See the detailed guide in [`environment-setup.md`](environment-setup.md).

| Tool | Minimum version | Check |
|------|----------------|-------|
| Docker + Docker Compose | Docker 24+ | `docker --version` |
| GNU Make | 3.81+ | `make --version` |
| Go | 1.26+ | `go version` |
| Node.js | 20+ | `node --version` |
| npm | 10+ | `npm --version` |
| Foundry (`forge`, `cast`) | nightly | `forge --version` |
| k6 | 0.50+ | `k6 version` |
| `jq` | 1.6+ | `jq --version` |
| `openssl` | 3.x | `openssl version` |

**Recommended Docker resources:** 16 GB RAM and 8 CPUs (the full stack runs approximately 35 containers, including hub + both spokes + relay + MLP).

All Makefile targets below must be run from the `scenario-b/` directory:

```bash
cd scenario-b
```

---

## Command overview

```bash
# One-shot full stack
make scenario-b.up      # infra + contracts + seed + relayer + all backends
make scenario-b.down    # teardown everything
make scenario-b.restart # down + up

# Step-by-step
make scenario-b.prepare-pki      # Phase 1 — X.509 certificates
make scenario-b.up-infra         # Phase 2 — Keycloak + Postgres + Redis
make scenario-b.deploy-contracts # Phase 4 — hub + spoke contracts + sync addresses
make scenario-b.seed-sovereign-pair  # Phase 5 — LiquidityCommitRegistry + wrapped tokens
make scenario-b.build-backend-images
make scenario-b.up-backend       # Phase 6 — all 6 entity backends
make scenario-b.up-backend-mlp   # Phase 7 — MLP stack (requires ENABLE_MLP=true)
make scenario-b.up-relayer       # Phase 8 — Cacti LiquidityCommitWatcher
```

---

## Upgrade safety — irreversible steps

This release contains **forward-only** steps. Once each has run, downgrading the binaries or the
toolkit leaves the platform in a state the previous version cannot operate. There is no
automated rollback. Read this section before deploying and take the snapshot in the checklist.

### 1. Residue leg — replay-protection index

**What changed.** The unique index on `bridged_asset_positions` moved from `swap_tx_hash` alone
to `(swap_tx_hash, leg)`, with `leg ∈ {SETTLEMENT, RESIDUE}`. A single swap now legitimately
produces two positions — the payment and the return of the unspent slippage buffer. The
replacement index is created by `AutoMigrate` **before** the legacy one is dropped, and the drop
is refused if the replacement is absent, so replay protection is never missing mid-migration.

**Point of no return.** The first `RESIDUE` row.

**What a downgrade breaks.** The previous binary queries `swap_tx_hash` without a leg filter, so
a `RESIDUE` row reads as an already-consumed settlement and legitimate bridge-outs are rejected
as replays. Payments stop settling.

**Recovery.** None in place. Restore each CB's Postgres from a snapshot taken before the first
residue return.

### 2. Sovereign issuance-authority handover

**What changed.** `register-currency` deploys the W-token, registers the currency with the hub
signer as interim central bank (`CurrencyRegistry.registerCurrency` admits only
`getCentralBankOf(token)` as caller), then hands the CB both `CENTRAL_BANK_ROLE` (issuance) and
`DEFAULT_ADMIN_ROLE` (administration of the token's roles) and **revokes both from the hub
governance signer**. Handing over issuance alone would be cosmetic: while the hub kept
administration it could grant issuance back to itself at any time.

**Point of no return.** The administration revoke — per currency. It is applied last, after the
CB holds both roles, so the token is never left without an administrator.

**What a downgrade breaks.** An older toolkit provisions the CB's gateway and relayer with the
spoke deployer key as their hub signer. That address holds no `CENTRAL_BANK_ROLE` on the
W-token, and the hub's signer no longer holds it either, so **nobody** can mint or burn that
currency: bridge-in and bridge-out both stop.

**Verify (per W-token).**

```bash
TOKEN=$(curl -sS -b "access_token=$CB_TOKEN" "$CB_GW/api/v2/hub/currencies" \
  | python3 -c 'import sys,json;print(next(c["token_address"] for c in json.load(sys.stdin)["currencies"] if c["symbol"]=="W-tCeBM_BRL"))')
ROLE=$(cast call "$TOKEN" 'CENTRAL_BANK_ROLE()(bytes32)' --rpc-url "$HUB_RPC")

# Expect true for the CB's own hub address, false for the hub governance signer.
cast call "$TOKEN" 'hasRole(bytes32,address)(bool)' "$ROLE" "$CB_HUB_ADDR"  --rpc-url "$HUB_RPC"
cast call "$TOKEN" 'hasRole(bytes32,address)(bool)' "$ROLE" "$HUB_ADMIN_ADDR" --rpc-url "$HUB_RPC"

# Same expectation for administration — this is what makes the revoke above irreversible by
# the hub rather than a courtesy.
ADMIN=$(cast call "$TOKEN" 'DEFAULT_ADMIN_ROLE()(bytes32)' --rpc-url "$HUB_RPC")
cast call "$TOKEN" 'hasRole(bytes32,address)(bool)' "$ADMIN" "$CB_HUB_ADDR"   --rpc-url "$HUB_RPC"
cast call "$TOKEN" 'hasRole(bytes32,address)(bool)' "$ADMIN" "$HUB_ADMIN_ADDR" --rpc-url "$HUB_RPC"
```

> **The administration expectation above changes after item 8.** The handover described here gives
> the CB's **gateway** `DEFAULT_ADMIN_ROLE`; provisioning then moves it to a dedicated administration
> identity and revokes it from the gateway. So on a stack that has run `separate-token-admin`, the
> `$CB_HUB_ADDR` administration check reads **false** — expected, not a failed handover. Use item 8's
> table for the post-separation expectations. Issuance (`CENTRAL_BANK_ROLE`) for `$CB_HUB_ADDR` stays
> **true** either way.

**Recovery.** Re-run `apply` with the current toolkit: `EnsureCurrencyAuthority` reads the
on-chain state and completes only the outstanding steps, so a run interrupted between
registration and handover converges. Handing authority back to the hub is **not** automated — it
requires an explicit `grantRole` signed with the token's `DEFAULT_ADMIN_ROLE`.

### 3. Per-CB hub identity in the environment

**What changed.** The hub signing key is per central bank and derived deterministically from the
spoke id: `HUB_SIGNER_PRIVATE_KEY`, plus `LOCAL_CB_HUB_SIGNER` (the same identity as an address)
and `HUB_CHAIN_ID`. A commercial bank's gateway receives an **empty** hub signing key by design —
it delegates every hub act to its central bank. The spoke key (`CB_PRIVATE_KEY`) is unchanged: it
holds the roles granted when that spoke's own contracts were deployed.

**Point of no return.** Coupled with item 2 — the roles on-chain now follow the derived address.

**Check after apply.**

| Entity | `SIGNER_PRIVATE_KEY` in the api-gateway | `SIGNER_PRIVATE_KEY` in the relayer |
|---|---|---|
| Central bank | its derived hub identity (same as `LOCAL_CB_HUB_SIGNER`) | its derived **relayer** identity, a different address |
| Commercial bank | **empty** | no relayer |

A central bank runs **two** hub identities: the gateway's (mapped by IdentityRegistry as the
token's central bank, holder of the token's administration, signer of the corridor's governance
acts) and the relayer's (issuance only). They must differ — go-ethereum tracks nonces per
process, so one shared key means two containers each keeping their own counter, and a
transaction replaced by a nonce collision leaves a position waiting forever on a hash that never
mines, since the relayer persists it as an intent on broadcast. The gateway grants the relayer
`CENTRAL_BANK_ROLE` at boot, idempotently; look for `[relayer-role]` in its log. If that grant
fails, the relayer cannot mint or burn — the usual cause is an incomplete currency handover, so
re-run `apply`.

A bank that still carries a hub key is a finding, not a convenience: that key is the CB's, and it
is also the hub governance admin in local stacks.

> **These keys are NOT secret, and this deployment is not production-ready custody.** Read this
> before concluding that sovereign identity settled the key question — it settled *who signs what*,
> not *where the key lives*.
>
> - `HUB_SIGNER_PRIVATE_KEY` and `HUB_RELAYER_PRIVATE_KEY` are `keccak256(salt || spokeID)`. Both
>   the salt (a constant in `toolkit/engine/orchestrator/localdev.go`) and the spoke id (in the
>   manifest) are public, so anyone holding this repository can reproduce **every** central bank's
>   hub private key with one command.
> - `CB_PRIVATE_KEY` is a well-known Besu development account, published in the repository and
>   **identical in every entity**.
>
> That is deliberate for a local or lab stack and is what makes `apply` reproducible. It also means
> the on-chain separation of roles here is *structural* — it bounds which container does what and
> fixes the topology — and not a secrecy boundary. A real deployment replaces the custody without
> changing that topology: the `keyprovider` package already carries the interface and a production
> stub that refuses every operation (`ErrNotImplemented`), and note that `LocalKeyExporter`, which
> hands out private key material as hex, is implemented **only** by the local provider. Wiring a
> KMS therefore also means the gateway stops receiving a key in its environment and starts asking
> the KMS to sign, which is a new signing path in the backend rather than a configuration change.

### 4. Corridor opening is bilateral (procedure change)

**What changed.** No data migration — the procedure. `PairRegistry` admits only
`getCentralBankOf(tokenA)` as proposer and `getCentralBankOf(tokenB)` as confirmer. Since each
currency's authority now rests with its own CB, the hub is neither. The M2M shortcut
`POST /internal/v1/spokes/register-pair` deploys the AMM and then refuses, returning
`FailedPrecondition` naming the two addresses that owe each act.

**New procedure.** The issuing CB of token A calls `POST /api/v2/amm/pairs/propose` (omit
`amm_address` so the pair's dedicated AMM is deployed in the same signed call); the issuing CB of
token B calls `POST /api/v2/amm/pairs/confirm`. Both are governance-portal actions.

**If the old procedure is used.** The call fails and leaves an orphaned AMM — deployed but never
registered. Harmless but wasteful; do not retry the endpoint in a loop, as each attempt deploys
another one.

### 5. Residue returns are retried after upgrade (behaviour change on existing rows)

**What changed.** A residue return whose *enqueue* failed used to stay `RETURN_FAILED` forever:
no bridge position was created, so the relayer's queue had nothing to retry. A worker in the
api-gateway now re-drives those, with exponential backoff, up to five attempts, then records
`RETURN_ESCALATED`. Safe to automate because the issuing CB derives the amount from the
bridge-in position plus the on-chain `LogSwap` and the endpoint is idempotent on
`(swap_tx_hash, RESIDUE)`; a retry that turns out to be a duplicate is answered with the
existing position, which also repairs a *false* `RETURN_FAILED` where only the response was lost.

**What to expect on the first sweep.** Historical `RETURN_FAILED` rows have no schedule, so they
are all due immediately. The sweep runs at start-up and then every `RESIDUE_RETRY_INTERVAL_SEC`
(default 60s), bounded to 50 rows each. Backoff between attempts starts at 60s and doubles to a
30-minute cap, so five attempts span hours, not minutes — a CB restart or deploy does not consume
the budget. Old swaps whose inputs no longer resolve escalate to `RETURN_ESCALATED`: noisy in the
log, harmless in effect, and the point — it turns a silently stranded balance into a named one.
Grep `[residue-retry]` to follow it.

A pair paused by governance defers its retries without consuming an attempt, so pausing a
corridor also pauses the one unattended path that still moves value on it.

**To postpone it,** set `RESIDUE_RETRY_INTERVAL_SEC=0` before the upgrade; the worker then does
not start and the previous behaviour (no retry at all) is preserved. Additive schema only —
`residue_attempts` and `residue_next_attempt_at` via `AutoMigrate` — so this is not a rollback
blocker.

**A wedged Hub RPC parks settlements instead of escalating them — measured, and worth knowing
before you diagnose one.** The relayer's 5-attempt escalation only engages on failures that return
*fast*: a closed port (connection refused) or a revert. An endpoint that accepts the TCP connection
and never answers — a paused container, a hung node, a black-holing load balancer — leaves the
executor blocked inside a single attempt with no timeout. Observed live: `attempt_count` stayed at
`0` for 150s and the burn completed the instant the node answered again. So a stuck settlement whose
queue item sits in `IN_FLIGHT` with `attempt_count = 0` is a symptom of an unresponsive Hub, not of
an exhausted retry budget; `[residue-retry]` and `[RelayerWorker]` will both be silent. Check Hub
RPC liveness (`eth_blockNumber`) before looking at the queue.

### 6. Hub reconciliation is now watched (new, additive)

**What it is.** An issuing CB reconciles its own Hub W-token balance against its own records every
`HUB_RECONCILIATION_INTERVAL_SEC` (default 300). The banks never hold W-token — it is minted to the
CB, spent by the CB in the AMM trade and burned by the CB when the unspent part goes back — so that
balance is the CB's **obligation** toward banks whose reserves were consumed and whose payments have
not closed. The payment side of the balance is zero at rest, which makes the check one subtraction
rather than a judgement:

    on-chain balance − Σ(what each in-flight payment still legitimately has there)

Whatever remains is reported as `unexplained`. Read it as "not attributable to a bank payment", not
"money is missing": the value is on-chain and visible. The figure is **signed** — a negative
`unexplained` is a *shortfall* against the records (value that should be on the address and is not),
which is the more serious of the two findings.

**Already-flagged positions are listed, not subtracted.** `stranded_total` and the `stranded` list
are context for the operator. A residue the relayer gave up on is still sitting on the address *and*
still inside its parent payment's in-flight amount, so deducting it as well would count the same
money twice and drive the result negative exactly in the failure the report exists to name. Each
item carries a `direction`: `OUT` means the value is on the address (a burn that never went
through), `IN` means it never arrived (a mint that never landed).

**Where to look.** `[hub-reconciliation]` in the CB gateway log — one line per cycle, structured
JSON when it does not balance. Also `GET /api/v2/amm/hub-reconciliation` (CB role) and the
"Hub obligation" card on the treasury Dashboard, which shows the figure, the per-bank split and the
flagged positions. `per_bank` lists only banks with a **non-zero** exposure: a bridge-in stays
`ACTIVE` for life and contributes zero once its payment settles, so listing them all would grow
without bound and say nothing. A bank's absence means nothing is owed to it, not that it never
transacted.

**It only observes.** A mismatch is never acted on: halting payments over an accounting figure
would turn a reportable condition into an outage, and the circuit breaker already exists for when
stopping is deliberate. `HUB_RECONCILIATION_INTERVAL_SEC=0` disables the checker.

**What it deliberately does not cover — read this before treating a non-zero figure as a defect.**

- **Only this CB's own W-token.** A foreign W-token received as swap output is burned by the OTHER
  CB on bridge-out — an act this CB does not record — so reconciling it from here would be guesswork.
- **The CB's own liquidity is not netted off.** W-token this CB minted for itself and deployed into
  an AMM pool comes back to the same address on withdrawal, and no path burns it down. Those
  positions carry the CB's own `owner_bank_id` and are excluded from the expectation, so they show
  up inside `unexplained`. A CB reading its own report knows its own liquidity; deducting it would
  need a per-position record of pool deployments that does not exist, and inferring one from
  balances is the heuristic this report replaces. **On a stack with committed liquidity, expect
  `unexplained` to sit at roughly the CB's own undeployed/withdrawn W-token, not at zero.**
- **A residue whose *enqueue* failed** leaves no position on the CB, so it appears inside
  `unexplained` rather than as a named item; the payer's own gateway is what knows which swap it is
  (`RETURN_ESCALATED` in its history).
- **A position whose trade this CB did not execute is listed, not priced.** `consumed` comes from the
  CB's own swap records, which exist for the delegated Step 2 and for a Step 2 the CB ran itself. A
  bank that still swaps with a Hub key of its own leaves no such record here, so the position appears
  under a new `unattributed` array in the report (with `minted`, `returned` and the reason) and is
  **excluded from `expected_in_flight`** — on-chain it holds nothing once its residue is back.
  Previously the blank cost read as "consumed nothing", which claimed the whole mint was still on the
  Hub and drove `unexplained` NEGATIVE: the arithmetic contradicting the condition the report exists
  to raise. A non-empty `unattributed` on a fully delegated deployment means some gateway is still
  trading on the hub itself — worth chasing.

**Expect a non-zero figure on an upgraded stack** that has been running payments: anything stranded
before item 5's retry existed shows up here. That is the feature working, not a regression.

**Schema note.** The reconciliation adds a `direction` column to `bridged_asset_positions` and
backfills it at startup (`IN`/`OUT`, derived from the relayer queue's event type for pre-existing
rows), plus an index on `(mirrored_asset, leg, bridge_state)`. Both are additive and idempotent; the
backfill only touches rows whose direction is empty, so a re-run and a rollback are both no-ops.

### 7. Delegated-swap replay guard is now a CLAIM, taken before the trade (behaviour change)

`cross_currency_hub_swaps` is created by `AutoMigrate` and keys each delegated Hub AMM swap on the
bridge-in position that funded it, so a retried delegation cannot trade twice against the same
bridged balance.

**What changed.** The row used to be written *after* the trade, which deduplicated the WRITE and not
the TRADE: two deliveries of one delegation both found no record, both traded on the AMM — which is
not idempotent on-chain — and the loser's insert then failed and was answered as a harmless
`duplicate`, hiding a second trade that had really happened. The row is now inserted `PENDING`
**before** the AMM is touched and finalized with the realized cost afterwards, so the primary key
refuses the second delivery before it can trade.

`AutoMigrate` adds `status`, `failure_reason` and `updated_at`. Rows written by the previous binary
carry an empty status and are read as `EXECUTED` whenever they hold a transaction hash — without
that, every past swap would look like a delegation in flight and legitimate replays would answer 409.

**New responses on `POST /internal/amm/cross-currency-hub-swap`:**

| Response | Meaning | What to do |
|---|---|---|
| `200 duplicate` | the position already traded; the recorded cost and hash come back | nothing — the caller proceeds on the same facts |
| `409 SWAP_IN_PROGRESS` | another delivery of this delegation is trading right now | retry; it then reads the recorded outcome |
| `409 SWAP_CLAIM_FAILED` | an earlier attempt did not complete, and whether its transaction landed cannot be told from here | reconcile the position before retrying — see below |

A `409` does **not** roll the bridge-in back. Reversing it would reclaim tokens another delivery is
spending, or that a possibly-broadcast transaction already spent, so the payer's gateway leaves the
position untouched (`hub swap not ours` in its log).

**Reconciling a `FAILED` claim.** `failure_reason` on the row says why the attempt stopped. Check
whether its transaction exists on the hub before doing anything else:

```sql
SELECT bridge_in_position_id, status, amount_in, swap_tx_hash, failure_reason
  FROM cross_currency_hub_swaps WHERE status = 'FAILED';
```

If the trade did happen, finalize the row from the on-chain `LogSwap` (`amount_in`, `swap_tx_hash`,
`status='EXECUTED'`) and let the payment continue. If it demonstrably did not, the bridge-in position
is the thing to reverse; deleting the claim is what re-opens the double-spend and must be a
deliberate, recorded decision.

**Reconciling a claim stranded in `PENDING`.** A claim is finalized (or abandoned) by the same request
that took it, so if that request dies between the two — the trade broadcast, then `Finalize` fails or
the 60 s relay timeout cancels the context — the row stays `PENDING` and every retry answers
`409 SWAP_IN_PROGRESS` forever. This is the **safe** failure (it refuses rather than trades twice) and
the gateway logs it CRITICAL, but nothing clears it on its own, and nothing should: expiring a
`PENDING` claim automatically is precisely the double-spend this claim exists to prevent, since "old"
and "still trading" are indistinguishable from the row.

A claim older than the request that could still hold it — minutes, not seconds — is stranded:

```sql
SELECT bridge_in_position_id, status, created_at, updated_at, swap_tx_hash
  FROM cross_currency_hub_swaps
 WHERE status = 'PENDING' AND updated_at < NOW() - INTERVAL '15 minutes';
```

Resolve it exactly as a `FAILED` claim, and in the same order: establish from the hub whether the
trade landed (`LogSwap` for that pair around the claim's timestamp) **before** touching the row.
Landed → finalize it with the realized `amount_in` and `swap_tx_hash`. Demonstrably did not → reverse
the bridge-in position; deleting the claim is a deliberate, recorded decision, not cleanup.

A downgrade to the previous binary keeps working (it ignores the new columns) but silently returns to
guarding the write instead of the trade.

### 8. W-token administration is separated from issuance (point of no return)

**What changes.** The currency handover left this CB's **gateway** identity holding both
`CENTRAL_BANK_ROLE` (mint/burn) and `DEFAULT_ADMIN_ROLE` (decide who may issue). Provisioning now
splits them:

| Role on the CB's W-token | Before | After |
|---|---|---|
| `CENTRAL_BANK_ROLE` (gateway) | held | **still held** — the gateway mints when provisioning liquidity |
| `CENTRAL_BANK_ROLE` (relayer) | granted by the gateway at every boot | granted once at provisioning |
| `DEFAULT_ADMIN_ROLE` (gateway) | held | **revoked** |
| `DEFAULT_ADMIN_ROLE` (administration identity) | — | held; its key is in **no container** |

**Why.** Issuance is operational and lives in a long-running container; administration is the
authority to grant issuance and should be exercised at provisioning. Fused, a compromise of the
gateway container yields *permanent* issuance rights: the attacker grants the role to an address of
their own, and rotating the gateway key afterwards does not take it back.

**Point of no return.** The three acts are applied in order by the toolkit step
`separate-token-admin`, all signed by the gateway key because it is the current administrator. The
last one revokes that key's own administration — after it, **the gateway can no longer grant any
role**. Recovery is possible but only through the administration key, which the toolkit re-derives
deterministically; nothing in a container can do it.

**Behaviour change at boot.** The gateway used to grant the relayer's issuance on every start. It now
only CHECKS it, because it no longer can grant. A missing grant logs:

```
[relayer-role] WARNING: relayer 0x… does NOT hold CENTRAL_BANK_ROLE on 0x… — bridge-in mint and
bridge-out burn will revert. The grant is a provisioning act (toolkit step separate-token-admin …):
re-run `apply` for this spoke.
```

That is an incomplete provisioning run, not a code fault. Re-running `apply` for the spoke fixes it;
the step is idempotent and reads the chain, so an already separated token issues no transaction.

**Check after apply** (`cast call <w-token> "hasRole(bytes32,address)(bool)" <role> <addr>`):

| Expected | Address |
|---|---|
| `DEFAULT_ADMIN_ROLE` = **false** | the gateway (`LOCAL_CB_HUB_SIGNER`) |
| `DEFAULT_ADMIN_ROLE` = **true** | the administration identity (`W_TOKEN_ADMIN_ADDRESS`, written to the spoke env file) |
| `CENTRAL_BANK_ROLE` = **true** | the gateway — it must keep issuing |
| `CENTRAL_BANK_ROLE` = **true** | the relayer (`HUB_RELAYER_ADDRESS`) |

**What this is and is not.** With derived keys the split is **structural**: it bounds which container
can do what and fixes the topology, and the administration key is derivable by anyone holding the
repository (see the custody warning under item 3). It becomes a secrecy boundary only when production
custody lands — at which point this topology does not change, only where the key lives.

### 9. Service-to-service authentication is per entity (behaviour change + opt-in enforcement)

**What changed.** The `/internal/*` routes authenticated callers with `INTERNAL_RELAY_AUTH_SECRET`, a
symmetric secret **identical in every entity**. It therefore proved that *some* entity was calling,
never *which* — so it could not attribute an act, and any entity could forge a call as any other.
Every caller now signs with its own key (ECDSA P-256 over a canonical string binding method, path,
body hash and timestamp), and the receiver verifies against that peer's pinned certificate.

| Caller | Signs as |
|---|---|
| a bank's gateway → its CB (bridge-in, hub swap, residue return, matched commit) | `RELAY_KEY_ID` (defaults to `BANK_CODE`) |
| a bank's payment proxy → its CB (deposits, escrows, redeems) | same |
| a bank's transfer-limit client → its CB (daily limit) | same |
| the Cacti relay → the beneficiary CB (bridge-out) | `cacti-relay` |

`RELAY_KEY_ID` exists because `BANK_CODE` is the entity **role**, so every central bank carries
`central-bank` — two CBs sharing an id means the receiver can pin only one of their keys. `BANK_CODE`
is deliberately untouched: it flows into `owner_bank_id` on bridge positions and into the hub
reconciliation's self-exclusion, so changing it would be a data migration.

**Where the pins come from.** A CB already holds each bank's certificate — it signed the CSR at
onboarding and stored the result, and that certificate certifies the very key the bank signs with. So
the participants table is the authoritative source for onboarded peers, and it carries the `ACTIVE`
status: **deactivating a bank in compliance revokes its ability to authenticate.** File pins
(`PKI_DIR/<key-id>.crt`) remain the source for peers that are never onboarded — the Cacti relay is the
case that matters. An INACTIVE participant also suppresses any file pin for the same id, otherwise a
leftover file would resurrect a revoked bank.

**Enforcement is opt-in, per entity, and NOT flipped by the toolkit.**
`RELAY_REQUIRE_SIGNATURE=true` stops the shared secret being accepted. Enable it only when every
caller signs and every peer is pinned. Three properties make that safe to get wrong:

- a gateway with enforcement set and **no pinned peer refuses to start**, naming both sources,
  instead of answering 401 to every internal request — which is what it would otherwise do, including
  to correctly signed ones, taking bridge-in, the delegated hub swap and the residue return down.
  The guard reads **both** pin sources at boot, files *and* the participants table: judging the files
  alone refused exactly the deployment enforcement is for, since a central bank's peers are the banks
  it onboarded and its `PKI_DIR` holds no peer certificate at all. A database that cannot be read at
  boot is not a refusal — nothing is known about the pins then, and the periodic refresh repairs it —
  but the log says so explicitly;
- an **unknown key-id triggers one rate-limited registry reload** before rejection, so a bank becomes
  verifiable the moment it finishes onboarding rather than at the next periodic sweep;
- a **commercial bank never inherits the flag** — the toolkit forces it empty on `join`, because
  enforcement is a receiver-side setting and a bank hosts no internal routes. Without that, exporting
  the flag for the CBs would take every bank gateway down (a bank's registry is empty by design: the
  pin loader skips `-participant` certificates).

**Order of operations on a clean deploy.** The relay is started *before* the hub (it is a hard
prerequisite of `register-relay-spoke`), so it boots before `found-hub`'s `gen-relay-identity-cacti`
writes its key — and a signer is read once, at construction. **Restart the relay after the hub apply**
or it forwards the bridge-out leg unsigned; `samples/deploy-all.sh` does this. On the CB side,
`start-spoke-backend` now depends on `pin-relay-cert`, so the gateway never boots with an incomplete
registry.

**Check after deploy.**

```bash
# The CB should list every peer it must verify: its onboarded banks, the relay, and itself.
docker logs <cb-gateway> 2>&1 | grep '\[relay-auth\]'
#   [relay-auth] registry refreshed: 3 peer key(s) pinned [bank-itau cacti-relay central-bank-brazil]
#   [app] relay auth: 3 peer key(s) pinned [...]; require_signature=true

# Each sender should report its signing identity.
docker logs <bank-gateway> 2>&1 | grep 'signature enabled'
docker logs cbweb3-cacti-liquidity-relay 2>&1 | grep 'signature enabled'

# With enforcement on, the shared secret alone must be REFUSED (this is the check that proves it):
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$CB_GW/internal/amm/cross-currency-bridge-in" \
  -H 'Content-Type: application/json' -H "X-Relay-Auth: $INTERNAL_RELAY_AUTH_SECRET" -d '{}'
#   401   {"code":"RELAY_SIGNATURE_REQUIRED"}
```

A `WARNING: the only pinned key is this entity's own` at boot means no peer identity was found —
neither an onboarded participant with an issued certificate nor a `<key-id>.crt` in `PKI_DIR`. The
gateway still starts (an entity may legitimately receive no internal calls), but any signed request
from a peer will be rejected.

**Routes where the shared secret is NEVER enough, whatever `RELAY_REQUIRE_SIGNATURE` says.** Some
`/internal/*` endpoints act *on behalf of* a named institution, and the secret is identical in every
entity, so it cannot say who is asking. Those routes now require a verified signature and require the
verified identity to be the institution named in the request:

| Route | Bound field | Refusals |
|---|---|---|
| `/internal/amm/cross-currency-hub-swap` | `payer_bank_id` | `401 RELAY_CALLER_IDENTITY_REQUIRED`, `403 RELAY_CALLER_BANK_MISMATCH` |
| `/internal/amm/cross-currency-bridge-in` | `payer_bank_id` | same |
| `/internal/amm/cross-currency-residue-return` | `payer_bank_id` | same |
| `/internal/v2/transfer-limits/{check-and-deduct,restore}` | `payer_bank_id` | same |
| `/internal/v1/payments/{deposits,escrows,redeems}` (GET) | `requester_id` (derived, not read) | `401 RELAY_CALLER_IDENTITY_REQUIRED`, `403 REQUESTER_NOT_A_PARTICIPANT` |
| `/internal/v1/payments/{deposits,escrows,redeems}` (POST) | `requester_besu_address` (derived, not read) | same |
| `/internal/v1/payments/deposits/exchange` (POST) | `deposit_id` must belong to the caller | `403 REQUESTER_NOT_DEPOSIT_OWNER`, `503 DEPOSIT_OWNERSHIP_UNAVAILABLE` |

**These routes deliberately ignore `RELAY_REQUIRE_SIGNATURE`, and that divergence must not be
"fixed".** Everywhere else the flag decides whether the shared secret is still accepted; here it is
never accepted, flag or no flag. Making these routes honour the flag would mean that with enforcement
off, a caller presenting the secret is authenticated but unnamed — and an unnamed caller is exactly
what let one entity list, create or spend for another. It would reopen the tenant boundary rather
than align a policy. The visible cost is a 401 (`RELAY_CALLER_IDENTITY_REQUIRED`) on a partially
provisioned stack where a bank has no signing key yet; that is the intended answer, and the bank
portal renders it as the trust notice rather than a logout.

Authenticating a peer and then authorizing on a bank id from the request body is the same as not
authorizing: the position id and the bank code both travel in the request, and a UUID is not a
permission. So a bank whose signing key fails to load no longer falls back to the shared secret on
these routes — it gets a loud 401 instead of the ability to act as any other bank. **Operational
consequence:** a bank must have its key and its issued certificate in place before it can transact,
which `join` + onboarding already guarantee. Check `signature enabled` in the bank gateway's log
before its first payment.

The listing routes are scoped from the caller too, not from the query string: the signature covers
method, path, body hash and timestamp, so an onboarded bank could otherwise sign a listing request
and hang another bank's address on it. The central bank now resolves the caller's own address from the
participants table and replaces whatever `requester_id` arrived (the mismatch is logged). This relies
on `participants.wallet_address` being the bank's `ENTITY_BESU_ADDRESS` — the toolkit derives both
from the same per-bank key, so they agree by construction; a hand-built stack that sets them
differently will return empty listings.

**The payment CREATION routes are bound the same way.** `requester_besu_address` decides whose deposit,
escrow or redeem is created, and it travelled in the body — which the signature covers but does not
attribute. The bank proxy injects its own address, which protects honest proxy traffic only, so an
onboarded bank could POST directly to its CB naming another bank and have the record created against
it; approving one then burns the victim's fCeBM or converts its tCeBM. The CB now derives the field
from the verified identity and discards the body value, exactly as it does for `requester_id`. The
fiat exchange (`deposits/exchange`) carries no address to overwrite — it names a deposit and mints to
whoever registered it — so it is bound by ownership instead: a deposit id belonging to another
institution is refused with `403 REQUESTER_NOT_DEPOSIT_OWNER`. Same dependency on
`participants.wallet_address == ENTITY_BESU_ADDRESS` as the listings.

**Each signature authenticates one request.** A verified signature stays valid for the whole ±5 min
skew window, so a captured request could be resent inside it. Most routes absorb that (the receiver
deduplicates), but `/internal/v2/transfer-limits/restore` does not: it *subtracts* from a bank's
accumulated daily volume, so a replayed restore credits the allowance back and lets the bank transact
past its configured limit — a compliance control undone by resending bytes. The gateway now remembers
each accepted signature for the length of that window and refuses a second use with
`401 RELAY_SIGNATURE_REPLAYED`. No caller has to change: ECDSA signing is randomized, so two genuine
calls — even with the same body in the same second — carry different signatures. The cache holds only
signatures that already verified, so it cannot be grown by an unpinned caller.

**Where that memory lives decides whether the control actually holds.** In process memory it protects
one gateway: a restart forgets it (reopening the window for the remaining skew of anything captured
just before), and a gateway scaled to more than one replica never had the protection at all — the
capture simply goes to the replica that has not seen it. So the guard uses the entity's existing
Redis, via `REDIS_ADDR` (the same instance `auth` keeps its login nonces in; `entity-backend.compose.yaml`
passes it, and no new service is involved). Check it at boot:

```bash
docker logs <cb-gateway> 2>&1 | grep 'replay guard'
#   [app] relay auth: replay guard shared via Redis at <prefix>-<entity>-redis:6379 — one signature
#   is admitted once across every replica of this gateway
```

Two deliberate degradations, both loud and neither fatal. `REDIS_ADDR` unset leaves the guard
per-process and the boot line says so — acceptable on the hub, which serves no signed internal
routes, and a real gap on a central bank. An unreachable Redis at request time logs
`shared replay store unavailable` and falls back to this instance's own memory instead of refusing:
every internal route rides this middleware, so failing closed on a cache would stop bridge-in, the
delegated hub swap and the residue return — a far larger outage than the window it would close. The
store call is capped at 250 ms so a hung Redis costs fixed latency, not a parked settlement path.

**What deliberately still uses the shared secret.** `/internal/v1/spokes/{register,register-currency,
register-pair}` — the caller is the toolkit CLI running on the host, which has no pinned identity.
Giving the provisioning tool an identity is a separate decision. `/internal/amm/cross-currency-bridge-out`
and `/internal/amm/execute-matched-commit` also stay unbound to a bank: their caller is the Cacti
relay, acting for the corridor rather than for one institution.

**Rollback.** Unset `RELAY_REQUIRE_SIGNATURE` and the receiver accepts the secret again; senders keep
signing harmlessly. The signing keys and pins are additive, so a downgrade to a binary that does not
verify signatures also works — it simply ignores the headers.

### Pre-deploy checklist

- [ ] Postgres snapshot of every central bank — the only rollback path for item 1.
- [ ] **No in-flight cross-currency swaps.** Drain them first: a position bridged in under the
      previous code has its W-token on the old shared hub address, while after the upgrade
      bridge-out is told to burn from the executing CB's own address. Mixing the two strands the
      payment and needs manual reconciliation.
- [ ] Record each CB's derived hub identity (it appears in the api-gateway boot log as
      `hub signer address = 0x…`), so the on-chain role checks can be verified.
- [ ] Confirm no daily transfer limits are configured with values you have not re-read since this
      release: the limit comparison was fixed (amounts arrive in base units, only the configured
      limit is a human decimal). Previously any configured limit rejected every transfer.
- [ ] Decide per entity whether to set `RELAY_REQUIRE_SIGNATURE` (item 9). If you do, confirm first
      that every bank shows `signature enabled` in its log, that the relay does too, and that the CB
      lists them in `[relay-auth] registry refreshed`. A gateway with the flag and no pinned peer
      refuses to start — deliberately, but it is an outage if you learn it during a deploy window.
- [ ] Read item 8 before upgrading a CB: after provisioning, the gateway can no longer grant roles
      on its own W-token. Nothing to drain, but the boot log changes from granting the relayer's
      issuance to only checking it, and a `[relayer-role] WARNING` afterwards means the spoke's
      `apply` did not complete rather than a code fault.
- [ ] Sovereign liquidity positions in `LOCKING` must be drained. Funding a lock is now decided
      per position and recorded (`spoke_fund_tx_hash`), not by reading the signer's balance. A
      position left mid-flight has no funding record, so it will mint its own amount on the next
      attempt — correct going forward, but any balance a previous run stranded on the signer
      stays stranded instead of being silently consumed by the next lock. Reconcile it
      deliberately.

### Verification after deploy

Five checks, all reachable from the portals or with `cast`. The first two are the ones that
distinguish a per-CB identity from the previous shared key — a flow that merely completes does
not prove anything about whose identity signed.

1. **The corridor is bilateral.** On a freshly PROPOSED pair, the proposing CB's own
   `POST /api/v2/amm/pairs/confirm` must be **refused** (it is not the central bank of token B);
   the issuing CB of token B must then succeed. While every CB shared one key, the first call
   succeeded and a single holder closed both sides.
2. **Each CB resolves its own side.** `POST /api/v2/amm/liquidity/deposit-side` must return
   `side: A` for the token-A CB and `side: B` for the token-B CB. With one shared identity both
   returned the same side and side B could never be escrowed.
3. **Issuance authority is sovereign.** The `hasRole` checks in item 2 above: `true` for each
   CB's own hub address, `false` for the hub governance signer, on every W-token.
4. **No hub key on a bank.** `SIGNER_PRIVATE_KEY` empty in each commercial bank's api-gateway
   (see the table in item 3), and the `LogSwap` sender of a completed cross-currency swap equal
   to the source CB's hub address:

   ```bash
   cast receipt "$SWAP_TX_HASH" --rpc-url "$HUB_RPC" --json | python3 -c '
   import sys, json
   # LogSwap(address indexed user, address indexed tokenIn, ...) — match the event signature,
   # since the receipt also carries ERC-20 Transfer logs whose first topic is a sender too.
   SIG = "0x499f47d29fe8ad39124b5e7e7864cb954b8c73bb602f3853cac827a3128076d3"
   for lg in json.load(sys.stdin).get("logs", []):
       t = [x.lower() for x in lg.get("topics", [])]
       if t and t[0] == SIG:
           print("0x" + t[1][-40:]); break'
   ```

5. **The payer is not over-debited.** After a swap with a slippage buffer, the payer's tCeBM
   balance must end down by the realized `amount_in`, not by `max_amount_in`. The residue leg
   settles asynchronously (about 5–10 s on a local stack), so poll
   `GET /api/v1/token/balance` rather than reading it once — an immediate read shows the
   transient full-cap debit.

6. **A verified peer cannot act for another bank, and cannot act twice.** Both need a *signed*
   request — the shared secret is refused before the check is reached — so sign with one bank's key
   and name another bank:

   ```bash
   # Creation bound to the caller: the record must come back owned by the SIGNER, never by the
   # address in the body. Read it back with the same key.
   #   POST /internal/v1/payments/deposits  {"requester_besu_address":"<victim>","amount":"1"}
   #   → 201, and GET /internal/v1/payments/deposits lists it under the signer's own address
   #
   # Ownership bound on the exchange: name a deposit registered by another bank.
   #   POST /internal/v1/payments/deposits/exchange  {"deposit_id":"<victim's deposit>"}
   #   → 403 {"code":"REQUESTER_NOT_DEPOSIT_OWNER"}
   #
   # Replay: send one signed request twice, byte for byte, with the SAME headers.
   #   → the first 200, the second 401 {"code":"RELAY_SIGNATURE_REPLAYED"}
   ```

   A bank that legitimately repeats an operation re-signs it, so it is never affected; if a genuine
   caller ever sees `RELAY_SIGNATURE_REPLAYED`, it is reusing headers across requests, which is a
   client bug and not a tuning knob.

A scripted version of these checks is kept outside version control (under `tmp/`), so this
runbook does not depend on it.

---

## Step-by-step deployment

### Phase 1 — PKI certificate generation

```bash
make scenario-b.prepare-pki
```

Generates X.509 certificates (EC prime256v1) for all seven entities in `backend/config/pki/`. This phase is **idempotent**: if certificate files already exist, the phase is skipped automatically.

Files generated per entity: `<entity>-ca.key`, `<entity>-ca.crt`, `<entity>.key`, `<entity>.csr`, `<entity>.crt`.

To force regeneration:

```bash
make scenario-b.prepare-pki FORCE=1
```

---

### Phase 2 — Shared infrastructure

```bash
make scenario-b.up-infra
```

Brings up shared services via Docker Compose:

| Service | Port | Notes |
|---------|------|-------|
| **Keycloak** | 8081 | 7 realms created: bank-a, bank-b, bank-c, bank-d, central-bank-a, central-bank-b, mlp |
| **PostgreSQL** | 5432 | 7 databases created: cbweb3\_bank\_a, cbweb3\_bank\_b, cbweb3\_bank\_c, cbweb3\_bank\_d, cbweb3\_central\_bank\_a, cbweb3\_central\_bank\_b, cbweb3\_mlp |
| **Redis** | 6379 | 7 logical DBs (one per entity) |
| **Besu Hub** | 8845 | Chain **1337** — independent International Hub network (see Phase 3) |
| **Besu Spoke-A** | 8645 | Chain 1338 — domestic spoke only |
| **Besu Spoke-B** | 8745 | Chain 1339 |

> This phase waits for the Keycloak initialization script to complete (up to 10 minutes). Keycloak creates all realms, clients, and writes `KC_CLIENT_SECRET` values to `backend/config/.env.infra.*` files automatically.

---

### Phase 3 — Besu networks (Hub, Spoke-A, and Spoke-B)

All three Besu networks start as part of `make scenario-b.up-infra`. The Hub network **must be running before contracts are deployed** (Phase 4 deploys the shared contract suite to it).

**Important: the Hub must start first.** `make scenario-b.up-infra` starts networks in the order Hub → Spoke-A → Spoke-B automatically (see `deploy.up-besu` in `make/10-deploy.mk`).

To verify blocks are being produced on all three networks:

```bash
# International Hub — chain 1337 (independent neutral network)
curl -s -X POST http://localhost:8845 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' | jq .result
# Expect: "0x539"  (1337 in hex)

curl -s -X POST http://localhost:8845 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
# Re-run after a few seconds — block number must advance (2-second block period)

# Spoke-A — chain 1338
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# Spoke-B — chain 1339
curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
```

To start only the Hub (without the full stack) for independent verification:

```bash
make deploy.up-hub-besu
# Brings up ONLY the Hub network — no spokes, no backend services
```

#### Validator Configuration

The sandbox uses a **single-validator** Hub topology (`NODES=1` in `deploy/local/hub-besu/.env.network`, see [research.md Decision 3](../../specs/001-hub-network-isolation/research.md)). A single validator is sufficient for prototype and integration testing: the sole node acts as both bootnode and validator, producing 2-second QBFT blocks.

**For production deployments**, the single-validator topology is explicitly not recommended. Production requires a Byzantine-fault-tolerant validator set with **n ≥ 3f+1** nodes, where `f` is the maximum number of faulty nodes you wish to tolerate. Minimum practical configuration for 1 fault tolerance: **4 validators**. Additional steps for a multi-validator Hub:

1. Increase `NODES=4` (or your target count) in `hub-besu/.env.network`
2. The `startBesu.sh` script calls `besu operator generate-blockchain-config` with the updated count — rerun it to regenerate `genesis/genesis.json` with all validator keys in the QBFT `extraData` field
3. Each additional node needs its own `nodes/<name>/data/key` and `key.pub` provisioned and started with `--bootnodes=<hub-validator-enode>`
4. Ensure all validator nodes are accessible to each other via their P2P port (default 31503; add ports for subsequent nodes)

Until multi-validator is configured, the Hub is a convenience prototype that cannot tolerate node failure.

---

### Phase 4 — Smart contracts (hub + spokes)

```bash
make scenario-b.deploy-contracts
```

This target compiles all Solidity contracts via Foundry and deploys them in the following order:

1. **Hub contracts** (deployed to chain **1337** / `HUB_RPC_URL` = `http://localhost:8845` — the independent Hub network):
   - `IdentityRegistry` — hub participant registry; admin, CB-A, and CB-B receive `GOVERNANCE_ROLE`
   - `CurrencyRegistry` — tracks registered hub currencies
   - `PairRegistry` — bilateral CB pair approval lifecycle
   - `TokenizedCentralBankMoney` (tCeBM-BRL) — hub BRL token
   - `TokenizedCentralBankMoney` (tCeBM-EUR) — hub EUR token
   - `ManualOracle` — FX rate oracle; `GOVERNANCE_ROLE` required for `setRate()`
   - `AutomatedMarketMaker` — constant-product AMM using both hub tokens
   - `LiquidityCommitRegistry` — commit-reveal for sovereign CB liquidity; 72-hour TTL
   - `FXAgreement` (hub) — hub-side FX agreement lifecycle
   - `SpokeBridge` (per spoke) — Lock&Mint / Burn&Unlock bridge

2. **Spoke contracts** (deployed per spoke, same as Scenario A):
   - `IdentityRegistry` (per spoke)
   - `TokenizedCentralBankMoney` (per spoke)
   - `FiatCentralBankMoney` (per spoke)
   - `HashTimeLockedContract` (per spoke)
   - `SpokeBridge` (per spoke)

3. Address sync — `make scenario-b.deploy-contracts` internally runs `contracts.sync-addresses`, which propagates all deployed addresses into the backend `.env.infra.*` files and `interop/hub-and-spoke/cacti/.env`.

Required variables in `scenario-b/contracts/.env` before running this target — see [contract-configuration.md](contract-configuration.md).

#### TVL Reconciliation Baseline

After a fresh deployment (Phase 4 just run, no cross-chain activity yet), the Hub-minted token supply **must be exactly zero**. This is the TVL reconciliation baseline — it confirms no orphaned mints from a prior deployment are present and the Hub is starting clean.

Verify immediately after `contracts.deploy-hub` (or `scenario-b.deploy-contracts`):

```bash
# Read tCeBM_BRL address from synced config
TOKEN_BRL=$(grep '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.bank-a | cut -d= -f2-)
TOKEN_EUR=$(grep '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.bank-a | cut -d= -f2-)

# Verify zero supply on the independent Hub (chain 1337, port 8845)
cast call $TOKEN_BRL "totalSupply()(uint256)" --rpc-url http://localhost:8845
# Expect: 0

cast call $TOKEN_EUR "totalSupply()(uint256)" --rpc-url http://localhost:8845
# Expect: 0
```

**Distinguishing fresh start from restart with existing state:**

| Checkpoint | Expected Hub token supply | Explanation |
|---|---|---|
| `fresh-startup` (just after deploy) | **0** | No cross-chain activity has occurred; Hub has never minted |
| `post-lock-event` (after first spoke lock) | **> 0** | Hub has minted in response to a spoke lock event |
| `restart-with-state` (containers restarted, data preserved) | **unchanged from pre-restart** | Chain state persists in `nodes/hub-validator/data/` across restarts |

If supply is non-zero immediately after a deploy (fresh start), a stale container data directory may contain chain state from a previous run. Run `make deploy.down-hub-besu` (which removes the old node data) and re-run `make deploy.up-hub-besu` to start from a clean genesis, then redeploy contracts.

---

### Phase 5 — Sovereign pair seed (optional)

```bash
make scenario-b.seed-sovereign-pair
```

Deploys the `LiquidityCommitRegistry` with the first sovereign pair (BRL-EUR) and the wrapped token configurations required for the cooperative liquidity flows (US1, US2). This step is optional if you only want to test spoke-to-spoke HTLC without AMM liquidity.

---

### Phase 6 — Backend services

```bash
make scenario-b.build-backend-images
make scenario-b.up-backend
```

Starts four microservices per entity (6 entities = 24 containers total):

| Service | Protocol | Responsibility |
|---------|----------|---------------|
| `api-gateway` | REST (HTTP) | External entry point; routes to internal gRPC services |
| `auth` | gRPC | Identity, login, wallets, PKI |
| `compliance` | gRPC | Onboarding, AML, participant registry |
| `payment-orchestrator` | gRPC | HTLC, FX, AMM swap execution, escrow |

Docker Compose files per entity:

```
backend/docker-compose-backend.bank-a.yaml
backend/docker-compose-backend.bank-b.yaml
backend/docker-compose-backend.central-bank-a.yaml
backend/docker-compose-backend.central-bank-b.yaml
backend/docker-compose-backend.bank-c.yaml  (optional)
backend/docker-compose-backend.bank-d.yaml  (optional)
```

---

### Phase 7 — MLP stack (optional)

```bash
ENABLE_MLP=true make scenario-b.up-backend-mlp
```

Starts the Multilateral Liquidity Provider (MLP) backend stack. This includes the `api-gateway`, `auth`, `compliance`, and `payment-orchestrator` services for the MLP entity on port 68080.

The MLP stack requires `ENABLE_MLP=true` and uses a separate Keycloak realm (`mlp`), PostgreSQL database (`cbweb3_mlp`), and Redis logical DB.

To stop the MLP stack independently:

```bash
make scenario-b.down-backend-mlp
```

---

### Phase 8 — Cacti relay

```bash
make scenario-b.up-relayer
```

Starts the Hyperledger Cacti `LiquidityCommitWatcher` relay (TypeScript, Node.js 20) that:

- Monitors `LogHTLCLocked` and `LogHTLCClaimed` events on both Spoke-A and Spoke-B via `PluginLedgerConnectorBesu`
- Calls `PaymentOrchestratorService.SettleHTLC` gRPC on the target spoke's payment-orchestrator to propagate the secret and complete the cross-spoke bridge cycle
- Exposes a REST API on port 4000 consumed by the Go payment-orchestrator adapter

The relay configuration lives in `interop/hub-and-spoke/cacti/.env`. Addresses are synced automatically by `make scenario-b.deploy-contracts`.

To stop the relay independently:

```bash
make scenario-b.down-relayer
```

---

## Health verification

After `make scenario-b.up`, verify the main endpoints:

```bash
# Keycloak
curl -s http://localhost:8081/realms/master | jq .realm

# API Gateways — all entities
curl -s http://localhost:18080/healthz   # bank-a
curl -s http://localhost:28080/healthz   # bank-b
curl -s http://localhost:38080/healthz   # central-bank-a
curl -s http://localhost:48080/healthz   # bank-c
curl -s http://localhost:58080/healthz   # bank-d
curl -s http://localhost:60080/healthz   # central-bank-b
curl -s http://localhost:68080/healthz   # mlp (if enabled)

# Besu RPC — block production
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# Cacti relay
curl -s http://localhost:4000/api/v1/health
```

**Operational checklist:**

- [ ] Keycloak responds at `http://localhost:8081/realms/master`
- [ ] All 6 entity API gateways return 200 on `/healthz`
- [ ] Besu Hub (chain **1337**, port 8845) reports `eth_chainId` = `0x539` and blocks are advancing
- [ ] Besu Spoke-A (chain 1338, port 8645) is producing blocks
- [ ] Besu Spoke-B (chain 1339, port 8745) is producing blocks
- [ ] Hub has zero peers with either spoke (`net_peerCount` on port 8845 = `0x0`)
- [ ] Contract addresses synced (verify `backend/config/.env.infra.bank-a` has `AMM_CONTRACT_ADDRESS` set)
- [ ] `HUB_CHAIN_ID` in all backend `.env.infra.*` files = `1337` (not `1338`)
- [ ] Cacti relay healthy at port 4000
- [ ] `docker ps` shows all containers in `Up` state

---

## Smoke tests

### HTLC cross-spoke smoke test

Verify cross-spoke atomic settlement (same as Scenario A, no hub involvement):

```bash
cd scenario-b
bash tryouts/tryout-scenario-b-e2e.sh us3
```

This exercises the HTLC lock → Cacti relay → HTLC claim cycle between Spoke-A and Spoke-B.

---

### AMM smoke test (quote + swap)

Verify hub AMM quote and swap execution using the CB-A governance account:

```bash
# 1. Authenticate as CB-A
CB_A_TOKEN=$(curl -s -X POST \
  http://localhost:8081/realms/central-bank-a/protocol/openid-connect/token \
  -d "grant_type=client_credentials&client_id=central-bank-a-client&client_secret=<secret>" \
  | jq -r .access_token)

# 2. Check pool status
curl -s http://localhost:38080/api/v2/amm/pool/BRL-EUR/status \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .

# 3. Get a swap quote (exact-output)
curl -s "http://localhost:38080/api/v2/amm/quote/exact-output?pool_pair=BRL-EUR&amount_out=1000" \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .

# 4. List active pairs
curl -s http://localhost:38080/api/v2/amm/pairs \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .
```

Expected responses: pool status shows `ACTIVE` after both CBs have committed liquidity; quote returns `amount_in` and `price_impact`; pairs list returns `BRL-EUR` with status `ACTIVE`.

---

## End-to-end tryouts

Full scenario tryouts are in `scenario-b/tryouts/`:

```bash
# All user stories
bash tryouts/tryout-scenario-b-e2e.sh all

# Individual user stories
bash tryouts/tryout-scenario-b-e2e.sh us1   # cooperative sovereign liquidity
bash tryouts/tryout-scenario-b-e2e.sh us2   # MLP bilateral liquidity
bash tryouts/tryout-scenario-b-e2e.sh us3   # commercial bank cross-currency swap
bash tryouts/tryout-scenario-b-e2e.sh us5   # PairRegistry bilateral approval

# Additional scenario scripts
bash tryouts/tryout-commercial-swap-e2e.sh
bash tryouts/tryout-cross-currency-full-lifecycle.sh
```

To skip the `make scenario-b.up` step if the stack is already running:

```bash
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh all
```

---

## Teardown

```bash
# Full teardown
make scenario-b.down

# Partial teardown
make scenario-b.down-backend      # stop all 6 entity backends
make scenario-b.down-backend-mlp  # stop MLP stack only
make scenario-b.down-relayer      # stop Cacti relay
make scenario-b.down-infra        # stop Keycloak, Postgres, Redis, Besu nodes
```

> **Warning:** `make scenario-b.down-infra` removes Docker volumes (`-v`). All database data (Keycloak realms, Postgres records) will be lost. Re-run the full `make scenario-b.up` to restore.

---

## Troubleshooting

### Internal calls answer 401 RELAY_SIGNATURE_REQUIRED

`RELAY_REQUIRE_SIGNATURE` is on at the receiver and the caller did not sign. Check the caller's log
for `signature enabled`: a bank needs `PKI_DIR/<key-id>.key` (written by `gen-csr` on join) and the
relay needs `RELAY_SIGNING_KEY_FILE` (written by the hub's `gen-relay-identity-cacti` — remember the
relay must be restarted after that step, since it reads its key once at construction).

### Internal calls answer 401 RELAY_CALLER_IDENTITY_REQUIRED

The route acts on behalf of a named institution and the request carried no verified signature — the
shared secret is not accepted there, whatever `RELAY_REQUIRE_SIGNATURE` says (see item 9's table).
Same fix as above: get the caller signing. In the bank portal this surfaces as the trust notice, not
as a logout.

### Internal calls answer 403 RELAY_CALLER_BANK_MISMATCH

The signature verified, but as a different entity than the `payer_bank_id` in the request. Two real
causes: `RELAY_KEY_ID` on the caller does not match the `bank_code` it transacts under (they must be
the same for a bank — the pin comes from the participants row keyed by `bank_code`), or something is
genuinely driving a payment for another institution. The message names both sides; compare them
against the caller's `BANK_CODE`.

### Internal calls answer 403 REQUESTER_NOT_DEPOSIT_OWNER

`POST /internal/v1/payments/deposits/exchange` named a deposit that was not registered by the calling
institution. The honest path cannot produce it: the bank lists its own deposits (scoped to itself) and
exchanges one of those. Three real causes, in the order worth checking:

1. **The payment-orchestrator restarted.** Deposits, escrows and redeems live in
   `MemoryEscrowRepository` — they do **not** survive a restart of that container. A deposit the bank
   still holds an id for is then gone, and "gone" reads as "not yours": the ownership check answers
   `403`, not `404`. This is the common cause on a local or redeployed stack. Confirm with
   `docker ps` (uptime of `<prefix>-<entity>-payment-orchestrator`) and by listing the bank's
   deposits — if the listing is empty but the bank has an id, the records were lost, not hidden.
2. **`participants.wallet_address` ≠ `ENTITY_BESU_ADDRESS`** — the same split described under
   "payment listings come back empty". The deposit is the caller's, but ownership is being asked
   about a different address. The listing symptom appears alongside it.
3. **A caller genuinely driving another institution's deposit** — which is the case the check exists
   for. The log line names the caller and the deposit.

`503 DEPOSIT_OWNERSHIP_UNAVAILABLE` is a different failure: the CB could not reach the
payment-orchestrator to establish ownership, so it refused rather than assume. Check the orchestrator.

### Internal calls answer 401 RELAY_SIGNATURE_REPLAYED

The receiver has already accepted that exact signature. A correct caller signs every request, so this
means headers are being reused across requests — a client that caches `X-Relay-*` and re-sends them, or
a retry that replays a captured request instead of rebuilding it. It is not a tuning knob and there is
no window to widen: re-sign the retry. If it appears on the Cacti relay, check that `headersFor` is
called per forward (it is signed per call today, and `cross-currency-swap-relay.test.ts` pins that).

### A bank's payment listings come back empty

The central bank scopes `/internal/v1/payments/*` to the caller's own address, resolved from its
participants row — a supplied `requester_id` is discarded. Empty listings with records visible in the
CB's own portal mean `participants.wallet_address` is not the address the bank's proxy stamps on its
writes (`ENTITY_BESU_ADDRESS`). The toolkit derives both from the same per-bank key; a hand-edited
compose file can split them. Compare:

```bash
docker exec <cb-postgres> psql -U postgres -d compliance \
  -c "SELECT bank_code, wallet_address, status FROM participants;"
docker inspect <bank-gateway> --format '{{range .Config.Env}}{{println .}}{{end}}' | grep ENTITY_BESU_ADDRESS
```

### Internal calls answer 401 RELAY_SIGNATURE_INVALID

The caller signed but the receiver could not verify. Either the peer is not pinned — check
`[relay-auth] registry refreshed` on the receiver for its key-id — or the signature does not match the
request. The signature covers method, path and **body bytes**, so any component that re-serializes the
body between signing and sending invalidates it. The canonical string is pinned on both sides by test
(`relayauth` in Go, `relay-auth.test.ts` in the relay, against a shared fixture).

### A newly onboarded bank is briefly rejected

It should not be: an unknown key-id triggers one registry reload before rejection, rate-limited to
once every 5s. If it persists, the bank has no issued certificate stored — check that onboarding
completed (`certificate_data` on its `participants` row), since that is the pin source.

### A central bank's CA material on disk is misleading (open, low severity)

**Reproduced on a clean deploy**, so this is systematic, not the residue of repeated re-applies. A
CB's PKI volume holds three distinct keys where the filenames suggest two pairs:

| File | State |
|---|---|
| `central-bank-ca.crt` + `central-bank-ca.key` | a matched pair, but NOT the issuer in use |
| `central-bank.crt` | the certificate that actually signs participant credentials |
| `central-bank.key` | matches no certificate present |

Compliance is configured with `CA_CERT_FILE=central-bank.crt` and `CA_KEY_FILE=central-bank.key` — a
pair that does not match.

**Credential issuance nevertheless works, and this was verified.** On a freshly deployed stack two
banks were onboarded end to end (credential request → KYC approval → COMPLETE), and the issued
certificate verifies against `central-bank.crt`. The consistent explanation is that compliance
generates and keeps its CA in memory during bootstrap, writes the certificate to `central-bank.crt`,
and never reads `CA_KEY_FILE` back; the `.key` files in the volume are residue from another generator.

**So the risk is not what it looks like.** Onboarding is not blocked. What is broken is the *disk
representation*: any component that treats `central-bank.crt` + `central-bank.key` as a CA pair fails
with `x509: provided PrivateKey doesn't match parent's PublicKey`. That is a trap for future work, not
an outage — the toolkit's `gen-relay-identity` hit exactly it and now falls back to a self-signed
identity, which is equivalent under pinning since the issuer is never consulted.

Note also that `genCBCA`'s idempotency check is `volumeHasFile(central-bank.crt)`, so it will never
repair the pairing on an existing volume.

Verify before building anything that signs with the CB's CA:

```bash
docker cp <cb-gateway>:/workspace/backend/config/pki/central-bank.crt /tmp/ca.crt
docker cp <cb-gateway>:/workspace/backend/config/pki/central-bank.key /tmp/ca.key
# These two hashes are currently DIFFERENT; treat the pair as unusable until that is fixed.
openssl x509 -in /tmp/ca.crt -pubkey -noout | openssl dgst -sha256
openssl ec   -in /tmp/ca.key -pubout      | openssl dgst -sha256
```

The CA design is deferred to a later phase of the project; this entry exists so the disk state is not
mistaken for a usable CA in the meantime.

### Keycloak does not initialize

```bash
docker logs cbweb3-keycloak --tail 50
docker ps | grep postgres
```

If PostgreSQL takes too long to become ready, increase the retry count:

```bash
KEYCLOAK_READY_ATTEMPTS=60 make scenario-b.up-infra
```

### Contract deployment fails

```bash
# Verify Spoke-A/Hub is producing blocks
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Check contracts/.env variables
cat scenario-b/contracts/.env
```

Ensure `HUB_RPC_URL`, `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, `CENTRAL_BANK_ADDRESS`, and `CENTRAL_BANK_B_ADDRESS` are all set in `contracts/.env`.

### Backend service crashlooping

```bash
# View logs for a specific service
docker logs cbweb3-api-gateway-bank-a --tail 50
docker logs cbweb3-payment-orchestrator-central-bank-a --tail 50
```

Check that contract addresses were synced: look for `AMM_CONTRACT_ADDRESS` in `backend/config/.env.infra.central-bank-a`.

### AMM pool stuck in PENDING state

A pool enters `PENDING_COUNTERPART` when only one CB has committed. If the counterpart commit does not arrive within 72 hours, the commit expires and the pool returns to `EMPTY`. To re-seed:

1. Have both CBs submit a fresh `/api/v2/amm/liquidity/commit` with matching `pool_pair`
2. The second commit auto-matches and transitions the pool to `ACTIVE`

If a commit is in the `MATCHED` state but the pool did not activate, sovereign liquidity auto-recovery is not yet implemented. Manual intervention is required — contact the platform operator.

### Cacti relay not forwarding events

```bash
docker logs cbweb3-cacti-relay --tail 50
curl -s http://localhost:4000/api/v1/health
```

Verify `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` is set correctly in `interop/hub-and-spoke/cacti/.env`. Re-run `make scenario-b.deploy-contracts` to re-sync addresses.

### Port already in use

```bash
lsof -i :8081   # Keycloak
lsof -i :5432   # PostgreSQL
lsof -i :8645   # Besu Spoke-A / Hub
lsof -i :8745   # Besu Spoke-B
lsof -i :18080  # API bank-a
lsof -i :38080  # API central-bank-a
lsof -i :4000   # Cacti relay
```

---

---

## Decommissioning the Legacy Hub-on-Spoke-A Deployment

Prior to this change (feature branch `001-hub-network-isolation`), Scenario B ran hub contracts on Spoke-A's Besu node (chain 1338, port 8645). This configuration was a placeholder — the hub and Spoke-A shared a network identity, making cross-chain relay verification degenerate.

**The prior Hub-on-Spoke-A deployment is now retired and non-authoritative.** Do not use chain-1338 addresses as Hub contract references. The new sole source of truth for all Hub contracts is the independent Hub network (chain 1337, port 8845).

### Legacy contract addresses (for reference only — retired)

These addresses were deployed to chain 1338 as the "Hub" before this change. They are listed here only to support decommissioning workflows (e.g., confirming that on-chain state was not migrated). Do not call these addresses for any new operation:

```
# Legacy Hub-on-Spoke-A addresses (chain 1338) — NON-AUTHORITATIVE
# See contracts/broadcast/CBWeb3Hub.s.sol/1338/ for historic broadcast records
# (if any were saved — these are NOT valid Hub addresses post-cutover)
```

The current authoritative Hub addresses are found in `contracts/broadcast/CBWeb3Hub.s.sol/1337/run-latest.json` and propagated to `backend/config/.env.infra.*.` via `contracts.sync-addresses`.

### Cutover procedure (clean rebuild — no state migration)

There is no on-chain state to migrate. The prototype carries no production data, and the clarification decision (`/speckit.clarify`) explicitly chose a clean rebuild. Proceed as follows:

1. **Stop everything**: `make scenario-b.down`
2. **Bring up only the Hub**: `make deploy.up-hub-besu`
3. **Verify chain 1337**: `eth_chainId` must return `0x539`
4. **Deploy contracts to Hub**: `make contracts.deploy-hub`
5. **Bring up spokes and relay**: continue from Phase 3 of this runbook
6. **Re-sync addresses**: `make contracts.sync-addresses` (already called by `scenario-b.deploy-contracts`)
7. **Verify zero stale defaults**: `grep -rn "HUB_CHAIN_ID" backend contracts make | grep 1338` must return 0 results

### Detecting and correcting a stale locally cached 1338 value

If a service logs `HUB_CHAIN_ID not set; defaulting to 1337` at startup, the `HUB_CHAIN_ID` env var is absent from the entity's `.env.infra.*` file. After running `contracts.sync-addresses`, this variable should be populated with `1337`. If it shows `1338`, you have a stale cached file:

```bash
# Check for stale 1338 defaults
grep -rn "HUB_CHAIN_ID=1338" backend/config/

# Fix: re-run address sync (overwrites stale values)
make contracts.sync-addresses

# Verify all entity configs now show 1337
grep -rn "HUB_CHAIN_ID" backend/config/.env.infra.* | grep -v "1338"
```

After fixing, restart the affected backend services.

---

## Known limitations

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| Hub sandbox uses single validator | Hub cannot tolerate node failure in prototype | For production, provision a multi-validator QBFT set (n ≥ 3f+1) — see Phase 3 Validator Configuration above |
| MLP stack requires explicit opt-in | MLP backend does not start automatically | Pass `ENABLE_MLP=true` and run `make scenario-b.up-backend-mlp` |
| US3 (commercial bank swap) partially implemented | Full cross-currency swap lifecycle requires governance account for some steps | Use CB-A governance account for testing; see tryout scripts |
| Sovereign liquidity auto-recovery not implemented | Failed matched commits leave the pool in a stuck state | Manual re-commit by both CBs |
| Paladin/Zeto privacy not available on hub | Hub AMM transactions are not privacy-preserving | Privacy is enforced per-spoke only |

---

## Changelog

| Date | Deliverable | Change |
|------|-------------|--------|
| 2026-08-05 | Sovereign hub delegation (review fixes) | **Authorization now follows the verified caller.** The endpoints that act for a named institution bind the signature's identity to `payer_bank_id`, and the internal payment listings derive `requester_id` from it instead of reading the query string (item 9). The delegated-swap guard became a claim taken *before* the trade, with new `409` answers (item 7). The enforcement boot guard reads both pin sources, so a central bank whose peers are its onboarded banks can enable `RELAY_REQUIRE_SIGNATURE` (item 9). Hub reconciliation lists positions it cannot price under `unattributed` instead of counting them as unspent (item 6). |
| 2026-07-23 | D6 v2 → D12 | **Network parameter rebase.** Besu image `24.x → 25.8.0` (pinned) and chain IDs `80000/80001/80002 → 1337/1338/1339` (hub/spoke-A/spoke-B). Rationale, compatibility verification, LNET coordination, and a flagged Besu version-skew risk in the bring-up scripts are documented in [besu-chainid-migration-notes.md](besu-chainid-migration-notes.md). Operational cutover for the chain-ID change: see [Decommissioning the Legacy Hub-on-Spoke-A Deployment](#decommissioning-the-legacy-hub-on-spoke-a-deployment). |
| 2026-05-29 | D9 | Initial deployment runbook (PKI → infra → Besu → contracts → seed → backend → relay), health checks, smoke tests, teardown. |
