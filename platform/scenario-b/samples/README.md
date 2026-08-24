# Samples — Provisioning the hub + sovereign spokes (Brazil, Argentina, Colombia)

Ready-to-use `ParticipantDeployment` manifests for the **Scenario B** toolkit
(`cbweb3b`), demonstrating the complete hub-and-spoke topology:

- **One neutral interoperability hub**, founded once (`mode: found-hub`):
  - `hub-cbweb3` — base `tCeBM` reserve tokens + registries + AMM + LCR + NOC (chainId 1337)
- **One external Cacti liquidity relay**, deployed outside the toolkit
  (`start-cacti.sh`) and reached via each manifest's `spec.relay.endpoint`
  (`http://localhost:7000`); spokes register on it dynamically at `found-spoke`.
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

Each CB/bank manifest has `launcher: enable`, so every entity also gets its per-entity
launcher (see the Port matrix). Build the generic launcher image once first, or the
launcher step just logs a hint and is skipped:

```bash
( cd ../../launcher && ./build.sh )   # → cbweb3/launcher:local
```

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

### Per-entity launcher (distributed A/B entry point)

Each **commercial bank and central bank** also gets a **launcher** — a small SPA that
lists that entity's own portals, grouped by Scenario A / Scenario B, and redirects to the
chosen one (login happens on the destination portal). It is the per-entity entry point in
the distributed model (one entity per host); the **hub has no launcher**.

The toolkit deploys it when the entity's manifest sets `launcher: enable`, on the host
port `spec.launcherPort`. Since this sample runs every entity on one host, each declares a
distinct port — and the **same** port in Scenario A and B, so both scenarios share that
entity's launcher (which merges its A and B portal fragments):

| Entity | launcherPort | Entity | launcherPort |
|--------|:---:|--------|:---:|
| central-bank-brazil    | 5191 | central-bank-colombia | 5197 |
| bank-itau              | 5192 | bank-bancolombia      | 5198 |
| bank-bradesco          | 5193 | bank-davivienda       | 5199 |
| central-bank-argentina | 5194 | bank-galicia          | 5195 |
| bank-macro             | 5196 |                       |      |

The launcher image is **generic** and built once (platform step) **before** deploying:

```bash
( cd ../../launcher && ./build.sh )   # → cbweb3/launcher:local
```

`launcher: disable` (or omitting it) removes this scenario's fragment and tears the
launcher container down when no scenario fragment remains. The launcher step is soft: a
missing image only logs a hint and never blocks the entity's deploy.

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
(`http://localhost:7000`).

```bash
bash "$ROOT/scenario-b/provisioning/scripts/start-cacti.sh"
# waits for health at http://localhost:7000/api/v1/health
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
  --repo-root "$ROOT" --out-dir "$PWD"
# → emits bundles/spoke-brl.bundle.yaml
```

> The toolkit reaches the spoke's own Besu node (readiness/enode/contract gates) at
> `http://localhost:<node.rpc.port>`, taken from the manifest — for
> `central-bank-brazil`, `8645`. Override it only for a non-default host/port with
> `--spoke-rpc <url>`.

### Step 6 — Join the Brazilian banks (`mode: join`)

Each bank consumes `spoke-brl.bundle.yaml`, syncs the spoke genesis as a
non-validating full node, provisions its Keycloak operator, starts its app stack +
bank portal, and generates its key + CSR (`gen-csr`). The CB signs the CSR and
registers the bank on-chain at **runtime** (onboarding portal), not as a toolkit step.

```bash
"$BIN" apply -f brazil/bank-itau.yaml     -o yaml --repo-root "$ROOT" --out-dir "$PWD"
"$BIN" apply -f brazil/bank-bradesco.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD"
```

### Step 7 — Found the Argentina spoke and join its banks

Same sequence with the Argentine manifests (the relay from Step 3 already serves
all spokes; each registers under its own id):

```bash
"$BIN" apply -f argentina/central-bank-argentina.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD"
"$BIN" apply -f argentina/bank-galicia.yaml           -o yaml --repo-root "$ROOT" --out-dir "$PWD"
"$BIN" apply -f argentina/bank-macro.yaml             -o yaml --repo-root "$ROOT" --out-dir "$PWD"
```

### Step 8 — (optional) Found the Colombia spoke

