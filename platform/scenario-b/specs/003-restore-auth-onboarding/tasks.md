# Tasks: Restaurar Auth e Onboarding (Scenario A)

**Feature**: `003-restore-auth-onboarding` | **Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06  
**Input**: Design documents from `/specs/003-restore-auth-onboarding/`  
**Prerequisites**: [plan.md](plan.md) ✅ | [spec.md](spec.md) ✅ | [research.md](research.md) ✅ | [data-model.md](data-model.md) ✅ | [contracts/onboarding-api.md](contracts/onboarding-api.md) ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependências de tarefas incompletas)
- **[Story]**: US1 a US5 — mapeia diretamente às User Stories do spec.md
- Todos os caminhos de arquivo são absolutos a partir da raiz do repositório

---

## Phase 1: Setup — Remover Bloqueio de Startup

**Propósito**: Remover o código de cleanup do Scenario A que impede o API Gateway de iniciar corretamente.  
O erro `"relation swap_order_scenario_b does not exist"` ocorre porque `DropScenarioATables` tenta dropar tabelas que nunca existiram no DB do API Gateway.

**⚠️ CRÍTICO**: Sem esta fase, o container do API Gateway falha no startup e nenhuma rota pode ser testada.

- [X] T001 Deletar o arquivo `backend/services/api-gateway/internal/db/init/cleanup_scenarioa.go` inteiro (motivo: tabelas referenciadas nunca existiram no DB do API Gateway; Scenario B é ambiente 100% novo — ver D-002 e D-007 em research.md)

- [X] T002 Remover a chamada `dbinit.DropScenarioATables(db)` do arquivo `backend/services/api-gateway/internal/app/app.go` e remover o import de `dbinit` se tornar não utilizado após a remoção

**Checkpoint**: Após T001 e T002, o API Gateway deve compilar e iniciar sem erros de "relation does not exist".

---

## Phase 2: Foundational — Restaurar Roteamento de Onboarding

**Propósito**: Reconectar os handlers de onboarding ao router e ao app.go. Estes handlers já existem e são funcionais; estão apenas desconectados do router desde o commit `9831d50`.

**⚠️ CRÍTICO**: Esta fase desbloqueia US1, US2, US3 e US4 simultaneamente. Nenhuma user story pode ser validada sem esta fase completa.

- [X] T003 Adicionar `OnboardingHandler *handlers.OnboardingHandler` e `OnboardingProxyHandler *handlers.OnboardingProxyHandler` à struct `Dependencies` em `backend/services/api-gateway/internal/http/router/router.go` (referência: campo `AuthHandler` já existente na mesma struct)

- [X] T004 Restaurar o bloco de rotas de onboarding em `backend/services/api-gateway/internal/http/router/router.go`: registrar condicionalmente as rotas `/api/v1/onboarding/initiate` (POST), `/api/v1/onboarding/status/:requestId` (GET), `/api/v1/onboarding/my-status` (GET) e `/api/v1/onboarding/complete` (POST) — quando `deps.OnboardingProxyHandler != nil`, usar `OnboardingProxyHandler`; senão usar `OnboardingHandler`; rotas do banco comercial protegidas por `RequireCookieAuth`

- [X] T005 Restaurar a rota `/api/v1/auth/pki-login` (POST) no grupo de auth em `backend/services/api-gateway/internal/http/router/router.go` — disponível apenas quando `deps.OnboardingProxyHandler != nil` (bancos comerciais); adicionar import `"os"` se necessário para `os.Getenv("INTERNAL_RELAY_AUTH_SECRET")`

- [X] T006 Restaurar a instanciação condicional dos handlers em `backend/services/api-gateway/internal/app/app.go`: quando `cfg.CentralBankAPIURL != ""`, instanciar `handlers.NewOnboardingProxyHandler(cfg.CentralBankAPIURL, cfg.PKIDir, cfg.BankCode, identityManager)` e atribuir a `deps.OnboardingProxyHandler`; senão, instanciar `handlers.NewOnboardingHandler(identityManager)` e atribuir a `deps.OnboardingHandler`

**Checkpoint**: `go build ./...` no serviço `backend/services/api-gateway` deve compilar sem erros. O endpoint `POST /api/v1/onboarding/initiate` deve retornar 201 (e não 404) para Bank-A.

---

## Phase 3: User Story 1 — Banco Comercial completa onboarding (Priority: P1) 🎯 MVP

**Goal**: Todos os 4 bancos comerciais conseguem completar o fluxo de onboarding de 3 fases (Credencial → KYC → Blockchain) nos seus spokes sem erros HTTP 404 ou 500.

