# Reuse Matrix — Scenario B

> Tabela de capacidades reaproveitáveis x pendentes de implementação.
> Mapeamento: cada entrada → FR de origem + evidência de implementação atual.

| ID | Capacidade | Layer | Status | FR | Evidência |
|----|-----------|-------|--------|----|-----------|
| REUSE-001 | Keycloak OIDC/JWT para autenticação e gestão de roles | Identity/Auth | ✅ Disponível | FR-030, FR-056 | `deploy/local/keycloak/realms/scenario-b-realm.json` |
| REUSE-002 | PostgreSQL como datastore primário | Datastore | ✅ Disponível | FR-055 | `backend/services/api-gateway/internal/db/init/migrate.go` |
| REUSE-003 | GORM AutoMigrate para schema evolution | Datastore | ✅ Disponível | FR-055 | `RunAutoMigrate` em `migrate.go` |
| REUSE-004 | Redis para cache | Cache | ✅ Disponível | Decision 2 | `deploy/local/docker-compose.yml` |
| REUSE-005 | Docker Compose para ambiente local | Runtime | ✅ Disponível | Decision 2 | `deploy/local/docker-compose.yml` |
| REUSE-006 | Fiber v2 HTTP framework | HTTP | ✅ Disponível | H4 | `backend/services/api-gateway/go.mod` |
| REUSE-007 | gRPC adapters (Auth, Identity, Compliance) | gRPC | ✅ Disponível | H4 | `internal/adapters/` |
| REUSE-008 | Audit log append-only via GORM | Audit | ✅ Disponível | FR-048, FR-049 | `internal/audit/audit.go` + `internal/db/init/triggers.go` |
| REUSE-009 | Middleware RBAC + Bearer JWT | Middleware | ✅ Disponível | FR-056 | `internal/http/middleware/` |
| REUSE-010 | OpenAPI 3.1 spec base | API Spec | ✅ Disponível | FR-059 | `openapi/v2/scenario-b.yaml` |
| REUSE-011 | go-ethereum ethclient abstraction | Blockchain | ✅ Disponível (stub) | FR-037 | `backend/shared/blockchain/scenariob/` |
| REUSE-012 | ZK Compliance Gate | Compliance | ✅ Disponível | FR-025, FR-058 | `backend/services/compliance/internal/services/zk_compliance_gate.go` |
| REUSE-013 | AMM EVM Client | Blockchain | ⚠️ Parcial (stub) | FR-027, FR-037 | `amm/client.go` — `TODO: implement` via abigen |
| REUSE-014 | SpokeBridge EVM Client | Blockchain | ⚠️ Parcial (stub) | FR-029 | `spokebridge/client.go` — `TODO: implement` |
| REUSE-015 | Paladin JSON-RPC Client | Compliance | ⚠️ Parcial (stub) | FR-034 | `paladin/client.go` — aguarda infraestrutura Paladin |
| REUSE-016 | Relayer Cacti Client | Bridge | ⚠️ Parcial (stub) | FR-031, FR-039 | `relayer/client.go` — aguarda Hyperledger Cacti |
| REUSE-017 | Solidity Contracts (AMM, SpokeBridge, etc.) | Smart Contracts | ❌ Não implementado | FR-022 — FR-030 | Pendente compilação Foundry + abigen |
| REUSE-018 | Frontend UI | UI | ❌ Fora de escopo | SC-025 | `frontend/apps/` — intocado (Decision 18) |
| REUSE-019 | Performance Baseline (k6/hey) | Observability | ❌ Pendente | FR-037, SC-021 — SC-023 | `tests/performance/` — a criar na Phase 6 |
| REUSE-020 | Rate Limiting / Throttling | Middleware | ❌ Fora de escopo (iteração atual) | FR-053, FR-054 | Risco aceito — ver risk-register.md |

---

**Legenda**:
- ✅ **Disponível**: Implementado e funcional nesta iteração.
- ⚠️ **Parcial**: Estrutura criada (stub/interface), pendente implementação completa com infraestrutura real.
- ❌ **Não implementado / Fora de escopo**: Explicitamente excluído desta iteração ou bloqueado por dependência externa.
