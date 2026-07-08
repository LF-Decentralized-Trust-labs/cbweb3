# Samples — Provisioning three spokes (Brazil, Colombia and Argentina)

> For a step-by-step walkthrough with copy-paste-ready `cbweb3` commands (the same
> sequence `deploy-all.sh` runs, but by hand), see
> [MANUAL-DEPLOYMENT.md](./MANUAL-DEPLOYMENT.md).

This directory contains ready-to-use `ParticipantDeployment` manifests for the
`cbweb3` toolkit, demonstrating the complete scenario:

- **Three independent spokes**, each founded by its own central bank:
  - `spoke-brl` — founded by `central-bank-brazil` (currency BRL, chainId 1337)
  - `spoke-cop` — founded by `central-bank-colombia` (currency COP, chainId 1338)
  - `spoke-ars` — founded by `central-bank-argentina` (currency ARS, chainId 1339)
- **Two commercial banks per spoke**, each joining via a join bundle:
  - Brazil: `bank-itau`, `bank-bradesco` → `spoke-brl`
  - Colombia: `bank-bancolombia`, `bank-davivienda` → `spoke-cop`
  - Argentina: `bank-galicia`, `bank-macro` → `spoke-ars`

> The bank names are illustrative, used only to demonstrate provisioning.

Everything is provisioned **by configuration** (YAML manifest), without editing
code and without touching the reference network (`deploy/local` and the Makefile
remain intact).

Covers PHASES 1B (toolkit `mode: found`) and 3 (`mode: join`). **PHASE 4**
(staging/prod: real KMS, real CA, registry images) **is not implemented** — so
every manifest uses `environment: local`, `keyProvider: kms://local-emulator`,
and `certSource: self-signed`.

---

## Automation (shortcut)

The manual steps below are automated by two idempotent scripts that build the
CLI, start the relay, and apply every manifest in order:

```bash
./deploy-all.sh      # Brazil + Colombia (2 spokes)
./deploy-three.sh    # Brazil + Colombia + Argentina (3 spokes)
```

`deploy-three.sh` founds the Argentina spoke as soon as the Colombian banks
finish. Pass `--clean` to wipe Docker (containers + volumes) and data directories
before starting. The rest of this document describes the equivalent manual,
step-by-step flow that the scripts execute.

---

## Structure

```
samples/
  brazil/
    central-bank-brazil.yaml      # found  → spoke-brl
    bank-itau.yaml                # join   → spoke-brl
    bank-bradesco.yaml            # join   → spoke-brl
  colombia/
    central-bank-colombia.yaml    # found  → spoke-cop
    bank-bancolombia.yaml         # join   → spoke-cop
    bank-davivienda.yaml          # join   → spoke-cop
  argentina/
    central-bank-argentina.yaml   # found  → spoke-ars
    bank-galicia.yaml             # join   → spoke-ars
    bank-macro.yaml               # join   → spoke-ars
  bundles/                        # output of the `apply` mode:found runs (not versioned)
  deploy-all.sh                   # automates Brazil + Colombia (2 spokes)
  deploy-three.sh                 # automates Brazil + Colombia + Argentina (3 spokes)
```

## Port matrix (all on the same host)

Each Besu node needs distinct host ports. This is the allocation used by the manifests:

| Participant             | Spoke      | Mode  | RPC  | WS   | P2P   | chainId |
|-------------------------|------------|-------|------|------|-------|---------|
| central-bank-brazil     | spoke-brl  | found | 8645 | 8655 | 31303 | 1337    |
| bank-itau               | spoke-brl  | join  | 8646 | 8656 | 31304 | 1337    |
| bank-bradesco           | spoke-brl  | join  | 8647 | 8657 | 31305 | 1337    |
| central-bank-colombia   | spoke-cop  | found | 8745 | 8755 | 31403 | 1338    |
| bank-bancolombia        | spoke-cop  | join  | 8746 | 8756 | 31404 | 1338    |
| bank-davivienda         | spoke-cop  | join  | 8747 | 8757 | 31405 | 1338    |
| central-bank-argentina  | spoke-ars  | found | 8845 | 8855 | 31503 | 1339    |
| bank-galicia            | spoke-ars  | join  | 8846 | 8856 | 31504 | 1339    |
| bank-macro              | spoke-ars  | join  | 8847 | 8857 | 31505 | 1339    |

---

## Prerequisites

- Go 1.26+, Docker + Docker Compose v2, `jq`, `openssl`, `curl`.
- Docker image `hyperledger/besu:25.8.0` available (pulled automatically on the first `up`).
- A writable data directory. The manifests use a **relative** `spec.node.dataDir`
  (`cbweb3-data/<participant>`), which the CLI resolves against the current working
  directory (CWD) and creates automatically. Running `./deploy-all.sh` from
  `samples/`, the node data and join bundles land under `samples/cbweb3-data/` — no
  `sudo` and no privileged path. To use another location, edit `spec.node.dataDir`
  (relative or absolute).