**Independent Test**: Executar `tryouts/tryout-spoke-a-bank-a.sh` com o spoke-a no ar e verificar que completa com `FAIL=0`. Repetir para `tryout-spoke-b-bank-b.sh`.

### Implementação para User Story 1

- [X] T007 [US1] Verificar que `go build ./...` compila sem erros para todos os serviços afetados: `backend/services/api-gateway`, `backend/services/auth` e `backend/services/compliance` — executar localmente ou via `make build` se disponível

- [ ] T008 [US1] Verificar que a rota `POST /api/v1/onboarding/initiate` retorna HTTP 201 com `request_id` para Bank-A (porta 18080) após login Keycloak — confirmar que a resposta contém os campos `request_id` e `user_id` conforme contrato em `specs/003-restore-auth-onboarding/contracts/onboarding-api.md`

- [ ] T009 [US1] Verificar que a rota `GET /api/v1/onboarding/status/:requestId` retorna HTTP 200 com o status `CREDENTIAL_REQUESTED` usando o `request_id` obtido no T008 — confirmar que o banco Bank-A está com status correto no compliance service

**Checkpoint**: US1 está completamente funcional — fluxo de 3 fases conclui sem erros para Bank-A e Bank-B.

---

## Phase 4: User Story 2 — Recuperar status sem request_id (Priority: P2)

**Goal**: Após recarregar sessão, o banco recupera seu status de onboarding via `GET /api/v1/onboarding/my-status` sem precisar do `request_id` original.

**Independent Test**: Autenticar como Bank-A, chamar `GET /api/v1/onboarding/my-status` sem parâmetros e verificar resposta HTTP 200 com o status atual. Chamar como banco que nunca iniciou e verificar `{"status": "NONE"}` com HTTP 200.

### Implementação para User Story 2

> **Nota**: Nenhuma alteração de código é necessária além do Tier 1 (Phase 2 Foundational). O endpoint `GET /api/v1/onboarding/my-status` e o método `GetOnboardingStatusByBankCode` já existem e são funcionais em `backend/services/api-gateway/internal/adapters/identity/identity_grpc_manager.go`. Esta fase valida que o endpoint funciona corretamente após a reconexão do router.

- [ ] T010 [US2] Verificar que `GET /api/v1/onboarding/my-status` retorna HTTP 200 com `{"status": "CREDENTIAL_REQUESTED"}` (ou o status atual) para Bank-A autenticado — o claim `BankID` do JWT deve ser usado para resolver o `bank_code` sem parâmetro explícito

- [ ] T011 [US2] Verificar que `GET /api/v1/onboarding/my-status` retorna HTTP 200 com `{"status": "NONE"}` quando chamado por um banco que nunca iniciou onboarding — confirmar que não retorna HTTP 404

**Checkpoint**: US2 funcional — banco recupera status sem `request_id` para qualquer banco (A, B, C, D).

---

## Phase 5: User Story 3 — Central Bank aprova/rejeita KYC (Priority: P2)

**Goal**: O Central Bank consegue aprovar KYC (gerando `pop_nonce`) e rejeitar KYC (preenchendo `rejection_reason`). Transições de estado são auditadas via `audit_logs`. Banco com `KYC_REJECTED` pode resubmeter gerando novo `request_id`.

**Independent Test**: Autenticar como operador de governança do Central Bank, chamar `POST /api/v1/compliance/approve-kyc` com o `user_id` de Bank-A, depois verificar que `GET /api/v1/onboarding/status/:requestId` retorna status `KYC_APPROVED` com campo `pop_nonce`. Repetir para rejeição.

### Implementação para User Story 3

- [X] T012 [P] [US3] Adicionar campo `RejectionReason string` ao struct `ParticipantModel` em `backend/services/compliance/internal/repository/models.go` com tag GORM `gorm:"column:rejection_reason;type:text"` — o GORM `AutoMigrate` no startup criará a coluna automaticamente

- [X] T013 [P] [US3] Adicionar chamada `s.emitAudit(ctx, "CREDENTIAL_REQUEST", ...)` ao final do handler `SubmitCredentialRequest` em `backend/services/auth/internal/grpc/server/onboarding.go` — após a chamada a `s.compliance.UpsertParticipant(...)` bem-sucedida; usar o mesmo padrão de `emitAudit` já existente em `CompleteOnboarding` no mesmo arquivo

