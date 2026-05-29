# Implementation Plan: Restaurar Auth e Onboarding (Scenario A)

**Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-restore-auth-onboarding/spec.md`

## Summary

Restaurar as rotas de autenticação PKI de 2 fatores e onboarding em 3 fases (Credencial → KYC → Blockchain) que foram removidas no commit `9831d50` ao implementar o Scenario B. A mudança é exclusivamente de **fiação** (wiring) — todos os handlers, interfaces e adaptadores já existem na branch atual mas estão desconectados do router e do `app.go`. Inclui 3 correções de defeitos: remoção do `DropScenarioATables` (causa erro de startup), correção do TTL do nonce PKI (5 min → 30 min), e correção do `clientId` hardcoded no script de tryout.

## Technical Context

**Language/Version**: Go 1.25.5 (api-gateway, auth service, compliance service); Bash 5+ (scripts de tryout)  
**Primary Dependencies**: Fiber v2.52.9 (HTTP), gRPC/protobuf (inter-service), GORM + PostgreSQL driver, Redis (noncestore), go-ethereum  
**Storage**: PostgreSQL — tabela `participants` e `audit_logs` no compliance service; Redis — noncestore no auth service; API Gateway é stateless para onboarding (sem DB próprio de onboarding)  
**Testing**: `go test ./...` (Go unit tests), scripts Bash `tryout-*.sh` (integration smoke tests)  
**Target Platform**: Linux Docker containers (desenvolvimento local e staging)  
**Project Type**: Web service multi-componente (API Gateway + Auth Service + Compliance Service)  
**Performance Goals**: login PKI completo < 5s (SC-006); `my-status` < 1s (SC-005)  
**Constraints**: Zero regressão nas rotas Scenario B v2 (SC-004); sem alterações de schema na DB do API Gateway  
**Scale/Scope**: 4 bancos comerciais (A, B, C, D) × 2 spokes (A, B); uso local/staging apenas nesta fase

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

A constituição do projeto está em formato de template (não preenchida). Não há gates formais a verificar. Princípios gerais observados:

- ✅ **Simplicidade (YAGNI)**: nenhum arquivo novo criado no código-fonte; apenas reconexão de componentes existentes
- ✅ **Sem over-engineering**: reutilização de `audit_logs` para FR-013 em vez de tabela nova
- ✅ **Zero regressão**: rotas Scenario B v2 mantidas intactas (coexistência validada em D-005)
- ✅ **Mudanças reversíveis**: todas as alterações são locais e facilmente revertíveis

## Project Structure

### Documentation (this feature)

```text
specs/003-restore-auth-onboarding/
├── plan.md              ← este arquivo
├── research.md          ← Phase 0 (gerado)
├── data-model.md        ← Phase 1 (gerado)
├── quickstart.md        ← Phase 1 (gerado)
├── contracts/
│   └── onboarding-api.md   ← Phase 1 (gerado)
└── tasks.md             ← Phase 2 (/speckit.tasks)
```

### Source Code (arquivos afetados)

```text
backend/
├── services/
│   ├── api-gateway/
│   │   └── internal/
│   │       ├── app/
│   │       │   └── app.go                          ← MODIFICAR: restaurar OnboardingProxyHandler/Handler
│   │       ├── http/router/
│   │       │   └── router.go                       ← MODIFICAR: restaurar Dependencies + rotas
│   │       └── db/init/
│   │           └── cleanup_scenarioa.go             ← REMOVER: arquivo inteiro
│   │
│   ├── auth/
│   │   └── internal/grpc/server/
│   │       └── server.go                           ← MODIFICAR: nonce TTL 5min → 30min (linha 460)
│   │
│   └── compliance/
│       └── internal/
│           ├── repository/
│           │   └── models.go                       ← MODIFICAR: adicionar campo rejection_reason
│           └── grpc/server/
│               └── server.go                       ← MODIFICAR: adicionar emitAudit nos handlers KYC
│
tryouts/
└── tryout-my-onboarding-status.sh                  ← MODIFICAR: fix clientId hardcoded (linha 199)
```

**Structure Decision**: Projeto multi-serviço existente. Nenhuma nova estrutura de diretório necessária. As mudanças são cirúrgicas em arquivos já existentes.

## Complexity Tracking

Sem violations. As mudanças são todas aditivas ou corretivas em código existente.

---

## Implementation Approach

### Tier 1 — API Gateway: Router & App Wiring (core fix, P1)

**Problema**: `router.go` e `app.go` foram simplificados no commit `9831d50` e removeram os handlers de onboarding.

**Solução**: Restaurar, baseado no estado do `develop-scenario-a`:

1. **`router.go`** — Restaurar na struct `Dependencies`:
   - `OnboardingHandler *handlers.OnboardingHandler`
   - `OnboardingProxyHandler *handlers.OnboardingProxyHandler`
   - Bloco `if deps.OnboardingProxyHandler != nil { ... } else if deps.OnboardingHandler != nil { ... }` com todas as rotas `/api/v1/onboarding/`
   - `authGroup.Post("/pki-login", ..., deps.OnboardingProxyHandler.PKILogin)` no bloco do proxy
   - Adicionar `"os"` de volta aos imports (necessário para `os.Getenv("INTERNAL_RELAY_AUTH_SECRET")`)

2. **`app.go`** — Restaurar instanciação condicional após wiring do `complianceGRPC`:
   ```go
   if cfg.CentralBankAPIURL != "" {
       deps.OnboardingProxyHandler = handlers.NewOnboardingProxyHandler(
           cfg.CentralBankAPIURL, cfg.PKIDir, cfg.BankCode, identityManager,
       )
   } else {
       deps.OnboardingHandler = handlers.NewOnboardingHandler(identityManager)
   }
   ```
   - Remover a chamada `dbinit.DropScenarioATables(db)` do bloco de inicialização do DB

3. **`cleanup_scenarioa.go`** — Remover o arquivo inteiro.

---

### Tier 2 — Auth Service: Nonce TTL (P3)

**Arquivo**: `backend/services/auth/internal/grpc/server/server.go`  
**Linha**: 460  
**Mudança**: `5*time.Minute` → `30*time.Minute`

---

### Tier 3 — Compliance Service: rejection_reason + emitAudit (P2)

**`backend/services/compliance/internal/repository/models.go`**:
- Adicionar campo `RejectionReason string` ao `ParticipantModel` (GORM AutoMigrate cria a coluna)

**`backend/services/compliance/internal/grpc/server/server.go`**:
- Localizar handler de approve KYC → adicionar `s.emitAudit(ctx, "KYC_APPROVED", ...)`
- Localizar handler de reject KYC → adicionar `s.emitAudit(ctx, "KYC_REJECTED", ...)`

**`backend/services/auth/internal/grpc/server/onboarding.go`**:
- Ao final de `SubmitCredentialRequest` (após `UpsertParticipant`) → adicionar `s.emitAudit(ctx, "CREDENTIAL_REQUEST", ...)`

---

### Tier 4 — Script Fix: clientId hardcoded (P3)

**Arquivo**: `tryouts/tryout-my-onboarding-status.sh`, linha 199  
**Antes**:
```bash
-d "{\"clientId\": \"bank-a-client\", \"clientSecret\": \"$kc_secret\"}"
```
**Depois**:
```bash
local kc_client_id
kc_client_id=$(grep KC_CLIENT_ID "$BANK_ENV" | cut -d= -f2-)
# ...
-d "{\"clientId\": \"$kc_client_id\", \"clientSecret\": \"$kc_secret\"}"
```

---

## Artifacts

| Artefato | Status |
|---|---|
| [research.md](research.md) | ✅ Completo |
| [data-model.md](data-model.md) | ✅ Completo |
| [contracts/onboarding-api.md](contracts/onboarding-api.md) | ✅ Completo |
| [quickstart.md](quickstart.md) | ✅ Completo |
| tasks.md | ⏳ Próximo (`/speckit.tasks`) |

