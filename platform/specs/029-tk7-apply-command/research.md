# Research: TK-7 — Comando `apply`

**Phase 0 output** | **Branch**: `029-tk7-apply-command`

Todas as questões técnicas levantadas no Technical Context foram resolvidas antes da Phase 1. Nenhum "NEEDS CLARIFICATION" permanece.

---

## R-01 — CLI framework: `flag` stdlib vs framework externo

**Decision**: `flag` stdlib do Go, com subcomandos via `flag.NewFlagSet`.

**Rationale**: FR-016 exige zero novas dependências externas. O subcomando `apply` tem 3 flags (`-f`, `--dry-run`, `--output`) — simples demais para justificar um framework. `flag.NewFlagSet("apply", flag.ExitOnError)` cobre o caso de uso completamente.

**Alternatives considered**:
- `github.com/spf13/cobra` — rejeitado: nova dependência externa; overkill para 1 subcomando + 3 flags
- `github.com/urfave/cli/v2` — rejeitado: mesma razão; cobra e urfave são equivalentes no contexto
- `github.com/spf13/pflag` — rejeitado: nova dep; `flag` stdlib suporta os aliases necessários via dois `FlagSet.BoolVar` compartilhando a mesma variável

**Implementation note**: aliases `-n` (para `--dry-run`) e `-o` (para `--output`) são implementados com dois registros apontando para a mesma variável:
```go
fs.BoolVar(&dryRun, "dry-run", false, "show plan without executing")
fs.BoolVar(&dryRun, "n", false, "alias for --dry-run")
```

---

## R-02 — Separação `engine/apply/` vs monolito em `cmd/cbweb3/main.go`

**Decision**: Lógica de negócio em `engine/apply/` (pacote testável); `cmd/cbweb3/main.go` é thin wrapper (< 80 linhas).

**Rationale**: SC-001 exige `go test -race ./cmd/cbweb3/...` com testes antes da implementação — incluindo testes de `Run`, `DryRun`, e `ResolveDeps` sem invocar a CLI via `os/exec`. Um monolito em `main.go` tornaria esses testes impossíveis. A separação é o padrão idiomático Go para CLIs testáveis (`cmd/` thin, `internal/` ou pacote separado com lógica).

**Alternatives considered**:
- Tudo em `main.go` — rejeitado: `main` packages não são importáveis; os testes de lógica precisariam usar `os/exec` para cada caso, tornando-os lentos e frágeis
- `internal/apply/` — considerado; preferido `engine/apply/` para manter consistência com os demais pacotes do toolkit (`engine/orchestrator/`, `engine/bundle/`, etc.)

---

## R-03 — Dry-run: inspeção de estado sem executar passos

**Decision**: `engine/apply/dryrun.go` chama `orchestrator.LoadState(dataDir)` (função exportada) e itera sobre `orchestrator.CanonicalStepOrder` (slice exportado) para determinar `pending` vs `skipped`. Nenhuma chamada a `orchestrator.RunFound`.

**Rationale**: `ProvisioningState` e `StepState` já são exportados do orchestrator. Exportar `LoadState` e `CanonicalStepOrder` é um one-line change (capitalizar as letras iniciais) que evita duplicação da lógica de parse e da lista canônica de steps.

**Alternatives considered**:
- Duplicar a lógica YAML em `engine/apply/dryrun.go` — rejeitado: dois parsers do mesmo arquivo; risco de divergência silenciosa se o schema mudar
- Expor `DryRunPlan(dataDir string) ([]StepResult, error)` no orchestrator — rejeitado: move responsabilidade de construção do relatório para o orchestrator, que não deve conhecer o tipo `StepResult` (pertence à camada CLI)
- Chamar `step.Check(ctx)` para cada step sem executar `Run` — rejeitado: `Check` para steps como `start-paladin` acessa o Besu/Paladin via RPC; dry-run deve ser offline (SC-003: < 2 s sem acesso a serviços externos)

