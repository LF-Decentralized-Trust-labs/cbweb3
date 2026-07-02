# Scenario A — Manual Deployment Tutorial

This tutorial reproduces, **step by step and by hand**, exactly what
[`deploy-all.sh`](./deploy-all.sh) (Brazil + Colombia) and
[`deploy-three.sh`](./deploy-three.sh) (Brazil + Colombia + Argentina) do
automatically. Run these commands in order to stand up the complete Scenario A
sample environment through the `cbweb3` CLI: three independent spokes, each
founded by its own central bank, with two commercial banks joining each spoke.

| Spoke       | Founding central bank    | Commercial banks (join)               | Currency | chainId |
|-------------|--------------------------|---------------------------------------|----------|---------|
| `spoke-brl` | `central-bank-brazil`    | `bank-itau`, `bank-bradesco`          | BRL      | 1337    |
| `spoke-cop` | `central-bank-colombia`  | `bank-bancolombia`, `bank-davivienda` | COP      | 1338    |
| `spoke-ars` | `central-bank-argentina` | `bank-galicia`, `bank-macro`          | ARS      | 1339    |

> The bank names are illustrative, used only to demonstrate provisioning.

Each central bank **founds** its spoke (Besu bootnode, spoke contracts, Paladin,
relay registration, and the full operational backend plus frontend portals) and
emits a **join bundle**. Each commercial bank **joins** as a full node using that
bundle and brings up its own operational stack.

Every step is **idempotent**: re-running a command resumes from the first
incomplete step for that entity, so it is safe to re-run after an interruption.

---

## Port matrix (single host)

Each Besu node needs distinct host ports. This is the allocation used by the
sample manifests.

| Participant             | Spoke      | Mode  | Besu RPC | WS   | P2P   | chainId |
|-------------------------|------------|-------|----------|------|-------|---------|
| `central-bank-brazil`   | spoke-brl  | found | 8645     | 8655 | 31303 | 1337    |
| `bank-itau`             | spoke-brl  | join  | 8646     | 8656 | 31304 | 1337    |
| `bank-bradesco`         | spoke-brl  | join  | 8647     | 8657 | 31305 | 1337    |
| `central-bank-colombia` | spoke-cop  | found | 8745     | 8755 | 31403 | 1338    |
| `bank-bancolombia`      | spoke-cop  | join  | 8746     | 8756 | 31404 | 1338    |
| `bank-davivienda`       | spoke-cop  | join  | 8747     | 8757 | 31405 | 1338    |
| `central-bank-argentina`| spoke-ars  | found | 8845     | 8855 | 31503 | 1339    |
| `bank-galicia`          | spoke-ars  | join  | 8846     | 8856 | 31504 | 1339    |
| `bank-macro`            | spoke-ars  | join  | 8847     | 8857 | 31505 | 1339    |

---

## Prerequisites

- Go 1.26 or newer
- Docker and Docker Compose v2
- `jq`, `openssl`, `curl`
- Docker image `hyperledger/besu:25.8.0` (pulled automatically on the first `up`)
- A writable data directory. The sample manifests use a **relative**
  `spec.node.dataDir` (`cbweb3-data/<participant>`), which the CLI resolves
  against the current working directory. Running the commands below from
  `scenario-a/samples/`, the node data lands under `samples/cbweb3-data/` and the
  join bundles under `samples/bundles/` — no `sudo` and no privileged path.

All commands assume the repository is checked out and that you start from the
repository root.

---

## Step 0 — Build the `cbweb3` CLI

Build the toolkit binary once. It locates the Scenario A root automatically, so
it works from anywhere in the repository.

```bash
cd scenario-a/toolkit
go build -o ./cbweb3 ./cmd/cbweb3
```

Capture its absolute path and move to the samples directory, which is the working
directory for every command that follows:

```bash
export CBWEB3="$(pwd)/cbweb3"          # absolute path to the binary just built
cd ../samples                          # working directory for all steps below
export CBWEB3_HOME="$(cd .. && pwd)"   # Scenario A root (robust template/script resolution)
```

### Where the join bundles are written

`mode: found` emits a join bundle that the `mode: join` manifests consume. The
join manifests reference it as `../bundles/<spoke>.bundle.yaml` (relative to each
manifest), which resolves to `samples/bundles/`. Point the CLI's output directory
at `samples/` so the bundle lands exactly there:

```bash
export CBWEB3_OUTPUT_DIR="$(pwd)"      # bundles emit straight into samples/bundles/
```

> This one export replaces the `copy_bundle` step in `deploy-all.sh`. Without it,
> the CLI writes the bundle under `samples/cbweb3-data/bundles/` and you would
> have to copy it into `samples/bundles/` manually after each `found`.

---

## Step 1 — (Optional) Validate the manifests

`--dry-run` validates the schema, resolves the deployment profile, and prints the
execution plan without changing anything. Run it against every manifest before
provisioning:

```bash
for f in brazil/*.yaml colombia/*.yaml argentina/*.yaml; do
  echo "== $f =="
  "$CBWEB3" apply -f "$f" --dry-run -o yaml
done
```

Any manifest error (missing field, invalid value) is reported with a clear
message and a non-zero exit code.

---

## Step 2 — Start the Cacti relay

The `register-relay` step of `mode: found` is a **hard** prerequisite: the relay
must be healthy **before** the `apply`. When the LNET-operated relay is not
available, start a local one:

```bash
../provisioning/scripts/start-cacti.sh
```

The script brings the relay up with Docker Compose and waits for its health
endpoint. When it finishes, the relay is available at:

- Health: `GET  http://localhost:4000/api/v1/health`
- Register spoke: `POST http://localhost:4000/api/v1/spokes` (used by `register-relay`)
- List spokes: `GET  http://localhost:4000/api/v1/spokes`

The sample manifests already point `spec.relay.endpoint` at `http://localhost:4000`.
A single relay serves all three spokes; each registers under its own id.

---

## Step 3 — Found the Brazil spoke (`spoke-brl`)

`found` is central-bank-only. It creates the country network from the manifest
and runs an idempotent sequence of steps: it brings up the Besu bootnode and
generates the genesis (never regenerated on re-run), deploys the spoke contracts
(Paladin node registry, ZetoFactory, PenteFactory, Zeto token, `IdentityRegistry`
participant whitelist, fiat token, HTLC), starts the central bank's Paladin node,
registers the spoke on the relay, and finally stands up the central bank's
operational backend (api-gateway plus services), Keycloak, and the governance,
treasury, supervisor, and NOC portals.

```bash
"$CBWEB3" apply -f brazil/central-bank-brazil.yaml -o yaml
```

On completion the Brazil network is live, registered on the relay, and the join
bundle has been emitted to:

```
samples/bundles/spoke-brl.bundle.yaml
```

The bundle contains the bootnode enode, the genesis (hash and content), the
spoke-level contract addresses, the QBFT validator set, the spoke CA (trust
anchor), and the central bank endpoint. It contains **no private keys**.

---

## Step 4 — Join the Brazilian commercial banks

With the `spoke-brl` bundle emitted, provision the two banks. Each `join` writes
the bundle's genesis, syncs its Besu node to the network via the bootnode enode,
brings up the bank's own Paladin node and registers it on-chain, stands up the
bank's operational stack (infra, Keycloak, backend services, and Bank portal),
and creates the bilateral CB↔bank Pente context with its FXAgreement.

```bash
"$CBWEB3" apply -f brazil/bank-itau.yaml     -o yaml
"$CBWEB3" apply -f brazil/bank-bradesco.yaml -o yaml
```

> A joining bank completes onboarding (KYC) later through its Governance Portal.
> On approval the central bank issues the CB-signed certificate and whitelists the
> bank's wallet in the on-chain `IdentityRegistry`. No manual CLI step is required
> for that.

---

## Step 5 — Found the Colombia spoke (`spoke-cop`)

The Colombia spoke is independent of Brazil. It can be provisioned after Brazil
(or in parallel); the relay from Step 2 already serves all three spokes.

**Single-host caveat — read before running.** Every founding central bank defaults
its Paladin node to host port `31648`. On a single host each additional spoke must
use a distinct port, so set the Paladin CB URL override **before** founding
Colombia (Besu, backend, and frontend ports are already derived per-entity from
each manifest and do not collide):

