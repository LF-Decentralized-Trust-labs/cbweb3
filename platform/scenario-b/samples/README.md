# Samples — Provisioning the hub + sovereign spokes (Brazil, Argentina, Colombia)

Ready-to-use `ParticipantDeployment` manifests for the **Scenario B** toolkit
(`cbweb3b`), demonstrating the complete hub-and-spoke topology:

- **One neutral interoperability hub**, founded once (`mode: found-hub`):
  - `hub-cbweb3` — base `tCeBM` reserve tokens + registries + AMM + LCR + NOC (chainId 1337)
- **One external Cacti liquidity relay**, deployed outside the toolkit
  (`start-cacti.sh`) and reached via each manifest's `spec.relay.endpoint`
  (`http://localhost:4000`); spokes register on it dynamically at `found-spoke`.
- **Sovereign spokes**, each founded by its own central bank (`mode: found-spoke`):
  - `spoke-brl` — `central-bank-brazil`    (BRL, chainId 1338)
  - `spoke-ars` — `central-bank-argentina` (ARS, chainId 1339)
  - `spoke-cop` — `central-bank-colombia`  (COP, chainId 1340)
- **Two commercial banks per spoke**, joining as non-validating full nodes (`mode: join`):
  - Brazil: `bank-itau`, `bank-bradesco` → `spoke-brl`
  - Argentina: `bank-galicia`, `bank-macro` → `spoke-ars`
  - Colombia: `bank-bancolombia`, `bank-davivienda` → `spoke-cop`
- **One sovereign FX corridor** `W-BRL-W-ARS` between Brazil and Argentina — opened
  at runtime from the CB governance portal (not by provisioning; see below).
  Colombia joins the hub **without** opening a corridor.

> The bank names are illustrative, used only to demonstrate provisioning.

Everything is provisioned **by configuration** (YAML manifest), without editing
code. Every manifest uses `environment: local`, `keyProvider: kms://local-emulator`,
and `certSource: self-signed` / `ca://…` (production KMS/CA is deferred — see the
toolkit roadmap).

---

## Automation (shortcut)

Two idempotent scripts do everything below for you — build the CLI, install the
contract dependencies, start the relay, and apply every manifest in order:

```bash
./deploy-all.sh      # hub + Brazil + Argentina (the BRL<->ARS corridor)
./deploy-three.sh    # + Colombia (a third spoke, no corridor)
```

Pass `--clean` to wipe Docker (containers + volumes + networks) and the data
directories first. **The rest of this document is the equivalent manual,
step-by-step flow** those scripts run — command by command, calling the toolkit by
hand. After the stacks are up, `./sample-tryout.sh` walks a full onboarding +
cross-currency swap over the REST API.

---

## Structure

```
samples/
  hub/
    hub-cbweb3.yaml                 # found-hub → the neutral hub (contracts + AMM + NOC)
  brazil/
    central-bank-brazil.yaml        # found-spoke → spoke-brl
    bank-itau.yaml                  # join → spoke-brl
    bank-bradesco.yaml              # join → spoke-brl
  argentina/
    central-bank-argentina.yaml     # found-spoke → spoke-ars
    bank-galicia.yaml               # join → spoke-ars
    bank-macro.yaml                 # join → spoke-ars
  colombia/
    central-bank-colombia.yaml      # found-spoke → spoke-cop (no pair)
    bank-bancolombia.yaml           # join → spoke-cop
    bank-davivienda.yaml            # join → spoke-cop
  bundles/                          # emitted hub/spoke bundles (gitignored)
  cbweb3-data/                      # per-entity state + PKI (gitignored)
  deploy-all.sh                     # hub + Brazil + Argentina
  deploy-three.sh                   # + Colombia
  sample-tryout.sh                  # end-to-end onboarding + cross-currency swap
```

## Port matrix (all on the same host)

