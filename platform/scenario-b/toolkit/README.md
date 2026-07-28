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
