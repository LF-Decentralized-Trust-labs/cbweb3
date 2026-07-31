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

**Expect a non-zero figure on an upgraded stack** that has been running payments: anything stranded
before item 5's retry existed shows up here. That is the feature working, not a regression.

**Schema note.** The reconciliation adds a `direction` column to `bridged_asset_positions` and
backfills it at startup (`IN`/`OUT`, derived from the relayer queue's event type for pre-existing
rows), plus an index on `(mirrored_asset, leg, bridge_state)`. Both are additive and idempotent; the
backfill only touches rows whose direction is empty, so a re-run and a rollback are both no-ops.

### 7. Delegated-swap replay guard (additive)

`cross_currency_hub_swaps` is created by `AutoMigrate` and keys each delegated Hub AMM swap on the
bridge-in position that funded it, so a retried delegation cannot trade twice against the same
bridged balance. Additive: an older binary ignores the table. Not a rollback blocker on its own,
but a downgrade silently loses the guard.

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