```bash
export CBWEB3_PALADIN_CB_URL="http://localhost:31748"
"$CBWEB3" apply -f colombia/central-bank-colombia.yaml -o yaml
```

This emits `samples/bundles/spoke-cop.bundle.yaml`.

---

## Step 6 — Join the Colombian commercial banks

```bash
"$CBWEB3" apply -f colombia/bank-bancolombia.yaml -o yaml
"$CBWEB3" apply -f colombia/bank-davivienda.yaml  -o yaml
```

Brazil and Colombia are now deployed — this is the point `deploy-all.sh` reaches.
Continue to Step 7 to add the third spoke (Argentina), matching `deploy-three.sh`.

---

## Step 7 — Found the Argentina spoke (`spoke-ars`)

The Argentina spoke is the third independent spoke, founded after Colombia; the
relay from Step 2 already serves all three. On a single host it needs yet another
distinct Paladin CB port, so set the override **before** founding Argentina
(`31648` → `31748` → `31848`):

```bash
export CBWEB3_PALADIN_CB_URL="http://localhost:31848"
"$CBWEB3" apply -f argentina/central-bank-argentina.yaml -o yaml
```

This emits `samples/bundles/spoke-ars.bundle.yaml`.

---

## Step 8 — Join the Argentine commercial banks

```bash
"$CBWEB3" apply -f argentina/bank-galicia.yaml -o yaml
"$CBWEB3" apply -f argentina/bank-macro.yaml   -o yaml
```

The full sample environment is now deployed.

---

## Verification

Query the block number on each Besu node by its RPC port:

```bash
for p in 8645 8646 8647 8745 8746 8747 8845 8846 8847; do
  echo -n "port $p: "
  curl -s -X POST "http://localhost:$p" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
done
```

List the running spoke containers:

```bash
docker ps --filter "name=cbweb3-spoke-"
```

Confirm all three spokes are registered on the relay:

```bash
curl -s http://localhost:4000/api/v1/spokes | jq
```

The structured report printed by each `apply` (`-o yaml` or `-o json`) also shows
the per-step status: `success`, `skipped`, `failed`, or `pending`.

---

## Endpoints and operator logins

Every api-gateway exposes `/healthz`; every portal is served on `/`.

### Brazil (`spoke-brl`)

| Entity          | API (gateway)              | Portals                                                                                                                              |
|-----------------|----------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| central bank    | `http://localhost:18645`   | governance `http://localhost:25645` · treasury `http://localhost:26645` · supervisor `http://localhost:30645` · noc `http://localhost:32645` |
| `bank-itau`     | `http://localhost:18646`   | bank portal `http://localhost:25646`                                                                                                 |
| `bank-bradesco` | `http://localhost:18647`   | bank portal `http://localhost:25647`                                                                                                 |

### Colombia (`spoke-cop`)

| Entity             | API (gateway)              | Portals                                                                                                                              |
|--------------------|----------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| central bank       | `http://localhost:18745`   | governance `http://localhost:25745` · treasury `http://localhost:26745` · supervisor `http://localhost:30745` · noc `http://localhost:32745` |
| `bank-bancolombia` | `http://localhost:18746`   | bank portal `http://localhost:25746`                                                                                                 |
| `bank-davivienda`  | `http://localhost:18747`   | bank portal `http://localhost:25747`                                                                                                 |

### Argentina (`spoke-ars`)

| Entity          | API (gateway)              | Portals                                                                                                                              |
|-----------------|----------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| central bank    | `http://localhost:18845`   | governance `http://localhost:25845` · treasury `http://localhost:26845` · supervisor `http://localhost:30845` · noc `http://localhost:32845` |
| `bank-galicia`  | `http://localhost:18846`   | bank portal `http://localhost:25846`                                                                                                 |
| `bank-macro`    | `http://localhost:18847`   | bank portal `http://localhost:25847`                                                                                                 |

### Operator credentials (local only)

These accounts are declared in the sample manifests for local use only. In
staging or production the passwords must come from a secret store, not the
manifest.

