---
description: "Task list — TK-B4 (templates de compose parametrizados)"
---

# Tasks: Toolkit do Cenário B — Templates de compose parametrizados (TK-B4)

**Input**: Design documents from `/specs/035-tk-b4-parametrized/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/template-contracts.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). A Constituição (Princípio V) e a spec exigem teste que falha
antes da implementação. Escreva cada teste, veja-o **falhar**, depois implemente.

**Organization**: tarefas agrupadas por user story (US1–US4), independentemente testáveis.

**Paths**: assets sob `scenario-b/provisioning/templates/`; validação Go no módulo
`scenario-b/toolkit` (`engine/composetemplate`). Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3 / US4

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: estrutura de diretórios (assets + pacote de validação) e a convenção base.

- [X] T001 Criar os diretórios `scenario-b/provisioning/templates/`, `scenario-b/provisioning/templates/vars/` e `scenario-b/toolkit/engine/composetemplate/`
- [X] T002 [P] Escrever a convenção determinística de portas (offset) e nomes em `scenario-b/provisioning/templates/vars/NAMING.md` (conforme contracts/template-contracts.md)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: o núcleo da validação (Load + Validate) que **todas** as user stories usam para
verificar seus templates.

**⚠️ CRITICAL**: nenhuma validação de template é possível antes disto.

- [X] T003 Teste do núcleo em `scenario-b/toolkit/engine/composetemplate/composetemplate_test.go`: `Load` faz parse de um template YAML; `Validate` detecta `${VAR}` obrigatório ausente (regra `interpolation`) e literal de segredo (regra `no-secrets`), usando `testdata/` mínimo
- [X] T004 Implementar `scenario-b/toolkit/engine/composetemplate/composetemplate.go`: `Load(path)`, `Validate(t, env)` com as regras `interpolation` e `no-secrets`, e os tipos `Result`/`RuleError` (conforme contracts/template-contracts.md)

**Checkpoint**: `go test ./engine/composetemplate/...` verde para o núcleo; user stories podem começar.

---

## Phase 3: User Story 1 — Templates parametrizados de hub e entidade (Priority: P1) 🎯 MVP

**Goal**: templates de compose para hub e as camadas de entidade (infra, keycloak, backend,
frontend) com todo valor discriminante externado por variável; interpolam por completo e não contêm
segredos.

**Independent Test**: cada template valida (interpolação completa, sem segredo) com seu
`vars/*.env.example`; variável obrigatória ausente falha explicitamente.

### Tests for User Story 1 ⚠️ (escrever primeiro, ver falhar)

- [X] T005 [P] [US1] Teste em `scenario-b/toolkit/engine/composetemplate/us1_templates_test.go`: cada um de hub/entity-infra/entity-keycloak/entity-backend/entity-frontend valida 100% interpolado com seu `.env.example` (SC-001) e sem segredos (SC-006); var obrigatória ausente → erro (SC-005)

### Implementation for User Story 1

- [X] T006 [P] [US1] `scenario-b/provisioning/templates/hub.compose.yaml` + `vars/hub.env.example` — derivado de `deploy/local/hub-besu/startBesu.sh` (hub-validator; `docker run` → serviço compose), valores por `${VAR:?}`, named volumes (sem bind host)
- [X] T007 [P] [US1] `scenario-b/provisioning/templates/entity-infra.compose.yaml` + `vars/entity-infra.env.example` — derivado de `deploy/local/compose.yml` (postgres, redis), named volumes por entidade
- [X] T008 [P] [US1] `scenario-b/provisioning/templates/entity-keycloak.compose.yaml` + `vars/entity-keycloak.env.example` — derivado de `deploy/local/compose.yml` (keycloak)
- [X] T009 [P] [US1] `scenario-b/provisioning/templates/entity-backend.compose.yaml` + `vars/entity-backend.env.example` — serviços de backend da entidade
- [X] T010 [P] [US1] `scenario-b/provisioning/templates/entity-frontend.compose.yaml` + `vars/entity-frontend.env.example` — app de frontend da entidade

**Checkpoint**: 5 templates de US1 validam isolados (`go test -run US1 ./engine/composetemplate/...`).

---

## Phase 4: User Story 2 — Persistência em named volumes determinísticos (Priority: P1)

**Goal**: todo estado de participante em named volumes determinísticos por spoke/entidade; única
exceção de bind host = `pki/` do banco.

**Independent Test**: a regra `named-volumes` confirma que estado usa named volume determinístico e
que o único bind host presente é `pki/`.

### Tests for User Story 2 ⚠️

- [X] T011 [US2] Teste em `scenario-b/toolkit/engine/composetemplate/us2_volumes_test.go`: a regra `named-volumes` rejeita bind mount de host para estado e aceita **apenas** o `pki/` do banco (SC-003); estado de nó/DB usa `name: ${..._VOLUME_PREFIX}_<papel>`

### Implementation for User Story 2

- [X] T012 [US2] Adicionar a regra `named-volumes` a `Validate` em `scenario-b/toolkit/engine/composetemplate/composetemplate.go` (named volume para estado; exceção `pki/`)
- [X] T013 [US2] Garantir nos templates de US1 (`hub`, `entity-infra`, `entity-backend`) named volumes determinísticos para todo estado (`besu_data`, `genesis`, `config`, `tls`, `pg_data`, `paladin_data`) e o bind mount `pki/` apenas no template do banco

**Checkpoint**: regra de volumes verde; templates de US1 conformes à política de persistência.

---

## Phase 5: User Story 3 — Endereçamento determinístico e alcance cross-stack (Priority: P2)

**Goal**: portas por offset e alcance cross-stack, de modo que N entidades coexistam sem colisão.

**Independent Test**: `CheckNoCollision` com dois envs de entidades distintas retorna 0 colisões; o
backend alcança o hub por rota parametrizada.

### Tests for User Story 3 ⚠️

- [X] T014 [US3] Teste em `scenario-b/toolkit/engine/composetemplate/us3_collision_test.go`: `CheckNoCollision` com dois envs (entidades distintas, de `testdata/`) → 0 colisão de porta/nome/rede/volume (SC-002); o `entity-backend` alcança o hub via `host.docker.internal:${HUB_RPC_PORT}` (FR-007)

### Implementation for User Story 3

- [X] T015 [US3] Adicionar `CheckNoCollision(t, envA, envB)` a `scenario-b/toolkit/engine/composetemplate/composetemplate.go` (regra `no-collision`)
- [X] T016 [US3] Garantir no `entity-backend.compose.yaml` o `extra_hosts: ["host.docker.internal:host-gateway"]` + rota `host.docker.internal:${HUB_RPC_PORT}`; adicionar `testdata/` com dois envs de exemplo (offsets distintos) coerentes com `vars/NAMING.md`

**Checkpoint**: duas entidades validam sem colisão; rota cross-stack parametrizada.

---

## Phase 6: User Story 4 — Templates de relay e NOC (Priority: P2)

**Goal**: templates parametrizados do relay e do NOC (stack do hub + `noc-agent` por entidade),
completando o conjunto `hub + entity-* + relay + NOC`.

**Independent Test**: relay e NOC validam (interpolação, sem spoke fixo, named volumes) com seus
`.env.example`.

### Tests for User Story 4 ⚠️

- [X] T017 [P] [US4] Teste em `scenario-b/toolkit/engine/composetemplate/us4_relay_noc_test.go`: `relay.compose.yaml` e `noc.compose.yaml` validam interpolados, sem identificador de spoke fixo (FR-010) e conformes às regras named-volumes/no-secrets

### Implementation for User Story 4

- [X] T018 [P] [US4] `scenario-b/provisioning/templates/relay.compose.yaml` + `vars/relay.env.example` — derivado do compose do relay em `interop/hub-and-spoke`, sem spokes fixos (registro em runtime é do TK-B5)
- [X] T019 [P] [US4] `scenario-b/provisioning/templates/noc.compose.yaml` + `vars/noc.env.example` — derivado de `deploy/local/compose.noc.yml` (noc-db/backend/portal + `noc-agent` parametrizável por entidade, sem `spoke-a`/`spoke-b`)

**Checkpoint**: os 7 templates existem e validam; conjunto completo.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T020 [P] Teste de integração opcional em `scenario-b/toolkit/engine/composetemplate/dockerconfig_test.go`: roda `docker compose -f <template> --env-file <example> config -q` para os 7 templates, com `t.Skip` se o Docker não estiver disponível
- [X] T021 [P] Verificação SC-004 em `scenario-b/toolkit/engine/composetemplate/deploylocal_test.go` (ou passo no quickstart): garantir que `scenario-b/deploy/local` permanece intocado (`git status --short` vazio para o path)
- [X] T022 `gofmt`/`go vet ./engine/composetemplate/...` e suíte `cd scenario-b/toolkit && go test ./engine/composetemplate/...`; validar os passos do `quickstart.md`
- [X] T023 [P] Atualizar nota de status TK-B4 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências.
- **Foundational (Phase 2)**: depende do Setup; **bloqueia** todas as user stories (Validate núcleo).
- **US1 (Phase 3)**: depois da Foundational. Cria os templates hub + entity-*.
- **US2 (Phase 4)**: depois de US1 (refina os mesmos templates com a política de volumes) e da
  Foundational (estende `Validate`).
- **US3 (Phase 5)**: depois de US1 (precisa dos templates) e da Foundational; adiciona
  `CheckNoCollision` + cross-stack.
- **US4 (Phase 6)**: depois da Foundational; **independente** de US2/US3 (templates novos). Pode
  correr em paralelo a US2/US3 uma vez que a Foundational esteja pronta.
- **Polish (Phase 7)**: depois dos templates desejados.

### Within Each User Story

- Teste escrito e **falhando** antes da implementação.
- Regra de validação antes de (ou junto com) o template que ela verifica.

### Parallel Opportunities

- Setup: T002 [P].
- US1: T006–T010 são [P] (arquivos de template distintos); o teste T005 cobre todos.
- US4: T018/T019 [P]; independente de US2/US3.
- Polish: T020/T021/T023 [P].

---

## Parallel Example: US1 (após a Foundational)

```bash
# Os 5 templates de entidade/hub em paralelo (arquivos distintos):
Task: "T006 hub.compose.yaml + vars"
Task: "T007 entity-infra.compose.yaml + vars"
Task: "T008 entity-keycloak.compose.yaml + vars"
Task: "T009 entity-backend.compose.yaml + vars"
Task: "T010 entity-frontend.compose.yaml + vars"
```

---

## Implementation Strategy

### MVP First (US1)

1. Phase 1 Setup → Phase 2 Foundational (Validate núcleo).
2. Phase 3 US1 (hub + entity-*) → `go test -run US1` verde.
3. **STOP & VALIDATE**: família de templates parametrizados interpola e não vaza segredo.

### Incremental Delivery

1. Setup + Foundational.
2. US1 (templates hub+entity-*) → MVP.
3. US2 (named volumes) → política de persistência.
4. US3 (offset + cross-stack) → multi-entidade sem colisão.
5. US4 (relay + NOC) → conjunto completo.
6. Polish (docker compose config opcional, SC-004, roadmap/README).

---

## Notes

- [P] = arquivos distintos, sem dependência pendente.
- **Não editar** `deploy/local/*` nem `scenario-b/Makefile`/`make/*.mk` (FR-003, SC-004).
- Sem cálculo de offset/nomes em código (fica no motor, TK-B6) — o TK-B4 só documenta a convenção
  (`NAMING.md`) e a valida.
- Sem novas dependências Go (reusa `gopkg.in/yaml.v3`); Docker Compose é opcional/só teste.
- Não importar `scenario-a/` (Princípio I).

---

## Phase 8: Convergence

> Origem: achado **C1** do `/speckit-analyze` (2026-07-10). Resolvido incluindo o template
> `entity-besu` (nó Besu + Paladin do spoke/banco) — lar do estado `besu_data`/`genesis`/`tls`/
> `paladin_data` que US2/§7 já pressupunham. Artefatos de design (spec/plan/data-model/contracts)
> atualizados; as tarefas abaixo entregam o template e sua cobertura de validação.

- [X] T024 [P] [US1] Criar `scenario-b/provisioning/templates/entity-besu.compose.yaml` + `vars/entity-besu.env.example` — nó Besu (+Paladin) do spoke/banco derivado de `deploy/local/*-besu/startBesu.sh`; valores por `${VAR:?}`; named volumes `besu_data`/`genesis`/`config`/`tls`/`paladin_data`; sem bind host per FR-001 (missing)
- [X] T025 [US1] Estender o teste de US1 em `scenario-b/toolkit/engine/composetemplate/us1_templates_test.go` para cobrir `entity-besu` (interpolação completa, sem segredos, var obrigatória ausente → erro) per SC-001/SC-006 (missing)
- [X] T026 [US2] Estender o teste/garantia de named volumes em `scenario-b/toolkit/engine/composetemplate/us2_volumes_test.go` para incluir `entity-besu` (estado em named volume determinístico; único bind host = `pki/` do banco) per SC-003 (missing)
