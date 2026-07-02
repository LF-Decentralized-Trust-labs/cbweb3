# Tasks: MD-3 — Atualizar Consumidores do Proto para Campos Spoke-Keyed (Scenario A)

**Input**: Design documents from `specs/022-update-proto-consumers/`
**Branch**: `022-update-proto-consumers`
**Organization**: Tasks agrupadas por User Story para entrega independente e incremental.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependências incompletas)
- **[Story]**: A qual User Story a task pertence (US1, US2, US3)

---

## Phase 1: Setup (Vendor + Proto Sync)

**Purpose**: Sincronizar o único artefato stale (`api-gateway` vendor) e confirmar que o proto-gen está idempotente. DEVE ser concluído antes de qualquer work por User Story.

- [x] T001 Atualizar vendor do api-gateway: `cd scenario-a/backend/services/api-gateway && go mod vendor` para sincronizar `vendor/github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go` com os novos campos (`SourceSpokeId`, `DestSpokeId`, `SourceReceiver`, `DestReceiver`)

- [x] T002 [P] Verificar idempotência do proto-gen: `cd scenario-a && make proto-gen` e confirmar que `backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go` não apresenta diff após a execução (proto já estava regenerado no MD-1)

- [x] T003 [P] Compilação Go completa do backend: `cd scenario-a/backend && go build ./...` — deve compilar sem erros após T001; qualquer erro indica referência residual a campos antigos

**Checkpoint**: Vendor atualizado, build limpa — prontos para work por story.

---

## Phase 2: Foundational (Baseline de Referências)

**Purpose**: Estabelecer baseline de grep que confirme estado atual e bloqueie regressões.

**⚠️ CRÍTICO**: Executar antes de qualquer implementação para ter linha de base clara.

- [x] T004 Executar grep de baseline de campos antigos em scenario-a: `grep -rn "spoke_a_receiver\|spoke_b_receiver\|SpokeAReceiver\|SpokeBReceiver" scenario-a/ --include="*.go" --include="*.ts" --include="*.tsx" --include="*.proto" --exclude-dir=vendor` e documentar os resultados (apenas `fx_agreement_gorm.go` e `gorm_repos_test.go` devem aparecer; qualquer outro arquivo é regressão)

- [x] T005 [P] Executar grep de campos antigos no vendor do api-gateway ANTES de T001: `grep -rn "SpokeAReceiver\|SpokeBReceiver" scenario-a/backend/services/api-gateway/vendor/` para confirmar que o vendor está de fato stale (esperado: aparecer; após T001: deve desaparecer)

**Checkpoint**: Baseline documentado. User Stories podem avançar em paralelo após Phase 1 completa.

---

## Phase 3: User Story 1 — Backend lê/escreve campos spoke-keyed (Priority: P1) 🎯 MVP

**Goal**: Confirmar e fechar cobertura de testes para que o `payment-orchestrator` do Scenario A aceite, persista e retorne corretamente `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver` via gRPC.

**Independent Test**: Chamar `ProposeFXAgreement` com os novos campos via gRPC e verificar que a resposta e o banco refletem os mesmos valores — `go test ./...` no payment-orchestrator deve passar com assertions explícitas nos novos campos.

- [x] T006 [US1] Ler `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/server_test.go` (ou equivalente) e identificar se existe ao menos um test case que asserte `SourceSpokeId`/`DestSpokeId`/`SourceReceiver`/`DestReceiver` na resposta de `ProposeFXAgreement`

- [x] T007 [US1] Adicionar test case em `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/server_test.go` para `ProposeFXAgreement` que: (a) envia request com `SourceSpokeId = "spoke-brl"`, `DestSpokeId = "spoke-usd"`, `SourceReceiver = "funded_operator@spoke-brl-bank-a"`, `DestReceiver = "funded_operator@spoke-usd-bank-b"`; (b) asserte que a response retorna os mesmos valores nos campos correspondentes — apenas se T006 identificar ausência do teste

- [x] T008 [P] [US1] Verificar que `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go` contém assertions para `SourceSpokeId`/`DestSpokeId` nos testes de round-trip de repositório (leitura após escrita dos novos campos)

- [x] T009 [US1] Executar `cd scenario-a/backend/services/payment-orchestrator && go test ./...` e confirmar 100% de sucesso — todos os testes devem passar incluindo os de migração (MD-2) e os novos de campos spoke-keyed

- [x] T010 [P] [US1] Verificar grep final no api-gateway vendor APÓS T001: `grep -n "SourceSpokeId\|DestSpokeId\|SourceReceiver\|DestReceiver" scenario-a/backend/services/api-gateway/vendor/github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go` — deve encontrar os quatro campos novos

**Checkpoint**: Backend do Scenario A lê, persiste e retorna campos spoke-keyed. US1 independentemente testável.

