# Tasks: Toolkit do Cenário B — Schema de Manifesto e Validação (TK-B1)

**Input**: Design documents from `/specs/033-tk-b1-manifest-schema/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: INCLUÍDOS — a Constituição (Princípio V, Test-First) é obrigatória; cada story tem
testes que devem **falhar antes** da implementação (Red-Green-Refactor).

**Organization**: tarefas agrupadas por user story (P1→P3) para implementação e teste
independentes. **MVP = US1**.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos distintos, sem dependência pendente)
- Caminhos são relativos à raiz do repo.

## Path Conventions

Módulo Go isolado (plan.md): fonte em `scenario-b/toolkit/`, schema em
`scenario-b/provisioning/schema/v1/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: inicializar o módulo e a estrutura.

- [x] T001 Criar o módulo `scenario-b/toolkit` (`go.mod`, Go 1.26) e a estrutura de diretórios: `scenario-b/toolkit/cmd/cbweb3b/`, `scenario-b/toolkit/engine/manifest/`, `scenario-b/toolkit/engine/manifest/testdata/`, `scenario-b/provisioning/schema/v1/`
- [x] T002 [P] Adicionar dependência `gopkg.in/yaml.v3` ao `scenario-b/toolkit/go.mod` e configurar `go vet`/`gofmt` (sem outras deps — decisão R1)
- [x] T003 [P] Copiar os manifestos de exemplo para fixtures: `scenario-b/toolkit/engine/manifest/testdata/{found-hub,found-spoke,join}.yaml` (a partir de `specs/033-tk-b1-manifest-schema/contracts/examples/`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: tipos, parse, resultado e esqueleto de CLI — bloqueiam todas as stories.

- [x] T004 Definir os tipos `ParticipantDeployment` + specs por modo (hub/spoke/pair/node/…) em `scenario-b/toolkit/engine/manifest/types.go` (conforme data-model.md)
- [x] T005 Implementar o parse YAML (arquivo → struct; suportar múltiplos arquivos) em `scenario-b/toolkit/engine/manifest/parse.go`
- [x] T006 Definir o tipo de resultado de validação (`Errors[]`, `Warnings[]`, coleta-tudo) + serialização JSON/YAML em `scenario-b/toolkit/engine/manifest/result.go`
- [x] T007 Esqueleto da CLI em `scenario-b/toolkit/cmd/cbweb3b/main.go`: comandos `validate` e `apply --dry-run`, flags `-f` (repetível) e `-o json|yaml`, exit codes 0/1/2, fiando parse→(validação stub)→report

**Checkpoint**: módulo compila; CLI parseia e emite report vazio.

---

## Phase 3: User Story 1 — Validar um manifesto de um modo (Priority: P1) 🎯 MVP

**Goal**: aceitar manifestos válidos dos três modos e rejeitar inválidos com erro nomeado.

**Independent Test**: `validate -f` nos três exemplos → válidos; introduzir apiVersion/kind
errado, `environment: prod`, segredo embutido, ou campo obrigatório do modo faltando → rejeitado
com mensagem apontando o campo.

- [x] T008 [P] [US1] Testes que falham em `scenario-b/toolkit/engine/manifest/validate_test.go`: os 3 exemplos válidos; `apiVersion`/`kind` errado → erro; `environment: prod` → erro; segredo embutido (`PRIVATE KEY`/hex 64) → erro; `found-hub` sem `hub`, `found-spoke` sem `hubBundleRef`, `join` sem `bankId` → erro (SC-001, SC-002, SC-005)
- [x] T009 [US1] Implementar validação estrutural (apiVersion/kind/scenario/enums de mode|role|environment; `node.*.port` inteiro; `advertisedHost` não-vazio; `keyProvider` `kms://`; `certSource` `self-signed`|`ca://`) em `scenario-b/toolkit/engine/manifest/validate.go`
- [x] T010 [US1] Implementar `environment == local` + varredura "sem segredos" (rejeitar `PRIVATE KEY`/chave hex em qualquer campo) em `scenario-b/toolkit/engine/manifest/validate.go`
- [x] T011 [US1] Implementar presença dos campos obrigatórios por modo (hub / hubBundleRef / spoke / joinBundleRef / bankId) em `scenario-b/toolkit/engine/manifest/validate.go`
- [x] T012 [US1] Fiar a validação no report da CLI (erros coletados, `-o json|yaml`, exit 0/1) em `scenario-b/toolkit/cmd/cbweb3b/main.go`
- [x] T013 [US1] Publicar o JSON-Schema `scenario-b/provisioning/schema/v1/participant-deployment.schema.yaml` (a partir de `contracts/participant-deployment.schema.yaml`) + teste garantindo que schema e validação Go concordam nas regras estruturais (SC-004)

**Checkpoint**: US1 entregável — validação de manifesto único funcional (MVP).

---

## Phase 4: User Story 2 — Detectar colisões e unicidade (Priority: P2)

**Goal**: ao validar um conjunto de manifestos, detectar colisão de chainId/portas/rede e
duplicidade de `spoke.id`/`metadata.name`.

**Independent Test**: `validate -f a -f b` com `chainId` igual (ou portas sobrepostas, ou
`spoke.id` duplicado) → erro nomeando os dois manifestos.

- [x] T014 [P] [US2] Testes que falham em `scenario-b/toolkit/engine/manifest/collision_test.go`: chainId igual entre 2 manifestos → erro; bandas de porta (rpc/ws/p2p + derivadas) sobrepostas → erro; `spoke.id`/`metadata.name` duplicados → erro (SC-003)
- [x] T015 [US2] Implementar validação de conjunto (colisão de `chainId`, bandas de porta, `spoke.id`, `metadata.name`, nome de rede derivado) em `scenario-b/toolkit/engine/manifest/collision.go`
- [x] T016 [US2] Fiar a validação de conjunto (múltiplos `-f`) e o report por-manifesto na CLI em `scenario-b/toolkit/cmd/cbweb3b/main.go`

**Checkpoint**: US2 entregável — colisões detectadas na validação de conjunto.

---

## Phase 5: User Story 3 — Requisitos por modo (Priority: P3)

**Goal**: rejeitar campos de outro modo, validar o par soberano e emitir warning de validador.

**Independent Test**: `found-hub` com `hubBundleRef`, `join` com `hub`/`pair`/`cbEndpoint` →
erro; `pair` com `proposerCB == confirmerCB` → erro; `join` com `validator: true` → warning.

- [x] T017 [P] [US3] Testes que falham em `scenario-b/toolkit/engine/manifest/validate_test.go`: campos de outro modo rejeitados; `pair` `proposerCB==confirmerCB`/`symbolA==symbolB` → erro; `join` `validator: true` → warning
- [x] T018 [US3] Implementar checagem de campos **proibidos** por modo + validação de `pair` (`proposerCB≠confirmerCB`, `symbolA≠symbolB`) em `scenario-b/toolkit/engine/manifest/validate.go`
- [x] T019 [US3] Implementar o **warning** de `node.validator: true` no `join` (aceita, não rejeita — FR-013) em `scenario-b/toolkit/engine/manifest/validate.go`

**Checkpoint**: US3 entregável — matriz por modo completa + warnings.

---

## Phase 6: Polish & Cross-Cutting

- [x] T020 [P] Suíte de aceite cobrindo SC-001..SC-005 em `scenario-b/toolkit/engine/manifest/acceptance_test.go` (roda os exemplos + casos-borda ponta-a-ponta pela CLI)
- [x] T021 [P] README de uso do `cmd/cbweb3b` (alinhado ao `quickstart.md`) em `scenario-b/toolkit/README.md`
- [x] T022 Atualizar o status no `scenario-b/README.md` (toolkit: manifesto/validação = "In progress")

---

## Dependencies & Execution Order

- **Setup (T001–T003)** → **Foundational (T004–T007)** → **US1 (T008–T013)**.
- **US2 (T014–T016)** e **US3 (T017–T019)** dependem de US1, mas são **independentes entre si**
  (podem ser feitas em paralelo/em qualquer ordem após US1).
- **Polish (T020–T022)** por último.
- **MVP** = Setup + Foundational + **US1** (validação de manifesto único).

## Parallel Opportunities

- Setup: T002 e T003 em paralelo.
- Em cada story, a tarefa de testes `[P]` (T008 / T014 / T017) é escrita primeiro e em paralelo
  às demais preparações; as implementações no mesmo arquivo (`validate.go`) são sequenciais.
- Após US1: equipes podem tocar **US2 e US3 em paralelo**.
- Polish: T020 e T021 em paralelo.

## Implementation Strategy

1. Entregar o **MVP (US1)** primeiro — já dá valor (validação declarativa de um manifesto).
2. Incrementar com **US2** (colisões, essencial para N spokes) e **US3** (matriz por modo/pair).
3. Test-first em todas as stories (Constituição V): o teste falha, então implementa-se.

---

## Phase 7: Convergence

- [x] T023 Reconciliar a semântica de colisão **network-aware** implementada em `scenario-b/toolkit/engine/manifest/collision.go` (founder e join compartilham `chainId`/`spoke.id`; colisão só entre redes/founders distintos; portas globalmente únicas) com o texto de FR-008/SC-003 ("unicidade de `chainId`") — decidir entre **ajustar a redação de FR-008/SC-003** ao comportamento (recomendado; o código está alinhado ao `quickstart`, que valida found-spoke+join como conjunto válido) **ou** restringir o código à unicidade global de `chainId`; per FR-008, SC-003 (contradicts)
