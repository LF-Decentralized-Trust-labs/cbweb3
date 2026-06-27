# Data Model: TK-7 — Comando `apply`

**Phase 1 output** | **Branch**: `029-tk7-apply-command`

---

## Tipos Go — pacote `engine/apply`

### `ApplyInput`

Input para `apply.Run` e `apply.DryRun`. Centraliza tudo o que vem do parsing de flags + manifesto.

```go
// ApplyInput carries everything resolved from CLI flags and the parsed manifest.
// It is the boundary object between cmd/cbweb3/main.go and engine/apply.
type ApplyInput struct {
    Manifest   *manifest.Manifest // parsed + validated; never nil
    DryRun     bool               // --dry-run flag
    OutputFmt  string             // "yaml" | "json"; default "yaml"
    OutputDir  string             // where to write bundles/<spoke-id>.bundle.yaml
    BesuRPCURL string             // http://localhost:<rpc.port> (local profile)
}
```

### `ApplyResult`

Relatório estruturado emitido em stdout. Construído em memória antes de qualquer escrita — garante que stdout é sempre JSON/YAML válido mesmo em falha.

```go
// ApplyResult is the structured execution report written to stdout.
// It is always built (possibly partially) before any stdout write.
type ApplyResult struct {
    Spoke  string       `json:"spoke"  yaml:"spoke"`   // spec.spoke.id
    Mode   string       `json:"mode"   yaml:"mode"`    // "found" | "join"
    DryRun bool         `json:"dryRun" yaml:"dryRun"`
    Status string       `json:"status" yaml:"status"`  // "success" | "failed" | "dry-run" | "interrupted"
    Steps  []StepResult `json:"steps"  yaml:"steps"`
    Bundle *BundleRef   `json:"bundle,omitempty" yaml:"bundle,omitempty"` // nil when dry-run or failure
    Error  string       `json:"error,omitempty"  yaml:"error,omitempty"`  // top-level error; empty on success
}
```

**Invariantes**:
- `Status == "success"` → `Error == ""` e `Bundle != nil` (para `mode: found`)
- `Status == "dry-run"` → `Bundle == nil`, `Error == ""`, todos os steps têm `status` `"pending"` ou `"skipped"`
- `Status == "failed"` ou `"interrupted"` → pelo menos um step com `status: "failed"` ou `"interrupted"` OU `Error != ""`
- `Steps` NUNCA está vazio — contém sempre os 10 steps canônicos (mesmo em dry-run de spoke novo)

### `StepResult`

Resultado por passo. Status reflete o que ocorreu (execução live) ou o que ocorreria (dry-run).

```go
// StepResult records the outcome (live) or planned action (dry-run) for one step.
type StepResult struct {
    Name        string `json:"name"                  yaml:"name"`
    Status      string `json:"status"                yaml:"status"`       // ver tabela abaixo
    CompletedAt string `json:"completedAt,omitempty" yaml:"completedAt,omitempty"` // RFC-3339 UTC
    Error       string `json:"error,omitempty"       yaml:"error,omitempty"`
}
```

**Valores válidos de `Status`**:

| Valor | Contexto | Semântica |
|---|---|---|
| `"completed"` | live | Passo executou e concluiu com sucesso nesta invocação |
| `"skipped"` | live / dry-run | Passo já estava concluído (`.provisioning-state.yaml` mostra `done`) |
| `"failed"` | live | Passo foi executado e retornou erro |
| `"pending"` | dry-run | Passo ainda não concluído — seria executado em live |
| `"interrupted"` | live | Contexto cancelado enquanto o passo estava em andamento |

### `BundleRef`

Referência ao bundle emitido após `mode: found` bem-sucedido.

```go
// BundleRef points to the emitted join bundle artifact.
type BundleRef struct {
    Path string `json:"path" yaml:"path"` // e.g. "bundles/spoke-brl.bundle.yaml"
}
```

### `LocalProfile`

