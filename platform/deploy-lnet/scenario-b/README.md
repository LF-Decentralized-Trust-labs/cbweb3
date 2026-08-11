# Scenario B — LNET deployment

Toolkit-driven install of **Scenario B (International Hub — FXAgreement + AMM +
LiquidityCommitRegistry, relay + circuit breaker)** across the LNET 10-VM lab. One central **hub**
chain plus three sovereign spokes (Costa Rica, Chile, Peru); commercial banks join their spoke; a
Cacti relay federates cross-currency corridors.

The Scenario-B **hub** lives under [`../hub/`](../hub/) (`hub/manifests/`); shared bundles live under
[`../bundles/`](../bundles/) (`bundles/hub/`, `bundles/scenario-b/<spoke>/`, `bundles/scenario-b/<bankId>/`). Both Cacti relays are started from
the [deploy-lnet root](../README.md). This folder covers Scenario B's spokes and banks (VMs .21–.26, .30–.32).

## Topology

| VM | Role | Spoke | bank-id | Launcher FQDN | Manifest |
|----|------|-------|---------|---------------|----------|
| 10.10.0.20 | Hub (found-hub) + Cacti relay | hub (chainId 1337) | — | — | [../hub/manifests/hub.yaml](../hub/manifests/hub.yaml) |
| 10.10.0.21 | Central Bank Costa Rica (found-spoke) | spoke-costa-rica (CRC, chainId 2022) | — | cb-costa-rica.cbweb3.l-net.io | [manifests/cb-costa-rica.yaml](manifests/cb-costa-rica.yaml) |
| 10.10.0.22 | Commercial bank (join) | spoke-costa-rica | cb1 | cb1-costa-rica.cbweb3.l-net.io | [manifests/cb1.yaml](manifests/cb1.yaml) |
| 10.10.0.23 | Commercial bank (join) | spoke-costa-rica | cb2 | cb2-costa-rica.cbweb3.l-net.io | [manifests/cb2.yaml](manifests/cb2.yaml) |
| 10.10.0.24 | Central Bank Chile (found-spoke) | spoke-chile (CLP, chainId 2025) | — | cb-chile.cbweb3.l-net.io | [manifests/cb-chile.yaml](manifests/cb-chile.yaml) |
| 10.10.0.25 | Commercial bank (join) | spoke-chile | cb3 | cb3-chile.cbweb3.l-net.io | [manifests/cb3.yaml](manifests/cb3.yaml) |
| 10.10.0.26 | Commercial bank (join) | spoke-chile | cb4 | cb4-chile.cbweb3.l-net.io | [manifests/cb4.yaml](manifests/cb4.yaml) |
| 10.10.0.30 | Central Bank Peru (found-spoke) | spoke-peru (PEN, chainId 2028) | — | cb-peru.cbweb3.l-net.io | [manifests/cb-peru.yaml](manifests/cb-peru.yaml) |
| 10.10.0.31 | Commercial bank (join) | spoke-peru | cb5 | cb5-peru.cbweb3.l-net.io | [manifests/cb5.yaml](manifests/cb5.yaml) |
| 10.10.0.32 | Commercial bank (join) | spoke-peru | cb6 | cb6-peru.cbweb3.l-net.io | [manifests/cb6.yaml](manifests/cb6.yaml) |

The **sovereign cross-currency pairs (corridor + AMM liquidity)** are intentionally **not** provisioned
here. They are opened at runtime through the CB governance portals after the network is up — the
manifests say nothing about corridors, and the toolkit never opens one.

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

