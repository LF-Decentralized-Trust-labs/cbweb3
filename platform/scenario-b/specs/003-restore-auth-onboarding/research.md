# Research: Restaurar Auth e Onboarding (Scenario A)

**Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06

---

## Sumário Executivo

Toda a lógica de auth e onboarding **já existe** na branch atual (`fix/002-scenario-b-liquidity`). O problema é exclusivamente de **fiação** (wiring): os handlers existentes foram desconectados do router e do `app.go` no commit `9831d50`. A restauração é cirúrgica — sem reescrita de lógica de negócio.

---

## Decisões e Racionais

### D-001: Extensão do escopo — Nonce TTL atual vs especificado

**Decisão**: Alterar o TTL do nonce PKI de 5 minutos (valor hardcoded atual) para 30 minutos (valor definido na spec).

**Localização**: `backend/services/auth/internal/grpc/server/server.go`, linha 460:
```go
if err := s.nonceStore.Set(ctx, req.UserId, stored, 5*time.Minute); err != nil {
```

**Racional**: O nonce é armazenado no Redis (produção) ou in-memory (dev) via `noncestore.NonceStore`. A mudança é de 1 linha. Nenhuma migração de schema necessária.

**Alternativas rejeitadas**: Tornar o TTL configurável via env var — YAGNI para esta feature; pode ser adicionado depois.

---

### D-002: Tabela `onboarding_requests` — não está na API Gateway

**Decisão**: A tabela `onboarding_requests` referenciada em `cleanup_scenarioa.go` **nunca existiu no banco de dados da API Gateway**. O estado de onboarding é persistido na tabela `participants` do **compliance service**.

**Evidência**: 
- `backend/services/compliance/internal/repository/models.go` → `ParticipantModel` tem campos `status`, `pop_nonce`, `pop_nonce_expires_at`, `csr_pem`, `pop_nonce_expires_at` — todos os dados do ciclo de onboarding
- `backend/services/auth/internal/grpc/server/onboarding.go` → chama `s.compliance.UpsertParticipant(...)` para toda persistência de estado

**Implicação**: `DropScenarioATables` (e o arquivo `cleanup_scenarioa.go`) deve ser **removido completamente** do API Gateway. A chamada em `app.go` (`dbinit.DropScenarioATables(db)`) também deve ser removida.

**Racional**: Scenario B é ambiente 100% novo. Não há resíduos do Scenario A a limpar no DB do API Gateway.

---

### D-003: Tabela `onboarding_events` (FR-013) — coberta por `audit_logs`

**Decisão**: Não criar uma nova tabela `onboarding_events`. A auditoria de transições de estado do onboarding já é coberta pela tabela `audit_logs` do compliance service.

**Evidência**:
- `backend/services/compliance/internal/repository/models.go` → `AuditLogModel` com campos `action_type`, `actor_subject`, `target_subject`, `result`, `details` (jsonb)
- `backend/services/auth/internal/grpc/server/onboarding.go` → `s.emitAudit(ctx, "COMPLETE_ONBOARDING", ...)` já existe na fase 3
- `s.emitAudit()` persiste na tabela `audit_logs` do compliance service

**Gap identificado**: Os eventos de Fase 1 (`SubmitCredentialRequest`) e de aprovação/rejeição KYC ainda não emitem `emitAudit()`. Estas chamadas devem ser adicionadas para cumprir FR-013.

**Racional**: Reutilizar infraestrutura existente (YAGNI); evitar tabela duplicada.

---

### D-004: Handlers existem — apenas desconectados do router

**Decisão**: Nenhum handler precisa ser reescrito. Todos os arquivos necessários existem na branch atual:

| Arquivo | Status |
|---|---|
| `internal/http/handlers/onboarding.go` | ✅ Existe, funcional |
| `internal/http/handlers/onboarding_proxy.go` | ✅ Existe, funcional |
| `internal/adapters/identity/identity_grpc_manager.go` | ✅ Implementa `OnboardingManager` e `OnboardingKeyManager` |
| `internal/interfaces/kyc_checker.go` | ✅ Define todas as interfaces necessárias |
| `internal/http/router/router.go` | ❌ Removeu `OnboardingHandler`, `OnboardingProxyHandler` da struct `Dependencies` e as rotas |
| `internal/app/app.go` | ❌ Não instancia `OnboardingProxyHandler`/`OnboardingHandler` |
| `internal/db/init/cleanup_scenarioa.go` | ❌ Deve ser removido completamente |

**Racional**: Minimizar risco — re-conectar é muito menos arriscado que reescrever.

