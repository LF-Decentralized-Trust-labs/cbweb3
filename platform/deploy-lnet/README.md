# `deploy-lnet` — LNET lab deployment (Scenario A + Scenario B)

Toolkit-driven install of **both** CBDC scenarios across the same 6-VM LNET lab. Shared VM `.20`
infrastructure (the Scenario-B hub and both Cacti relays) lives at this root; each scenario's
spoke/bank manifests live in its own subfolder.

```
deploy-lnet/
  addresses.env       # the per-VM IPs (edit here or override via exported env)
  render.sh           # substitute ${IP_*} markers: *.yaml.tmpl -> *.yaml
  deploy.sh           # render + run the toolkit for one target host
  bundles/            # hub/ + scenario-{a,b}/<spoke|bankId>/ (+ <spoke>.bundle.yaml)
  hub/manifests/      # VM .20 — Scenario B found-hub
  scenario-a/         # Enhanced Correspondent Banking (dual-layer HTLC) — VMs .21–.26
  scenario-b/         # International Hub spokes/banks — VMs .21–.26
```

Read the per-scenario runbooks for the full step-by-step:

- [scenario-a/README.md](scenario-a/README.md)
- [scenario-b/README.md](scenario-b/README.md)



## Addressing: templates + env substitution

Manifests are **templates** (`*.yaml.tmpl`) whose IPs are `${IP_*}` markers. The toolkit does not
expand env vars, so you render the templates to real `*.yaml` first, then apply. All addresses live in
one file — [addresses.env](addresses.env) — so re-pointing a VM is a one-line change:

```bash
# 1. Set the addresses: edit addresses.env, OR export overrides ad hoc:
export IP_CB_BRAZIL=10.20.0.31        # (optional) override just this VM

# 2a. Render only — produces the git-ignored *.yaml next to each *.yaml.tmpl:
./render.sh

# 2b. …or render AND run the toolkit for one target in a single step:
./deploy.sh cacti                     # VM .20  — both Cacti relays (A :4000 + B :7000)
./deploy.sh cacti --down              # VM .20  — tear both relays down
./deploy.sh b hub                     # VM .20  — Scenario B found-hub
./deploy.sh b noc-hub                 # VM .20  — Scenario B NOC portal for the hub (after b hub)
./deploy.sh b cb-brazil --dry-run     # VM .21  — Scenario B found-spoke (adds --hub-rpc), preview
./deploy.sh b noc-brazil              # VM .21  — Scenario B NOC portal for spoke-brazil (after b cb-brazil)
./deploy.sh a cb1                     # VM .22  — Scenario A join
```

`deploy.sh` commands:

| Command | What it does |
|---------|--------------|
| `deploy.sh cacti [--down]` | VM .20 — start (or stop) **both** Cacti relays: Scenario A HTLC on `:4000` + Scenario B liquidity on `:7000`, each with a distinct `COMPOSE_PROJECT_NAME`. |
| `deploy.sh render` | Render every `*.yaml.tmpl` → `*.yaml` (no toolkit call). |
| `deploy.sh <a\|b> <target>` | Render, build the scenario's toolkit binary, and run `apply` with the right flags/cwd. `target` ∈ `hub` (B only), `cb-brazil`, `cb1`, `cb2`, `cb-colombia`, `cb3`, `cb4`. Extra flags (e.g. `--dry-run`) pass straight through. |
| `deploy.sh b noc-<x>` | Bring up the NOC control plane (portal + backend) for one spoke: `noc-hub`, `noc-brazil`, `noc-colombia` (B only). Runs `observe` mode — skips launcher/contracts, no bundle emit. Run **after** that spoke's `found-*` on the same VM. |

The rendered `*.yaml` and the transferred bundles are git-ignored; the `*.yaml.tmpl` templates and
`addresses.env` are the tracked source of truth.

## VM map (both scenarios share the VMs)


| VM         | Shared / .20 infra                              | Scenario A          | Scenario B                | Launcher FQDN               |
| ---------- | ----------------------------------------------- | ------------------- | ------------------------- | --------------------------- |
| 10.10.0.20 | **hub (B) + Cacti A (:4000) + Cacti B (:7000)** | Cacti relay         | hub + Cacti relay         | —                           |
| 10.10.0.21 | —                                               | CB Brazil (found)   | CB Brazil (found-spoke)   | cb-brazil.cbweb3.l-net.io    |
| 10.10.0.22 | —                                               | cb1 (join)          | cb1 (join)                | cb1-brazil.cbweb3.l-net.io   |
| 10.10.0.23 | —                                               | cb2 (join)          | cb2 (join)                | cb2-brazil.cbweb3.l-net.io   |
| 10.10.0.24 | —                                               | CB Colombia (found) | CB Colombia (found-spoke) | cb-colombia.cbweb3.l-net.io  |
| 10.10.0.25 | —                                               | cb3 (join)          | cb3 (join)                | cb3-colombia.cbweb3.l-net.io |
| 10.10.0.26 | —                                               | cb4 (join)          | cb4 (join)                | cb4-colombia.cbweb3.l-net.io |




## VM 10.10.0.20 — shared infrastructure (hub + both Cacti relays)

This host carries the Scenario-B **hub** chain and **both** Cacti relays (Scenario A on `:4000`,
Scenario B on `:7000`). Bring these up before any spoke.

