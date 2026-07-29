# Contracts — TK-B6 (CLI `apply` + API do motor/bundle/executor)

Fase 1. Contratos da CLI e das APIs Go internas.

## CLI — `cbweb3b apply`

```
cbweb3b apply -f <manifest> [--dry-run] [-o json|yaml] [--data-dir <dir>] [--out-dir <dir>]
```

| Flag | Efeito |
|---|---|
| `-f <manifest>` | manifesto (obrigatório); modo lido do `kind`/`spec.mode` |
| `--dry-run` | planeja sem efeitos externos (report com status `planned`) |
| `-o json\|yaml` | formato do report (default: yaml) |
| `--data-dir` | raiz do estado (`.provisioning-state.yaml`, `.provisioning.lock`) |
| `--out-dir` | destino do bundle (`bundles/hub.bundle.yaml`) |

**Comportamento**:
- Modo `found-hub` → executa o inventário; outro modo → erro "mode not supported yet"
  (found-spoke/join = TK-B7/B8).
- Report **sempre** em stdout. `SIGINT`/`SIGTERM` → report **parcial** + saída ≠ 0.
- Manifesto inválido → erro claro **antes** de qualquer efeito (exit ≠ 0).

**Exit codes**: `0` sucesso (todos done/skipped); `1` erro de validação/config; `2` falha de step
(report inclui o step falho).

## Go — `engine/orchestrator`

```go
type Step struct {
    Name  string
    Deps  []string
    Check func(ctx context.Context) (bool, error)
    Run   func(ctx context.Context) error
}

type Status string // "done" | "skipped" | "failed" | "planned"

type Orchestrator struct { /* steps, state, lock, runner, dryRun */ }

func New(steps []Step, st *State, lk *Lock, dryRun bool) *Orchestrator
func (o *Orchestrator) Run(ctx context.Context) (Report, error) // topo-order; Check→skip; persists
```

## Go — `engine/exec`

```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
// real: os/exec ; fake: records calls (tests) ; dry: no-op recording plan
```

## Go — `engine/bundle`

```go
type HubBundle struct {
    Version   string
    ChainID   uint64
    HubRPC    string
    HubWS     string
    Contracts map[string]string // name -> 0x-address
}
func EmitHub(b HubBundle, outDir string) (path string, err error)
func LoadHub(path string) (HubBundle, error)
func ValidateHub(b HubBundle) error // required fields; rejects any private-key-looking material
```

## Go — `engine/apply`

```go
func Apply(ctx context.Context, opts Options) (Report, error) // dispatch by mode; dry-run; report
type Options struct { ManifestPath, DataDir, OutDir, Format string; DryRun bool }
```

## Invariantes de contrato

- `Run` idempotente: `Check` true ⇒ `skipped`; re-run converge.
- `--dry-run` ⇒ nenhuma chamada à impl. real do `CommandRunner`.
- `ValidateHub` rejeita bundle com material que aparente chave privada (sem segredos).
- Report distingue `done`/`skipped`/`failed`/`planned`; nunca erro silencioso.