| Participant             | Spoke     | Mode        | RPC  | WS   | P2P   | chainId |
|-------------------------|-----------|-------------|------|------|-------|---------|
| hub-cbweb3              | (hub)     | found-hub   | 8845 | 8855 | 31503 | 1337    |
| central-bank-brazil     | spoke-brl | found-spoke | 8645 | 8655 | 31303 | 1338    |
| bank-itau               | spoke-brl | join        | 8646 | 8656 | 31304 | 1338    |
| bank-bradesco           | spoke-brl | join        | 8647 | 8657 | 31305 | 1338    |
| central-bank-argentina  | spoke-ars | found-spoke | 8745 | 8755 | 31403 | 1339    |
| bank-galicia            | spoke-ars | join        | 8746 | 8756 | 31404 | 1339    |
| bank-macro              | spoke-ars | join        | 8747 | 8757 | 31405 | 1339    |
| central-bank-colombia   | spoke-cop | found-spoke | 8945 | 8955 | 31603 | 1340    |
| bank-bancolombia        | spoke-cop | join        | 8946 | 8956 | 31604 | 1340    |
| bank-davivienda         | spoke-cop | join        | 8947 | 8957 | 31605 | 1340    |

The service host ports derive from each entity's RPC port by a fixed offset:

| Service           | Offset  | Example (central-bank-brazil, RPC 8645) |
|-------------------|---------|-----------------------------------------|
| api-gateway       | +8000   | 16645                                   |
| Keycloak          | +7000   | 15645                                   |
| governance / bank portal | +9000 | 17645 (bank apps also use +9000)   |
| NOC portal        | +12000  | 20645                                   |
| treasury portal   | +13000  | 21645  (CB only)                        |
| supervisor portal | +14000  | 22645  (CB only)                        |

A Central Bank brings up governance + treasury + supervisor operator portals (plus
the NOC portal); a commercial bank brings up the bank portal; the hub the governance
portal. Each SPA is built per entity with its own api-gateway URL baked in.

---

## Prerequisites (dependencies)

Install these **before** provisioning — the manual steps below assume they are present:

| Dependency | Used for | Check |
|------------|----------|-------|
| **Go 1.26+** | building the `cbweb3b` CLI (the toolkit) | `go version` |
| **Docker + Docker Compose v2** | every entity's Besu node + backend + frontend + NOC | `docker compose version` |
| **Foundry (`forge`/`cast`)** | **compiling and deploying the Solidity contracts** | `forge --version` |
| **`jq`, `curl`** | the verification + tryout scripts | `jq --version` |
| Docker image `hyperledger/besu:25.8.0` | pinned Besu; pulled automatically on the first node `up` | — |

Notes:

- The manifests use a **relative** `spec.node.dataDir` (`cbweb3-data/<entity>`),
  resolved against the current working directory. Run the commands **from
  `samples/`** so state and bundles land under `samples/cbweb3-data/` and
  `samples/bundles/` — no `sudo`, no privileged path.
- `--repo-root` must point at the **repository root**: the toolkit reads
  `scenario-b/contracts/` (to build/deploy contracts) and
  `scenario-b/provisioning/templates/` (the compose templates) from there.

---

## Contract build (highlight)

The Solidity contracts (`FXAgreement`, `AutomatedMarketMaker`, `LiquidityCommitRegistry`,
`HTLC`, `tCeBM`, `ZetoToken`, `NotoToken`, `IdentityRegistry`, …) are built with
**Foundry**, in two parts:

1. **Dependencies — one-time (you run this).** Foundry needs the Soldeer
   dependencies (`forge-std`, OpenZeppelin) present under `contracts/dependencies/`.
   Install them once:

   ```bash
   cd scenario-b/contracts && forge soldeer install   # == `make contracts.setup`
   ```

   Without this, the first `apply` fails at the `build-contracts` step with
   `Source "dependencies/forge-std-…/src/Test.sol" not found`.

2. **Compile + deploy — automatic (the toolkit runs this).** `found-hub` and
   `found-spoke` run a `build-contracts` step (`forge build`) and then deploy the
   contracts via Foundry scripts (`forge script … --broadcast`), reading the
   addresses back from the broadcast JSON. The hub deploys the base tokens +
   registries + AMM + LCR; each spoke deploys its own tCeBM/fCeBM/HTLC/IdentityRegistry
   and registers its sovereign W-token on the hub. You do **not** run `forge` for
   this — it happens inside the `apply`.

---

## Manual, step-by-step deployment

