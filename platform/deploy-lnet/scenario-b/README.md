# Scenario B — LNET deployment

Toolkit-driven install of **Scenario B (International Hub — FXAgreement + AMM +
LiquidityCommitRegistry, relay + circuit breaker)** across the LNET 6-VM lab. One central **hub**
chain plus two sovereign spokes (Brazil, Colombia); commercial banks join their spoke; a Cacti
relay federates cross-currency corridors.

The Scenario-B **hub** lives under [`../hub/`](../hub/) (`hub/manifests/`); shared bundles live under
[`../bundles/`](../bundles/) (`bundles/hub/`, `bundles/scenario-b/<spoke>/`, `bundles/scenario-b/<bankId>/`). Both Cacti relays are started from
the [deploy-lnet root](../README.md). This folder covers Scenario B's spokes and banks (VMs .21–.26).

## Topology

| VM | Role | Spoke | bank-id | Launcher FQDN | Manifest |
|----|------|-------|---------|---------------|----------|
| 10.10.0.20 | Hub (found-hub) + Cacti relay | hub (chainId 1337) | — | — | [../hub/manifests/hub.yaml](../hub/manifests/hub.yaml) |
| 10.10.0.21 | Central Bank Brazil (found-spoke) | spoke-brazil (BRL, chainId 2022) | — | cb-brazil.cbweb3.lnet.io | [manifests/cb-brazil.yaml](manifests/cb-brazil.yaml) |
| 10.10.0.22 | Commercial bank (join) | spoke-brazil | cb1 | cb1-brazil.cbweb3.lnet.io | [manifests/cb1.yaml](manifests/cb1.yaml) |
| 10.10.0.23 | Commercial bank (join) | spoke-brazil | cb2 | cb2-brazil.cbweb3.lnet.io | [manifests/cb2.yaml](manifests/cb2.yaml) |
| 10.10.0.24 | Central Bank Colombia (found-spoke) | spoke-colombia (COP, chainId 2025) | — | cb-colombia.cbweb3.lnet.io | [manifests/cb-colombia.yaml](manifests/cb-colombia.yaml) |
| 10.10.0.25 | Commercial bank (join) | spoke-colombia | cb3 | cb3-colombia.cbweb3.lnet.io | [manifests/cb3.yaml](manifests/cb3.yaml) |
| 10.10.0.26 | Commercial bank (join) | spoke-colombia | cb4 | cb4-colombia.cbweb3.lnet.io | [manifests/cb4.yaml](manifests/cb4.yaml) |

The **sovereign BRL/COP pair (corridor + AMM liquidity)** is intentionally **not** provisioned here.
It is opened at runtime through the CB governance portals after the network is up. (`spec.pair` is
documentational only — the toolkit never opens a corridor — so it is omitted.)

## ⚠️ Read before deploying: multi-VM reality

The toolkit only executes `environment: local`, and that profile was validated **all-on-one-host**.
The manifests here are wired for genuinely separate VMs, but be aware of the seams:

1. **Besu P2P is the one genuinely cross-VM layer.** A joining bank syncs from its central bank over
   devp2p using the real IP baked into the spoke-bundle enode (`node.advertisedHost` + `node.p2p.port`).
   This works. Scenario B uses host P2P port **30304** (Scenario A on the same VMs owns 30303 — its
   join path forces the enode to 30303, so the two must differ).
2. **Export `BESU_NAT_PROFILE=NONE`** (or `HOST_IP=<vm-ip>`) before `apply` on every VM so the enode
   advertises the routable address instead of the Docker-bridge IP. `node.advertisedHost` must be the
   VM's real LAN IP (loopback / 172.16–31 are rejected).