---

## Phase 4: User Story 2 — Relay roteia pelo dest_spoke_id (Priority: P1)

**Goal**: Confirmar que o relay Cacti do Scenario A lê `dest_spoke_id` do FX Agreement via REST e o propaga corretamente no gRPC para o spoke de destino, sem assumir posição fixa de contraparte.

**Independent Test**: Compilação TypeScript limpa do relay (`tsc --noEmit`) sem erros + verificação de que `proposeOnCounterpart` usa `event.destSpokeId` para selecionar o endpoint gRPC de destino.

- [x] T011 [US2] Ler função `proposeOnCounterpart` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` (linhas ~736-787) e verificar como o endpoint gRPC de destino é selecionado: usa `event.destSpokeId` para lookup dinâmico, ou usa `config.counterpartGrpc` estático?

- [x] T012 [US2] Se T011 revelar que o endpoint selection usa `config.counterpartGrpc` estático (bilateral): documentar que esta é a limitação conhecida do "quick path" (Fase 2, step 8 do plano geral) e que o roteamento dinâmico por `dest_spoke_id` é escopo de RL-1/RL-2. Registrar como `KNOWN_LIMITATION` em comentário no arquivo `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` acima de `proposeOnCounterpart`

- [x] T013 [P] [US2] Verificar que `pollFXAgreementsRest` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` extrai corretamente `source_spoke_id` e `dest_spoke_id` do payload REST (campos `a["source_spoke_id"]` / `a["dest_spoke_id"]`) — grep: `grep -n "source_spoke_id\|dest_spoke_id" scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`

