---
description: "Task list — TK-B5 (relay generalizado)"
---

# Tasks: Toolkit do Cenário B — Relay generalizado (TK-B5)

**Input**: Design documents from `/specs/036-tk-b5-generalized/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/relay-api.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). Escreva cada teste, veja-o **falhar**, depois implemente. Relay:
lógica pura via `node:test` (+ `ts-node`), sem Besu. Go: `go test`.

**Organization**: tarefas agrupadas por user story (US1–US5), independentemente testáveis.

**Paths**: relay em `scenario-b/interop/hub-and-spoke/cacti/` (TypeScript, edição cirúrgica §9);
`RelayRegistrar` em `scenario-b/toolkit/engine/relayregistrar/` (Go). Caminhos relativos à raiz.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3 / US4 / US5

---

## Phase 1: Setup

- [X] T001 [P] Adicionar script `test` (`node --import ts-node/register --test "src/**/*.test.ts"`) ao `scenario-b/interop/hub-and-spoke/cacti/package.json` e criar `scenario-b/interop/hub-and-spoke/cacti/src/__tests__/` (ou co-localizar `*.test.ts`)
- [X] T002 [P] Criar o diretório do pacote Go `scenario-b/toolkit/engine/relayregistrar/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: o `SpokeRegistry` (+ tipo `Spoke`) que US1–US4 usam.

**⚠️ CRITICAL**: bloqueia US1–US4 (todas dependem do registry). US5 (Go) não depende disto.

- [X] T003 Teste em `scenario-b/interop/hub-and-spoke/cacti/src/spoke-registry.test.ts`: registry vazio no boot; `upsert` idempotente por `spokeId` (re-upsert não duplica); `get`/`list`
- [X] T004 Implementar `scenario-b/interop/hub-and-spoke/cacti/src/spoke-registry.ts`: tipo `Spoke` (id, besuRpc, besuWs, gatewayUrl) + `SpokeRegistry` (`Map`, `upsert`, `get`, `list`, `hydrate`)

**Checkpoint**: `npm test` verde para o registry.

---

## Phase 3: User Story 1 — Boot neutro e registry dinâmico de N spokes (Priority: P1) 🎯 MVP

**Goal**: relay sobe sem spokes fixos e opera com N spokes do registry.

**Independent Test**: boot com 0 spokes OK; com N=1 e N=3, um conector/watcher por spoke; sem
`spoke-a`/`spoke-b` nem `SPOKE_A/B_BESU_RPC`.

### Tests for User Story 1 ⚠️

- [X] T005 [US1] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/config.test.ts`: `loadSpokesFromEnv` retorna `[]` no boot neutro (sem erro) e uma lista com N spokes quando configurado; nenhuma variável fatal `SPOKE_A/B_BESU_RPC`
- [X] T005b [US1] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/spoke-runtimes.test.ts`: `createSpokeRuntimes(spokes, factories)` com **factory injetável** cria **N runtimes para N spokes** (0→0, 1→1, 3→3) — verifica "um conector/watcher por spoke" (SC-001) sem Besu real (achado C1)

### Implementation for User Story 1

- [X] T006 [US1] Reescrever `scenario-b/interop/hub-and-spoke/cacti/src/config.ts`: remover `spokeA`/`spokeB` e os `requireEnv("SPOKE_A/B_BESU_RPC")`; adicionar `loadSpokesFromEnv()` (lista dinâmica, ≥0) mantendo `apiPort`
- [X] T007 [US1] Extrair a fiação em `scenario-b/interop/hub-and-spoke/cacti/src/spoke-runtimes.ts` (`createSpokeRuntimes(spokes, factories)` — função pura, factory injetável) e ajustar `src/index.ts` para hidratar o registry (env + store), chamar `createSpokeRuntimes` **em loop** por spoke, registrar web services por spoke, boot neutro; remover os dois literais `connectorSpokeA/B`

**Checkpoint**: relay sobe com 0 e com N spokes; sem literais de spoke.

---

## Phase 4: User Story 2 — Registro de spokes em runtime, persistido (Priority: P1)

**Goal**: `POST /api/v1/spokes` registra em runtime (sem restart), persistido e recarregado no boot.

**Independent Test**: POST registra sem restart; reinício recarrega do store; payload inválido → 400.

### Tests for User Story 2 ⚠️