3. **HTTP integration flows are NOT turnkey cross-VM.** The emitted bundles bake `localhost` /
   `host.docker.internal` into cross-host URLs (hub RPC/WS/gateway, spoke RPC/WS, CB gateway), and the
   compose templates reach hub/CB/relay via `host.docker.internal:host-gateway` (each VM's own host).
   Before transferring a bundle you must patch those URLs to the real IP (see the `sed` steps below),
   and for full cross-VM backend traffic you must additionally remap `host.docker.internal` per VM
   (edit `extra_hosts` in `../../scenario-b/provisioning/templates/*.compose.yaml`). Treat cross-VM HTTP
   as an operator-patched path, not a toolkit feature. The blockchain layers (hub chain, per-spoke QBFT
   chains, bank joins) form correctly regardless.

DNS: point each launcher FQDN at the matching VM IP.

## Manifests are templates

The hub manifest (`../hub/manifests/hub.yaml`) and `manifests/*.yaml` are **generated** from `*.yaml.tmpl` by
substituting the `${IP_*}` address markers from [`../addresses.env`](../addresses.env) (the toolkit
does not expand env vars). Render them first, or let the wrapper do render + apply in one step:

```bash
../deploy.sh render          # render all templates -> *.yaml
../deploy.sh b hub           # render + apply the hub (VM .20)
../deploy.sh b cb-brazil     # render + apply this found-spoke (adds --hub-rpc http://$IP_HUB:8845)
../deploy.sh b cb1           # render + apply this join
```

The manual `cbweb3b apply -f …` commands below operate on the **rendered** `.yaml`; run
`../deploy.sh render` (or edit `addresses.env`) before using them.

## Prerequisites (every VM)

- The repo checked out at the same path; run `cbweb3b` from the **repo root** so `--repo-root .` finds
  `scenario-b/contracts` and `scenario-b/provisioning/templates`.
- Docker Compose v2, Foundry (`forge`), Go 1.26+ (or the prebuilt `scenario-b/toolkit/samples/.cbweb3b`).
- `image: build` builds all images on first hub/spoke run (api-gateway, compliance, auth,
  payment-orchestrator, frontends, NOC, relay) — ensure build deps are present.

Below, `REPO` is the repo root and commands run from it.

## Deployment order

Relay + hub first (VM .20 — see [root README](../README.md)) → each CB spoke (needs the hub bundle) →
each bank (needs its spoke bundle).

### 1 — VM 10.10.0.20 — Cacti relay + hub (shared .20 infra)

```bash
cd $REPO
# Start both Cacti relays (Scenario A :4000 + Scenario B :7000) in one step:
deploy-lnet/deploy.sh cacti
#   (manual, Scenario-B relay only: scenario-b/provisioning/scripts/start-cacti.sh  # :7000)

# Found the hub (render + apply):
deploy-lnet/deploy.sh b hub
#   -> emits deploy-lnet/bundles/hub/hub.bundle.yaml
```

Patch the hub bundle's cross-host URLs (emitted as `localhost`), then copy into
`deploy-lnet/bundles/hub/` on both CB spoke VMs:

```bash
sed -i 's#http://localhost:8845#http://10.10.0.20:8845#; \
        s#ws://localhost:8846#ws://10.10.0.20:8846#; \
        s#http://localhost:16845#http://10.10.0.20:16845#' \
  $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml

scp $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml op@10.10.0.21:$REPO/deploy-lnet/bundles/hub/
scp $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml op@10.10.0.24:$REPO/deploy-lnet/bundles/hub/
```

### 2 — VM 10.10.0.21 — Central Bank Brazil (found-spoke)

```bash
# Preferred: render + apply via the LNET wrapper (creates dataDir under bundles/)
./deploy-lnet/deploy.sh b cb-brazil --dry-run   # preview
./deploy-lnet/deploy.sh b cb-brazil
# dataDir:  deploy-lnet/bundles/scenario-b/spoke-brazil/
# Emits → relocates: deploy-lnet/bundles/scenario-b/spoke-brazil/spoke-brazil.bundle.yaml
# (--hub-rpc is added automatically by deploy.sh)
```

The spoke manifest reads the hub bundle via `hubBundleRef: ../../bundles/hub/hub.bundle.yaml`
(the central drop-zone under `bundles/hub/`). With a current toolkit, `emit-spoke-bundle` already
bakes `node.advertisedHost` into `spokeRpc` / `spokeWs` / `cbGateway` — no `sed` needed for those.
Copy the bundle to the bank VMs:

```bash
scp $REPO/deploy-lnet/bundles/scenario-b/spoke-brazil/spoke-brazil.bundle.yaml \
    op@10.10.0.22:$REPO/deploy-lnet/bundles/scenario-b/spoke-brazil/
scp $REPO/deploy-lnet/bundles/scenario-b/spoke-brazil/spoke-brazil.bundle.yaml \
    op@10.10.0.23:$REPO/deploy-lnet/bundles/scenario-b/spoke-brazil/
```

### 3 — VMs 10.10.0.22 / 10.10.0.23 — cb1 / cb2 (join Brazil)

```bash
./deploy-lnet/deploy.sh b cb1     # .22 ; use `b cb2` on .23
#   -> write-genesis (from bundle) -> start-besu-join (--bootnodes=<CB>:30304)
#      -> wait-sync (eth_syncing) -> gateway/keycloak/backend -> gen-csr
```

`joinBundleRef` (`../../bundles/scenario-b/spoke-brazil/spoke-brazil.bundle.yaml`) resolves relative
to the manifest — the scp target above (spoke drop-zone). Bank **dataDir** is per bank id so
provisioning state never mixes with the CB spoke:
`deploy-lnet/bundles/scenario-b/cb1/` (or `cb2/`). The CSR (`<dataDir>/pki/cb1.csr`) is
generated locally and never transmitted; signing / on-chain registration are runtime steps handled via
the CB governance portal.

### 4 — VM 10.10.0.24 — Central Bank Colombia (found-spoke)

Same as step 2 with `cb-colombia`; emits `spoke-colombia/spoke-colombia.bundle.yaml`; scp it to `.25`
and `.26` (into `deploy-lnet/bundles/scenario-b/spoke-colombia/`).

### 5 — VMs 10.10.0.25 / 10.10.0.26 — cb3 / cb4 (join Colombia)

Same as step 3 with `cb3.yaml` / `cb4.yaml`.

All modes are idempotent (`<dataDir>/.provisioning-state.yaml` + flock); re-run `apply` to resume.
Use `--dry-run` to plan without side effects.

## Port map (per VM)

One entity per VM, so ports are reused across VMs. Scenario B ports end in **`845`**; Scenario A on a
shared VM ends in `645`, so both coexist.

| Service | Offset from RPC | Host port |
|---------|-----------------|-----------|
| Besu RPC | base | 8845 |
| Besu WS | — | 8846 |
| Besu P2P | — | 30304 |
| Postgres | +5000 | 13845 |
| Redis | +6000 | 14845 |
| Keycloak | +7000 | 15845 |
| API Gateway | +8000 | 16845 |
| Frontend (governance/bank) | +9000 | 17845 |
| NOC backend | +11000 | 19845 |
| NOC portal | +12000 | 20845 |
| Frontend treasury | +13000 | 21845 |
| Frontend supervisor | +14000 | 22845 |
| Launcher (shared A+B) | — | 5190 |
| Cacti relay (VM .20 only) | — | 7000 |

## Notes

- Distinct `chainId` per chain is enforced: hub 1337, spoke-brazil 2022, spoke-colombia 2025
  (banks inherit their spoke's chainId).
- `certSource`: CBs use `self-signed` (each is its own spoke CA); banks use
  `ca://central-bank-brazil` / `ca://central-bank-colombia`.
- `frontendHost` carries the launcher FQDN (used to build launcher portal links). The portal SPAs
  still bake `VITE_API_URL=http://localhost:<port>` — browsing from a remote machine hits the same
  local-first limitation as Scenario A.
- No private keys ever appear in these manifests (the validator rejects them). `change-me-local`
  passwords are lab placeholders — do not commit real credentials.
