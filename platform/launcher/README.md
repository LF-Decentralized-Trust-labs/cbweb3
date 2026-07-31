# CBWeb3 Launcher

A neutral, per-entity entry point that lists **this entity's own portals** (grouped by
Scenario A / Scenario B) and redirects to the chosen one. It is **standalone** — it
lives outside both `scenario-a/` and `scenario-b/` and imports no scenario code — so it
respects the platform's scenario-isolation rule.

## Distributed model

Each institution runs its stacks on its **own VPS** and gets its **own launcher**
deployed there. A launcher knows **only its own entity's portals** — there is no global
map of other institutions to maintain. Adding a new commercial bank or spoke means
standing up that VPS with its own launcher; **no existing launcher changes**, and no
third-party addresses are ever hardcoded.

- **Commercial bank** → `A → Bank Portal` · `B → Bank Portal` (login on the portal).
- **Central bank** → `A → Governance / Treasury / Supervisor / NOC` · `B → Governance /
  Treasury / Supervisor` (the NOC portal in Scenario B belongs to the hub).
- **Hub** (Scenario B only) → `Governance / NOC`.

```
VPS do Itaú                          VPS do CB Brasil
[launcher] ─ A ▶ bank portal A       [launcher] ─ A ▶ {governance,treasury,supervisor,noc} A
           └ B ▶ bank portal B                  └ B ▶ {governance,treasury,supervisor}    B
  (só conhece a si)                     (só conhece a si)
```

## Return path (portal → launcher)

Each scenario's portals close the loop back to here: their login screen shows a "back to
launcher" button and, on logout, the browser is redirected to the launcher. The launcher
URL is **not hardcoded** — each scenario's toolkit bakes it into the portal bundle at build
time as `VITE_LAUNCHER_URL` (`http://<frontendHost>:<launcherPort>`) whenever the entity has
`launcher: enable`. When it is absent (launcher disabled), the button is hidden and logout
falls back to the local `/login` route.

## Configuration (runtime, not build-time)

The image is **generic** — no URLs are baked in. Each entity's portal list is loaded at
runtime from **one fragment per scenario** under `configs/`, merged client-side:
`configs/config.a.json` (Scenario A) and `configs/config.b.json` (Scenario B). Each
scenario's toolkit writes ONLY its own fragment, so a missing scenario (404) simply
doesn't appear and no toolkit ever touches the other's file. A fragment looks like:

```json
{
  "entity": "Banco Itaú",
  "portals": [
    { "scenario": "A", "role": "bank", "label": "Bank Portal", "url": "http://localhost:25646" }
  ]
}
```

### Automatic (via the toolkits)

Set `launcher: enable` in each scenario's manifest for a **commercial bank or central
bank** (not the hub). On `apply`, that scenario's toolkit runs the generic launcher
image (once, idempotent) and writes its own `config.<scenario>.json` fragment into the
shared launcher config dir. `disable` removes the fragment and tears the container down
when no fragment remains. Pre-build the image once (platform step): `./build.sh`.

**One launcher per host** is the target model (each entity on its own VPS), so the
container name, port and config dir are keyed to the launcher port. Each entity
declares its port in its manifest as `spec.launcherPort` (distinct per entity, and the
SAME value in Scenario A and B so both scenarios share that entity's launcher). If the
manifest omits it, the toolkit falls back to the `LAUNCHER_PORT` env, then `5190`.
`LAUNCHER_STATE_DIR` overrides the config dir explicitly.

### Manual (`gen-config.sh`, for debugging)

Generate a single scenario fragment — ports are derived from the entity's Besu RPC port
by the toolkit's fixed frontend offsets (A: governance/bank +17000, treasury +18000,
supervisor +22000, noc +24000; B: governance/bank +9000, treasury +13000,
supervisor +14000, noc +12000):

```bash
./gen-config.sh --scenario a --entity "Banco Itaú" --role commercial-bank \
  --host localhost --rpc 8646 > configs/config.a.json

./gen-config.sh --scenario b --entity "Central Bank of Brazil" --role central-bank \
  --host 10.0.0.5 --rpc 8645 > configs/config.b.json
```

`--role` is `commercial-bank | central-bank | hub`; `--host` is the browser-facing host
of the entity's VPS; `--rpc` is the entity's Besu RPC port in that scenario.

## Develop / build / deploy

```bash
cd launcher
npm install
npm run dev            # http://localhost:5190 (serves public/configs/*.json)
npm run build          # generic static bundle in dist/

# Docker: build the generic image once, then mount the entity's fragment dir.
./build.sh                                  # → cbweb3/launcher:local
docker run -p 5190:80 \
  -v "$LAUNCHER_STATE_DIR/configs:/usr/share/nginx/html/configs:ro" \
  cbweb3/launcher:local
```

## Stack

React 19 + Vite, served by nginx. No dependency on either scenario's frontend packages
(`@cbweb3/ui` etc.) — that would couple the launcher to a scenario.
