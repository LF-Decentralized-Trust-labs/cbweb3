# Implementation Plan: TK-7 — Comando `apply`

**Branch**: `029-tk7-apply-command` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/029-tk7-apply-command/spec.md`

## Summary

Implementar o comando `cbweb3 apply -f manifest.yaml` — ponto de composição e única interface operacional do toolkit de provisionamento do Scenario A. O comando lê e valida o manifesto, resolve implementações concretas (`KeyProvider`, `CertSource`, `RelayRegistrar`, `BesuRPCURL`) a partir dos campos do manifesto e do perfil `local`, chama `orchestrator.RunFound` (TK-5), emite o join bundle via `bundle.EmitBundle` (TK-6), e serializa um relatório estruturado YAML/JSON em stdout. Suporta `--dry-run` (inspeciona estado sem executar), `--output json|yaml`, e tratamento de sinais para relatório parcial em interrupções. Zero novas dependências externas além das já presentes em `toolkit/go.mod`.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: `gopkg.in/yaml.v3` (serialização do relatório em YAML), `encoding/json` (serialização JSON), `os/signal` + `syscall` (tratamento de sinais SIGINT/SIGTERM), `flag` stdlib (parsing de flags) — todos stdlib ou já em `toolkit/go.mod`
**Storage**: Filesystem — lê `<dataDir>/.provisioning-state.yaml` (dry-run e pós-execução); escreve nenhum arquivo diretamente (delegado a `orchestrator.RunFound` e `bundle.EmitBundle`)
**Testing**: `go test -race ./engine/apply/...` com injeção de `orchestrator.Deps` mockada; `go test -race ./cmd/cbweb3/...` com `os/exec` para testes de integração da CLI; `testdata/` com manifests de fixture
**Target Platform**: Linux (Docker Compose / Kubernetes); mesma plataforma do engine TK-5/TK-6
**Project Type**: CLI binary (`cmd/cbweb3/main.go`) + library Go (`engine/apply/`)
**Performance Goals**: `--dry-run` completa em < 2 s (leitura de arquivo de estado — sem acesso a Besu); `apply` completo dominado pelo runtime do engine (TK-5 — minutos)
**Constraints**: Zero novas dependências externas; stdout sempre emite JSON/YAML válido mesmo em falha; no silent errors (FR-013); `loadState` e `canonicalStepOrder` do orchestrator exportados para uso pelo dry-run
**Scale/Scope**: Um spoke por invocação; um binário por toolkit; todos os ambientes compartilham o mesmo binário

## Constitution Check

*GATE: Avaliado antes de Phase 0. Re-avaliado após Phase 1.*

| Princípio | Avaliação | Justificativa |
|---|---|---|
| **I — Scenario-Scoped Independence** | ✅ PASS | Pacotes residem em `scenario-a/toolkit/cmd/cbweb3/` e `engine/apply/`; zero referência a `scenario-b/`; nenhum código compartilhado além de `engine/orchestrator/`, `engine/manifest/`, `engine/keyprovider/`, `engine/certsource/`, `engine/bundle/` — todos internos ao toolkit do Scenario A |
| **II — Privacy by Design** | ✅ N/A | CLI é camada de orquestração e configuração; não executa transações de valor, não opera contratos de privacy layer diretamente. A CLI não contém nem exibe chaves privadas — os fields `keyProvider` e `certSource` são URIs de referência |
| **III — Atomic Settlement Guarantee** | ✅ N/A | `apply` invoca `orchestrator.RunFound` que gerencia a sequência de provisionamento idempotente; não é um caminho de liquidação. Não interfere com HTLCs, AMM, ou circuit breaker |
| **IV — Compliance Gate Before Participation** | ✅ PASS | `apply` não bypassa nenhum compliance gate; o passo `onboard-registry` (step 9 do TK-5) é parte da sequência `RunFound` e executa o onboarding real no IdentityRegistry antes do spoke ficar operacional |
| **V — Test-First at Every Layer** | ✅ PASS | SC-001 exige `go test -race ./cmd/cbweb3/...` verde antes de qualquer implementação. Cada fase começa com teste failing (red-green cycle). Testes de integração CLI em `cmd/cbweb3/` com `os/exec` |
| **VI — Observability and Auditability** | ✅ PASS | FR-010: relatório estruturado com estado de cada step em stdout; FR-012: relatório parcial emitido em interrupção (SIGINT/SIGTERM); FR-013: erros vão para stderr (sem silent failures); logs do engine (TK-5) via `log/slog` já em stdout |

**Resultado**: Sem violações. Todas as gates passam. Pode prosseguir para Phase 0.

**Re-check pós-design (Phase 1)**: Princípio I confirmado — `engine/apply/` não importa nenhum pacote de `scenario-b/`. Princípio V confirmado — estrutura de testes definida em data-model.md antes das implementações.

## Project Structure

### Documentation (this feature)

```text
specs/029-tk7-apply-command/
├── plan.md              # Este arquivo (/speckit.plan command output)
├── research.md          # Phase 0 output — 9 decisões (R-01 a R-09)
├── data-model.md        # Phase 1 output — tipos Go, ApplyResult, StepResult, transições
├── quickstart.md        # Phase 1 output — build, uso, dry-run, exemplos de output
├── contracts/
│   └── apply-command.md # Phase 1 output — contrato CLI: flags, exit codes, stdout schema
└── tasks.md             # Phase 2 output (/speckit.tasks — NÃO criado aqui)
```

### Source Code (repository root)

```text
scenario-a/toolkit/
├── cmd/
│   └── cbweb3/
│       ├── main.go                   # thin: flag parsing → apply.Run ou apply.DryRun → serializar → exit
│       └── main_test.go              # testes de integração CLI com os/exec (mode geral)
├── engine/
│   └── apply/
│       ├── apply.go                  # Run(ctx, Input) → (ApplyResult, error)
│       ├── apply_test.go             # testa Run com deps mockados (engine stub)
│       ├── dryrun.go                 # DryRun(ctx, Input) → (ApplyResult, error)
│       ├── dryrun_test.go            # testa DryRun com state fixtures
│       ├── deps.go                   # ResolveDeps(m *manifest.Manifest) → (orchestrator.Deps, error)
│       ├── deps_test.go              # testa resolução de URI e profile defaults
│       ├── profile.go                # LocalProfile: defaults de BesuRPCURL, PaladinCBURL, paths
│       ├── result.go                 # ApplyResult, StepResult, BundleRef — tipos de saída
│       └── result_test.go            # serialização JSON/YAML e omitempty
│   └── orchestrator/
│       ├── step.go                   # MODIFICAR: exportar CanonicalStepOrder e canonicalStepOrder
│       └── state.go                  # MODIFICAR: exportar LoadState
```

**Structure Decision**: Arquitetura two-layer — `cmd/cbweb3/main.go` é thin wrapper (< 80 linhas) responsável apenas por flag parsing, sinalização e serialização final; `engine/apply/` contém toda a lógica testável de forma isolada. Essa separação é requerida por SC-001 (testes sem CLI) e FR-016 (zero novas deps — `flag` stdlib suficiente). O pacote `engine/apply/` importa `engine/orchestrator`, `engine/manifest`, `engine/keyprovider`, `engine/certsource`, `engine/bundle` — todos internos ao mesmo módulo Go.

## Complexity Tracking

> **Dois exports adicionais ao orchestrator**: `LoadState` e `CanonicalStepOrder` precisam ser exportados para que `engine/apply/dryrun.go` possa inspecionar estado dos passos sem duplicar a definição canônica da ordem dos passos.

| Desvio | Por que necessário | Alternativa mais simples rejeitada porque |
|---|---|---|
| Exportar `LoadState` de `orchestrator` | dry-run precisa saber estado atual dos steps sem executar o engine; `ProvisioningState` e `StepState` já são exportados; exige apenas tornar a função pública | Duplicar a lógica de parse YAML em `engine/apply/dryrun.go` cria dois parsers do mesmo formato — risco de divergência silenciosa se o formato mudar |
| Exportar `CanonicalStepOrder` de `orchestrator` | dry-run precisa iterar sobre todos os 10 steps em ordem para montar o relatório de plano; a ordem é a fonte da verdade do `mode: found` | Duplicar o slice em `engine/apply/` — mesma objeção: dois lugares com a verdade canônica |