- [X] T014 [US3] Localizar o handler de aprovação KYC em `backend/services/compliance/internal/grpc/server/server.go` e adicionar chamada `s.emitAudit(ctx, "KYC_APPROVED", ...)` após a transição de estado para `KYC_APPROVED` bem-sucedida — usar os campos `actor_user_id` (operador de governança) e `target_subject` (user_id do banco)

- [X] T015 [US3] Localizar o handler de rejeição KYC em `backend/services/compliance/internal/grpc/server/server.go` e adicionar chamada `s.emitAudit(ctx, "KYC_REJECTED", ...)` após a transição de estado para `KYC_REJECTED` bem-sucedida — garantir que o campo `rejection_reason` é persistido no `ParticipantModel` junto com a mudança de status

**Checkpoint**: US3 funcional — Central Bank aprova e rejeita KYC; campo `rejection_reason` salvo; todas as transições auditadas em `audit_logs`.

---

## Phase 6: User Story 4 — Login PKI de dois fatores (Priority: P3)

**Goal**: Bancos comerciais com role `ROLE_COMMERCIAL_BANK` fazem login com `clientId`+`clientSecret` e recebem um `nonce` (não um token direto). O nonce tem TTL de 30 minutos. Após assinar com a chave X.509, recebem o JWT de sessão.

**Independent Test**: Chamar `POST /api/v1/auth/login` com credenciais de Bank-A e verificar que a resposta contém `{"nonce": "..."}` sem JWT. Depois assinar o nonce e chamar `POST /api/v1/auth/pki-login` para obter o token.

### Implementação para User Story 4

> **Nota**: A rota `POST /api/v1/auth/pki-login` já é restaurada pela Phase 2 (T005). Esta fase foca exclusivamente na correção do TTL do nonce PKI de 5 para 30 minutos.

- [X] T016 [US4] Alterar o TTL do nonce PKI em `backend/services/auth/internal/grpc/server/server.go` na linha que contém `s.nonceStore.Set(ctx, req.UserId, stored, 5*time.Minute)` — substituir `5*time.Minute` por `30*time.Minute`; nenhuma outra mudança necessária neste arquivo

**Checkpoint**: US4 funcional — login PKI retorna nonce com TTL de 30 minutos; operadores de governança recebem JWT diretamente (fluxo sem nonce).

---

## Phase 7: User Story 5 — Scripts de tryout para todos os bancos (Priority: P3)

**Goal**: Os scripts `tryout-my-onboarding-status.sh` e os de spoke funcionam corretamente para Bank-B, Bank-C e Bank-D (não apenas Bank-A).

**Independent Test**: Executar `BANK_URL=http://localhost:28080 BANK_ENV=backend/config/bank-b.env bash tryouts/tryout-my-onboarding-status.sh` e verificar `FAIL=0`.

### Implementação para User Story 5

- [X] T017 [US5] Corrigir `tryouts/tryout-my-onboarding-status.sh` linha 199: substituir o valor hardcoded `"bank-a-client"` no campo `clientId` do payload de login por leitura dinâmica da variável `KC_CLIENT_ID` do arquivo `$BANK_ENV` — usar `grep KC_CLIENT_ID "$BANK_ENV" | cut -d= -f2-` para ler o valor; declarar variável local `kc_client_id` antes do uso; garantir que o payload usa `"$kc_client_id"` interpolado

**Checkpoint**: US5 funcional — `tryout-my-onboarding-status.sh` executa com `PASS=4, FAIL=0` para qualquer banco (A, B, C, D).

---

## Phase Final: Polish & Cross-Cutting Concerns

**Propósito**: Validação final do ambiente completo e verificação de zero regressão.

- [ ] T018 [P] Executar o script `tryouts/tryout-my-onboarding-status.sh` para os 4 bancos (A, B, C, D) e confirmar que todos retornam `PASS=4, FAIL=0` — este é o critério SC-003 do spec.md

- [ ] T019 [P] Verificar que as rotas do Scenario B v2 (`/api/v2/pool`, `/api/v2/swap`, `/api/v2/bridge`) continuam respondendo corretamente após todas as mudanças — zero regressões (critério SC-004 do spec.md); executar qualquer teste de smoke do Scenario B disponível

- [ ] T020 Verificar o fluxo de onboarding completo de 3 fases para ao menos um banco de cada spoke (Bank-A no spoke-A, Bank-B no spoke-B) usando `tryouts/tryout-spoke-a-bank-a.sh` e `tryouts/tryout-spoke-b-bank-b.sh` com os containers no ar — critério SC-001 e SC-002

