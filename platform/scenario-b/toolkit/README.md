# cbweb3b — Scenario B provisioning toolkit

The declarative entry point of the Scenario B toolkit (TK-B1). It parses,
validates, and reports on `ParticipantDeployment` manifests (`apiVersion:
cbweb3b/v1`). This phase does **not** run steps, emit bundles, render compose
templates, or execute anything — later phases (TK-B2+) consume this model.

Per the project constitution (Principle I, scenario-scoped independence) this is
an independent reimplementation of the Scenario A manifest/validation pattern;
it does not import `scenario-a/toolkit`. The only external dependency is
`gopkg.in/yaml.v3`.

## Layout

```
toolkit/
├── cmd/cbweb3b/        CLI (validate / apply --dry-run)
└── engine/manifest/    types, parse, result, validation, collision checks
```

The publishable JSON-Schema lives outside the Go module at
`scenario-b/provisioning/schema/v1/participant-deployment.schema.yaml`. The Go
validator is the source of truth; the schema is the editor/CI contract. The two
are kept in parity by `engine/manifest/schema_parity_test.go` (SC-004).

## Manifest modes

A single `kind: ParticipantDeployment` covers four modes, discriminated by
`spec.mode` + `spec.topology.role`:

| Mode          | Requires                          | Role             |
|---------------|-----------------------------------|------------------|
| `found-hub`   | `hub`                             | `hub`            |
| `found-spoke` | `spoke`, `hubBundleRef`           | `central-bank`   |
| `join`        | `spoke`, `joinBundleRef`, `bankId`| `commercial-bank`|
| `observe`     | `nocBundleRef`                    | `noc`            |

Secrets are never expressible in a manifest: key/cert material is referenced via
`keyProvider` (`kms://…`) and `certSource` (`self-signed` | `self-signed://…` |
`ca://…`). Only `environment: local` is supported in this phase. The `observe`
mode stands up an observability control plane rather than an on-chain node, so
`node`/`keyProvider`/`certSource`/`relay` are not required for it.

## Sovereign currency and token metadata

Each spoke declares its own currency in the manifest; nothing about a currency is
hardcoded in the toolkit:

```yaml
spoke:
  id: spoke-colombia
  chainId: 2025
  currency: COP              # ISO 4217 — the routing key
  tokenName: Tokenized COP   # optional overrides (derived when absent)
  tokenSymbol: tCeBM_COP
  fiatTokenName: Fiat COP
  fiatTokenSymbol: fCeBM_COP
```

- `currency` is the only **routing** key: the relay is registered as
  `spoke-<currency>`, the hub registers the currency and mirrors it as
  `W-tCeBM_<ISO>`, and the api-gateway derives `spoke_in`/`spoke_out` from it. A
  token symbol can never override it.
- The four token fields are **presentation** metadata for this spoke's own
  `tCeBM`/`fCeBM` ERC-20s. Absent, they derive from `currency` as
  `Tokenized <ISO>` / `tCeBM_<ISO>` and `Fiat <ISO>` / `fCeBM_<ISO>`.
- Symbols must keep the `<prefix>_<ISO>` shape and end in the spoke's own
  currency (validated). The displayed currency code is read from the segment
  after the last underscore, in the api-gateway (`currencyCodeFromSymbol`) and in
  every portal (`currencyFromTokenSymbol`); a symbol without it degrades the UI
  to the generic "fiat units" label.
- `found-spoke` deploys the tokens with these values and publishes
  `currency`/`tokenSymbol`/`fiatTokenSymbol` in the spoke bundle. `join` rejects a
  bank manifest whose `spoke.currency` differs from the bundle's, and warns that
  token overrides in a `join` manifest are inert (only the founding CB deploys
  tokens).

## NOC observability (`observe` mode)

The NOC (Network Operations Center) is modelled as a first-class participant,
mirroring how a commercial bank consumes a bundle from its central bank:

- **found-hub / found-spoke emit a NOC bundle** — a public, no-secrets artifact
  (`<spokeId>.noc.bundle.yaml`) describing that entity's monitoring topology:
  the deterministic spoke UUID, currency/jurisdiction, and the components to
  probe (Besu, and the Cacti relay when configured).