---

## Step 0 — Build the toolkit

```bash
cd scenario-a/toolkit
go build -o ./cbweb3 ./cmd/cbweb3
```

The binary locates the `scenario-a` root automatically (anchor search from the
executable and the current directory), so it works from **anywhere inside the
repository** — including running `./cbweb3` from within `samples/`. If you run the
binary **outside** the repository, point it at the root with `CBWEB3_HOME`:

```bash
export CBWEB3_HOME="$(cd ../ && pwd)"   # scenario-a root
```

> The templates (`provisioning/templates/...`) and scripts (`deploy/local/...`) are
> canonical toolkit assets — they are not copied into `samples/`. `deploy-contracts`
> runs `go test` against those scripts, so the engine always requires the repository
> to be present.

For the whole session, set the directory where join bundles are emitted —
pointing it at this `samples/` folder, so that `mode: join` manifests find the
bundle at `../bundles/`:

```bash
export CBWEB3_OUTPUT_DIR="$(cd ../samples && pwd)"
CBWEB3="$(pwd)/cbweb3"
```

> Without `CBWEB3_OUTPUT_DIR`, the bundle is written to the parent directory of
> `dataDir` (with the sample manifests: `<CWD>/cbweb3-data/bundles/...`). In that
> case, adjust `joinBundleRef` in the join manifests to the matching path.

---

## Step 1 — Validate the manifests (dry-run)

`--dry-run` validates the schema, resolves the profile, and prints the execution
plan without running anything. Run it against all of them before provisioning:

```bash
for f in ../samples/brazil/*.yaml ../samples/colombia/*.yaml ../samples/argentina/*.yaml; do
  echo "== $f =="
  "$CBWEB3" apply -f "$f" --dry-run --output yaml
done
```

Manifest errors (missing field, invalid value) are reported all at once, with a
clear message, and the command exits with code 1.

---

## Step 2 — Start the local Cacti relay

`mode: found` manifests register the spoke on the relay, and `register-relay` is a
**mandatory** (hard) step: the relay must be up **before** the `apply`. Since LNET
is not available, start a local relay:

```bash
../provisioning/scripts/start-cacti.sh
# waits for health at http://localhost:4000/api/v1/health
```

The relay exposes the registration endpoints (RL-1): `found` performs
`POST /api/v1/spokes` and the toolkit confirms via `GET /api/v1/spokes/<id>`. The
sample manifests already point `spec.relay.endpoint` at `http://localhost:4000`.

---

## Step 3 — Found the Brazil spoke (`mode: found`)

```bash
"$CBWEB3" apply -f ../samples/brazil/central-bank-brazil.yaml --output yaml
```

`found` is **CB-only**: it creates the country network (central bank) from the
manifest — without bringing up Besu manually and **without** fixed bank nodes. The
engine runs the idempotent sequence of **9 steps**:

1. `start-besu` — brings up the Besu bootnode and **generates the genesis** on the first run (idempotent; never regenerated)
2. `deploy-contracts` — Paladin node registry, ZetoFactory, PenteFactory
3. `gen-tls` — TLS cert for the CB's Paladin node (self-signed)
4. `render-configs` — CB Paladin config
5. `register-nodes` — registers **the CB's Paladin node** on-chain (native logic, parameterized per spoke — no `spoke-a` fallback)
6. `start-paladin` — brings up the CB's Paladin + health-check
7. `create-zeto-token`
8. `onboard-registry` — deploys `IdentityRegistry.sol` (participant whitelist) and registers the CB via the governance key
9. `register-relay` — registers the spoke on the Cacti relay (hard: fails if the relay does not respond)

> The **Pente** context and the **FXAgreement** (bilateral) are **not** created in
> `found` — they are created in `join`, in the CB↔bank relationship (pairwise).

On completion, the Brazil network is **up and ready to operate**, registered on
the relay and waiting for the commercial banks. The **join bundle** is emitted:

```
samples/bundles/spoke-brl.bundle.yaml
```

It contains the bootnode enode, the genesis (hash + content), the spoke-level
contract addresses (node registry, ZetoFactory, PenteFactory, ZetoToken, and the
participant whitelist), the QBFT validator set, the spoke CA (trust anchor), and
the `cbEndpoint`. **It contains no private keys.**

> Idempotency: running `apply` again re-executes only what is missing. The genesis
> is **never** regenerated on an existing spoke.

---

## Step 4 — Add the Brazilian banks (`mode: join`)

With the `spoke-brl` bundle emitted, provision the two banks:

```bash
"$CBWEB3" apply -f ../samples/brazil/bank-itau.yaml     --output yaml
"$CBWEB3" apply -f ../samples/brazil/bank-bradesco.yaml --output yaml
```

The join engine runs **15 steps**, in three blocks:

- **Entering the Besu network (1–8):** writes the bundle's genesis, brings up Besu
  syncing via the bootnode enode, waits for sync, votes the QBFT validator,
  generates a key pair + CSR (via `keyProvider`), sends the CSR to the CB (via
  `cbEndpoint`), receives the signed cert, and performs proof-of-possession +
  registration in the IdentityRegistry.