- [X] T008 [US2] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/relay-store.test.ts`: `load()` em arquivo ausente → vazio (sem erro); `save()`+`load()` round-trip; escrita atômica (tmp+rename)
- [X] T009 [US2] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/spokes-api.test.ts`: handler valida payload (id/rpc/ws/gateway); upsert idempotente; payload inválido → 400 e store inalterado

### Implementation for User Story 2

- [X] T010 [US2] Implementar `scenario-b/interop/hub-and-spoke/cacti/src/relay-store.ts`: RelayStore JSON (`load`/`save` atômico; caminho configurável); ignora o `cacti-relay-store.json` legado
- [X] T011 [US2] Implementar `scenario-b/interop/hub-and-spoke/cacti/src/spokes-api.ts`: handler `POST /api/v1/spokes` (valida → `registry.upsert` → `store.save` → hidrata conector/watcher/rota em runtime); idempotente
- [X] T012 [US2] Fiar no `scenario-b/interop/hub-and-spoke/cacti/src/index.ts`: `store.load()` no boot (hidrata registry) e registrar a rota `POST /api/v1/spokes`

**Checkpoint**: registro runtime persistido; recarrega pós-reinício.

---

## Phase 5: User Story 3 — Roteamento cross-currency por spoke de destino (Priority: P1)

**Goal**: encaminhar `bridge-out` ao gateway do `spoke_out` (lookup), não a um CB fixo.

**Independent Test**: dois spokes com gateways distintos → cada `spoke_out` roteia ao seu gateway;
`spoke_out` desconhecido → erro claro.

### Tests for User Story 3 ⚠️

- [X] T013 [US3] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/cross-currency-swap-relay.test.ts`: `resolveGateway(spoke_out)` retorna o gateway do registry; `spoke_out` desconhecido → erro; encaminhamento usa o gateway resolvido (fetch injetável). O payload inclui `amm_address` (repassado ao gate do breaker)

### Implementation for User Story 3

- [X] T014 [US3] Alterar `scenario-b/interop/hub-and-spoke/cacti/src/cross-currency-swap-relay.ts`: substituir `forwardToCBB`/`cbBGatewayUrl` fixo por **lookup de gateway por `spoke_out`** no registry; `spoke_out` desconhecido → rejeição com erro claro

**Checkpoint**: roteamento por `spoke_out` para N spokes.

---

## Phase 6: User Story 4 — Validação de circuit breaker antes do swap (Priority: P1)

**Goal**: consultar `isPaused()` no AMM do par antes de encaminhar; recusar se pausado; falha segura.

**Independent Test**: par pausado → recusa; consulta indisponível → recusa; ativo → encaminha.

### Tests for User Story 4 ⚠️

- [X] T015 [US4] Teste em `scenario-b/interop/hub-and-spoke/cacti/src/circuit-breaker.test.ts`: `checkNotPaused(ammAddress, provider)` com provider injetável → `paused=true` recusa; `ammAddress` ausente/inválido recusa; erro/timeout recusa (falha segura); `paused=false` permite

### Implementation for User Story 4

- [X] T016 [US4] Implementar `scenario-b/interop/hub-and-spoke/cacti/src/circuit-breaker.ts`: `checkNotPaused(ammAddress, provider)` — lê `isPaused()` on-chain no `ammAddress` via ethers v6; `ammAddress` ausente/inválido ou erro/timeout → tratado como pausado (falha segura)
- [X] T017 [US4] Integrar o gate no `scenario-b/interop/hub-and-spoke/cacti/src/cross-currency-swap-relay.ts`: extrair `amm_address` do payload e consultar `checkNotPaused` **antes** de encaminhar; recusar com motivo "circuit breaker"; log estruturado (FR-009)

**Checkpoint**: swaps recusados quando pausado; Princípio III satisfeito.

---

## Phase 7: User Story 5 — Interface `RelayRegistrar` no toolkit (Priority: P2)

**Goal**: fronteira Go plugável para registrar spokes (local in-memory + stub HTTP), factory por URI.

**Independent Test**: local registra idempotente + `List()`; factory `local` vs `relay://…`; URI
inválida → erro; `Spoke` inválido → `ErrInvalidSpoke`.

### Tests for User Story 5 ⚠️

- [X] T018 [P] [US5] Teste em `scenario-b/toolkit/engine/relayregistrar/relayregistrar_test.go`: local `Register` idempotente + `List`; `New("local")`/`New("relay://h:4000")`/URI inválida; `Spoke` inválido → `ErrInvalidSpoke`