A third independent spoke that joins the hub **without** opening a corridor:

```bash
"$BIN" apply -f colombia/central-bank-colombia.yaml -o yaml --repo-root "$ROOT" --out-dir "$PWD"
"$BIN" apply -f colombia/bank-bancolombia.yaml      -o yaml --repo-root "$ROOT" --out-dir "$PWD"
"$BIN" apply -f colombia/bank-davivienda.yaml       -o yaml --repo-root "$ROOT" --out-dir "$PWD"
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

## Per-spoke currency and token metadata

Every spoke declares its own currency; nothing about a currency is hardcoded in
the toolkit. Each CB manifest carries:

```yaml
spoke:
  currency: BRL              # ISO 4217 — the routing key
  tokenName: Tokenized BRL   # optional; derived from `currency` when absent
  tokenSymbol: tCeBM_BRL
  fiatTokenName: Fiat BRL
  fiatTokenSymbol: fCeBM_BRL
```

`currency` is the only **routing** key (relay id `spoke-<currency>`, hub currency
registration, mirrored `W-tCeBM_<ISO>`). The four token fields are the spoke's own
`tCeBM`/`fCeBM` ERC-20 metadata and must keep the `<prefix>_<ISO>` symbol shape —
the portals and the api-gateway read the displayed currency code from the segment
after the last underscore. A joining bank repeats `spoke.currency` in its own
manifest and the join rejects it if it disagrees with the spoke bundle. See
`toolkit/README.md` for the full contract.

---

## Sovereign corridor (BRL ↔ ARS) — opened at runtime via the CB portal

Each sovereign currency (W-token) is already deployed and registered on the hub
by `found-spoke` (via the hub compliance service) — so no runtime currency step
is needed. Provisioning does **not** open the FX corridor, however, and the
manifests deliberately say nothing about it: the toolkit never holds sovereign
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

## Service-mesh mTLS (R2-H-8) — opt-in

Inter-service gRPC (api-gateway → auth/compliance/payment-orchestrator, and
auth → compliance) runs **plaintext by default**, unchanged. The toolkit always
seeds a per-entity service-mesh CA + one leaf certificate per gRPC service into a
`<prefix>_svc_tls` volume (`gen-svc-tls`), and every backend service mounts it at
`/svc-tls` read-only — but the certificates are **only used when you opt in**.

Enable mutual TLS (and, optionally, rejection of unauthenticated callers) by
exporting two environment variables before the toolkit run — they flow through to
the rendered compose. This works identically for the local samples and for
`deploy-lnet` (which runs the same toolkit): each entity's gRPC services are
co-located on one host, so the per-entity CA covers every internal hop.

```bash
# mutual TLS on all internal gRPC (services present + verify peer certs):
export GRPC_MTLS_ENABLE=1
# additionally reject unauthenticated/unauthorized callers (requires mTLS):
export GRPC_AUTHZ_ENFORCE=true

