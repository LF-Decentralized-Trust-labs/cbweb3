# Implementation Plan: MD-3 — Atualizar Consumidores do Proto para Campos Spoke-Keyed

**Branch**: `022-update-proto-consumers` | **Date**: 2026-06-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/022-update-proto-consumers/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Python 3.11, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., FastAPI, UIKit, LLVM or NEEDS CLARIFICATION]  
**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]  
**Testing**: [e.g., pytest, XCTest, cargo test or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Linux server, iOS 15+, WASM or NEEDS CLARIFICATION]
**Project Type**: [e.g., library/cli/web-service/mobile-app/compiler/desktop-app or NEEDS CLARIFICATION]  
**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Avaliação por princípio (Constitution v1.0.4):

- **I. Scenario-Scoped Independence — PASS.** Todo o trabalho está restrito a `scenario-a/` (proto, `payment-orchestrator`, relay `htlc-relay.ts`, frontend `apps/bank`). O proto do Scenario B e seus consumidores (`spoke_a_receiver`/`spoke_b_receiver`) NÃO são tocados; qualquer atualização do Scenario B fica para uma PR separada com justificativa explícita (ver Nota sobre Scenario B na spec). Nenhum código é compartilhado entre cenários.

- **II. Privacy by Design — PASS.** Os quatro campos spoke-keyed (`source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`) são identificadores de roteamento, não PII nem valores monetários (data-model.md, seção Validações). A mudança não introduz dados em plaintext on-chain nem altera o uso de `ZetoToken`/`NotoToken`.

- **III. Atomic Settlement Guarantee — PASS.** A correção do roteamento por `dest_spoke_id` (FR-004) reforça a atomicidade do HTLC dual-layer do Scenario A: o relay passa a travar o lock no spoke de destino correto em vez de inferir contraparte por eliminação posicional. Os caminhos de timeout/refund do HTLC não são alterados por esta feature.

- **IV. Compliance Gate Before Participation — PASS (sem impacto).** A feature não altera os gates de IdentityRegistry, Compliance ou Keycloak OIDC no api-gateway; apenas a forma como os legs FX são identificados. Nenhum bypass é introduzido.

- **V. Test-First at Every Layer — PASS.** Disciplina Red-Green-Refactor aplicada: os testes unitários do `payment-orchestrator` são atualizados para os novos campos e devem falhar antes da implementação do handler (FR-008, SC-002); testes de repositório usam SQLite conforme padrão do projeto (Assumptions). Como se trata de breaking change no proto, a coordenação com a migração/backfill do MD-2 é pré-condição (Assumptions): o MD-2 já migrou os dados existentes, então o backend não precisa lidar com o schema antigo em runtime; a build Go falha se algum consumidor ainda referenciar os campos removidos (Edge Cases, SC-004).

- **VI. Observability and Auditability — PASS.** O relay DEVE registrar em log o `dest_spoke_id` utilizado para roteamento (US2 Acceptance Scenario 1) e logar erro descritivo para spoke desconhecido sem executar transação (US2 Acceptance Scenario 3), preservando a reconstrução do ciclo de liquidação a partir dos logs. Nenhuma falha silenciosa é introduzida.

**Resultado**: Todos os seis princípios passam. Sem violações — a tabela de Complexity Tracking permanece vazia.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