4. **Service-to-service authentication is per entity, and one half of it does not cross VMs.** The
   internal `/internal/*` routes now verify a per-entity signature instead of the shared
   `INTERNAL_RELAY_AUTH_SECRET` (see the runbook's item 9). Banks and central banks get their
   identities from onboarding and provisioning, which work fine here. The **Cacti relay** does not:
   the toolkit writes its certificate into each CB's PKI volume with a volume-to-volume copy
   (`pin-relay-cert`), and that only works when the relay and the CB share a Docker daemon. On LNET
   the relay lives on `.20` while the CBs are on `.21`/`.24`, so the step finds nothing and is skipped
   (it is soft on purpose). Consequence: those CBs cannot verify the relay. Harmless while
   enforcement is off — the relay falls back to the shared secret — but it must be done by hand before
   enabling it (see §6).

DNS: point each launcher FQDN at the matching VM IP.

## Manifests are templates

The hub manifest (`../hub/manifests/hub.yaml`) and `manifests/*.yaml` are **generated** from `*.yaml.tmpl` by
substituting the `${IP_*}` address markers from [`../addresses.env`](../addresses.env) (the toolkit
does not expand env vars). Render them first, or let the wrapper do render + apply in one step:

```bash
../deploy.sh render             # render all templates -> *.yaml
../deploy.sh b hub              # render + apply the hub (VM .20)
../deploy.sh b cb-costa-rica    # render + apply this found-spoke (adds --hub-rpc http://$IP_HUB:8845)
../deploy.sh b cb1              # render + apply this join
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
each bank (needs its spoke bundle). The three countries are independent; within a country the CB
found-spoke must precede its two bank joins.

### 1 — VM 10.10.0.20 — Cacti relay + hub (shared .20 infra)

```bash
cd $REPO
# Start both Cacti relays (Scenario A :4000 + Scenario B :7000) in one step:
deploy-lnet/deploy.sh cacti
#   (manual, Scenario-B relay only: scenario-b/provisioning/scripts/start-cacti.sh  # :7000)

# Found the hub (render + apply):
deploy-lnet/deploy.sh b hub
#   -> emits deploy-lnet/bundles/hub/hub.bundle.yaml

# REQUIRED: the relay was started BEFORE the hub (it is a hard prerequisite of
# register-relay-spoke), so it booted before found-hub wrote its signing identity — and a
# signer is read once, at construction. Without this restart the relay forwards the
# bridge-out leg unsigned, which the central banks reject once enforcement is on.
docker restart cbweb3-cacti-liquidity-relay
docker logs cbweb3-cacti-liquidity-relay 2>&1 | grep 'signature enabled'
#   [CrossCurrencySwapRelay] per-entity signature enabled (key-id=cacti-relay)
```

Patch the hub bundle's cross-host URLs (emitted as `localhost`), then copy into
`deploy-lnet/bundles/hub/` on every CB spoke VM:

```bash
sed -i 's#http://localhost:8845#http://10.10.0.20:8845#; \
        s#ws://localhost:8846#ws://10.10.0.20:8846#; \
        s#http://localhost:16845#http://10.10.0.20:16845#' \
  $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml

scp $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml op@10.10.0.21:$REPO/deploy-lnet/bundles/hub/
scp $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml op@10.10.0.24:$REPO/deploy-lnet/bundles/hub/
scp $REPO/deploy-lnet/bundles/hub/hub.bundle.yaml op@10.10.0.30:$REPO/deploy-lnet/bundles/hub/
```

### 2 — VM 10.10.0.21 — Central Bank Costa Rica (found-spoke)

```bash
# Preferred: render + apply via the LNET wrapper (creates dataDir under bundles/)
./deploy-lnet/deploy.sh b cb-costa-rica --dry-run   # preview
./deploy-lnet/deploy.sh b cb-costa-rica
# dataDir:  deploy-lnet/bundles/scenario-b/spoke-costa-rica/
# Emits → relocates: deploy-lnet/bundles/scenario-b/spoke-costa-rica/spoke-costa-rica.bundle.yaml
# (--hub-rpc is added automatically by deploy.sh)
```

> **Costa Rica testing team.** The Costa Rica CB manifest provisions the Banco Central de Costa Rica
> (BCCR) operators — each granted the full set of central-bank roles (GOVERNANCE + TREASURY +
> SUPERVISOR + NOC) — plus one SUGEVAL (securities regulator) SUPERVISOR account, in addition to the
> baseline lab operator accounts. The same username repeated across role entries is seeded once in
> Keycloak and granted every role. Passwords are `change-me-local` placeholders; rotate them to unique
> per-user passwords out of band (Keycloak) before real use — never commit real credentials.

The spoke manifest reads the hub bundle via `hubBundleRef: ../../bundles/hub/hub.bundle.yaml`
(the central drop-zone under `bundles/hub/`). With a current toolkit, `emit-spoke-bundle` already
bakes `node.advertisedHost` into `spokeRpc` / `spokeWs` / `cbGateway` — no `sed` needed for those.
Copy the bundle to the bank VMs:

```bash
scp $REPO/deploy-lnet/bundles/scenario-b/spoke-costa-rica/spoke-costa-rica.bundle.yaml \
    op@10.10.0.22:$REPO/deploy-lnet/bundles/scenario-b/spoke-costa-rica/
scp $REPO/deploy-lnet/bundles/scenario-b/spoke-costa-rica/spoke-costa-rica.bundle.yaml \
    op@10.10.0.23:$REPO/deploy-lnet/bundles/scenario-b/spoke-costa-rica/
```

### 3 — VMs 10.10.0.22 / 10.10.0.23 — cb1 / cb2 (join Costa Rica)

```bash
./deploy-lnet/deploy.sh b cb1     # .22 ; use `b cb2` on .23
#   -> write-genesis (from bundle) -> start-besu-join (--bootnodes=<CB>:30304)
#      -> wait-sync (eth_syncing) -> gateway/keycloak/backend -> gen-csr
```

`joinBundleRef` (`../../bundles/scenario-b/spoke-costa-rica/spoke-costa-rica.bundle.yaml`) resolves
relative to the manifest — the scp target above (spoke drop-zone). Bank **dataDir** is per bank id so
provisioning state never mixes with the CB spoke:
`deploy-lnet/bundles/scenario-b/cb1/` (or `cb2/`). The CSR (`<dataDir>/pki/cb1.csr`) is
generated locally and never transmitted; signing / on-chain registration are runtime steps handled via
the CB governance portal.

### 4 — VM 10.10.0.24 — Central Bank Chile (found-spoke)

Same as step 2 with `cb-chile`; emits `spoke-chile/spoke-chile.bundle.yaml`; scp it to `.25`
and `.26` (into `deploy-lnet/bundles/scenario-b/spoke-chile/`).

### 5 — VMs 10.10.0.25 / 10.10.0.26 — cb3 / cb4 (join Chile)

Same as step 3 with `cb3.yaml` / `cb4.yaml`.

### 6 — VM 10.10.0.30 — Central Bank Peru (found-spoke)

Same as step 2 with `cb-peru`; emits `spoke-peru/spoke-peru.bundle.yaml`; scp it to `.31`
and `.32` (into `deploy-lnet/bundles/scenario-b/spoke-peru/`).

### 7 — VMs 10.10.0.31 / 10.10.0.32 — cb5 / cb6 (join Peru)

Same as step 3 with `cb5.yaml` / `cb6.yaml`.

All modes are idempotent (`<dataDir>/.provisioning-state.yaml` + flock); re-run `apply` to resume.
Use `--dry-run` to plan without side effects.

### 6 — Optional: enforce per-entity signatures on the internal routes

The `/internal/*` routes accept either a per-entity signature or the shared
`INTERNAL_RELAY_AUTH_SECRET` — a secret identical in every entity, so it proves that *some* entity is
calling and never *which*. Enforcement stops accepting it. It is **off by default** and is an operator
decision per central bank; the full description is in the runbook's "service-to-service authentication"
item.

On LNET one prerequisite is not automatic: the relay's certificate must reach each CB's PKI volume,
which the toolkit only manages when they share a Docker daemon (see seam 4). Copy it by hand, in the
same spirit as the hub-bundle `scp` above — the certificate is public, the private key stays on `.20`:

```bash
# On .20 — extract the relay's certificate (NOT its key) from the relay's volume:
docker run --rm -v cbweb3-relay_data:/t:ro alpine:3.20 cat /t/cacti-relay.crt > /tmp/cacti-relay.crt
scp /tmp/cacti-relay.crt op@10.10.0.21:/tmp/
scp /tmp/cacti-relay.crt op@10.10.0.24:/tmp/

# On each CB VM (.21 and .24) — pin it in that CB's PKI volume. <prefix> is the entity's
# volume prefix (e.g. spoke-brl_central-bank); `docker volume ls | grep cb_tls` finds it.
docker run --rm -v <prefix>_cb_tls:/t -v /tmp:/in:ro alpine:3.20 \
  sh -c 'cp /in/cacti-relay.crt /t/cacti-relay.crt && chmod 0644 /t/cacti-relay.crt'
```

Then verify on each CB that every peer it must authenticate is pinned — its onboarded banks, the
relay, and itself — before turning enforcement on:

```bash
docker logs <cb-gateway> 2>&1 | grep '\[relay-auth\] registry refreshed'
#   [relay-auth] registry refreshed: N peer key(s) pinned [bank-… cacti-relay <cb-name>]
```

Enable it by exporting `RELAY_REQUIRE_SIGNATURE=true` before `apply` on the **central bank** VMs only
— never on a bank VM. Enforcement is a receiver-side setting and a bank hosts no internal routes; the
toolkit forces the variable empty on `join` for that reason, but exporting it in a shell that also
runs a join is a habit worth avoiding. A gateway with the flag set and no pinned peer **refuses to
start** rather than answering 401 to every internal request, so a mistake here is loud, not silent.

Prove it took effect — this is the check that shows behaviour rather than configuration:

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$CB_GW/internal/amm/cross-currency-bridge-in" \
  -H 'Content-Type: application/json' -H "X-Relay-Auth: $INTERNAL_RELAY_AUTH_SECRET" -d '{}'
#   401   (the shared secret alone is no longer accepted)
```

To roll back, unset the variable and re-apply: the receiver accepts the secret again and senders keep
signing harmlessly.

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

- Distinct `chainId` per chain is enforced: hub 1337, spoke-costa-rica 2022, spoke-chile 2025,
  spoke-peru 2028 (banks inherit their spoke's chainId).
- `certSource`: CBs use `self-signed` (each is its own spoke CA); banks use
  `ca://central-bank-costa-rica` / `ca://central-bank-chile` / `ca://central-bank-peru`.
- `frontendHost` carries the launcher FQDN (used to build launcher portal links). The portal SPAs
  still bake `VITE_API_URL=http://localhost:<port>` — browsing from a remote machine hits the same
  local-first limitation as Scenario A.
- No private keys ever appear in these manifests (the validator rejects them). `change-me-local`
  passwords are lab placeholders — do not commit real credentials.
