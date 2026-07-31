# Implementation Plan: Toolkit do Cenário B — Schema de Manifesto e Validação (TK-B1)

**Branch**: `033-tk-b1-manifest-schema` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/033-tk-b1-manifest-schema/spec.md`

## Summary

Entregar a porta de entrada do toolkit do Cenário B: o pacote Go de manifesto
(`ParticipantDeployment`, `cbweb3b/v1`) com parse e validação dos três modos
(`found-hub`/`found-spoke`/`join`), um JSON-Schema v1 (contrato para editores/CI) e um caminho
de invocação que **apenas valida e reporta** (`--dry-run`/`validate`, saída JSON|YAML). Sem
motor de steps, bundles, templates de compose ou execução — as fases TK-B2+ consomem este
modelo. Abordagem: reimplementar (não importar) o padrão de manifesto/validação do toolkit de
referência (Cenário A), com os campos e regras já decididos no roadmap (§4, §13).

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: `gopkg.in/yaml.v3` (parse YAML). Validação **programática em Go** como
fonte de verdade; JSON-Schema como contrato de editor/CI (ver `research.md` — sem dependência de
lib de JSON-Schema em runtime).
**Storage**: nenhum — validação stateless (lê 1 manifesto ou um conjunto; não escreve estado).
**Testing**: `go test` (table-driven sobre manifestos válidos/inválidos + os três exemplos do
roadmap como fixtures de aceite).
**Target Platform**: CLI local (Linux/macOS dev).
**Project Type**: módulo Go isolado — CLI + lib (`scenario-b/toolkit`).
**Performance Goals**: N/A (validação de arquivo; sub-segundo).
**Constraints**: sem segredos no manifesto; só `environment: local`; **não** importar
`scenario-a/toolkit` (Constituição, Princípio I).
**Scale/Scope**: 1 manifesto por participante; validação de conjunto para N manifestos
(colisão de chainId/porta/rede). Escopo: schema + validação + report; **nada** de execução.

## Constitution Check

*GATE: deve passar antes da Fase 0. Re-checado após a Fase 1 (design).*

| Princípio | Avaliação (TK-B1) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Módulo novo `scenario-b/toolkit`, sob o diretório do Cenário B; **não importa** `scenario-a/toolkit` (padrão reimplementado). Não toca `scenario-a/`. |
| **II. Privacy by Design** | ✅ O manifesto **proíbe** segredos/PII (FR-006). Sem valor on-chain nesta fase; camada de privacidade (Zeto/Noto) fora de escopo (revisão futura no roadmap). |
| **III. Atomic Settlement** | ✅ N/A — TK-B1 é schema/validação, sem caminho de settlement. |
| **IV. Compliance Gate** | ✅ N/A em runtime; o modelo apenas **representa** papéis/entidades (`topology.role`, `adminUsers`). Nenhum gate é contornado. |
| **V. Test-First** | ✅ Validação e schema nascem com testes `go test` que falham antes da implementação (Red-Green-Refactor); os exemplos do roadmap viram fixtures. |
| **VI. Observability** | ✅ Report estruturado (`-o json|yaml`), erros nomeados e **coletados** (FR-011); sem falhas silenciosas. |

**Resultado (pré-Fase 0)**: PASS — sem violações. Nova dependência fora do stack: nenhuma além
de `yaml.v3` (já usado no toolkit de referência); a decisão de **não** adicionar lib de
JSON-Schema em runtime evita dependência nova (ver `research.md`).

**Re-avaliação (pós-Fase 1)**: PASS — o design (pacote `manifest` + schema em `provisioning/`)
mantém os seis princípios; nenhuma nova dependência introduzida.

## Project Structure

### Documentation (this feature)

```text
specs/033-tk-b1-manifest-schema/
├── plan.md              # este arquivo
├── spec.md              # a especificação
├── research.md          # Fase 0 — decisões (JSON-Schema vs validação Go, etc.)
├── data-model.md        # Fase 1 — ParticipantDeployment + regras por modo
├── quickstart.md        # Fase 1 — como validar um manifesto
├── contracts/           # Fase 1 — schema v1 + contrato da CLI + exemplos
│   ├── participant-deployment.schema.yaml
│   ├── cli-validate.md
│   └── examples/{found-hub,found-spoke,join}.yaml
└── checklists/
    └── requirements.md  # checklist de qualidade da spec
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── go.mod                          # módulo .../scenario-b/toolkit (Go 1.26)
├── cmd/cbweb3b/
│   └── main.go                     # CLI: validate / apply --dry-run -f <m> -o json|yaml
└── engine/
    └── manifest/
        ├── types.go                # ParticipantDeployment + specs por modo
        ├── validate.go             # validação semântica (modos, colisões, segredos, warnings)
        ├── validate_test.go        # tabela de casos + fixtures dos exemplos do roadmap
        └── testdata/               # manifestos válidos/inválidos

scenario-b/provisioning/schema/v1/
└── participant-deployment.schema.yaml   # JSON-Schema v1 (editor/CI) — espelha as regras Go
```

**Structure Decision**: módulo Go isolado `scenario-b/toolkit` (CLI + lib), espelhando o layout
do toolkit de referência do Cenário A; assets não-Go (schema) sob `scenario-b/provisioning/`.
Nesta fase existem apenas `cmd/cbweb3b` e `engine/manifest` — `orchestrator`, `bundle`,
`keyprovider`, `certsource`, `dockervolume`, etc. chegam nas fases seguintes.

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. Nota (não é violação): a **duplicação** do padrão de manifesto/validação do
Cenário A, em vez de uma lib compartilhada, é o caminho **exigido** pela Constituição (Princípio
I proíbe compartilhamento cross-cenário exceto por lib versionada) e a decisão registrada no
roadmap (§13). Portanto é conforme, não um desvio.
