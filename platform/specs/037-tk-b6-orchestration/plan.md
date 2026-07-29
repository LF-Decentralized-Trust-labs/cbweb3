# Implementation Plan: Toolkit do Cenário B — Motor + found-hub + hub bundle + apply (TK-B6)

**Branch**: `037-tk-b6-orchestration` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/037-tk-b6-orchestration/spec.md`

## Summary

Entregar o primeiro modo executável do toolkit: um **motor de steps idempotente**
(`Step{Name,Check,Run}` + estado YAML durável + `flock` + dry-run + report), o **inventário de
steps do `found-hub`**, o **hub bundle emitter** e o **comando `apply`** (CLI). Os efeitos externos
passam por um **executor injetável** (`CommandRunner`) — real (docker compose / `forge` / Keycloak)
em produção, **fake** nos testes de unidade — permitindo testar ordem, gates, idempotência e dry-run
sem Docker; e uma **suíte E2E** funda o hub de ponta a ponta com Docker + Foundry + Besu reais,
validando o hub bundle contra a chain viva (pulada com aviso onde o ambiente E2E falta). O deploy dos
contratos do hub é **um** `forge script CBWeb3Hub.s.sol:DeployCBWeb3Hub` (a ordem
IdentityRegistry→…→ManualOracle é interna ao script Solidity); o step extrai os endereços do
broadcast JSON. Consome as interfaces do TK-B2/B3/B5 e os templates do TK-B4.

## Technical Context

**Language/Version**: Go 1.26 (módulo `scenario-b/toolkit`).
**Primary Dependencies**: **nenhuma nova** — `github.com/ethereum/go-ethereum` (probes RPC /
verificação de endereços na chain) e `gopkg.in/yaml.v3` (estado, bundle, report yaml) já presentes;
stdlib `os/exec` (executor), `encoding/json` (broadcast do forge + report json), `os/signal`+
`syscall` (report parcial), `flag` (CLI). **Ferramentas externas** (runtime/E2E, não deps Go):
Docker Compose v2, **Foundry `forge` 1.5.1** (presente), imagem `hyperledger/besu:25.8.0`, Keycloak.
**Storage**: estado por step em `<dataDir>/.provisioning-state.yaml` + `flock`
`<dataDir>/.provisioning.lock`; hub bundle em `<outDir>/bundles/hub.bundle.yaml`; endereços lidos do
broadcast Foundry (`contracts/broadcast/CBWeb3Hub.s.sol/<chainId>/run-latest.json`).
**Testing**: `go test` — motor (idempotência, ordem topológica, dry-run, lock, retomada, report) e
bundle (round-trip, sem segredos) com **FakeRunner**; **suíte E2E** (build tag `e2e`) que roda
`apply found-hub` real e valida o bundle contra o RPC — **skip com aviso** se `forge`/Docker/Besu
ausentes.
**Target Platform**: binário CLI `cbweb3b` (Linux) + biblioteca do toolkit.
**Project Type**: CLI + biblioteca Go (motor de orquestração).
**Performance Goals**: N/A (orquestração; E2E limitado por Besu/Foundry).
**Constraints**: idempotência (re-apply converge); dry-run sem efeitos; sem segredos no bundle; não
alterar Makefiles/`deploy/local`; não importar `scenario-a/`; `found-spoke`/`join` → "não suportado
ainda"; skip-com-aviso quando o ambiente E2E falta (nunca falso verde).
**Scale/Scope**: 1 modo (`found-hub`), ~9 steps, 1 bundle (hub), 1 comando (`apply`).

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B6) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Motor reimplementa o padrão do Cenário A (Step+estado+flock) no módulo `scenario-b/toolkit`; **não** importa `scenario-a/`. Não edita Makefiles/`deploy/local`. |
| **II. Privacy by Design** | ✅ Hub bundle **sem segredos** (FR-012) — só endereços/RPC/chainId públicos. `tCeBM_BRL/EUR` são deployados no hub como **camada de reserva** (papel correto); nada de PII/valores on-chain. |
| **III. Atomic Settlement** | ✅ N/A — `found-hub` não é caminho de settlement. O gate de breaker no relay é do TK-B5 (já entregue). |
| **IV. Compliance Gate** | ✅ **Reforça**: o `found-hub` estabelece o `IdentityRegistry` + o Keycloak (OIDC) — a fundação do gate de compliance. O toolkit não o contorna. |
| **V. Test-First** | ✅ Motor/bundle com testes `go test` (FakeRunner) antes da implementação; a suíte E2E é a aceitação final (skip-com-aviso onde o ambiente falta — nunca falso verde). |
| **VI. Observability** | ✅ Report estruturado (json/yaml) distingue concluído/pulado/falho/planejado; erros tipados; sem swallow (FR-014). |

**Novas dependências**: nenhuma dependência Go nova. Docker/Foundry/Besu já são o stack declarado
(Constituição/§3). Nada a justificar.

**Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações (reforça IV).

## Project Structure

### Documentation (this feature)

```text
specs/037-tk-b6-orchestration/
├── plan.md, spec.md
├── research.md          # Fase 0 — motor de ref., deploy via CBWeb3Hub.s.sol, keycloak write-back, E2E skip
├── data-model.md        # Fase 1 — Step, State, Report, HubBundle, Executor, steps found-hub
├── quickstart.md        # Fase 1 — apply --dry-run e apply real (E2E)
├── contracts/           # Fase 1 — CLI apply + API do motor/bundle/executor
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── cmd/cbweb3b/                 # estende o binário do TK-B1 com o subcomando `apply`
│   ├── main.go                  # apply -f <manifest> [--dry-run] [-o json|yaml]; signal → report parcial
│   └── apply_test.go
├── engine/apply/                # dispatch por modo, resolução de caminhos, dry-run, report
│   ├── apply.go, report.go, apply_test.go
├── engine/orchestrator/         # o motor
│   ├── step.go                  # Step{Name,Check,Run,Deps}
│   ├── orchestrator.go          # execução topológica, Check→skip/Run, dry-run
│   ├── state.go                 # .provisioning-state.yaml (persistência por step)
│   ├── lock.go                  # flock (.provisioning.lock) + política de lock órfão
│   ├── deps.go                  # conjunto de steps do found-hub (ordem/deps)
│   ├── step_found_hub.go        # start-besu-hub, deploy-hub-contracts, provision-keycloak (+write-back),
│   │                            # render-env, infra/backend/frontend, start-relay, start-noc, emit-bundle
│   └── *_test.go                # idempotência, ordem, dry-run, lock, retomada (FakeRunner)
├── engine/exec/                 # fronteira injetável de efeitos externos
│   ├── runner.go                # CommandRunner (os/exec real)
│   ├── fake.go                  # FakeRunner (grava comandos; para testes)
│   └── runner_test.go
├── engine/bundle/               # hub bundle
│   ├── types.go, hub.go         # emit/load/validate (yaml, versionado, sem segredos)
│   └── bundle_test.go
├── engine/addrs/                # extrair endereços do broadcast Foundry + escrever env
│   ├── broadcast.go, env.go, addrs_test.go
└── tests/e2e/                    # suíte E2E (build tag `e2e`)
    └── found_hub_e2e_test.go     # apply found-hub real; valida bundle vs RPC; skip-com-aviso
```

**Structure Decision**: estender o binário `cmd/cbweb3b` (TK-B1) com `apply`. O motor
(`engine/orchestrator`) espelha `engine/orchestrator` do Cenário A (Step/state/deps), reimplementado.
Os efeitos externos ficam atrás de `engine/exec.CommandRunner` (real vs fake) — a chave para testar
sem Docker e para o `--dry-run`. O deploy de contratos é **um** `forge script CBWeb3Hub.s.sol` (a
ordem é interna ao Solidity); `engine/addrs` lê o broadcast JSON e alimenta o env/bundle. A suíte E2E
fica sob build tag `e2e` para não travar o `go test` padrão.

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. O escopo E2E (decisão do usuário) é maior, mas **não** adiciona dependências Go nem
desvia de padrão: reusa o motor de referência e o stack já declarado (Docker/Foundry/Besu). O
executor injetável mantém o motor testável em unidade; a suíte E2E é isolada por build tag e pula
com aviso onde o ambiente falta — sem falso verde e sem bloquear o CI padrão.