---

### D-005: Coexistência Scenario A e Scenario B no mesmo router

**Decisão**: As rotas são compatíveis sem conflito de path. O Scenario B v2 usa o subpacote `v2router` que registra rotas sob prefixos distintos. As rotas de onboarding (`/api/v1/onboarding/`) e auth (`/api/v1/auth/pki-login`) não colidem com nenhuma rota v2.

**Evidência**: 
```go
// router.go atual — as rotas v2 são registradas por v2router.Register(app, deps.V2Deps)
// Rotas de onboarding: /api/v1/onboarding/* — sem conflito
```

**Racional**: Validado por análise de todos os prefixos em `backend/services/api-gateway/internal/http/router/v2/router.go`.

---

### D-006: Variável `CENTRAL_BANK_API_URL` já configurada em todos os bancos

**Decisão**: Nenhuma alteração de infraestrutura ou variáveis de ambiente é necessária.

**Evidência**:
```
.env.infra.bank-a:  CENTRAL_BANK_API_URL=http://api-gateway-central-bank-a:8080
.env.infra.bank-b:  CENTRAL_BANK_API_URL=http://api-gateway-central-bank-b:8080
.env.infra.bank-c:  CENTRAL_BANK_API_URL=http://api-gateway-central-bank-a:8080
.env.infra.bank-d:  CENTRAL_BANK_API_URL=http://api-gateway-central-bank-b:8080
```

A lógica condicional em `app.go` já lê essa variável — quando presente, instancia `OnboardingProxyHandler` (banco comercial); quando ausente, instancia `OnboardingHandler` (Central Bank).

---

### D-007: `DropScenarioATables` quebra o startup — erro de partição

**Descoberta adicional**: O log de startup do API Gateway mostra:
```
ERROR: relation "swap_order_scenario_b" does not exist (SQLSTATE 42P01)
```

Isso ocorre porque `cleanup_scenarioa.go` tenta dropar `swap_order_scenario_b` **antes** de `RunAutoMigrate` criá-la. Remover `DropScenarioATables` corrige este bug colateral também.

---

## Escopo Final de Mudanças

### Arquivos a modificar

| Arquivo | Tipo de mudança | Risco |
|---|---|---|
| `backend/services/api-gateway/internal/http/router/router.go` | Restaurar struct `Dependencies` + rotas de onboarding | Baixo — copiar do develop-scenario-a |
| `backend/services/api-gateway/internal/app/app.go` | Restaurar instanciação de handlers + remover `DropScenarioATables` call | Baixo |
| `backend/services/auth/internal/grpc/server/server.go` | Alterar `5*time.Minute` → `30*time.Minute` (linha 460) | Mínimo |
| `tryouts/tryout-my-onboarding-status.sh` | Ler `KC_CLIENT_ID` do env file em vez de hardcode | Mínimo |

### Arquivos a adicionar

Nenhum arquivo novo necessário.

### Arquivos a remover

| Arquivo | Motivo |
|---|---|
| `backend/services/api-gateway/internal/db/init/cleanup_scenarioa.go` | Scenario B é ambiente 100% novo; tabelas referenciadas não existem no DB do API Gateway |

### Chamadas `emitAudit` a adicionar (FR-013)

| Local | Evento a emitir |
|---|---|
| `backend/services/auth/internal/grpc/server/onboarding.go` → `SubmitCredentialRequest` | `"CREDENTIAL_REQUEST"` — Fase 1 iniciada |
| `backend/services/compliance/internal/grpc/server/server.go` → função de approve KYC | `"KYC_APPROVED"` — Fase 2 aprovada |
| `backend/services/compliance/internal/grpc/server/server.go` → função de reject KYC | `"KYC_REJECTED"` — Fase 2 rejeitada |
| `CompleteOnboarding` | Já emite `"COMPLETE_ONBOARDING"` — OK |

---

## Alternativas Consideradas

### Alt-1: Criar branch separada a partir de develop-scenario-a e fazer cherry-pick do Scenario B
- **Rejeitada**: Muito trabalho de merge; alto risco de conflitos; a abordagem de re-conectar handlers é mais segura.

### Alt-2: Criar um segundo `router.go` paralelo para Scenario A
- **Rejeitada**: Complexidade desnecessária; coexistência no mesmo router é viável e já validada (D-005).

### Alt-3: Mover o armazenamento de onboarding para o API Gateway (DB próprio)
- **Rejeitada**: Quebraria a arquitetura existente; os dados já estão na compliance service; YAGNI.
