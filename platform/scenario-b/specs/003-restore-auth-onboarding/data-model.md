# Data Model: Restaurar Auth e Onboarding (Scenario A)

**Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06

> **Nota importante (D-002, D-003)**: Os dados de onboarding são persistidos no **compliance service** (tabela `participants`) e **não** na API Gateway. A API Gateway é stateless para onboarding — delega ao auth service via gRPC, que por sua vez chama o compliance service. Portanto, não há schema novo a criar no banco da API Gateway para onboarding.

---

## Entidades Conceituais

### 1. OnboardingRequest (conceitual → física: `participants` no compliance service)

Representa o pedido de registro de um banco comercial. Cada banco tem no máximo um registro **ativo** (o mais recente) e pode ter registros históricos com status `KYC_REJECTED`.

| Campo conceitual | Campo físico (`ParticipantModel`) | Tipo | Notas |
|---|---|---|---|
| `request_id` | `user_id` | string (UUID Keycloak) | Gerado na Phase 1; serve como identificador do pedido |
| `bank_code` | `bank_code` | string | Ex: `"a"`, `"b"`, `"c"`, `"d"` |
| `institution_name` | `institution_name` | string | Nome do banco |
| `status` | `status` | string | Ver máquina de estados abaixo |
| `wallet_address` | `wallet_address` | string | Derivado do `blockchain_pub_key_hex` na Phase 1 |
| `csr_pem` | `csr_pem` | text | CSR P-256 do banco |
| `blockchain_pub_key_hex` | `blockchain_pub_key_hex` | string | Chave pública secp256k1 |
| `pop_nonce` | `pop_nonce` | string | Nonce de prova de posse; presente somente em `KYC_APPROVED` |
| `pop_nonce_expires_at` | `pop_nonce_expires_at` | timestamp | TTL do PoP nonce (definido pelo Central Bank ao aprovar KYC) |
| `rejection_reason` | *(campo a adicionar)* | string | Preenchido quando status = `KYC_REJECTED` |
| `certificate_data` | `certificate_data` | text | Cert X.509 emitido na Phase 3 |
| `created_at` | `created_at` | timestamp | Automático |
| `updated_at` | `updated_at` | timestamp | Automático |

**Tabela física**: `participants` (compliance service PostgreSQL)  
**Arquivo**: `backend/services/compliance/internal/repository/models.go`

#### Campo a adicionar: `rejection_reason`

```go
// Em ParticipantModel:
RejectionReason string `gorm:"column:rejection_reason;type:text"`
```

Adicionado via GORM `AutoMigrate` no startup do compliance service.

---

### 2. Máquina de Estados do Onboarding

```
               POST /onboarding/initiate
[INÍCIO] ──────────────────────────────────► CREDENTIAL_REQUESTED
                                                    │
                              POST /compliance/approve-kyc │
                                                    ▼
                                              KYC_APPROVED ──► COMPLETED
                                                    │               (Phase 3: /onboarding/complete)
                              POST /compliance/reject-kyc  │
                                                    ▼
                                              KYC_REJECTED
                                                    │
                              POST /onboarding/initiate (novo request_id)
                                                    ▼
                                         CREDENTIAL_REQUESTED (novo registro)
```

**Regras**:
- `KYC_REJECTED` → registro permanece imutável; resubmissão cria **novo registro** com novo `user_id`/`request_id`
- `GET /onboarding/my-status` sempre retorna o registro **mais recente** para o `bank_code` (maior `created_at`)
- `GET /onboarding/my-status` retorna `{"status": "NONE"}` quando nenhum registro existe para o banco

---

### 3. PKI Nonce (efêmero → `noncestore` Redis/In-Memory no auth service)

Representa o nonce de curta duração gerado no login de 2 fatores PKI dos bancos comerciais.

| Campo | Tipo | Notas |
|---|---|---|
| `key` | string | `"pki:nonce:{userID}"` no Redis |
| `value` | string | `"{nonce_hex}|{client_secret}"` |
| `ttl` | duration | **30 minutos** (alterado de 5 min; `server.go:460`) |

**Storage**: Redis (produção) / In-Memory (dev)  
**Arquivo**: `backend/services/auth/internal/noncestore/`  
**Comportamento**: one-time use — `GetAndDelete` consome o nonce atomicamente via script Lua no Redis

---

### 4. OnboardingEvent (conceitual → física: `audit_logs` no compliance service)

Registro imutável de cada transição de estado do onboarding (FR-013). Implementado reutilizando a infraestrutura de auditoria existente.

| Campo conceitual | Campo físico (`AuditLogModel`) | Notas |
|---|---|---|
| `event_id` | `log_id` (UUID auto) | Gerado pelo PostgreSQL |
| `actor_user_id` | `actor_subject` | Quem executou a ação |
| `to_status` | `action_type` | Ex: `"CREDENTIAL_REQUEST"`, `"KYC_APPROVED"`, `"KYC_REJECTED"`, `"COMPLETE_ONBOARDING"` |
| `from_status` | `details` (jsonb) | Campo `from_status` no JSON de detalhes |
| `request_id` | `target_subject` | `user_id` do banco |
| `occurred_at` | `timestamp` | Automático |
| `result` | `result` | `"SUCCESS"` ou `"FAILURE"` |

**Tabela física**: `audit_logs` (compliance service PostgreSQL)  
**Arquivo**: `backend/services/compliance/internal/repository/models.go`

#### Eventos a adicionar (gap identificado em D-003):

| Fase | Evento | Onde emitir |
|---|---|---|
| Phase 1 | `"CREDENTIAL_REQUEST"` | `auth/grpc/server/onboarding.go` → `SubmitCredentialRequest()` |
| Phase 2 (approve) | `"KYC_APPROVED"` | `compliance/grpc/server/server.go` → handler de approve KYC |
| Phase 2 (reject) | `"KYC_REJECTED"` | `compliance/grpc/server/server.go` → handler de reject KYC |
| Phase 3 | `"COMPLETE_ONBOARDING"` | Já implementado ✅ |

---

## Schema Changes Summary

| Serviço | Tabela | Mudança | Arquivo |
|---|---|---|---|
| compliance | `participants` | Adicionar coluna `rejection_reason TEXT` | `repository/models.go` + GORM AutoMigrate |
| API Gateway | `cleanup_scenarioa.go` | **Remover** todo o arquivo | `db/init/cleanup_scenarioa.go` |
| API Gateway | Nenhuma nova tabela | — | — |
| auth | Nenhuma tabela | Alterar TTL de nonce (in-memory/Redis) | `grpc/server/server.go:460` |