Conjunto de defaults para `environment: local`. Não é serializado — usado apenas internamente por `deps.go` e `profile.go`.

```go
// LocalProfile holds the resolved runtime defaults for environment: local.
// Fields come from env vars with binary-relative fallbacks.
type LocalProfile struct {
    BesuRPCURL          string // http://localhost:<rpc.port>
    PaladinCBURL        string // CBWEB3_PALADIN_CB_URL | http://localhost:31648
    ScriptsDir          string // CBWEB3_SCRIPTS_DIR | <exDir>/../../deploy/local/paladin/scripts/
    ComposeTemplatePath string // CBWEB3_COMPOSE_TEMPLATE | <exDir>/../../provisioning/templates/central-bank/paladin-compose.yaml
    PaladinConfigDir    string // CBWEB3_PALADIN_CONFIG_DIR | <exDir>/../../provisioning/templates/central-bank/paladin-config/
    OutputDir           string // CBWEB3_OUTPUT_DIR | <dataDir>/../ (pai do dataDir do spoke)
}
```

---

## Funções públicas — pacote `engine/apply`

```go
// Run provisions the spoke described by in.Manifest.
// Calls orchestrator.RunFound, then bundle.EmitBundle.
// Returns a fully populated ApplyResult even on error (partial state captured).
func Run(ctx context.Context, in ApplyInput) (ApplyResult, error)

// DryRun reads current provisioning state and returns a plan report.
// No side effects: no files written, no engine called, no Besu accessed.
func DryRun(ctx context.Context, in ApplyInput) (ApplyResult, error)

// ResolveDeps constructs orchestrator.Deps from the manifest and local profile.
// Returns error for unsupported environment, invalid keyProvider/certSource URIs.
func ResolveDeps(m *manifest.Manifest) (orchestrator.Deps, error)
```

---

## Modificações no pacote `orchestrator`

Dois identificadores exportados (atualmente unexported):

```go
// orchestrator/state.go — rename loadState → LoadState
func LoadState(dir string) (ProvisioningState, error) { ... }

// orchestrator/step.go — rename canonicalStepOrder → CanonicalStepOrder
var CanonicalStepOrder = []string{
    StepDeployContracts,
    StepGenTLS,
    StepRenderConfigs,
    StepRegisterNodes,
    StepStartPaladin,
    StepCreateZetoToken,
    StepCreatePente,
    StepDeployFXAPente,
    StepOnboardRegistry,
    StepRegisterRelay,
}
```

Chamadores internos de `loadState` (no mesmo pacote `orchestrator`) são atualizados para `LoadState`. Zero impacto externo.

---

## Transições de estado — execução live (`Run`)

```
                   ┌─────────────────────────────────────┐
                   │          apply.Run(ctx, in)          │
                   └─────────────────┬───────────────────┘
                                     │
                    ┌────────────────▼────────────────┐
                    │  manifest.Load + manifest.Validate │
                    └────────────────┬────────────────┘
                         error? ─────┤──────────► exit 1 (stderr + no stdout report)
                                     │ ok
                    ┌────────────────▼────────────────┐
                    │         ResolveDeps(m)           │
                    └────────────────┬────────────────┘
                         error? ─────┤──────────► ApplyResult{status:"failed"} → exit 1
                                     │ ok
                    ┌────────────────▼────────────────┐
                    │   orchestrator.RunFound(ctx,m,d) │
                    │   (10 steps, idempotent)         │
                    └────────────────┬────────────────┘
                    ctx.Done? ───────┤──────────► ApplyResult{status:"interrupted"} → exit 1
                    step error? ─────┤──────────► ApplyResult{status:"failed"} → exit 1
                                     │ ok (all steps done/skipped)
                    ┌────────────────▼────────────────┐
                    │      bundle.EmitBundle(ctx, in)  │
                    └────────────────┬────────────────┘
                    bundle error? ───┤──────────► ApplyResult{status:"failed", bundleError} → exit 1
                                     │ ok
                    ┌────────────────▼────────────────┐
                    │  ApplyResult{status:"success"}   │
                    │  + Bundle{path: "bundles/..."}   │
                    └────────────────┬────────────────┘
                                     │
                                  exit 0
```