Run everything from `samples/`. Set two shell variables for the session:

```bash
cd scenario-b/samples
ROOT="$(cd ../.. && pwd)"   # repository root (for --repo-root)
BIN="$PWD/.cbweb3b"         # the CLI we build in Step 1
```

### Step 0 — Install the contract dependencies (one-time)

```bash
( cd "$ROOT/scenario-b/contracts" && forge soldeer install )
```

### Step 1 — Build the toolkit CLI

```bash
( cd "$ROOT/scenario-b/toolkit" && go build -o "$BIN" ./cmd/cbweb3b )
"$BIN" --help
```

### Step 2 — Validate the manifests (pre-flight, schema only)

`validate` checks the schema without needing any bundle, so it works before
anything is provisioned. (`apply --dry-run` for a found-spoke/join needs the bundle
it consumes, so it only works after the producing step has run.)

```bash
for f in hub/*.yaml brazil/*.yaml argentina/*.yaml colombia/*.yaml; do
  echo "== $f =="; "$BIN" validate -f "$f" -o yaml
done
```

### Step 3 — Start the external Cacti relay (hard prerequisite)

`found-spoke` registers the spoke on the relay (`register-relay-spoke`), a
**mandatory** step, so the relay must be up first. It is deployed outside the
toolkit; its address reaches the toolkit via each manifest's `spec.relay.endpoint`
(`http://localhost:4000`).

```bash
bash "$ROOT/scenario-b/provisioning/scripts/start-cacti.sh"
# waits for health at http://localhost:4000/api/v1/health
```

### Step 4 — Found the hub (`mode: found-hub`)

Brings up the hub Besu (generates the genesis on first run), builds + deploys the
base contracts (reserve tokens, registries, **AMM**, **LiquidityCommitRegistry**),
and starts the hub infra + api-gateway + governance portal + NOC. Emits the hub
bundle consumed by every spoke.

```bash
"$BIN" apply -f hub/hub-cbweb3.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD"
# → emits bundles/hub.bundle.yaml  (contracts + hub RPC port; no private keys)
```

### Step 5 — Found the Brazil spoke (`mode: found-spoke`)

CB-only: brings up the spoke Besu (CB is the sole QBFT validator), deploys the
spoke contracts, registers the sovereign W-token on the hub, seeds the CB's CA +
Keycloak operators (from `spec.adminUsers`), starts the app stack
(compliance + auth + api-gateway), the **bridge relayer** (payment-orchestrator),
the operator portals, and registers the spoke on the relay. Emits the spoke bundle
the banks consume.

```bash
"$BIN" apply -f brazil/central-bank-brazil.yaml -o yaml \
  --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8645
# → emits bundles/spoke-brl.bundle.yaml
```

> `--spoke-rpc` is the RPC of the node this `apply` operates on (used for gates like
> block-height / sync checks); it matches the entity's RPC port in the matrix above.

### Step 6 — Join the Brazilian banks (`mode: join`)

Each bank consumes `spoke-brl.bundle.yaml`, syncs the spoke genesis as a
non-validating full node, provisions its Keycloak operator, starts its app stack +
bank portal, and generates its key + CSR (`gen-csr`). The CB signs the CSR and
registers the bank on-chain at **runtime** (onboarding portal), not as a toolkit step.

```bash
"$BIN" apply -f brazil/bank-itau.yaml     -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8646
"$BIN" apply -f brazil/bank-bradesco.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8647
```

### Step 7 — Found the Argentina spoke and join its banks

Same sequence with the Argentine manifests (the relay from Step 3 already serves
all spokes; each registers under its own id):

```bash
"$BIN" apply -f argentina/central-bank-argentina.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8745
"$BIN" apply -f argentina/bank-galicia.yaml           -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8746
"$BIN" apply -f argentina/bank-macro.yaml             -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8747
```

### Step 8 — (optional) Found the Colombia spoke

A third independent spoke that joins the hub **without** opening a corridor:

```bash
"$BIN" apply -f colombia/central-bank-colombia.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8945
"$BIN" apply -f colombia/bank-bancolombia.yaml      -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8946
"$BIN" apply -f colombia/bank-davivienda.yaml       -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8947
```