./deploy-all.sh          # (or deploy-lnet/deploy.sh …)
```

With `GRPC_MTLS_ENABLE` unset the `GRPC_MTLS_*` paths interpolate to empty and the
transport stays plaintext — existing deployments are unaffected. Setting
`GRPC_AUTHZ_ENFORCE=true` without `GRPC_MTLS_ENABLE=1` is refused at service start
(enforcing over a plaintext, header-settable identity is a fail-open).

Verify it with `./mtls-smoke.sh`, which is that check made repeatable. It reads each
gateway's own environment to establish whether the mesh is on **per entity** (so a pass
cannot be vacuous, and an entity still running plaintext by design is not judged by its
neighbour's posture), then runs two checks that are deliberately kept separate:

1. **Cleartext HTTP/2 is not answered.** The HTTP/2 preface goes out over raw TCP from
   inside the entity's docker network, and the reply bytes are read as hex. An HTTP/2
   SETTINGS frame back means the port is serving h2c and the check fails.
2. **A TLS handshake with no client certificate is refused.** This is the mutual half:
   server-only TLS passes check 1 and still accepts any client.

Finally it confirms the gateway still serves. Run `sample-tryout.sh` after it for the
full login → governance-list round-trip over the mesh.

Two properties of the script are load-bearing, and both were verified against purpose-built
listeners (a client-cert-requiring TLS server, a server-only TLS server, an h2c responder,
and a dead port) rather than assumed:

- **A port that cannot be dialled is reported as *not verified*, never as a pass.**
  Reachability is probed explicitly, because busybox `nc` signals a refused connection
  with an exit status and nothing else — "no answer" and "nobody listening" are otherwise
  the same observation.
- **The no-client-certificate probe writes application bytes and waits.** Under TLS 1.3
  the server lets the handshake reach `Cipher is ...` and only then sends
  `alert certificate required`; a probe that closes at EOF races that alert and reports a
  healthy mesh as broken.

An earlier revision of this script used `openssl s_client` for check 1. That cannot test
what check 1 claims: `s_client` opens with a TLS ClientHello, so nothing it sends ever
reaches the wire in the clear. It measured check 2 twice.

```bash
export GRPC_MTLS_ENABLE=1 GRPC_AUTHZ_ENFORCE=true
./deploy-all.sh --clean      # provision with the mesh on
./mtls-smoke.sh              # verify enforcement
```

### Per-method authorization

With enforcement on, each service also restricts its **value-moving** methods to the
callers that legitimately make them, rather than to any authenticated peer: minting,
burning, settlement, deposit/escrow/redeem approval and the FX lifecycle accept only
the `api-gateway` identity; participant freeze and CSR signing accept the gateway and
`auth`; the participant record accepts the gateway (its governance route) and `auth`
(onboarding); certificate issuance accepts `auth` only. Audit writes are restricted the
same way, which stops a mesh peer injecting entries — but note that `CreateAuditLog`
still records the actor its caller states, because that actor is the human operator from
the JWT rather than the calling service. Attesting it is tracked as the open half of
R2-H-8 item 3. Reads keep
the baseline policy and still honour `GRPC_AUTHZ_ALLOWED_CALLERS`. The lists are
compiled in (`internal/grpc/server/authz_policy.go` per service) because the caller
identities are the mesh certificate CNs the toolkit issues — known without operator
configuration — and two tests fail the build: one if a new mutating RPC is added without
a restriction, and one if a restriction excludes the gateway without being listed as
deliberately internal. The second exists because a caller set is easy to get wrong in
the dangerous direction: the gateway reaches `UpsertParticipant` through an adapter
method named `RegisterParticipant`, so auditing by RPC name alone concludes that no
route touches it and takes a live route down when enforcement is switched on.

> Cross-entity gRPC is not part of this mesh (there is none today — cross-VM
> traffic is Besu JSON-RPC + HTTP). A federated/shared service CA would be needed
> before any cross-entity gRPC hop could enforce mTLS.

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

---

## Reverse proxy — single-host smoke test

`deploy-all.sh` / `deploy-three.sh` are **port-based** (many entities on one host, portals
on host ports, one per-entity launcher). They do **not** enable the reverse proxy, and that
is deliberate — the Caddy proxy is **one container per host with a single site and one
route fragment per scenario**, i.e. one entity per host (the `deploy-lnet` multi-host
model). Enabling it for the many-entity local deploy would make the entities collide.

To exercise the proxy on one machine, use the dedicated smoke test, which brings up the
**hub + a single spoke** (`central-bank-brazil` found-spoke) with `proxy: enable` on the CB:

```bash
./proxy-smoke.sh                 # self-signed TLS (internal CA) — one browser warning
PROXY_TLS_MODE=off ./proxy-smoke.sh   # plain HTTP on :80 — no warning
./proxy-smoke.sh --clean         # wipe docker + work dir first
```

It uses `frontendHost: cb-brazil.localtest.me` (resolves to `127.0.0.1`), so the portals are
reachable path-based on this host with no `/etc/hosts` edit (browser **on this machine**):

```
https://cb-brazil.localtest.me/              -> launcher
https://cb-brazil.localtest.me/b/governance/ -> Governance   (also /b/treasury/, /b/supervisor/)
https://cb-brazil.localtest.me/b/api/v1/     -> api-gateway
```

The hub stays port-based (it is infra, not proxied). Run `scenario-a/samples/proxy-smoke.sh`
with the same host and **one** proxy serves both `/a/*` and `/b/*`. Manifest:
[proxy-smoke/central-bank-brazil.yaml](./proxy-smoke/central-bank-brazil.yaml).
