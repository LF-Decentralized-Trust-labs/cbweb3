# CBWeb3 Reverse Proxy

A single HTTP entrypoint per **entity host** (VPS/VM), so every portal and API is reached
on **port 80** and a user switches portals by **changing only the URL path** — never the
port. Built on [Caddy](https://caddyserver.com/). Like the [launcher](../launcher/), it is
**standalone** and **generic**: no scenario code, no per-entity hosts or ports baked in.

## Routing model (path-based)

One hostname per entity (e.g. `cb-brazil.cbweb3.l-net.io`). The path selects the scenario
and portal:

```
/                 -> launcher (the neutral A/B landing page)
/a/governance/    -> Governance portal (Scenario A)
/a/treasury/      -> Treasury portal   (Scenario A)
/a/supervisor/    -> Supervisor portal (Scenario A)
/a/noc/           -> NOC portal        (Scenario A)
/a/api/           -> api-gateway       (Scenario A; only the /a prefix is stripped)
/b/governance/ …  -> Scenario B portals, same shape
```

Commercial banks expose only `/a/bank/` + `/b/bank/` (and their `/a/api/`, `/b/api/`).

Because every portal and the API share one origin, CORS collapses to a single origin and
auth cookies are same-origin.

## Configuration (runtime, not build-time)

The image ships only the base [`Caddyfile`](./Caddyfile), which `import`s per-scenario route
fragments from `/etc/caddy/conf.d/*.conf` and falls back to the launcher at the site root.
Each scenario's toolkit writes ONLY its own fragment (`caddy.a.conf` / `caddy.b.conf`) into a
shared, neutral conf dir on the host and bind-mounts it read-only — the same distributed,
one-per-host model the launcher uses. A missing scenario simply contributes no fragment.

A fragment lists one route block per portal the entity exposes, e.g.:

```
redir /a/governance /a/governance/
handle_path /a/governance/* {
	reverse_proxy cbweb3-central-bank-brazil-governance-frontend:80
}
handle /a/api/* {
	uri strip_prefix /a
	reverse_proxy cbweb3-central-bank-brazil-api-gateway:8080
}
```

`handle_path` strips the portal prefix so each SPA's nginx still serves at `/` (the SPA is
built base-path-aware, so its assets are requested under the prefix). The `/a/api` route
strips only `/a`, leaving `/api/v1/…` for the gateway.

### Automatic (via the toolkits)

Set `proxy: enable` in each scenario's manifest. On `apply`, that scenario's toolkit runs
this generic image (once per host, idempotent), connects it to the entity's Docker network so
it can reach the portal/api containers by name, writes its `caddy.<scenario>.conf` fragment,
and reloads Caddy. `disable` removes the fragment and tears the container down when no
fragment remains. Pre-build the image once (platform step): `./build.sh`.

## Build / run

```bash
cd proxy
./build.sh                                  # → cbweb3/proxy:local
docker run -d --name cbweb3-proxy --restart always \
  -p 80:80 \
  --add-host host.docker.internal:host-gateway \
  -v "$PROXY_STATE_DIR/conf.d:/etc/caddy/conf.d:ro" \
  cbweb3/proxy:local
```

## TLS

When `PROXY_SITE` is set to the entity host, the proxy serves **HTTPS on `:443`** with an
automatic `:80`→`:443` redirect. The certificate source is chosen by `PROXY_TLS_MODE` on the
deploy host (the toolkits set this and the matching env for you):

| `PROXY_TLS_MODE` | Certificate | Notes |
|---|---|---|
| `internal` (default) | Caddy local CA (self-signed) | Offline; browsers warn until the CA root is trusted. |
| `cloudflare` | Let's Encrypt via ACME **DNS-01** | **Publicly trusted on hosts with no inbound reachability.** Needs `CF_API_TOKEN` (scoped Cloudflare token) and outbound access to Let's Encrypt + the Cloudflare API. The image bundles the `caddy-dns/cloudflare` module (built by `build.sh` via `xcaddy`). |
| `acme` | Let's Encrypt via HTTP/TLS-ALPN | Requires the host reachable from the internet on `:80`/`:443`. |
| `custom` | Operator-provided cert | Mount a dir with `proxy.crt` + `proxy.key`; set `PROXY_CERT_DIR`. |
| `off` | none | Plain HTTP on `:80` (for an external TLS edge). |

The cert store (certs, keys, ACME account key) is persisted in the `cbweb3-proxy-data` volume.
Snapshot/restore it on the host with [`backup-certs.sh`](./backup-certs.sh) — useful before
wiping an environment, so certificates survive a token revocation or a redeploy.