> **Idempotency.** Re-running any `apply` converges — completed steps are skipped,
> the genesis is never regenerated, and an `ACTIVE` pair is left untouched. State is
> per entity under `cbweb3-data/<entity>/.provisioning-state.yaml`.

---

## Operator credentials

Each entity seeds its Keycloak operators from its manifest's `spec.adminUsers`
(scenario-a naming). Log in with `POST /api/v1/auth/login {"clientId":<username>,
"clientSecret":<password>}` at the entity's api-gateway port:

- **Central Bank:** `admin@<country>.<role>.gov` / `<country>-<role>-local`
  (roles GOVERNANCE, TREASURY, SUPERVISOR) — e.g. `admin@brasil.governance.gov` /
  `brasil-governance-local`. The governance operator carries `central_bank` +
  `ROLE_GOVERNANCE` (drives the sovereign AMM and KYC approval).
- **Commercial bank:** `admin@<bank>.<country>.com` / `<bank>-bank-local` (role BANK,
  i.e. `commercial_bank`) — e.g. `admin@itau.brasil.com` / `itau-bank-local`.

`sample-tryout.sh` uses these to onboard both banks through the governance portal and
settle a cross-currency swap.

---

## Sovereign corridor (BRL ↔ ARS) — opened at runtime via the CB portal

Each sovereign currency (W-token) is already deployed and registered on the hub
by `found-spoke` (via the hub compliance service) — so no runtime currency step
is needed. Provisioning does **not** open the FX corridor, however: `spec.pair`
only documents the intended corridor, and the toolkit never holds sovereign
signing keys. Once the stacks are up, each central bank opens the corridor from
its **governance portal** (the *Cooperative Liquidity* wizard) — or the v2 API
directly — authenticated by its governance operator's Keycloak session
(`central_bank` + `ROLE_GOVERNANCE`):

- `POST /api/v2/amm/pairs/propose` — the CB of token A proposes the pair.
- `POST /api/v2/amm/pairs/confirm` — the CB of token B confirms → `PROPOSED → ACTIVE`.
- `POST /api/v2/amm/liquidity/add` — each CB adds its cooperative liquidity.

(Registered currencies are listable at `GET /api/v2/hub/currencies`.)

Strict sovereignty holds: each CB signs only its own act, through its own portal
session — no counterparty key, no raw keys in the toolkit or the manifest.
Colombia simply never opens a corridor.

---

## Verification

Block height on each node (RPC ports from the matrix):

```bash
for p in 8845 8645 8646 8647 8745 8746 8747; do
  echo -n "port $p: "
  curl -s -X POST "http://localhost:$p" -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
done
```

Running containers:

```bash
docker ps --filter "name=cbweb3-"
```

The structured report (`--output yaml|json`) from each `apply` shows every step's
status (`done` / `skipped` / `failed` / `soft-failed` / `planned`).

---

## Notes

- **External relay (Scenario A strategy).** The Cacti relay is deployed outside
  the toolkit (`start-cacti.sh`), before any `apply`; its address is passed in via
  each manifest's `spec.relay.endpoint`. Each founding CB registers its spoke on it
  at runtime via `register-relay-spoke` (`POST /api/v1/spokes`).
- **One Docker network per entity.** Every entity's services (besu + infra +
  keycloak + backend + frontend + NOC) share a single `<prefix>_net` network,
  keeping Docker's address pool from being exhausted at N entities.
- **One hub, N spokes.** Entities reach the hub by RPC (they do not join the hub's
  P2P network); the hub bundle carries the hub contract addresses consumed by each
  `found-spoke`.
- **Non-validating banks.** A joining bank syncs the spoke genesis as a full node;
  the CB is the sole QBFT validator.
- **PKI.** `join` performs only `gen-csr` (keypair + CSR under
  `cbweb3-data/<bank>/pki/`); the CB signs the CSR and registers the bank on-chain
  at runtime (onboarding portal), not a toolkit step.
- **End-to-end test suite.** For the automated pipeline E2E + performance baseline,
  see `scenario-b/toolkit/E2E-STATUS.md` (`go test -tags e2e ./tests/e2e/...`).