**dry-run algorithm**:
```
state, err = orchestrator.LoadState(dataDir)
for each stepName in orchestrator.CanonicalStepOrder:
    status = "pending"
    for each s in state.Steps:
        if s.Step == stepName && s.Status == "done":
            status = "skipped"
    results = append(results, StepResult{Name: stepName, Status: status})
```

---

## R-04 — Signal handling (SIGINT, SIGTERM)

**Decision**: `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` — padrão idiomático Go 1.16+. Ao receber sinal: contexto é cancelado, `orchestrator.RunFound` propaga `ctx.Err()`, `apply.Run` retorna com relatório parcial, main serializa e sai com exit não-zero.

**Rationale**: `signal.NotifyContext` é thread-safe, não requer goroutines manuais, e compõe naturalmente com o padrão `context.Context` já usado pelo engine. Disponível desde Go 1.16 — dentro da exigência Go 1.26+ do projeto.

**Alternatives considered**:
- `signal.Notify` com canal + goroutine de cancelamento manual — funciona mas requer boilerplate; `signal.NotifyContext` é a simplificação idiomática desde 1.16
- Ignorar sinais — rejeitado: a interrupção sem relatório parcial viola FR-012 e deixa o operador sem visibilidade de qual passo foi interrompido

**Implementation note**: `main.go` estrutura:
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
result, err := apply.Run(ctx, input)
// result sempre populado (parcialmente) mesmo quando ctx.Err() != nil
emitReport(result)
if err != nil { os.Exit(1) }
```

---

## R-05 — Resolução de paths de runtime (relativos ao binário + env vars)

**Decision**: `os.Executable()` retorna o path do binário; paths derivados via `filepath.Join(filepath.Dir(exPath), "../..", subpath)`. Env vars sobreescrevem qualquer default.

**Rationale**: Paths hardcoded absolutos quebram em qualquer máquina diferente (FR-015 proíbe explicitamente). Paths relativos ao binário funcionam em qualquer install location. Env vars permitem overrides em CI/CD sem recompilar.

**Defaults locais** (relativos ao binário em `scenario-a/toolkit/cmd/cbweb3/`):
| Env var | Default relativo ao binário |
|---|---|
| `CBWEB3_SCRIPTS_DIR` | `../../deploy/local/paladin/scripts/` |
| `CBWEB3_COMPOSE_TEMPLATE` | `../../provisioning/templates/central-bank/paladin-compose.yaml` |
| `CBWEB3_PALADIN_CONFIG_DIR` | `../../provisioning/templates/central-bank/paladin-config/` |
| `CBWEB3_PALADIN_CB_URL` | `http://localhost:31648` |

**Alternatives considered**:
- Config file separado (e.g., `~/.cbweb3/config.yaml`) — rejeitado: introduz nova superfície de configuração sem necessidade; env vars são suficientes e mais simples em CI
- Paths relativos ao `cwd` — rejeitado: `cwd` é imprevisível quando o binário é invocado de diretórios arbitrários; binário-relativo é determinístico

---

## R-06 — Serialização da saída estruturada (JSON vs YAML)

**Decision**: `encoding/json` para `--output json`; `gopkg.in/yaml.v3` para `--output yaml` (default). Ambos já em `toolkit/go.mod`. Tags `json:"..."` e `yaml:"..."` nos tipos de `result.go`.

**Rationale**: Os dois formatos já estão no módulo. `encoding/json` é stdlib — zero overhead. `gopkg.in/yaml.v3` já é usado pelo toolkit para parse de manifesto e estado. Usar os dois garante paridade de campos (mesmos tags em structs compartilhadas).

**stdout invariant**: o relatório é construído em memória (`ApplyResult`) antes de qualquer escrita em stdout. Mesmo em falha, o `ApplyResult` existe (com `status: failed` e os steps até o ponto de falha). Isso garante que stdout é sempre JSON/YAML válido (FR-013).