| Entity                | Role       | Username                          | Password                     |
|-----------------------|------------|-----------------------------------|------------------------------|
| central-bank-brazil   | Governance | `admin@brasil.governance.gov`     | `brasil-governance-local`    |
| central-bank-brazil   | Treasury   | `admin@brasil.treasury.gov`       | `brasil-treasury-local`      |
| central-bank-brazil   | Supervisor | `admin@brasil.supervisor.gov`     | `brasil-supervisor-local`    |
| central-bank-brazil   | NOC        | `admin@brasil.noc.gov`            | `brasil-noc-local`           |
| bank-itau             | Bank       | `admin@itau.brasil.com`           | `itau-bank-local`            |
| bank-bradesco         | Bank       | `admin@bradesco.brasil.com`       | `bradesco-bank-local`        |
| central-bank-colombia | Governance | `admin@colombia.governance.gov`   | `colombia-governance-local`  |
| central-bank-colombia | Treasury   | `admin@colombia.treasury.gov`     | `colombia-treasury-local`    |
| central-bank-colombia | Supervisor | `admin@colombia.supervisor.gov`   | `colombia-supervisor-local`  |
| central-bank-colombia | NOC        | `admin@colombia.noc.gov`          | `colombia-noc-local`         |
| bank-bancolombia      | Bank       | `admin@bancolombia.colombia.com`  | `bancolombia-bank-local`     |
| bank-davivienda       | Bank       | `admin@davivienda.colombia.com`   | `davivienda-bank-local`      |
| central-bank-argentina| Governance | `admin@argentina.governance.gov`  | `argentina-governance-local` |
| central-bank-argentina| Treasury   | `admin@argentina.treasury.gov`    | `argentina-treasury-local`   |
| central-bank-argentina| Supervisor | `admin@argentina.supervisor.gov`  | `argentina-supervisor-local` |
| central-bank-argentina| NOC        | `admin@argentina.noc.gov`         | `argentina-noc-local`        |
| bank-galicia          | Bank       | `admin@galicia.argentina.com`     | `galicia-bank-local`         |
| bank-macro            | Bank       | `admin@macro.argentina.com`       | `macro-bank-local`           |

---

## Functional walkthrough (optional)

With the stack up, [`sample-tryout.sh`](./sample-tryout.sh) drives an end-to-end
flow through the real api-gateway REST calls: onboarding, reserve issuance,
tokenization, a cross-spoke FX agreement, relay, acceptance, and the dual-layer
HTLC PvP settlement.

```bash
./sample-tryout.sh
```

---

## Teardown

Stop the relay:

```bash
../provisioning/scripts/start-cacti.sh --down
```

Wipe all Docker containers and volumes plus the sample data directories (the same
`--clean` behaviour `deploy-all.sh` offers):

```bash
docker rm -f $(docker ps -aq) 2>/dev/null || true
docker volume rm $(docker volume ls -q) 2>/dev/null || true
rm -rf ./cbweb3-data
```

---

## Notes and troubleshooting

- **Idempotency.** Re-running any `apply` re-executes only the missing steps. The
  genesis is never regenerated on an existing spoke.
- **Relay is a hard dependency.** `mode: found` fails if the relay at
  `spec.relay.endpoint` does not respond. Start it (Step 2) before founding a
  spoke.
- **Single host, additional spokes.** Each founding central bank defaults its
  Paladin node to host port `31648`. On a single host, export a distinct
  `CBWEB3_PALADIN_CB_URL` before founding each additional spoke — Colombia
  `http://localhost:31748` (Step 5), Argentina `http://localhost:31848` (Step 7).
- **Backends and portals are automatic.** Both `found` and `join` now bring up the
  operational backend, Keycloak, and portals as part of the sequence — there is no
  separate manual backend start.
- **Local only.** The CLI accepts `environment: local` only. Staging and
  production key providers (real KMS) and certificate sources (real CA) are not
  yet implemented; promoting to production will require different manifest values,
  not engine changes.
- **Reference network untouched.** This toolkit does not modify or depend on
  `deploy/local` or `make/*.mk`; they remain the sample reference network.

---

## One-shot equivalent

Every step above is what the deploy scripts perform in one run. To automate the
same sequence instead of running it by hand:

```bash
./deploy-all.sh              # Brazil + Colombia (Steps 3–6)
./deploy-three.sh            # Brazil + Colombia + Argentina (Steps 3–8)
./deploy-three.sh --clean    # wipe Docker (containers + volumes) and data dirs first
```