- **Bank Paladin, dynamic (9–12):** `gen-tls-join` (cert for the bank's Paladin
  node, derived from `bankId`), `render-config-join`, `start-paladin-join`
  (brings up the bank's Paladin), `register-paladin-node` (registers the node
  identity on-chain — native logic, no fixed bank name).
- **Private CB↔bank relationship (13–15):** `create-pente-context` (bilateral
  CB↔bank Pente group), `deploy-fxa-pente` (FXAgreement inside the group),
  `start-backend`.

> **Join prerequisites:**
> 1. The `request-cert` step POSTs the CSR to `cbEndpoint`. The **central bank's
>    backend (api-gateway)** must be up, otherwise the join fails (`ErrCBUnreachable`).
> 2. The Pente steps require **both Paladin nodes (CB and bank) to see each other**
>    via mTLS transport on the spoke network.
>
> The toolkit's ability to bring up the CB and bank backend stacks automatically is
> a future increment (today the backend is an external prerequisite).

---

## Step 5 — Found the Colombia spoke and add its banks

Same sequence, Colombian manifests. Since it is an independent spoke, it can be
done in parallel or after Brazil (the Cacti relay from Step 2 already serves all
three spokes — each registers under its own id):

```bash
# CB Paladin on a distinct host port (see the note below)
export CBWEB3_PALADIN_CB_URL="http://localhost:31748"

# Found spoke-cop
"$CBWEB3" apply -f ../samples/colombia/central-bank-colombia.yaml --output yaml
# → emits samples/bundles/spoke-cop.bundle.yaml

# Add the Colombian banks
"$CBWEB3" apply -f ../samples/colombia/bank-bancolombia.yaml --output yaml
"$CBWEB3" apply -f ../samples/colombia/bank-davivienda.yaml  --output yaml
```

> **Paladin on a single host.** Every founding central bank points its Paladin at
> host port `31648` by default. On a single host, each additional spoke needs a
> distinct port — export `CBWEB3_PALADIN_CB_URL` before `found`
> (Colombia: `http://localhost:31748`; Argentina: `http://localhost:31848`). The
> `deploy-all.sh`/`deploy-three.sh` scripts already do this automatically.

---

## Step 6 — Found the Argentina spoke and add its banks

Same sequence, Argentine manifests. As the third independent spoke, it can be done
after Colombia (the Cacti relay from Step 2 serves all three spokes):

```bash
# CB Paladin on a distinct host port (see the note above)
export CBWEB3_PALADIN_CB_URL="http://localhost:31848"

# Found spoke-ars
"$CBWEB3" apply -f ../samples/argentina/central-bank-argentina.yaml --output yaml
# → emits samples/bundles/spoke-ars.bundle.yaml

# Add the Argentine banks
"$CBWEB3" apply -f ../samples/argentina/bank-galicia.yaml --output yaml
"$CBWEB3" apply -f ../samples/argentina/bank-macro.yaml   --output yaml
```

---

## Verification

Query the block number on each node by its RPC port (matrix above):

```bash
for p in 8645 8646 8647 8745 8746 8747 8845 8846 8847; do
  echo -n "port $p: "
  curl -s -X POST "http://localhost:$p" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
done
```

Running containers:

```bash
docker ps --filter "name=cbweb3-spoke-"
```

The structured report (`--output yaml|json`) from `apply` shows the status of each
step (`success` / `skipped` / `failed` / `pending`).

---

## Notes

- **Local Cacti relay (RL-1).** Without LNET, start the local relay with
  `provisioning/scripts/start-cacti.sh` (Step 2). `register-relay` is
  **mandatory**: `found` fails if the relay (`spec.relay.endpoint`) does not
  respond. Registration is dynamic via `POST /api/v1/spokes` and persists in the
  relay's volume. Note: registration enables spoke **discovery**; for the relay to
  also actively *poll* for settlement, the complete connection data (besuWs,
  grpcEndpoint, internalApiUrl) is required in the relay's polling config
  (`CACTI_SPOKES_CONFIG`) — this is part of the N-spokes generalization (Phase 2).
- **PHASE 4 (staging/prod) not implemented.** Only `environment: local` is accepted
  by the CLI. The prod implementations of `keyProvider` (real KMS) and `certSource`
  (real CA) are stubs that return `ErrNotImplemented`. Promoting to prod will
  require only different values in the manifest — no engine changes.
- **Cross-stack addressing.** `advertisedHost` is always explicit, never inferred
  from co-location or container IP (see
  `provisioning/docs/adr-001-cross-stack-enode-addressing.md`). In a multi-stack
  local deployment, participants of the same spoke share the Docker network
  `cbweb3-<spoke-id>-besu`.
- **Reference network untouched.** This toolkit does not modify or depend on
  `deploy/local` or `make/*.mk` — they remain the sample network.