### Implementation for User Story 5

- [X] T019 [P] [US5] `scenario-b/toolkit/engine/relayregistrar/relayregistrar.go`: interface `RelayRegistrar` + `LocalRegistry` + tipo `Spoke` + erros tipados (conforme contracts/relay-api.md)
- [X] T020 [US5] `scenario-b/toolkit/engine/relayregistrar/local.go`: impl in-memory (`map[id]Spoke` + lock, upsert idempotente, `List`)
- [X] T021 [US5] `scenario-b/toolkit/engine/relayregistrar/prod.go`: stub de produção — `Register` faz `POST /api/v1/spokes` via `net/http`; sem endpoint → `ErrNotImplemented`
- [X] T022 [US5] `scenario-b/toolkit/engine/relayregistrar/factory.go`: `New(uri)` — `local` → in-memory; `relay://…` → prod; else `ErrUnsupportedURI`

**Checkpoint**: `go test ./engine/relayregistrar/...` verde.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T023 [P] Ajustar `scenario-b/interop/hub-and-spoke/cacti/env-sample` e `docker-compose.yaml` para o modelo dinâmico (sem `SPOKE_A/B_*` fatais; boot neutro) — FR-011
- [X] T024 [P] Documentar no `scenario-b/interop/hub-and-spoke/cacti/README.md`: auth-por-CB fora de escopo (§14.D, segredo compartilhado como fallback) e que o `cacti-relay-store.json` legado não é usado — FR-010
- [X] T025 Rodar as suítes: `cd scenario-b/interop/hub-and-spoke/cacti && npm test`; `cd scenario-b/toolkit && go test ./...`; validar os passos do `quickstart.md`
- [X] T026 [P] Atualizar nota de status TK-B5 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências.
- **Foundational (Phase 2)**: depois do Setup; **bloqueia** US1–US4 (registry).
- **US1 (Phase 3)**: depois da Foundational. Toca `config.ts`/`index.ts`.
- **US2 (Phase 4)**: depois de US1 (usa o registry hidratado e fia no `index.ts`).
- **US3 (Phase 5)**: depois de US1 (usa o registry para lookup). Toca `cross-currency-swap-relay.ts`.
- **US4 (Phase 6)**: depois de US3 (integra o gate no mesmo `cross-currency-swap-relay.ts`) e da Foundational.
- **US5 (Phase 7)**: **independente** do relay — só depende do Setup (T002). Pode correr em paralelo a US1–US4.
- **Polish (Phase 8)**: depois das user stories desejadas.

### Within Each User Story

- Teste escrito e **falhando** antes da implementação (test-first).
- Lógica pura antes da fiação (`index.ts` fino).

### Parallel Opportunities

- Setup: T001 e T002 [P].
- **US5 inteira** pode correr em paralelo ao relay (linguagem/arquivos distintos).
- Testes [P] em arquivos distintos.
- Polish: T023/T024/T026 [P].

---

## Parallel Example: US5 em paralelo ao relay

```bash
# Após o Setup, a trilha Go (US5) corre independente da trilha TS (US1–US4):
Task: "T018 relayregistrar_test.go"     # US5 (Go)
Task: "T003 spoke-registry.test.ts"     # Foundational (TS)
```

---

## Implementation Strategy

### MVP First (US1)

1. Setup → Foundational (registry).
2. US1 (boot neutro + N spokes) → `npm test` verde; relay sobe sem hardcode.
3. **STOP & VALIDATE**: bloqueador central (§14.B) resolvido.

### Incremental Delivery

1. Setup + Foundational.
2. US1 (registry dinâmico) → MVP.
3. US2 (registro runtime persistido).
4. US3 (roteamento por `spoke_out`).
5. US4 (circuit breaker — Princípio III).
6. US5 (RelayRegistrar Go) — em paralelo quando houver capacidade.
7. Polish (env/compose, docs, suítes, roadmap/README).

---

## Notes

- [P] = arquivos distintos, sem dependência pendente.
- **Não** implementar auth-por-CB (§14.D — pendente do project-lead); manter segredo compartilhado (FR-010).
- **Não** alterar a lógica de circuit breaker dos contratos — só **validar** `isPaused` no relay (Princípio III).
- `isPaused` indisponível ⇒ **falha segura** (não encaminhar).
- Sem novas dependências (relay reusa express/ethers/cactus; testes `node:test`; Go stdlib).
- Não importar `scenario-a/` (Princípio I).