```bash
cd <repo>
# 1. Both Cacti relays (Scenario A HTLC :4000 + Scenario B liquidity :7000) in one step.
#    They coexist — distinct container_name / port / volume; the wrapper also gives each a
#    distinct COMPOSE_PROJECT_NAME so a --down of one never drops the other's network.
deploy-lnet/deploy.sh cacti
#    Tear both down with: deploy-lnet/deploy.sh cacti --down

# 2. Scenario-B hub (found-hub) — render + apply in one step
deploy-lnet/deploy.sh b hub
#   -> emits deploy-lnet/bundles/hub/hub.bundle.yaml
#   -> also emits deploy-lnet/bundles/hub.noc.bundle.yaml + starts the hub's noc-agent

# 3. NOC portal for the hub (observe) — brings up noc-backend :8090 + noc-portal :3030
deploy-lnet/deploy.sh b noc-hub
#   -> portal at http://${IP_HUB}:3030 ; registers the "hub" spoke + provisions the agent key
```

Both relays boot **neutral** (no fixed spokes); each founding CB self-registers its spoke at
`found`/`found-spoke` (`POST /api/v1/spokes`). The Scenario-B liquidity relay also has a
LiquidityCommitWatcher that reads the hub Besu RPC — its defaults (`HUB_BESU_RPC`,
`LIQUIDITY_COMMIT_REGISTRY_ADDRESS`, `GATEWAY_INTERNAL_URLS`) matter only once the **sovereign
BRL/COP pair** is opened (deferred). For the initial bring-up they can stay unset; wire them (via
`scenario-b/interop/hub-and-spoke/cacti/.env`, e.g. `HUB_BESU_RPC=http://host.docker.internal:8845`)
when you activate the corridor.

Patch the hub bundle's cross-host URLs, then `scp` `deploy-lnet/bundles/hub/hub.bundle.yaml` onto both CB
spoke VMs (see [scenario-b/README.md](scenario-b/README.md) step 1 for the exact `sed` + `scp`).
Scenario A has no hub — its `.20` role is only the Cacti relay.

## NOC observability (per spoke + hub)

The NOC is **individualized per spoke and per hub** (client requirement): each
gets its own portal + backend, co-located on that spoke's founding VM. There is
no central NOC.

| NOC deployment | VM | Monitors | Command (after `found-*`) |
|----------------|----|----------|---------------------------|
| `noc-hub`      | .20 (hub)          | the hub node                    | `deploy.sh b noc-hub`      |
| `noc-brazil`   | .21 (CB Brazil)    | spoke-brazil (CB + cb1 + cb2)   | `deploy.sh b noc-brazil`   |
| `noc-colombia` | .24 (CB Colombia)  | spoke-colombia (CB + cb3 + cb4) | `deploy.sh b noc-colombia` |

How it fits the flow:

- **`found-hub` / `found-spoke` already do the agent side automatically**: each
  emits a `*.noc.bundle.yaml` (relocated next to the main bundle — `bundles/hub/`
  for the hub, `bundles/scenario-b/<spoke>/` for a spoke, so it is scenario-
  scoped) and starts that entity's own `noc-agent`. No extra step for the CB/hub
  agent. The `observe` state also lives under `bundles/scenario-b/<noc-x>/`.
- **`deploy.sh b noc-<x>`** then stands up the portal + backend (`observe` mode)
  on the same VM and registers the spoke. Run it **after** the spoke's `found-*`
  on that VM. Portal: `http://<VM-IP>:3030`, backend: `:8090`.
- **Commercial banks need no NOC step**: `join` starts the bank's own `noc-agent`
  and it pushes to its CB's NOC via `spec.noc.backendURL` (already set in the
  `cb1`–`cb4` manifests → `http://${IP_CB_<region>}:8090`). Same-VM entities
  (the CB/hub) use the `host.docker.internal:8090` default, so they set no
  `backendURL`.

> Order per VM: `found-*` → `noc-*`. The agent that `found-*` started keeps
> retrying; once `noc-<x>` registers the spoke and provisions the key, its
> pushes authenticate (self-healing, non-blocking).

## Coexistence on a shared VM

Both scenarios run on the same hosts, so their ports are kept disjoint:


|                            | Scenario A                              | Scenario B   |
| -------------------------- | --------------------------------------- | ------------ |
| Besu RPC / WS              | 8645 / 8655                             | 8845 / 8846  |
| Besu P2P (host)            | **30303** (forced by the join localize) | **30304**    |
| Service ports              | suffix `645`                            | suffix `845` |
| Cacti (VM .20)             | :4000                                   | :7000        |
| NOC portal / backend       | —                                       | 3030 / 8090  |
| Launcher (shared per host) | 5190                                    | 5190         |




## ⚠️ Multi-VM caveat (applies to both scenarios)

The toolkit only executes `environment: local`, a profile validated all-on-one-host. The **Besu/P2P**
layer is genuinely cross-VM (banks sync from their CB via the real IP in the enode), but **service→service
HTTP is not turnkey cross-VM**: bundles bake `localhost`/`host.docker.internal` and compose templates
reach hub/CB/relay via `host.docker.internal` (each VM's own host). The runbooks include the mandatory
bundle `sed` patches and flag the `host.docker.internal` container remap as an operator step. See each
scenario README for details.

> Compliance: the FQDNs and topology here are internal project details — do not share externally.
> The `change-me-local` passwords are lab placeholders; never commit real credentials.