- **`observe` (`noc-<name>.yaml`) consumes it** and stands up the NOC control
  plane — Postgres + `noc-backend` + `noc-portal` (its own network, not an
  entity's) — then registers the spoke and provisions the founding agent's key
  against the backend admin API.
- **Each entity runs its own `noc-agent`** (the CB and hub in found-*, each bank
  in `join`), which renders a multi-component `agent.yaml`, joins that entity's
  network to probe Besu by container DNS, and pushes health to the NOC backend.
  This gives **node-level** monitoring: a bank node outage is visible even while
  the shared chain keeps producing blocks.

Identifiers are derived deterministically from public ids, so participants agree
without exchanging state: the spoke UUID is `uuidv5(spokeId)`, and each entity's
agent key is `sha256(spokeId, entity)` — the CB/hub use the fixed `cb` label
(provisioned by `observe`); each bank self-provisions its own key
(`spokeId, bankId`) at join time. All agents of one spoke push under the same
spoke UUID (the backend keys components by `(agent_id, name)`), so the portal
shows the CB node plus every bank node under one spoke.

> Local only: the toolkit points agents at a `noc-backend` running with
> `NOC_SKIP_AUTH=true`, whose admin API accepts any bearer. Production NOC
> backends (real Keycloak realm + out-of-band agent keys) are out of scope here.
> The portal's Keycloak login is likewise a follow-up.

### NOC configuration worth knowing

| Field | Effect |
|---|---|
| `spec.noc.keycloakURL` | Realm the portal password-grants against (baked as `VITE_KEYCLOAK_URL`). |
| `spec.noc.launcherURL` | "Back to launcher" target (baked as `VITE_LAUNCHER_URL`). |
| `spec.noc.ammGatewayURL` | api-gateway the **backend** reads AMM pool status from, for Pool Stability. |
| `spec.noc.backendURL` | Where this entity's agent pushes (default `host.docker.internal:8090`). |
| `spec.relay.containerName` | Relay container on this host, so the agent can collect its **logs**. |

Two of these are easy to get wrong:

- **`ammGatewayURL` is resolved from inside the NOC container.** The NOC stack owns
  its own docker network, so another entity's compose service name does not resolve
  there — use the host and the gateway's published port
  (`http://host.docker.internal:<rpcPort + 8000>`). The backend then *discovers* the
  active pairs from that gateway, so no pair list has to be configured; `AMM_PAIRS`
  remains available as an env-only pin. With the field unset, Pool Stability stays
  empty and the page reports that the gateway could not be read.
- **`relay.containerName` is what makes relay logs work.** Component log collection
  is done by the agent over the docker socket, so a component registered without a
  container name always reports "No logs available." in the portal. The relay itself
  is never touched — the name is only read.

### Redeploying after a code change (`--rebuild`)

Image tags encode the **baked build args**, not the source tree. So after editing a
service or portal, a plain `apply` finds the tag already present, reports every step
as `done`/`skipped`, and leaves the previous binary running. Use:

```bash
go run ./cmd/cbweb3b apply -f <manifest.yaml> --repo-root <repo> --rebuild
```

`--rebuild` forces this entity's image builds *and* the steps that bring its
containers up (compose then recreates them on the new image id), including the
readiness gate that follows — a recreated backend is not ready yet, and the steps
after it would otherwise race it. It does not touch chain state, contracts, PKI or
genesis.

## Usage

Validate a single manifest:

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b validate -f engine/manifest/testdata/found-hub.yaml
# → valid: true (exit 0)
```

Validate a set (enables cross-manifest collision checks: chainId, declared
ports, founding spoke.id, metadata.name):

```bash
go run ./cmd/cbweb3b validate \
  -f engine/manifest/testdata/found-spoke.yaml \
  -f engine/manifest/testdata/join.yaml \
  -o json
```

`apply --dry-run` is equivalent to `validate` in this phase; `apply` without
`--dry-run` is not yet implemented and exits non-zero.

### Flags

- `-f`, `--file` — manifest path (repeatable; multiple files enable collision checks). Required.
- `-o`, `--output` — report format `json` | `yaml` (default `yaml`).
- `--dry-run` — (apply) validate only; required in this phase.

### Exit codes

- `0` — all manifests valid (warnings allowed, e.g. `join` with `validator: true`).
- `1` — one or more validation errors (report is still emitted).
- `2` — usage/parse error (missing file, malformed YAML, invalid flags).

## Report shape

```yaml
manifests:
  - file: <path>
    name: <metadata.name>
    mode: <spec.mode>
    valid: <bool>
    errors:   [{ field, message }]
    warnings: [{ field, message }]
setErrors:    [{ field, message }]   # cross-manifest collisions (only when >1 file)
valid: <bool>
```

All findings are collected in a single pass — validation never stops at the
first violation (FR-011).

## Development

```bash
gofmt -l .        # must be empty
go vet ./...
go test ./...
```
