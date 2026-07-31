# Implementation Plan: Toolkit do Cenário B — Templates de compose parametrizados (TK-B4)

**Branch**: `035-tk-b4-parametrized` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/035-tk-b4-parametrized/spec.md`

## Summary

Criar um conjunto de **templates de compose net-new** sob `scenario-b/provisioning/templates/`
(hub, entity-besu, entity-infra, entity-keycloak, entity-backend, entity-frontend, relay, NOC),
derivados dos compose atuais de `deploy/local` e dos scripts `startBesu.sh`, com **todos os valores
discriminantes externados por variável de ambiente**, **named volumes determinísticos por
spoke/entidade** (substituindo bind mounts de host, exceto o `pki/` do banco), **esquema de porta
por offset** e **alcance cross-stack** (`host.docker.internal:host-gateway`). Cada template tem um
**contrato de variáveis** (`vars/*.env.example` + convenção de nomes/portas). A entrega inclui
**validação estática** (pacote Go que faz parse YAML e checa interpolação, named volumes, ausência
de bind mounts host indevidos, ausência de segredos e ausência de colisão cross-entidade), mais uma
verificação opcional via `docker compose config` quando o Docker está disponível. **Fora de escopo
(decisão 2026-07-10):** derivação em código de portas/nomes (fica no motor, TK-B6), renderização em
runtime, subida de containers e steps do motor. Os compose de `deploy/local` **não são tocados**.

## Technical Context

**Language/Version**: Go 1.26 (validação/testes no módulo `scenario-b/toolkit`); assets em YAML
(Compose Specification) + arquivos `.env` de contrato.
**Primary Dependencies**: `gopkg.in/yaml.v3` (já presente no `toolkit/go.mod` desde TK-B1) para o
parse de validação. **Nenhuma dependência nova.** Docker Compose v2 é dependência **opcional** de
teste (só para a verificação `docker compose config`, que é pulada se ausente).
**Storage**: nenhum — os templates são arquivos em disco sob `provisioning/`; a validação é
stateless (lê template + env de exemplo, retorna resultado).
**Testing**: `go test` — validação estática dos 7 templates (interpolação completa, named volumes,
regra de bind mount, sem segredos, sem colisão cross-entidade); teste de integração opcional
`docker compose config` (com `t.Skip` se o Docker não estiver disponível).
**Target Platform**: assets sob `scenario-b/provisioning/templates/`; validação Go sob
`scenario-b/toolkit/engine/composetemplate/`.
**Project Type**: assets de provisionamento (YAML/env) + biblioteca de validação Go.
**Performance Goals**: N/A (validação local de arquivos).
**Constraints**: não editar `deploy/local/*` nem `scenario-b/Makefile`/`make/*.mk`; sem segredos nos
templates; named volumes determinísticos (exceção `pki/`); não importar `scenario-a/`; sem cálculo
de offset/nomes em código (fica no motor).
**Scale/Scope**: 8 templates (inclui `entity-besu`); N entidades dinâmicas (validação exercitada
com ≥2 entidades distintas para provar ausência de colisão).

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B4) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Assets sob `scenario-b/provisioning/` e validação sob `scenario-b/toolkit/`; **não** importa `scenario-a/`. Os templates são **derivados** (reescritos), não copiados de outro cenário. |
| **II. Privacy by Design** | ✅ FR-009: **nenhum segredo** embutido nos templates; material sensível chega por volume/env em runtime. `tCeBM` permanece camada de reserva (os templates não o inserem em caminho de retail). Reforça o princípio. |
| **III. Atomic Settlement** | ✅ N/A — sem caminho de settlement nesta fase. |
| **IV. Compliance Gate** | ✅ N/A em runtime; os templates **preservam** a topologia de compliance (API gateway + Keycloak + serviços), sem removê-los nem contorná-los. |
| **V. Test-First** | ✅ A validação Go nasce com testes que falham antes de os templates existirem/estarem corretos (Red→Green): cada regra (interpolação, volume, segredo, colisão) tem teste. |
| **VI. Observability** | ✅ Os templates preservam a saída de logs dos serviços derivados; a validação reporta erros tipados (variável ausente, bind mount indevido, segredo, colisão) — sem falha silenciosa. |

**Novas dependências:** nenhuma. `gopkg.in/yaml.v3` já está no módulo (TK-B1). Docker Compose é
opcional e só de teste. Portanto **não há** justificativa de dependência a registrar.

**Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações.

## Project Structure

### Documentation (this feature)

```text
specs/035-tk-b4-parametrized/
├── plan.md, spec.md
├── research.md          # Fase 0 — derivação dos assets, estratégia de validação, esquema de portas
├── data-model.md        # Fase 1 — entidades (Template, Contrato de vars, esquema de nomes/portas, volumes)
├── quickstart.md        # Fase 1 — como validar um template com um env de exemplo
├── contracts/           # Fase 1 — contrato de variáveis por template + convenção de portas/nomes
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/provisioning/templates/          # ASSETS (não-Go)
├── hub.compose.yaml
├── entity-besu.compose.yaml                 # nó Besu + Paladin do spoke/banco (estado de nó)
├── entity-infra.compose.yaml
├── entity-keycloak.compose.yaml
├── entity-backend.compose.yaml
├── entity-frontend.compose.yaml
├── relay.compose.yaml
├── noc.compose.yaml
└── vars/
    ├── hub.env.example                     # contrato de variáveis (obrigatórias/opcionais)
    ├── entity-besu.env.example
    ├── entity-infra.env.example
    ├── entity-keycloak.env.example
    ├── entity-backend.env.example
    ├── entity-frontend.env.example
    ├── relay.env.example
    ├── noc.env.example
    └── NAMING.md                            # convenção determinística de portas (offset) e nomes

scenario-b/toolkit/engine/composetemplate/  # VALIDAÇÃO (Go) — parte do escopo "validação"
├── composetemplate.go                       # Load + Validate (interpolação, volumes, segredos, colisão)
├── composetemplate_test.go                  # SC-001..006 (estático, sem Docker)
├── dockerconfig_test.go                     # integração opcional: `docker compose config` (t.Skip sem Docker)
└── testdata/                                # envs de exemplo p/ 2 entidades distintas
```

**Structure Decision**: os **assets** ficam sob `scenario-b/provisioning/` (mesmo lar do
`schema/v1/` publicado no TK-B1), fora do módulo Go, preservando `deploy/local` intacto. A
**validação** é um pacote Go pequeno em `engine/composetemplate` — coberto pelo escopo "assets +
contrato + **validação**" (a escolha do usuário excluiu apenas o utilitário de **derivação**, não a
validação). O pacote de validação **não** calcula portas/nomes: ele apenas confere que os templates,
dado um env de exemplo, satisfazem as regras. Sem CLI nesta fase (o motor de TK-B6 consumirá os
assets).

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. O pacote de validação Go **não** é um desvio: a validação é escopo explícito da
feature, reusa `yaml.v3` (já presente) e não adiciona dependências. Os templates são derivados por
reescrita (Princípio I), não compartilhados.
