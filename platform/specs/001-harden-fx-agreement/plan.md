# Implementation Plan: Harden FX Agreement for Production

**Branch**: `feature/agreement-v2` | **Date**: 2026-04-15 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/001-harden-fx-agreement/spec.md`

## Summary

Evolução do subsistema FX Agreement de estado demo/in-memory para operação de produção, cobrindo 8 dimensões: persistência durável (PostgreSQL + GORM), trilha de auditoria append-only, relay cross-spoke confiável com retry/dedup persistente, contexto bilateral privado via Paladin Pente, acoplamento forte HTLC-agreement (Phase A: service-layer imediato; Phase B: Pente `externalCalls` atômicas), expiração automática com job periódico, autenticação do canal interno e reconciliação operacional. **CRÍTICO**: Phase 7 garante operacionalização OBRIGATÓRIA de Zeto E Pente simultaneamente no Paladin local (não opcionais) para validação de produção. A estratégia two-phase (Decision 4 em research.md) garante que o gap de bypass on-chain seja fechado na camada de serviço imediatamente, com evolução atômica via Pente quando o ambiente-alvo estiver pronto.

## Delta Analysis (2026-04-15)

### Current state validated

- Setup local do Paladin está automatizado para Zeto (deploy factory, criação token instance, render configs e registro de nós).
- Integração de aplicação com Pente existe no payment-orchestrator (`PENTE_ENABLED`, `PENTE_BASE_URL`, adapter HTTP e uso no fluxo de aceite).
- Não há bootstrap local equivalente para Pente em `deploy/local/paladin` (manifests/scripts/targets make dedicados para contexto bilateral e deploy do FXAgreement privado).

### Planning impact

**CRÍTICO (2026-04-15)**: Zeto + Pente não são opcionais; ambos DEVEM estar operacionais na mesma instância local do Paladin para validação de produção. Phase 7 (T043–T050) é deliverable OBRIGATÓRIO que fecha o gap entre integração de aplicação e operacionalização real. Sem Phase 7, ambiente local não demonstra operação de produção confiável.

## Technical Context

**Language/Version**: Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contratos)  
**Primary Dependencies**: gRPC/protobuf, go-ethereum v1.17.1, GORM + Postgres driver (padrão compliance), ethers v6, @grpc/grpc-js, Hyperledger Cacti packages, Paladin Pente client  
**Storage**: PostgreSQL — `fx_agreements`, `fx_agreement_events`, relay durability tables; padrão GORM conforme compliance service  
**Testing**: `go test ./...` (Go), `jest` / `ts-jest` (TypeScript), `forge test` (contratos Solidity)  
**Target Platform**: Linux server / Docker Compose (multi-spoke: bank-a/b/c/d + central-bank-a/b)  
**Project Type**: Microservices backend — payment-orchestrator (gRPC service) + Cacti relay (TypeScript daemon) + contratos Solidity  
**Performance Goals**: 99,9% dos eventos relay entregues em ≤2 min em condição normal (SC-003); validação HTLC <100ms p95; job de expiração concluído dentro da janela operacional de 5 min  
**Constraints**: Retrocompatibilidade gRPC (campos aditivos apenas, sem breaking changes); **Em operação local: Zeto + Pente AMBOS OBRIGATÓRIOS (não feature-flagged, não opcionais)**; idempotência garantida em todas as operações de mutação.  
**Scale/Scope**: ~4 spokes ativos, dezenas de acordos FX concorrentes por ciclo diário; deduplicação e retry para ~10 eventos/min em pico

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

A constitution ainda não foi customizada para este projeto. Princípios inferidos da codebase e dos requisitos funcionais:

| Gate | Status | Justificativa |
|------|--------|---------------|
| Persistência durável obrigatória para dados de negócio | ✅ PASS | PostgreSQL + GORM conforme padrão do compliance service |
| Retrocompatibilidade de API (`proto` aditivo-only) | ✅ PASS | `group_id`/`contract_address` como campos opcionais adicionados |
| Idempotência em todas as mutações de estado | ✅ PASS | Idempotency keys por `trade_id + event_type` em RelayDeliveryRecord |
| Autenticação em endpoints internos (service-to-service) | ✅ PASS | FR-010: `X-Relay-Auth` shared secret ou mTLS no canal interno |
| Fail-closed em validação HTLC | ✅ PASS | Decision 4 Phase A: rejeita operação se acordo não encontrado ou estado inválido |
| Nenhuma chave privada ou segredo hardcoded | ✅ PASS | Configuração via variáveis de ambiente (padrão do projeto) |

## Project Structure

### Documentation (this feature)

```text
specs/001-harden-fx-agreement/
├── plan.md              # Este arquivo
├── research.md          # Decisões técnicas com rationale (5 decisions)
├── data-model.md        # 4 entidades + state machines + validações
├── quickstart.md        # Guia de implementação e verificação por fase
├── contracts/
│   ├── fx-agreement-http.openapi.yaml          # OpenAPI 3.0.3 para endpoints HTTP
│   └── payment-orchestrator-fx-grpc.md         # Notas de evolução gRPC
└── tasks.md             # 50 tarefas em 7 fases (inclui fase de operação local Zeto+Pente)
```

### Source Code (repositório raiz — arquivos impactados)

```text
backend/services/payment-orchestrator/
├── cmd/payment-orchestrator/
│   └── main.go                                 # Wiring: DB, repositórios, jobs, adapters
├── internal/
│   ├── domain/
│   │   └── fx.go                               # FXAgreementRecord, FXState, FXAgreementEvent
│   ├── ports/
│   │   └── fx_repository.go                    # Interface FXAgreementRepository
│   ├── repository/
│   │   └── fx_repository.go                    # Implementação GORM + Postgres
│   ├── grpc/server/
│   │   └── server.go                           # Substituir in-memory map por repository
│   └── adapters/paladin/
│       ├── client.go                            # Existente (Zeto)
│       └── pente_client.go                     # Novo: PenteClient bilateral context

