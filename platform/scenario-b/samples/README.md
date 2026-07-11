# Samples — Provisioning the hub + sovereign spokes (Brazil, Argentina, Colombia)

Ready-to-use `ParticipantDeployment` manifests for the **Scenario B** toolkit
(`cbweb3b`), demonstrating the complete hub-and-spoke topology:

- **One neutral interoperability hub**, founded once (`mode: found-hub`):
  - `hub-cbweb3` — base `tCeBM` reserve tokens + registries + NOC (chainId 1337)
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
- **One sovereign FX corridor** `W-BRL-ARS` between Brazil and Argentina — opened
  at runtime from the CB governance portal (not by provisioning; see below).
  Colombia joins the hub **without** opening a corridor.

> The bank names are illustrative, used only to demonstrate provisioning.

Everything is provisioned **by configuration** (YAML manifest), without editing
code and without touching the reference network (`deploy/local` and the Makefile
remain intact). Every manifest uses `environment: local`,
`keyProvider: kms://local-emulator`, and `certSource: self-signed` / `ca://…`
(production KMS/CA is deferred — see the toolkit roadmap).

---

## Automation (shortcut)

Two idempotent scripts build the CLI and apply every manifest in order:

```bash
./deploy-all.sh      # hub + Brazil + Argentina (the BRL<->ARS corridor)
./deploy-three.sh    # + Colombia (a third spoke, no corridor)
```

Pass `--clean` to wipe Docker (containers + volumes + networks) and the data
directories first. The scripts start the **external Cacti relay**
(`provisioning/scripts/start-cacti.sh`) before the first `apply` — same strategy
as Scenario A: the relay is deployed outside the toolkit and its address reaches
the toolkit via each manifest's `spec.relay.endpoint`.

---

## Structure

```
samples/
  hub/
    hub-cbweb3.yaml                 # found-hub → the neutral hub (contracts + NOC)
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

---

## Prerequisites

- Go 1.26+, Docker + Docker Compose v2, Foundry (`forge`/`cast`), `jq`, `curl`.
- Docker image `hyperledger/besu:25.8.0` (pulled on first `up`).
- **Contract dependencies installed** (one-time). The `found-hub`/`found-spoke`
  `build-contracts` step runs `forge build`, which needs the Soldeer dependencies
  (`forge-std`, OpenZeppelin) present under `contracts/dependencies/`. Install them
  once from the contracts project:

  ```bash
  cd scenario-b/contracts && forge soldeer install   # == `make contracts.setup`
  ```

  Without this, the first `apply` fails at `build-contracts` with
  `Source "dependencies/forge-std-…/src/Test.sol" not found`. The deploy scripts
  run `forge soldeer install` automatically (idempotent), so `./deploy-all.sh`
  works from a clean checkout.
- **External Cacti relay.** The relay is NOT started by the toolkit — bring it up
  first with `scenario-b/provisioning/scripts/start-cacti.sh` (the deploy scripts
  do this automatically). Each manifest's `spec.relay.endpoint`
  (`http://localhost:4000`) tells the toolkit where to register the spoke; a
  founding CB registers dynamically via `POST /api/v1/spokes` (`register-relay-spoke`).
- The manifests use a **relative** `spec.node.dataDir` (`cbweb3-data/<entity>`).
  The deploy scripts `cd` into `samples/` first, so state and bundles land under
  `samples/cbweb3-data/` and `samples/bundles/` — no `sudo`, no privileged path.

---

## Manual flow (what the scripts run)

Build the CLI and run each `apply` with `--repo-root <repo>` (the CLI needs the
repo for `contracts/` + `provisioning/templates/`) and `--out-dir samples` (so
bundles land in `samples/bundles/`, matching the `../bundles/…` refs):

