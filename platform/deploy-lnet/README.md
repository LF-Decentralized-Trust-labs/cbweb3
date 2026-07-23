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
./deploy.sh b cb-brazil --dry-run     # VM .21  — Scenario B found-spoke (adds --hub-rpc), preview
./deploy.sh a cb1                     # VM .22  — Scenario A join
```

`deploy.sh` commands:

| Command | What it does |
|---------|--------------|
| `deploy.sh cacti [--down]` | VM .20 — start (or stop) **both** Cacti relays: Scenario A HTLC on `:4000` + Scenario B liquidity on `:7000`, each with a distinct `COMPOSE_PROJECT_NAME`. |
| `deploy.sh render` | Render every `*.yaml.tmpl` → `*.yaml` (no toolkit call). |
| `deploy.sh <a\|b> <target>` | Render, build the scenario's toolkit binary, and run `apply` with the right flags/cwd. `target` ∈ `hub` (B only), `cb-brazil`, `cb1`, `cb2`, `cb-colombia`, `cb3`, `cb4`. Extra flags (e.g. `--dry-run`) pass straight through. |

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

## Coexistence on a shared VM

Both scenarios run on the same hosts, so their ports are kept disjoint:


|                            | Scenario A                              | Scenario B   |
| -------------------------- | --------------------------------------- | ------------ |
| Besu RPC / WS              | 8645 / 8655                             | 8845 / 8846  |
| Besu P2P (host)            | **30303** (forced by the join localize) | **30304**    |
| Service ports              | suffix `645`                            | suffix `845` |
| Cacti (VM .20)             | :4000                                   | :7000        |
| Launcher (shared per host) | 5190                                    | 5190         |




## Reverse proxy — port-free, path-based portal access

Every entity manifest sets `proxy: enable`, so each VM runs one **Caddy** reverse proxy on
**port 80** (image `cbweb3/proxy:local`, built once per host by `deploy.sh`). Portals and the
api-gateway are then reached by **path — no port** — on the entity's `frontendHost`:

```
http://cb-brazil.cbweb3.l-net.io/              -> launcher (the A/B landing page)
http://cb-brazil.cbweb3.l-net.io/a/governance/ -> Governance  (Scenario A)
http://cb-brazil.cbweb3.l-net.io/a/treasury/   -> Treasury    (Scenario A)
http://cb-brazil.cbweb3.l-net.io/a/supervisor/ -> Supervisor  (Scenario A)
http://cb-brazil.cbweb3.l-net.io/b/governance/ -> Governance  (Scenario B)  … etc.
http://cb1-brazil.cbweb3.l-net.io/a/bank/      -> Bank portal (commercial bank, Scenario A)
http://hub.cbweb3.l-net.io/                    -> hub governance (root redirect; hub has no launcher)
```

Both scenarios of an entity share the one proxy container on that host: each scenario's toolkit
writes its own route fragment (`caddy.a.conf` / `caddy.b.conf`) into a shared conf dir and the proxy
attaches to both entity Docker networks. Requirements per VM:

- **DNS:** one `A` record per entity → its VM IP (e.g. `cb-brazil.cbweb3.l-net.io → 10.10.0.21`).
  No wildcard needed (single hostname per entity).
- **Firewall:** open `:80` and `:443`. The high per-portal host ports no longer need to be exposed
  externally (the proxy reaches each portal container on the internal network).
- **TLS:** enabled automatically whenever `frontendHost` is a real host (not `localhost`). The proxy
  serves **HTTPS on `:443`** (with an automatic `:80`→`:443` redirect), and the portal SPAs are built
  with `https://` api/CORS/launcher URLs so there is no mixed content. Certificate source is chosen by
  the `PROXY_TLS_MODE` env on the deploy host:
  - `internal` (**default**) — Caddy's local CA (self-signed). Works with no external reachability
    (suits a permissioned network); browsers warn until the CA root is trusted. Trust it with the
    root at `docker cp cbweb3-proxy:/data/caddy/pki/authorities/local/root.crt .`.
  - `acme` — automatic Let's Encrypt via the HTTP/TLS-ALPN challenge; requires the host reachable from
    the internet on `:80`/`:443`.
  - `cloudflare` — **publicly trusted Let's Encrypt on VPN-internal hosts.** Uses the ACME **DNS-01**
    challenge via Cloudflare, so the host needs no inbound reachability — only outbound access to
    Let's Encrypt and the Cloudflare API, plus a scoped API token. This is the LNET setup (the
    `l-net.io` zone is on Cloudflare). Certificates are publicly trusted, so **testers see no warning
    and configure nothing.** Set on the deploy host before apply:
    ```bash
    export PROXY_TLS_MODE=cloudflare
    export CF_API_TOKEN=<scoped Cloudflare token: Zone → DNS → Edit on l-net.io>
    deploy-lnet/deploy.sh a cb-brazil   # and b, per host
    ```
    The proxy image must include the Cloudflare DNS module — `proxy/build.sh` builds it in via
    `xcaddy` (rebuild the image if it predates this: `docker rmi cbweb3/proxy:local` then re-apply).
    Caddy renews automatically (~30 days before expiry) with no operator action. If the token is later
    revoked, existing certs keep serving until expiry as long as the `cbweb3-proxy-data` volume
    persists — see the cert backup note below.
  - `custom` — operator cert: set `PROXY_CERT_DIR=/path` (must contain `proxy.crt` + `proxy.key`).
  - `off` — no TLS; serve plain HTTP on `:80` (use when an external edge terminates TLS and forwards
    to `:80` — the SPAs are then built with `http://` URLs).
