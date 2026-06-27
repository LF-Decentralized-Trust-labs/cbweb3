# Implementation Plan: MD-3 — Atualizar Consumidores do Proto para Campos Spoke-Keyed

**Branch**: `022-update-proto-consumers` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/022-update-proto-consumers/spec.md`

## Summary

O MD-1 (definição `.proto`) e o MD-2 (migração de banco) estão aplicados. Este plano finaliza o MD-3: propagar os campos spoke-keyed (`source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`) a todos os consumidores do Scenario A. Research pré-implementação confirmou que a maioria dos consumidores já foi atualizada — o trabalho restante é pontual: atualizar o vendor stale do `api-gateway`, verificar o endpoint-selection no relay e fechar a cobertura de testes.

## Technical Context

**Language/Version**: Go 1.26+ (backend), TypeScript 5.x (relay + frontend)
**Primary Dependencies**: `gorm.io/gorm` v2, `google.golang.org/grpc`, `google.golang.org/protobuf`, `buf` (proto codegen), Cacti HTLC relay (TypeScript)
**Storage**: PostgreSQL (produção, tabelas `fx_agreements`), SQLite (testes de repositório)
**Testing**: `go test ./...` (backend), `tsc --noEmit` (frontend type-check)
**Target Platform**: Linux container (Docker Compose), Scenario A spoke networks
**Project Type**: Microservices (gRPC backend) + TypeScript relay + React frontend
**Performance Goals**: Sem mudança — esta é uma atualização de campos; nenhum objetivo de throughput novo
**Constraints**: Nenhum consumidor do Scenario A pode referenciar `spoke_a_receiver`/`spoke_b_receiver` após a implementação (verificado via build e grep)
**Scale/Scope**: Scenario A exclusivamente; Scenario B fora de escopo

## Constitution Check

*GATE: Avaliado antes da Fase 0. Re-avaliado após a Fase 1.*

| Princípio | Status | Evidência |
|-----------|--------|-----------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Todos os arquivos alterados estão em `scenario-a/`. Nenhum toque em `scenario-b/`. |
| **II. Privacy by Design** | ✅ PASS | Nenhuma mudança em caminhos de valor on-chain. Os campos `source_receiver`/`dest_receiver` são identidades Paladin, não PII. |
| **III. Atomic Settlement Guarantee** | ✅ PASS | Nenhuma mudança na lógica de HTLC ou sequenciamento de liquidação. O relay continua verificando lock + reveal. |
| **IV. Compliance Gate** | ✅ PASS | Nenhuma mudança em paths de autenticação, `IdentityRegistry` ou `Compliance` service. |
| **V. Test-First** | ⚠️ PENDENTE | O código de produção já existe; os testes existentes de migração cobrem MD-2. É necessário verificar e fechar cobertura para os novos campos no gRPC handler e no relay. Testes explícitos para `source_spoke_id`/`dest_spoke_id` devem ser adicionados antes de fechar o PR. |
| **VI. Observability** | ✅ PASS | O relay já loga `destSpokeId` nas operações de routing. Nenhum silent failure introduzido. |

**Resultado**: Sem violações bloqueantes. O gate V requer ação (cobertura de teste) antes do merge.

## Project Structure

### Documentation (this feature)

```text
specs/022-update-proto-consumers/
├── plan.md              # Este arquivo
├── research.md          # Fase 0 output (state mapping de todos os consumidores)
├── data-model.md        # Fase 1 output (entidades proto + domínio)
├── contracts/           # Fase 1 output (interface gRPC e REST)
│   └── grpc-contract.md
└── tasks.md             # Fase 2 output (/speckit.tasks — não criado aqui)
```

### Source Code (scenario-a — arquivos relevantes para MD-3)

```text
scenario-a/
├── apis/proto/payment_orchestrator/v1/
│   └── payment_orchestrator.proto         ✅ MD-1 aplicado (campos novos, antigos reservados)
│
├── backend/
│   ├── shared/proto/payment_orchestrator/v1/
│   │   ├── payment_orchestrator.pb.go     ✅ Regenerado (expõe novos campos)
│   │   └── payment_orchestrator_grpc.pb.go ✅ Regenerado
│   │
│   └── services/
│       ├── api-gateway/
│       │   └── vendor/github.com/LACNetNetworks/cbweb3-platform/
│       │       └── backend/shared/proto/payment_orchestrator/v1/
│       │           └── payment_orchestrator.pb.go  ❌ STALE (expõe campos antigos)
│       │
│       └── payment-orchestrator/
│           └── internal/
│               ├── domain/fx.go            ✅ FXAgreementRecord com novos campos
│               ├── grpc/server/server.go   ✅ Handler lê/escreve novos campos
│               ├── repository/
│               │   ├── fx_agreement_gorm.go    ✅ Migração implementada (MD-2)
│               │   └── gorm_repos_test.go      ⚠️ Cobrir novos campos nos testes gRPC
│
├── interop/hub-and-spoke/cacti/src/
│   ├── config.ts            (fora do escopo MD-3 — é RL-1/Fase 2)
│   └── htlc-relay.ts
│       ├── FXProposalEvent interface    ✅ camelCase novos campos
│       ├── pollFXAgreementsRest()       ✅ Extrai source/dest_spoke_id do REST
│       ├── proposeOnCounterpart()       ✅ Passa novos campos no gRPC
│       └── resolveCounterpartContractId() ✅ Usa hashLock (mecânica HTLC correta)
│
└── frontend/apps/bank/src/
    ├── types/fx-agreement.types.ts      ✅ Interfaces TypeScript atualizadas
    └── pages/AgreementProposalPage.tsx  ✅ Formulário envia novos campos
```

**Structure Decision**: Monorepo com separação clara por camada (proto → shared/proto → services → relay → frontend). Nenhum arquivo novo de produção necessário — o trabalho é de atualização e validação.

## Complexity Tracking

> Nenhuma violação de Constitution Check identificada. Seção não aplicável.