interop/hub-and-spoke/cacti/src/
├── htlc-relay.ts                               # Relay hardening: retry, dedup, CANCELLED/SETTLED

apis/proto/payment_orchestrator/v1/
└── payment_orchestrator.proto                  # Campos aditivos: group_id, contract_address

contracts/src/
└── HashTimeLockedContract.sol                  # Phase B: remover bypass agreementIDBytes=zeroes

backend/services/api-gateway/internal/http/router/
└── router.go                                   # Autenticação nos endpoints /internal/v1/payments/fx/

migrations/
└── payment_orchestrator/
    ├── 001_fx_agreements.up.sql
    ├── 002_fx_agreement_events.up.sql
    └── 003_relay_delivery_records.up.sql

deploy/local/paladin/
├── contracts/                                 # manifests atuais de registry/zeto + novos manifests Pente
├── scripts/                                   # deploy/register atuais + novos scripts Pente
├── spoke-a/config/*/config.yaml.tmpl          # templates de nó Paladin
└── spoke-b/config/*/config.yaml.tmpl          # templates de nó Paladin

make/
└── 40-paladin.mk                              # targets Zeto atuais + novos targets de setup Pente
```

## Delivery Phases

### Phase 1 — Foundational Infrastructure (T001–T012)
Migrations SQL, FXAgreementRepository GORM, wiring em main.go, domain types expandidos. Bloqueia todas as user stories.

### Phase 2 — US1: Persistência e Auditoria (T013–T023)
Replace in-memory map por repositório, audit events append-only, listagem/consulta, expiry job, validação de consistência de termos (rate tolerance), controle de expiração em todas as operações.

### Phase 3 — US2: Relay Confiável (T024–T030)
Relay dedup persistente, retry queue com backoff, propagação completa de lifecycle (CANCELLED, SETTLED), autenticação do canal interno.

### Phase 4 — US3: Pente Bilateral + HTLC (T031–T039)
PenteClient adapter, criação de grupo bilateral, persistência de group_id/contract_address, Phase A enforcement no LockHTLC, Phase B Pente externalCalls atômicas (aguarda Decision 3 readiness).

### Phase 5 — Polish (T040–T042)
Métricas operacionais, script de reconciliação bilateral, documentação de operação e runbook de rollout.

### Phase 6 — Paladin Local Real Operation (T043+) — OBRIGATÓRIO
**CRITICAL DELIVERABLE**: Provisionamento e validação operacional OBRIGATÓRIA do Pente no Paladin local com paridade de automação ao setup atual de Zeto. **Zeto E Pente DEVEM estar operacionais simultaneamente** (não opcionais) para demonstrar operação de produção confiável. Este é um deliverable crítico que bloqueia validação de produção se não completado.
