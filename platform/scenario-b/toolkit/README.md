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

A single `kind: ParticipantDeployment` covers three modes, discriminated by
`spec.mode` + `spec.topology.role`:

| Mode          | Requires                          | Role             |
|---------------|-----------------------------------|------------------|
| `found-hub`   | `hub`                             | `hub`            |
| `found-spoke` | `spoke`, `hubBundleRef`           | `central-bank`   |
| `join`        | `spoke`, `joinBundleRef`, `bankId`| `commercial-bank`|

Secrets are never expressible in a manifest: key/cert material is referenced via
`keyProvider` (`kms://…`) and `certSource` (`self-signed` | `self-signed://…` |
`ca://…`). Only `environment: local` is supported in this phase.

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