## Transições de estado — dry-run (`DryRun`)

```
                   ┌─────────────────────────────────────┐
                   │         apply.DryRun(ctx, in)        │
                   └─────────────────┬───────────────────┘
                                     │
                    ┌────────────────▼────────────────┐
                    │  manifest.Load + Validate        │
                    └────────────────┬────────────────┘
                         error? ─────┤──────────► exit 1 (stderr, no report)
                                     │ ok
                    ┌────────────────▼────────────────┐
                    │   orchestrator.LoadState(dataDir) │
                    │   (lê .provisioning-state.yaml)   │
                    └────────────────┬────────────────┘
                    file missing? ───┤──────────► state vazio (tudo pending)
                                     │
                    ┌────────────────▼────────────────┐
                    │  for step in CanonicalStepOrder:  │
                    │    if state[step].status == "done"│
                    │      → StepResult{status:"skipped"}│
                    │    else                           │
                    │      → StepResult{status:"pending"}│
                    └────────────────┬────────────────┘
                                     │
                    ┌────────────────▼────────────────┐
                    │  ApplyResult{                    │
                    │    status: "dry-run",            │
                    │    dryRun: true,                 │
                    │    bundle: nil,                  │
                    │    steps: [...10 steps...]       │
                    │  }                               │
                    └────────────────┬────────────────┘
                                     │
                                  exit 0
```

---

## Exemplo de saída YAML (sucesso, `mode: found`)

```yaml
spoke: spoke-brl
mode: found
dryRun: false
status: success
steps:
  - name: deploy-contracts
    status: skipped
    completedAt: "2026-06-27T10:00:01Z"
  - name: gen-tls
    status: skipped
    completedAt: "2026-06-27T10:00:03Z"
  - name: render-configs
    status: completed
    completedAt: "2026-06-27T10:01:12Z"
  - name: register-nodes
    status: completed
    completedAt: "2026-06-27T10:01:15Z"
  - name: start-paladin
    status: completed
    completedAt: "2026-06-27T10:02:44Z"
  - name: create-zeto-token
    status: completed
    completedAt: "2026-06-27T10:03:01Z"
  - name: create-pente-context
    status: completed
    completedAt: "2026-06-27T10:03:18Z"
  - name: deploy-fxa-pente
    status: completed
    completedAt: "2026-06-27T10:03:45Z"
  - name: onboard-registry
    status: completed
    completedAt: "2026-06-27T10:04:02Z"
  - name: register-relay
    status: completed
    completedAt: "2026-06-27T10:04:05Z"
bundle:
  path: bundles/spoke-brl.bundle.yaml
```

## Exemplo de saída YAML (dry-run, spoke novo)

```yaml
spoke: spoke-brl
mode: found
dryRun: true
status: dry-run
steps:
  - name: deploy-contracts
    status: pending
  - name: gen-tls
    status: pending
  - name: render-configs
    status: pending
  - name: register-nodes
    status: pending
  - name: start-paladin
    status: pending
  - name: create-zeto-token
    status: pending
  - name: create-pente-context
    status: pending
  - name: deploy-fxa-pente
    status: pending
  - name: onboard-registry
    status: pending
  - name: register-relay
    status: pending
```

## Invariantes de segurança

- `ApplyResult` nunca contém chaves privadas — nenhum campo de `LocalProfile`, `orchestrator.Deps`, `keyprovider.KeyProvider`, ou `certsource.CertSource` é serializado
- `BundleRef.Path` é um path relativo (não absoluto) — nunca vaza a estrutura de diretórios do host
- `Error` no `StepResult` é a mensagem de erro do engine — pode conter nomes de arquivos e URLs, mas nunca material de chave privada (o engine não retorna chaves em erros)
