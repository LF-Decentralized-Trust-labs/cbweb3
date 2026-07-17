# Scenario A — LNET deployment

Toolkit-driven install of **Scenario A (Enhanced Correspondent Banking, dual-layer HTLC)**
across the LNET 6-VM lab. Two independent spokes (Brazil, Colombia) bridged by a Cacti relay.
There is **no hub** in Scenario A (the hub is a Scenario-B concept).

Shared VM `.20` infrastructure (both Cacti relays; the Scenario-B hub) is documented at the
[deploy-lnet root README](../README.md). This folder covers Scenario A's spokes and banks (VMs .21–.26).

## Topology

| VM | Role | Spoke | bank-id | Launcher FQDN | Manifest |
|----|------|-------|---------|---------------|----------|
| 10.10.0.20 | Cacti relay (Scenario A) | — | — | — | *(no manifest — see root README)* |
| 10.10.0.21 | Central Bank Brazil (found) | spoke-brazil (BRL, chainId 2021) | — | cb-brazil.cbweb3.lnet.io | [manifests/cb-brazil.yaml](manifests/cb-brazil.yaml) |
| 10.10.0.22 | Commercial bank (join) | spoke-brazil | cb1 | cb1-brazil.cbweb3.lnet.io | [manifests/cb1.yaml](manifests/cb1.yaml) |
| 10.10.0.23 | Commercial bank (join) | spoke-brazil | cb2 | cb2-brazil.cbweb3.lnet.io | [manifests/cb2.yaml](manifests/cb2.yaml) |
| 10.10.0.24 | Central Bank Colombia (found) | spoke-colombia (COP, chainId 2024) | — | cb-colombia.cbweb3.lnet.io | [manifests/cb-colombia.yaml](manifests/cb-colombia.yaml) |
| 10.10.0.25 | Commercial bank (join) | spoke-colombia | cb3 | cb3-colombia.cbweb3.lnet.io | [manifests/cb3.yaml](manifests/cb3.yaml) |
| 10.10.0.26 | Commercial bank (join) | spoke-colombia | cb4 | cb4-colombia.cbweb3.lnet.io | [manifests/cb4.yaml](manifests/cb4.yaml) |

## ⚠️ Read before deploying: multi-VM reality

The toolkit only executes `environment: local`, and that profile was validated **all-on-one-host**.
The manifests here are wired for genuinely separate VMs, but be aware of the seams:

1. **Besu P2P is the one genuinely cross-VM layer.** A joining bank syncs from its central bank
   over devp2p using the real IP baked into the spoke-bundle enode (`node.advertisedHost`). This
   works — provided the central bank publishes host P2P port **30303**. Scenario A's join path
   *forces* the bootnode enode to container port 30303, so `node.p2p.port: 30303` on the CBs is
   mandatory (not a free choice). Scenario B on the same VMs deliberately uses **30304** to avoid
   the clash.
2. **Export `BESU_NAT_PROFILE=NONE`** in the shell before `apply` on every VM, so Besu advertises
   the routable `advertisedHost` instead of the Docker-bridge address.
3. **Paladin cross-node transport needs `9000/tcp` open between every CB↔bank pair.** The bilateral
   Pente step (`create-pente-context`) drives Paladin's mutual-TLS gRPC transport on port **9000**.
   The transport is bidirectional (request AND reply), so `9000/tcp` must be reachable **both**
   directions between the CB VM and each bank VM. When `node.advertisedHost` is a routable IP/DNS
   (as in these manifests), each node advertises that host as its on-chain transport endpoint and
   carries it in its cert SAN, so no per-peer `extra_hosts` is needed — but the firewall/security
   group must allow `9000/tcp` (just like `30303/tcp` for Besu P2P). Symptom when blocked or when a
   node was provisioned by an older toolkit that advertised its container name: `create-pente-context`
   logs `POST ptx_resolveVerifier: ... context deadline exceeded` every ~30s and never completes.