- **Test deployments without public certificates:** leave `PROXY_TLS_MODE` unset (→ `internal`,
  self-signed) or set it to `off`. No token or Cloudflare access is needed; the `cloudflare` mode is
  strictly opt-in, so nothing about a normal test deploy changes.
- **Backing up the certificates:** the cert store (issued certs, keys, and the ACME account key) lives
  in the `cbweb3-proxy-data` volume. Snapshot it to the VM's disk with
  `proxy/backup-certs.sh backup` (default target `/opt/cbweb3/proxy-cert-backups`), and restore with
  `proxy/backup-certs.sh restore <file.tgz>`. Keep a snapshot before wiping an environment: restoring
  it brings back valid certificates (and the same Let's Encrypt account) without needing a live token.
- **Enabling/switching TLS on an already-running proxy:** the proxy container is recreated to pick up
  `:443`, and each scenario re-attaches its own Docker network on apply — so after changing TLS,
  **re-apply both scenarios on that host** (e.g. `deploy.sh a cb-brazil` and `deploy.sh b cb-brazil`)
  so the proxy is attached to both entity networks. To force a clean recreate: `docker rm -f cbweb3-proxy`
  then re-apply.
- To turn the proxy off entirely, set `proxy: disable` in the manifest — the entity falls back to the
  legacy host-port URLs.

**NOC exception:** the NOC portal is hub-owned and stays **port-based** (its browser-side Keycloak
OIDC redirect URIs are unchanged); it is intentionally excluded from proxy path routing and from the
proxy-mode launcher for now.

## ⚠️ Multi-VM caveat (applies to both scenarios)

The toolkit only executes `environment: local`, a profile validated all-on-one-host. The **Besu/P2P**
layer is genuinely cross-VM (banks sync from their CB via the real IP in the enode), but **service→service
HTTP is not turnkey cross-VM**: bundles bake `localhost`/`host.docker.internal` and compose templates
reach hub/CB/relay via `host.docker.internal` (each VM's own host). The runbooks include the mandatory
bundle `sed` patches and flag the `host.docker.internal` container remap as an operator step. See each
scenario README for details.

> Compliance: the FQDNs and topology here are internal project details — do not share externally.
> The `change-me-local` passwords are lab placeholders; never commit real credentials.