```bash
# one-time: install the Foundry/Soldeer contract dependencies (forge-std, OZ)
cd scenario-b/contracts && forge soldeer install

cd ../toolkit && go build -o ../samples/.cbweb3b ./cmd/cbweb3b
cd ../samples
BIN=./.cbweb3b ; ROOT="$(cd ../.. && pwd)"

# 0) validate every manifest first (schema only; no bundle needed).
#    (`apply --dry-run` for a found-spoke/join needs the bundle it consumes, so it
#     only works after the producing step has run — use `validate` for pre-flight.)
for f in hub/*.yaml brazil/*.yaml argentina/*.yaml colombia/*.yaml; do
  "$BIN" validate -f "$f" -o yaml
done

# 0.5) start the EXTERNAL Cacti relay (hard prerequisite of register-relay-spoke).
#      Its address is read from each manifest's spec.relay.endpoint (localhost:4000).
bash "$ROOT/scenario-b/provisioning/scripts/start-cacti.sh"

# 1) found the hub (emits bundles/hub.bundle.yaml)
"$BIN" apply -f hub/hub-cbweb3.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD"

# 2) found spoke-brl (consumes hub.bundle.yaml; emits spoke-brl.bundle.yaml)
"$BIN" apply -f brazil/central-bank-brazil.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" \
  --spoke-rpc http://localhost:8645

# 3) join the Brazilian banks (consume spoke-brl.bundle.yaml)
"$BIN" apply -f brazil/bank-itau.yaml     -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8646
"$BIN" apply -f brazil/bank-bradesco.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8647

# 4) found spoke-ars and join its banks
"$BIN" apply -f argentina/central-bank-argentina.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8745
"$BIN" apply -f argentina/bank-galicia.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8746
"$BIN" apply -f argentina/bank-macro.yaml   -o yaml --repo-root "$ROOT" --out-dir "$PWD" --spoke-rpc http://localhost:8747
```

Re-running any `apply` converges (completed steps are skipped) — the genesis is
never regenerated, and an `ACTIVE` pair is left untouched.

---

## Sovereign corridor (BRL ↔ ARS) — opened at runtime via the CB portal

Provisioning does **not** open the FX corridor. `spec.pair` only documents the
intended corridor; the toolkit never holds sovereign signing keys. Once the
stacks are up, each central bank opens the corridor from its **governance portal**
(the *Cooperative Liquidity* wizard) — or the v2 API directly — authenticated by
its `ROLE_CENTRAL_BANK` Keycloak session:

- `POST /api/v2/hub/currencies` — register the currency (W-token).
- `POST /api/v2/amm/pairs/propose` — the CB of token A proposes the pair.
- `POST /api/v2/amm/pairs/confirm` — the CB of token B confirms → `PROPOSED → ACTIVE`.
- `POST /api/v2/amm/liquidity/add` — each CB adds its cooperative liquidity.

Strict sovereignty holds: each CB signs only its own act, through its own portal
session — no counterparty key, no raw keys in the toolkit or the manifest.
Colombia simply never opens a corridor.

---

## Verification

Block height per node (RPC ports from the matrix):

```bash
for p in 8845 8645 8646 8647 8745 8746 8747 8945 8946 8947; do
  echo -n "port $p: "
  curl -s -X POST "http://localhost:$p" -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
done

docker ps --filter "name=spoke-" --filter "name=hub"
```

The structured report (`--output yaml|json`) from each `apply` shows every step's
status (`done` / `skipped` / `failed` / `soft-failed` / `planned`).

---

## Notes

- **External relay (Scenario A strategy).** The Cacti relay is deployed outside
  the toolkit (`start-cacti.sh`), before any `apply`; its address is passed in via
  each manifest's `spec.relay.endpoint`. Each founding CB registers its spoke on it
  at runtime via `register-relay-spoke` (`POST /api/v1/spokes`) — confirmed by the
  relay health showing the registered `spokes` count.
- **One Docker network per entity.** Every entity's services (besu + infra +
  keycloak + backend + frontend + NOC) share a single `<prefix>_net` network
  (created by the entity's besu compose; the rest join it as external) — mirrors
  Scenario A and keeps Docker's address pool from being exhausted at N entities.
- **One hub, N spokes.** Entities reach the hub by RPC (they do not join the hub's
  P2P network); the hub bundle carries the hub contract addresses consumed by each
  `found-spoke`.
- **Non-validating banks.** A joining bank syncs the spoke genesis as a full node;
  the CB is the sole QBFT validator (`node.validator: true` would only warn).
- **PKI.** `join` performs only `gen-csr` (keypair + CSR under
  `cbweb3-data/<bank>/pki/`); the CB signs the CSR and registers the bank on-chain
  at runtime (not a toolkit step).
- **Reference network untouched.** This sample does not modify or depend on
  `deploy/local` or `make/*.mk`.
- **End-to-end test suite.** For the automated pipeline E2E + performance baseline,
  see `scenario-b/toolkit/E2E-STATUS.md` (`go test -tags e2e ./tests/e2e/...`).