- [ ] T021 [P] Medir latência de `GET /api/v1/onboarding/my-status` para um banco autenticado: executar `time curl -s -o /dev/null -w "%{http_code} %{time_total}s" -H "Cookie: ..." http://localhost:18080/api/v1/onboarding/my-status` e confirmar que `time_total < 1.0s` — critério SC-005; documentar o resultado no output do tryout

- [ ] T022 [P] Medir latência do fluxo PKI completo (nonce → assinatura → pki-login → token): executar os 3 passos de `POST /auth/login`, assinar nonce e `POST /auth/pki-login` com `time` e confirmar que o total é menor que 5 segundos em rede local — critério SC-006; documentar o resultado

---

## Dependencies & Execution Order

### Dependências entre Fases

```
Phase 1 (Setup)
    └──► Phase 2 (Foundational)   ← BLOQUEIA todas as user stories
              ├──► Phase 3 (US1) — P1 — MVP
              ├──► Phase 4 (US2) — P2 — Independente de US1
              ├──► Phase 5 (US3) — P2 — Independente de US1/US2
              ├──► Phase 6 (US4) — P3 — Independente de US1/US2/US3
              └──► Phase 7 (US5) — P3 — Melhor após US1-US4 para teste completo
                        └──► Phase Final (Polish)
```

### Dependências entre User Stories

- **US1 (P1)**: Inicia após Phase 2 (Foundational) — nenhuma dependência de outras stories
- **US2 (P2)**: Inicia após Phase 2 (Foundational) — independente de US1 (mesma reconexão de router)
- **US3 (P2)**: Inicia após Phase 2 (Foundational) — independente de US1/US2; mudanças nos serviços compliance/auth
- **US4 (P3)**: Inicia após Phase 2 (Foundational) + T005 (pki-login route) — 1 linha de mudança
- **US5 (P3)**: Independente de todas as outras; pode ser feita a qualquer momento após Phase 2

### Dentro de Cada Fase

- T001 e T002 podem ser feitas em paralelo (arquivos diferentes)
- T003, T004, T005 dentro da Phase 2 devem ser feitas sequencialmente (mesmo arquivo `router.go`)
- T006 pode ser feita em paralelo com T003-T005 (arquivo `app.go` diferente de `router.go`)
- T012 e T013 (Phase 5) podem ser feitas em paralelo (arquivos diferentes)
- T014 e T015 devem ser feitas sequencialmente (mesmo arquivo `compliance/server.go`)

### Oportunidades de Paralelismo

- **Depois de Phase 2**: US3 (compliance changes) pode ser trabalhada em paralelo com US4 (nonce TTL) e US5 (script fix) — são arquivos completamente distintos
- **Dentro de US3**: T012 (compliance models) e T013 (auth onboarding.go) são paralelos [P]
- **Phase Final**: T018 e T019 podem ser executados em paralelo

---

## Exemplo de Execução Paralela: Após Phase 2 (Foundational)

```bash
# Terminal 1 — US3: Compliance changes
git checkout -b feat/us3-compliance-changes
# T012: models.go (add RejectionReason)
# T013: auth/onboarding.go (emitAudit CREDENTIAL_REQUEST)
# T014: compliance/server.go (emitAudit KYC_APPROVED)
# T015: compliance/server.go (emitAudit KYC_REJECTED)

# Terminal 2 — US4: Nonce TTL (1 linha)
git checkout -b feat/us4-nonce-ttl
# T016: auth/server.go:460 (5min → 30min)

# Terminal 3 — US5: Script fix
git checkout -b feat/us5-script-fix
# T017: tryout-my-onboarding-status.sh linha 199
```

---

## Implementation Strategy

### MVP Scope (apenas Phase 1 + Phase 2 + Phase 3)

O **MVP** para esta feature é completar as fases 1, 2 e 3 (T001–T009):
1. Remover o bloqueio de startup (`cleanup_scenarioa.go`)
2. Reconectar handlers ao router e ao `app.go`
3. Validar que o fluxo completo de 3 fases funciona para Bank-A

Após o MVP, todos os 4 bancos já podem fazer onboarding completo. As fases 4-7 são melhorias de conformidade, auditoria e qualidade operacional.

### Delivery Order Recommendation

1. **Imediato (MVP)**: T001, T002, T003, T004, T005, T006 — desbloqueia todos os bancos
2. **Curto prazo**: T016 (nonce TTL 30min) e T017 (script fix) — rápidos, 1 linha cada
3. **Médio prazo**: T012-T015 (compliance: rejection_reason + auditoria) — mudança em 3 arquivos
4. **Validação final**: T018-T020 (smoke tests)