4. **HTTP integration flows are NOT turnkey cross-VM.** In the `local` join path the bank→central-bank
   api-gateway URL is localized to `host.docker.internal` (i.e. the bank's *own* host), so the
   credential/onboarding call does not reach a remote CB unmodified. The bank still comes up and syncs
   the chain; the governance-gated CSR signing / on-chain registration are runtime steps done through
   the CB governance portal. If you need the bank→CB onboarding call to traverse VMs, remap
   `host.docker.internal` at the container level (edit `extra_hosts` in
   `../../scenario-a/provisioning/templates/*/docker-compose.yaml`) or provide a flat routable overlay.
   This is an operator-patched path, not a toolkit feature.

DNS: point each launcher FQDN (`cb-brazil.cbweb3.lnet.io`, `cb1-brazil...`, etc.) at the matching VM IP.

## Manifests are templates

The `manifests/*.yaml` files are **generated** from `*.yaml.tmpl` by substituting the `${IP_*}` address
markers from [`../addresses.env`](../addresses.env) (the toolkit does not expand env vars). Render them
first, or let the wrapper do render + apply in one step:

```bash
../deploy.sh render          # render all templates -> *.yaml
../deploy.sh a cb-brazil     # render + apply this Scenario-A target (adds BESU_NAT_PROFILE=NONE)
```

The manual `./toolkit/cbweb3 apply -f …` commands below operate on the **rendered** `.yaml`; run
`../deploy.sh render` (or edit `addresses.env`) before using them.

## Prerequisites (every VM)

- The repo checked out at the same path. `cbweb3` locates its templates by walking up from the current
  directory to `provisioning/templates/central-bank/docker-compose.yaml`, so run it from **`scenario-a/`**
  (the manifests live outside that dir, at repo-root `deploy-lnet/scenario-a/manifests/` — reference them
  with `../deploy-lnet/...`).
- Docker Compose v2, Foundry (`forge`), Go 1.26+ (or a prebuilt `cbweb3` binary).
- Build the launcher image once per host if you use `launcher: enable`, else the launcher step is a soft skip.

Build the toolkit binary once (or `go run` it):

```bash
cd scenario-a
go build -o toolkit/cbweb3 ./toolkit/cmd/cbweb3
```

## Deployment order

Relay first (see root README), then each founder, then the banks (a bank needs its founder's bundle to exist).

### 1 — VM 10.10.0.20 — Cacti relay (Scenario A)

The relay is **not** provisioned by `cbweb3`; start it separately. It must be reachable at
`http://10.10.0.20:4000` from both CB VMs. Easiest is the wrapper, which starts **both** scenarios'
relays at once (Scenario A `:4000` + Scenario B `:7000`):

```bash
../deploy.sh cacti          # both relays (recommended — run once on VM .20)
../deploy.sh cacti --down   # tear both down
```

Or start only the Scenario-A relay manually:

```bash
cd scenario-a
CACTI_PORT=4000 provisioning/scripts/start-cacti.sh
```

### 2 — VM 10.10.0.21 — Central Bank Brazil (founder)

```bash
cd scenario-a
export BESU_NAT_PROFILE=NONE
./toolkit/cbweb3 apply -f ../deploy-lnet/scenario-a/manifests/cb-brazil.yaml --dry-run   # preview the 17-step plan
./toolkit/cbweb3 apply -f ../deploy-lnet/scenario-a/manifests/cb-brazil.yaml
# Emits: /opt/cbweb3/data/scenario-a/bundles/spoke-brazil.bundle.yaml   (out dir = dirname(dataDir))
```

Copy the emitted bundle into each Brazil bank VM's `deploy-lnet/scenario-a/bundles/`:

```bash
scp /opt/cbweb3/data/scenario-a/bundles/spoke-brazil.bundle.yaml \
    op@10.10.0.22:<repo>/deploy-lnet/scenario-a/bundles/
scp /opt/cbweb3/data/scenario-a/bundles/spoke-brazil.bundle.yaml \
    op@10.10.0.23:<repo>/deploy-lnet/scenario-a/bundles/
```

### 3 — VMs 10.10.0.22 / 10.10.0.23 — cb1 / cb2 (join Brazil)

```bash
cd scenario-a
export BESU_NAT_PROFILE=NONE
./toolkit/cbweb3 apply -f ../deploy-lnet/scenario-a/manifests/cb1.yaml     # .22 ; cb2.yaml on .23
```

`joinBundleRef` (`../bundles/spoke-brazil.bundle.yaml`) resolves relative to the manifest file, i.e.
`deploy-lnet/scenario-a/bundles/spoke-brazil.bundle.yaml` — the scp target above.

### 4 — VM 10.10.0.24 — Central Bank Colombia (founder)

Same as step 2 with `cb-colombia.yaml`; emits `spoke-colombia.bundle.yaml`. scp it to `.25` and `.26`.

### 5 — VMs 10.10.0.25 / 10.10.0.26 — cb3 / cb4 (join Colombia)

Same as step 3 with `cb3.yaml` / `cb4.yaml`.

Re-runs are idempotent: state lives in `<dataDir>/.provisioning-state.yaml`; re-running `apply`
resumes from the first incomplete step. `--dry-run` plans without side effects.

## Port map (per VM)

All entities reuse one base (one entity per VM per scenario). Scenario A ports end in **`645`**;
Scenario B ports end in `845`, so both scenarios coexist on a shared VM.

| Service | Offset from RPC | Host port |
|---------|-----------------|-----------|
| Besu RPC | base | 8645 |
| Besu WS | — | 8655 |
| Besu P2P | — | 30303 |
| API Gateway | +10000 | 18645 |
| Auth gRPC | +11000 | 19645 |
| Compliance gRPC | +12000 | 20645 |
| Payment gRPC | +13000 | 21645 |
| Postgres | +14000 | 22645 |
| Redis | +15000 | 23645 |
| Keycloak | +16000 | 24645 |
| Frontend (governance/bank) | +17000 | 25645 |
| Frontend treasury | +18000 | 26645 |
| Frontend supervisor | +22000 | 30645 |
| Frontend NOC | +24000 | 32645 |
| Launcher (shared A+B) | — | 5190 |
| Cacti relay (VM .20 only) | — | 4000 |

## Notes

- `keyProvider: kms://local-emulator` and `certSource: self-signed` are the only values the `local`
  profile runs. No private keys ever live in these manifests.
- `frontendHost` carries the launcher FQDN; it is baked into `VITE_API_URL`/`VITE_KEYCLOAK_URL` at
  build time and into the launcher portal links. Portals browsed from a remote machine still call
  `localhost` in some paths — a known local-first limitation.
- Do **not** commit real credentials. The `change-me-local` passwords are lab placeholders.