**Alternatives considered**:
- `text/template` para YAML — rejeitado: mais frágil que marshal estruturado; sem validação de campo
- `sigs.k8s.io/yaml` — rejeitado: nova dependência; `gopkg.in/yaml.v3` já no módulo

---

## R-07 — BesuRPCURL para o perfil local

**Decision**: `http://localhost:<spec.node.rpc.port>` — concatenação direta da porta do manifesto.

**Rationale**: Para o perfil `local`, o Besu roda em container com a porta RPC mapeada para o host (via Docker Compose `ports`). A URI `http://localhost:<rpc.port>` é exatamente o que o container expõe. É o mesmo padrão usado em `make/40-paladin.mk` (`BESU_RPC_URL_A=http://localhost:8645`).

**Note**: Esta URL é usada pelo engine para chamar o Besu (deploy contratos, etc.). É diferente do `advertisedHost` do manifesto, que é o endereço que outros nós usam para se conectar via P2P — esses dois conceitos são ortogonais.

**Alternatives considered**:
- `http://<spec.node.advertisedHost>:<spec.node.rpc.port>` — rejeitado: `advertisedHost` é um nome de container DNS (`cbweb3-spoke-brl-besu.central-bank-brazil`), não resolvível na máquina host onde o `cbweb3` CLI roda
- Env var sem fallback — rejeitado: para `environment: local`, o default de `localhost` é sempre certo e evita config boilerplate

---

## R-08 — Exports adicionais do pacote `orchestrator`

**Decision**: Exportar duas entidades do `orchestrator`:
1. `LoadState(dir string) (ProvisioningState, error)` — atualmente `loadState` (unexported)
2. `CanonicalStepOrder []string` — atualmente `canonicalStepOrder` (unexported var)

**Rationale**: `engine/apply/dryrun.go` precisa de ambos para implementar dry-run sem duplicação. `ProvisioningState` e `StepState` já são exportados (verificado em `state.go`). As constantes `StepDeployContracts`, `StepGenTLS`, etc. já são exportadas (verificado em `step.go`). Exportar `LoadState` e `CanonicalStepOrder` é mudança de nomenclatura mínima, sem impacto no comportamento.

**Impact**: Mudanças em `orchestrator/state.go` e `orchestrator/step.go`:
- `loadState` → `LoadState` (capitalizar; atualizar chamadores internos)
- `canonicalStepOrder` → `CanonicalStepOrder` (capitalizar)

Todos os chamadores atuais de `loadState` estão no mesmo pacote (`orchestrator`) — atualização interna. Nenhum teste externo precisa mudar.

**Alternatives considered**:
- Duplicar lógica em `engine/apply/` — rejeitado (R-03)
- Adicionar `orchestrator.DryRunPlan(dir string) []StepResult` — rejeitado: cria dependência de tipo `StepResult` (pertence à camada `apply`) no pacote `orchestrator` — inversão de dependência

---

## R-09 — PaladinCBURL para o perfil local

**Decision**: Default `http://localhost:31648` (porta Paladin CB do spoke-a existente), overridável via `CBWEB3_PALADIN_CB_URL`.

**Rationale**: A porta 31648 é o `PALADIN_CB_URL` usado em `make/40-paladin.mk` para o spoke-a. É a porta que o container Paladin do banco central expõe para o host. Para um spoke diferente (e.g., spoke-brl com porta diferente), o operador usa a env var para override. Isso mantém o zero-config para o caso comum (um spoke com a porta default).

**Note**: `PaladinCBURL` é necessário para todos os passos que interagem com o Paladin (steps 5–8 do engine). Sem esse valor, `orchestrator.RunFound` retorna erro de validação.

**Alternatives considered**:
- Derivar porta do manifesto (e.g., `spec.node.paladin.port`) — considerado mas rejeitado: a porta Paladin não está no schema do manifesto (TK-1); adicioná-la aumenta a superfície do manifesto sem necessidade em TK-7; env var é suficiente para TK-7
- Hardcoded sem override — rejeitado (FR-015)
