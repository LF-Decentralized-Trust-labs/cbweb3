---
description: "Task list — 034 toolkit provisiona stacks per-entity (infra isolada + backend + frontend + NOC)"
---

# Tasks: Toolkit provisiona stacks per-entity

**Input**: `/specs/034-toolkit-entity-stacks/` (spec.md, plan.md)
**Tests**: incluídos e obrigatórios (test-first; mocks no unit, containers atrás de tag `integration`).
**Organization**: por user story / fatia, para implementação e validação independentes.

## Format: `[ID] [P?] [Story] Descrição` — caminhos relativos a `scenario-a/`.

---

## Phase 1: Setup (design)

- [ ] T001 `research.md`: matriz de env vars por serviço (a partir de `backend/config/.env.infra.*`), realm Keycloak (clients/roles por entidade), e esquema de derivação de portas por entidade/spoke.
- [ ] T002 [P] `data-model.md`: EntityInfra, EntityBackendStack, EntityFrontend, NOCStack, e o parâmetro `spec.keycloak`.
- [ ] T003 [P] `contracts/`: assinaturas das funções/steps (renderEntityEnv, start-infra, provision-keycloak, start-backend-stack, start-frontend, start-noc).

---

## Phase 2: Foundational (bloqueia US1–US3)

- [ ] T004 [US-] Teste + impl `ports.go`: derivação determinística de portas por entidade/spoke (api-gateway, gRPC, Postgres, Redis, Keycloak, frontend) sem colisão multi-spoke.
- [ ] T005 [US-] Teste + impl `entityenv.go`: render do `.env` da entidade a partir de `env.tmpl` (aponta p/ infra própria + contratos do spoke + `cbEndpoint` correto). Sem chaves privadas.
- [ ] T006 [US-] Teste + impl `step_start_infra.go`: `docker compose up` do `infra-compose.yaml` (Postgres+Redis, e Keycloak se `per-entity`) + health-gate (pg_isready / Keycloak `/realms/master`). Idempotente.
- [ ] T007 [US-] Teste + impl `step_provision_keycloak.go`: cria realms/clients via template (estratégia shared/per-entity), sem tocar no `init.sh` legado.
- [ ] T008 [US-] Teste + impl `step_start_backend_stack.go`: `docker compose up` do `backend-compose.yaml` (4 serviços) + health (`/api/v1/health` do api-gateway). Parametrizado p/ CB e banco.

**Checkpoint**: primitivas de infra/env/backend prontas e testadas (unit).

---

## Phase 3: US1 — found levanta a stack do banco central (Priority: P1) 🎯 MVP

**Goal**: found sobe infra própria do CB + backend + Governance/Treasury portals; api-gateway do CB pronto para autorizar onboarding.

### Tests ⚠️
- [ ] T009 [P] [US1] Teste: novos passos do found na ordem após `register-relay` (ou bloco dedicado): `start-cb-infra → provision-keycloak → start-cb-backend → start-cb-frontend`.
- [ ] T010 [P] [US1] Teste: `cbEndpoint` resolvido/escrito como `/api/v1/onboarding/credential-request` (FR-012).
- [ ] T011 [P] [US1] Teste de template: `central-bank/infra-compose.yaml` + `backend-compose.yaml` validam via `docker compose config`.

### Implementação
- [ ] T012 [US1] Templates `central-bank/{infra-compose,backend-compose,frontend-compose}.yaml` + `env.tmpl` + `keycloak-realm/`.
- [ ] T013 [US1] Wire dos passos do CB no `found` (orchestrator) + plumbing de deps/profile (image, keycloak strategy, ContractsOutDir já existe).
- [ ] T014 [US1] Corrigir `cbEndpoint` nos samples/bundle (`/api/v1/onboarding/credential-request`).
- [ ] T015 [US1] Validar em run real: found CB → `GET <cbEndpoint>/...` responde; containers Postgres/Redis dedicados do CB; portal governance carrega.

**Checkpoint**: api-gateway do CB no ar → **E2E do join destravado**.

---

## Phase 4: US2 — join levanta a stack do banco comercial (Priority: P1)

### Tests ⚠️
- [ ] T016 [P] [US2] Teste: `start-backend` do join passa a levantar a stack real (infra própria do banco + backend + frontend bank); ordem e idempotência.
- [ ] T017 [P] [US2] Teste de template: `commercial-bank/{infra-compose,backend-compose,frontend-compose}.yaml`.

### Implementação
- [ ] T018 [US2] Templates `commercial-bank/*` + `env.tmpl` (aponta p/ `CENTRAL_BANK_API_URL` do CB; Keycloak conforme estratégia — banco próprio só em `per-entity`).
- [ ] T019 [US2] Substituir o `start-backend` no-op por `start-bank-infra → (keycloak) → start-bank-backend → start-bank-frontend` no `buildJoinSteps`.
- [ ] T020 [US2] Validar em run real (sobre found CB): banco com infra+backend próprios; api-gateway aponta p/ CB.

**Checkpoint**: banco totalmente operável.

---

## Phase 5: US3 — NOC no found (Priority: P2)

### Tests ⚠️
- [ ] T021 [P] [US3] Teste: passo `start-noc` no found; agent configurado p/ componentes do spoke; NOC usa realm `cbweb3` do Keycloak do CB.

### Implementação
- [ ] T022 [US3] Template `noc/compose.yaml` (noc-backend+db+agent+frontend) parametrizado p/ o spoke.
- [ ] T023 [US3] Passo `start-noc` no found + render do agent-config (componentes provisionados).
- [ ] T024 [US3] Validar: `GET <noc-backend>/api/v1/health` + agent reportando.

---

## Phase 6: Polish & E2E

- [ ] T025 [P] Atualizar `samples/README.md` (found sobe stack do CB + NOC; join sobe stack do banco; estratégia Keycloak).
- [ ] T026 Atualizar contagens/asserts de passos (found/join) afetados pelos novos steps.
- [ ] T027 `go test ./...` + `go vet` + gofmt verdes.
- [ ] T028 **E2E**: found CB (rede + stack) → start-cacti → join banco (rede + Paladin + Pente + stack) → smoke de onboarding/operação. 2 spokes × 2 bancos.

---

## Dependencies & Order

- Phase 1 → Phase 2 (primitivas) → US1 (MVP, destrava E2E) → US2 → US3 → Polish/E2E.
- US2 depende de US1 (precisa do CB no ar para o onboarding). US3 independe de US2.

## Notes

- Não tocar nos legados (FR-011): `deploy/local/compose.yml`, `make/*.mk`, `backend/docker-compose-backend.*`, `frontend/docker-compose.spoke-*.yml`, `interop/hub-and-spoke/noc/*`, `init.sh`.
- Reuso de imagens/Dockerfiles via `image` (build/registry).
- Sem chaves privadas em arquivo/env/log; `keyProvider`/`certSource` reais na Fase 4.
- Cada fatia (US1/US2/US3) é um sub-PR com Constitution Check; Samuel revisa os P1.