- [x] T014 [P] [US2] Verificar que `proposeOnCounterpart` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` inclui `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` no payload gRPC enviado — grep: `grep -n "source_spoke_id\|dest_spoke_id\|source_receiver\|dest_receiver" scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`

- [x] T015 [US2] Executar `cd scenario-a/interop/hub-and-spoke/cacti && npx tsc --noEmit` e confirmar zero erros de tipo no relay — se houver erros relacionados a campos do FXAgreement, corrigi-los

**Checkpoint**: Relay compila limpo, parsing de legs usa novos campos. Limitação bilateral documentada explicitamente se existir.

---

## Phase 5: User Story 3 — Frontend Scenario A envia novos campos (Priority: P2)

**Goal**: Confirmar que o app `bank` do Scenario A envia `source_spoke_id`/`dest_spoke_id` (não os campos antigos) no payload de proposta de FX Agreement e que o type-check passa limpo.

**Independent Test**: `tsc --noEmit` no app `bank` sem erros + grep confirma ausência de `spoke_a_receiver`/`spoke_b_receiver` em código de produção do frontend.

- [x] T016 [US3] Executar grep no frontend do Scenario A: `grep -rn "spoke_a_receiver\|spoke_b_receiver\|spokeAReceiver\|spokeBReceiver" scenario-a/frontend/ --include="*.ts" --include="*.tsx"` — deve retornar zero ocorrências

- [x] T017 [P] [US3] Executar type-check do app bank: `cd scenario-a/frontend/apps/bank && npx tsc --noEmit` e confirmar zero erros relacionados a `fx-agreement.types.ts` ou `AgreementProposalPage.tsx`

- [x] T018 [P] [US3] Verificar que `scenario-a/frontend/apps/bank/src/types/fx-agreement.types.ts` contém `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` nas interfaces `FXAgreement` e `ProposeFXAgreementRequest` — e NÃO contém `spoke_a_receiver`/`spoke_b_receiver`

- [x] T019 [US3] Verificar que `scenario-a/frontend/apps/bank/src/pages/AgreementProposalPage.tsx` submete os quatro novos campos no payload via `propose({ ..., source_spoke_id, dest_spoke_id, source_receiver, dest_receiver })` — grep: `grep -n "source_spoke_id\|dest_spoke_id\|source_receiver\|dest_receiver" scenario-a/frontend/apps/bank/src/pages/AgreementProposalPage.tsx`

**Checkpoint**: Frontend Scenario A type-safe e enviando campos spoke-keyed. Todas as US independentemente verificadas.

---

## Phase 6: Polish & Validação Cross-Cutting

**Purpose**: Grep final de validação, atualização de documentação de status e confirmação end-to-end.

- [x] T020 [P] Executar grep final de validação de campos antigos em scenario-a (excluindo vendor, migration e testes de upgrade): `grep -rn "spoke_a_receiver\|spoke_b_receiver\|SpokeAReceiver\|SpokeBReceiver" scenario-a/ --include="*.go" --include="*.ts" --include="*.tsx" --exclude-dir=vendor | grep -v "_test.go" | grep -v "fx_agreement_gorm.go"` — deve retornar zero ocorrências

- [x] T021 [P] Executar suite completa de testes do backend Scenario A: `cd scenario-a/backend && go test ./...` — todos os testes devem passar

- [x] T022 [P] Executar type-check completo do frontend Scenario A: `cd scenario-a/frontend && npx turbo run type-check` — zero erros

- [x] T023 Verificar se `scenario-a/README.md` reflete corretamente o status de implementação de "spoke-keyed FX legs" como "Fully implemented" e atualizar se necessário

- [x] T024 [P] Executar os 7 passos de verificação do `specs/022-update-proto-consumers/quickstart.md` e confirmar todos passando

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1** (T001–T003): Sem dependências — pode começar imediatamente
- **Phase 2** (T004–T005): Pode rodar em paralelo com Phase 1 (usa apenas grep, sem build)
- **Phase 3** (T006–T010): Depende de Phase 1 completo (T001–T003) — vendor atualizado
- **Phase 4** (T011–T015): Pode iniciar em paralelo com Phase 3 após Phase 1 completa
- **Phase 5** (T016–T019): Pode iniciar em paralelo com Phases 3 e 4 após Phase 1 completa
- **Phase 6** (T020–T024): Depende de todas as fases anteriores

### User Story Dependencies

- **US1 (P1)**: Pode iniciar após Phase 1 — nenhuma dependência em US2 ou US3
- **US2 (P1)**: Pode iniciar após Phase 1 — nenhuma dependência em US1 ou US3
- **US3 (P2)**: Pode iniciar após Phase 1 — nenhuma dependência em US1 ou US2
- As três stories podem ser trabalhadas em paralelo por desenvolvedores diferentes

### Dentro de Cada User Story

- US1: T006 → T007 (se necessário) → T008 [P] → T009 → T010 [P]
- US2: T011 → T012 (se necessário) → T013 [P] + T014 [P] → T015
- US3: T016 → T017 [P] + T018 [P] → T019

---

## Parallel Example: Phase 1

```bash
# Rodar em paralelo (arquivos diferentes):
Task T001: cd scenario-a/backend/services/api-gateway && go mod vendor
Task T002: cd scenario-a && make proto-gen  (verificação idempotência)
Task T004: grep baseline de campos antigos  (apenas leitura)
Task T005: grep no vendor antes da atualização
```

## Parallel Example: User Stories após Phase 1

```bash
# Com dois desenvolvedores:
Dev A: US1 (T006→T009) — backend Go
Dev B: US2 (T011→T015) + US3 (T016→T019) — TypeScript relay e frontend
```

---

## Implementation Strategy

### MVP First (US1 — backend verificado)

1. Concluir Phase 1: Setup (T001–T003)
2. Concluir Phase 2: Baseline (T004–T005)
3. Concluir Phase 3: US1 backend (T006–T010)
4. **PARAR e VALIDAR**: `go test ./...` no payment-orchestrator — MVP pronto
5. Avançar para US2 e US3

### Entrega Incremental

1. Phase 1 + Phase 2 → Foundation pronta (vendor sincronizado, build limpa)
2. Phase 3 → Backend validado → PR parcial ou commit de checkpoint
3. Phase 4 → Relay validado → compilação e parsing confirmados
4. Phase 5 → Frontend validado → type-check limpo
5. Phase 6 → Validação cross-cutting → PR pronto para merge

---

## Notes

- A maioria do código de produção já está implementada — as tasks são majoritariamente de verificação e fechamento de cobertura de testes
- [P] tasks operam em arquivos diferentes e podem rodar concorrentemente
- T007 e T012 são condicionais: execute apenas se a verificação anterior (T006/T011) identificar lacuna
- O endpoint selection bilateral no relay (T012) é uma limitação conhecida e intencional do "quick path" — o roteamento dinâmico por `dest_spoke_id` é escopo de RL-1/RL-2 (Fase 2 do plano geral)
- Commit após cada checkpoint (T009, T015, T019, T024)

---

## Phase 7: Convergence

- [x] T025 Adicionar test case em `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/fx_agreement_test.go` que chama `ProposeFXAgreement` com `DestSpokeId = ""` e asserte que o retorno é `codes.InvalidArgument` com mensagem `"dest_spoke_id is required"` — e outro com `SourceSpokeId = ""` asserte a mensagem `"source_spoke_id is required"` per US1/AC3 (partial)

- [x] T026 Adicionar validação em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` na função `proposeOnCounterpart` (linha ~736): verificar que `event.destSpokeId` é igual ao nome do spoke contraparte configurado (`spoke.counterpartGrpc` pertence a este spoke); se não corresponder, logar `this.log.error(...)` com mensagem descritiva indicando spoke desconhecido e retornar sem executar a transação — satisfaz US2/AC3 sem depender do registro completo de RL-1 per US2/AC3 (missing)
