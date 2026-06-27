# CLI Contract: `cbweb3 apply`

**Phase 1 output** | **Branch**: `029-tk7-apply-command`

Este documento é o contrato externo do comando `cbweb3 apply` — a interface entre o toolkit e os operadores que o invocam (manualmente, em scripts ou em CI/CD pipelines). Mudanças breaking neste contrato requerem bump de versão do toolkit.

---

## Command signature

```
cbweb3 apply -f <manifest.yaml> [--dry-run | -n] [--output <format> | -o <format>]
```

### Binary name

`cbweb3` — built from `scenario-a/toolkit/cmd/cbweb3/main.go`

### Subcommand

`apply` — reconciles a spoke to the desired state described by the manifest.

---

## Flags

| Flag | Alias | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--file` | `-f` | string | **required** | Path to the ParticipantDeployment YAML manifest |
| `--dry-run` | `-n` | bool | `false` | Show what would be executed without running any step |
| `--output` | `-o` | string | `yaml` | Output format: `json` \| `yaml` |

### Flag rules

- `-f` / `--file` is **mandatory**. Omitting it causes exit 1 with `"required flag -f (--file) is missing"` on stderr before any manifest is read.
- `--output` values are case-sensitive. Any value other than `json` or `yaml` causes exit 1 with `"unknown output format: <value>"` before reading the manifest.
- `--dry-run` and `-n` are aliases for the same boolean flag; both may appear but the result is the same as one.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success — all steps completed or skipped; bundle emitted (mode: found) |
| `0` | Dry-run success — plan reported; no side effects |
| `1` | Manifest file not found or unreadable |
| `1` | YAML parse error in manifest |
| `1` | Schema validation error in manifest |
| `1` | Unsupported `mode` (only `found` is implemented; `join` → TK-9) |
| `1` | Unsupported `environment` (only `local` is implemented in TK-7) |
| `1` | Dependency resolution error (bad `keyProvider` URI, bad `certSource` URI) |
| `1` | Engine error (one or more steps failed) |
| `1` | Bundle emission error (engine succeeded but bundle could not be written) |
| `1` | Interrupted by SIGINT / SIGTERM (partial report emitted) |
| `1` | Invalid `--output` format |

---

## Stdout schema

Stdout always contains **valid JSON or YAML** (controlled by `--output`). The schema is the same for both formats; only serialization differs.

### Top-level object

```yaml
spoke:   <string>   # spec.spoke.id from the manifest
mode:    <string>   # "found" | "join"
dryRun:  <bool>     # true if --dry-run was passed
status:  <string>   # "success" | "failed" | "dry-run" | "interrupted"
error:   <string>   # optional; non-empty only when status is "failed" or "interrupted"
steps:   <array>    # always present; always 10 entries for mode:found
bundle:             # present only when status=="success" and mode=="found"
  path:  <string>   # relative path to the emitted bundle, e.g. "bundles/spoke-brl.bundle.yaml"
```

### Step object (`steps[]`)

```yaml
name:        <string>   # canonical step name (see table below)
status:      <string>   # "completed" | "skipped" | "failed" | "pending" | "interrupted"
completedAt: <string>   # RFC-3339 UTC; present when status is "completed" or "skipped"
error:       <string>   # present when status is "failed" or "interrupted"
```

### Canonical step names (mode: found, in execution order)

| # | Name | Description |
|---|------|-------------|
| 1 | `deploy-contracts` | Deploy IdentityRegistry, ZetoFactory, PenteFactory, FXAgreement |
| 2 | `gen-tls` | Generate or obtain TLS certificate (via certSource) |
| 3 | `render-configs` | Render Paladin config templates |
| 4 | `register-nodes` | Register Paladin nodes |
| 5 | `start-paladin` | Start Paladin and wait for health check |
| 6 | `create-zeto-token` | Create Zeto privacy token |
| 7 | `create-pente-context` | Create Pente private context |
| 8 | `deploy-fxa-pente` | Deploy FXAgreement inside Pente context |
| 9 | `onboard-registry` | Real onboarding: IdentityRegistry registration |
| 10 | `register-relay` | Register spoke with the Cacti relay |

---

## Stderr behavior

Stderr carries **human-readable** diagnostics only — not the structured report. It is safe to discard stderr in automated pipelines that consume stdout.

- Pre-execution errors (missing flag, bad format, manifest parse/validation): stderr only; stdout is empty; exit 1
- Engine errors and bundle errors: embedded in the structured stdout report (`steps[N].error` or top-level `error`); the error message may also appear in stderr as a prefix for interactive debugging
- Log output from `orchestrator.RunFound` (via `log/slog`) goes to stdout in the engine's own structured log format, **separate from the JSON/YAML report**

> **NOTE**: The structured report is the LAST thing written to stdout (after all engine log lines). Consumers parsing the report should read to EOF and then parse the last JSON/YAML object, or alternatively use `--output json` and rely on `jq` to extract the final object.

**Design note for automation**: To avoid mixing engine logs with the structured report, redirect engine logs to stderr by setting `CBWEB3_LOG_TARGET=stderr`. This keeps stdout as pure JSON/YAML for pipeline consumption. (TK-7 Phase 4 — not required for local/dev use.)

---

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CBWEB3_SCRIPTS_DIR` | Path to deploy/local/paladin/scripts/ | `<exDir>/../../deploy/local/paladin/scripts/` |
| `CBWEB3_COMPOSE_TEMPLATE` | Path to central-bank paladin-compose.yaml | `<exDir>/../../provisioning/templates/central-bank/paladin-compose.yaml` |
| `CBWEB3_PALADIN_CONFIG_DIR` | Path to paladin-config/ templates | `<exDir>/../../provisioning/templates/central-bank/paladin-config/` |
| `CBWEB3_PALADIN_CB_URL` | Paladin CB HTTP URL | `http://localhost:31648` |
| `CBWEB3_OUTPUT_DIR` | Directory for bundle output | Parent of `spec.node.dataDir` |

---

## Examples

### Provision a central-bank spoke (live)

```bash
cbweb3 apply -f manifests/central-bank-brl.yaml
```

**stdout** (YAML, truncated):
```yaml
spoke: spoke-brl
mode: found
dryRun: false
status: success
steps:
  - name: deploy-contracts
    status: completed
    completedAt: "2026-06-27T10:00:01Z"
  # ... 9 more steps ...
bundle:
  path: bundles/spoke-brl.bundle.yaml
```

### Inspect plan without executing

```bash
cbweb3 apply --dry-run -f manifests/central-bank-brl.yaml
```

**stdout** (YAML):
```yaml
spoke: spoke-brl
mode: found
dryRun: true
status: dry-run
steps:
  - name: deploy-contracts
    status: pending
  # ... 9 more steps ...
```

### Machine-readable JSON output for CI

```bash
cbweb3 apply -f manifests/central-bank-brl.yaml --output json | jq .status
# → "success"

cbweb3 apply -f manifests/central-bank-brl.yaml --output json | jq '.bundle.path'
# → "bundles/spoke-brl.bundle.yaml"
```

### Re-run on fully provisioned spoke (idempotent)

```bash
cbweb3 apply -f manifests/central-bank-brl.yaml
# All steps show status: skipped; bundle re-emitted; exit 0
```

---

## Versioning

This CLI contract is versioned together with the toolkit module (`github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit`). Breaking changes to flags, exit codes, or the stdout schema require a toolkit major version increment and a note in the scenario-a/README.md changelog.

**Current version**: v0 (pre-1.0; breaking changes permitted without major bump while in Fase 1B)
