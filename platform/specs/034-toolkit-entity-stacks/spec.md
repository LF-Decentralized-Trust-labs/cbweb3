# Feature Specification: Toolkit provisiona stacks per-entity (infra isolada + backend + frontend + NOC)

**Feature Branch**: `034-toolkit-entity-stacks`
**Created**: 2026-06-29
**Status**: Draft
**Input**: "O toolkit precisa subir toda a stack backend e frontend do banco central e do banco comercial. Cada banco terá seu próprio Postgres, Keycloak e Redis (hoje compartilhados). O Governance Portal (autoriza o onboarding do banco comercial) sobe no found; o NOC (monitoramento) também. Usar o toolkit sem alterar os scripts legados."

## Contexto e premissa (concat.md)

A meta do concat.md é tornar a plataforma **deployável para N participantes por configuração**, com o **toolkit** como ferramenta única, **sem modificar a rede de referência** (`deploy/local` + `make/*.mk` + composes legados permanecem verdes como amostra). O §3 nota que o backend já é per-entity e isolado; o gargalo restante é a **infraestrutura compartilhada** (uma instância de Postgres/Keycloak/Redis para todas as entidades), que impede N participantes de verdade.

Esta feature fecha esse gargalo e completa o ciclo de provisionamento: depois de `found`/`join` levantarem a rede (Besu + Paladin + contratos + Pente — features 026–033), o toolkit também levanta a **stack operacional** de cada entidade (infra própria + backend + frontend), de modo que um participante seja totalmente operável a partir do manifesto.

### Estado atual (verificado em código)

- **Backend per-entity:** `backend/docker-compose-backend.<entity>.yaml` + `backend/config/.env.infra.<entity>` — 4 serviços (`compliance`, `auth`, `payment-orchestrator`, `api-gateway`). Arquivos **fixos para 6 entidades**.
- **Infra compartilhada (legado):** `deploy/local/compose.yml` — 1 Postgres (7 DBs lógicos), 1 Keycloak (6 realms), 1 Redis (6 DBs lógicos). Isolação apenas lógica.
- **Frontend:** apps `bank`, `governance`, `treasury`, `supervisor`, `noc`, `dispatcher` (React/Vite), composes por-spoke com `VITE_API_URL` por entidade.
- **Governance Portal:** frontend `governance` + serviço `compliance`; o api-gateway do CB expõe `/api/v1/onboarding/credential-request` (autoriza o banco). `GOVERNANCE_ROLE` on-chain = banco central.
- **NOC:** monitoramento (noc-backend + noc-db próprio + noc-agent + frontend), standalone em `interop/hub-and-spoke/noc/`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — found levanta a stack operacional do banco central (Priority: P1)

Após `cbweb3 apply` de um manifesto `mode: found` provisionar a rede do spoke (features 026–033), o mesmo `apply` levanta a **stack operacional do CB**: sua **infra própria** (Postgres + Keycloak + Redis dedicados), o **backend** (compliance, auth, payment-orchestrator, api-gateway) e os **frontends de governança e tesouraria**. O api-gateway do CB fica pronto para **autorizar o onboarding** de bancos comerciais.

**Why this priority**: É o que destrava o E2E do join — o `request-cert` do join precisa do api-gateway do CB no ar. Sem isso, nenhum banco comercial completa o onboarding.

**Independent Test**: `apply` found em `local` → `GET <cbEndpoint>/health` responde e o endpoint de credential-request existe; os containers de infra do CB têm nomes/portas próprios do spoke (não `cbweb3-postgres` compartilhado).

**Acceptance Scenarios**:
1. **Given** um found que provisionou a rede, **When** os passos de stack rodam, **Then** existem containers Postgres/Keycloak/Redis **dedicados ao CB** (nomes derivados do spoke/entidade), e o backend do CB sobe apontando para essa infra própria.
2. **Given** o backend do CB no ar, **When** consulto `<cbEndpoint>`, **Then** a rota `/api/v1/onboarding/credential-request` responde (pronta para autorizar bancos).
3. **Given** o found completo, **When** abro o portal de governança, **Then** ele carrega apontando para o api-gateway do CB.

---

### User Story 2 — join levanta a stack operacional do banco comercial (Priority: P1)

Após o `mode: join` incorporar o banco à rede (feature 033), o mesmo `apply` levanta a **stack operacional do banco**: infra própria (Postgres + Keycloak + Redis dedicados), backend (4 serviços) e o **frontend `bank`**. O `start-backend` (hoje no-op) passa a levantar a stack real.

**Why this priority**: Completa a operação do participante — sem o backend/infra do banco, ele entra na rede mas não opera.

**Independent Test**: sobre um found com CB no ar, `apply` join → o banco tem infra própria + api-gateway respondendo, apontando para o CB via `CENTRAL_BANK_API_URL`.

**Acceptance Scenarios**:
1. **Given** o join completou a parte de rede, **When** os passos de stack rodam, **Then** o banco tem Postgres/Keycloak/Redis dedicados e backend apontando para eles.
2. **Given** o backend do banco no ar, **When** o api-gateway inicializa, **Then** ele aponta para `CENTRAL_BANK_API_URL` (api-gateway do CB) para o fluxo de onboarding/proxy.

---

### User Story 3 — NOC (monitoramento) levantado no found (Priority: P2)

O `found` levanta o stack **NOC** do spoke (noc-backend + noc-db + noc-agent + frontend noc), configurado para monitorar os componentes do spoke (Besu, Paladin, backend).

**Why this priority**: Observabilidade do spoke; importante para operação/pilotos, mas não bloqueia o fluxo de liquidação.

**Independent Test**: após found, `GET <noc-backend>/api/v1/health` responde e o agent reporta os componentes do spoke.

**Acceptance Scenarios**:
1. **Given** o found completo, **When** o NOC sobe, **Then** o noc-backend responde health e o agent do spoke começa a reportar métricas dos componentes provisionados.

---

### Edge Cases

- E se a porta da infra própria de uma entidade colidir com outra no mesmo host? → as portas são derivadas por entidade/spoke (esquema determinístico), evitando colisão.
- E se a infra própria já estiver no ar (re-`apply`)? → passos idempotentes (Check por health/container), sem recriar.
- E se o backend subir antes da infra estar pronta? → cada passo de backend só roda após health da infra própria (Postgres/Keycloak prontos).
- Migração de bundles antigos / entidades das 6 fixas → fora de escopo; a rede de referência legada continua usando a infra compartilhada (`deploy/local/compose.yml`) e não é tocada.
- `cbEndpoint` deve usar o caminho correto `/api/v1/onboarding/credential-request` (corrigir nos samples/bundle; bug atual aponta para `/api/v1/credential-request`).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O toolkit DEVE levantar, por entidade, uma instância **dedicada** de Postgres, Keycloak e Redis (isolamento físico, não apenas lógico). NÃO DEVE usar as instâncias compartilhadas do `deploy/local/compose.yml`.
- **FR-002**: A infra per-entity DEVE vir de **templates novos** parametrizados (em `provisioning/templates/...`), não dos composes legados. Nomes de container, portas, volumes e realm Keycloak são derivados da entidade/spoke.
- **FR-003**: O toolkit DEVE renderizar o `.env` da entidade (análogo ao `.env.infra.<entity>`) a partir de um template, apontando para a **infra própria** da entidade e para os endereços de contrato do spoke (found/bundle) — nunca reusando os `.env.infra.*` fixos.
- **FR-004**: O toolkit DEVE levantar o **backend** da entidade (compliance, auth, payment-orchestrator, api-gateway) via um **compose-template** parametrizado por entidade, reutilizando as imagens/Dockerfiles existentes como building blocks. NÃO DEVE modificar `backend/docker-compose-backend.*` legados.
- **FR-005**: No `found`, o toolkit DEVE levantar o backend do **banco central** e os frontends **governance** e **treasury**; o api-gateway do CB DEVE expor `/api/v1/onboarding/credential-request` (autorização de onboarding).
- **FR-006**: No `join`, o toolkit DEVE levantar o backend do **banco comercial** e o frontend **bank**, apontando para o CB via `CENTRAL_BANK_API_URL`. O passo `start-backend` passa a ser real (não no-op).
- **FR-007**: Cada Keycloak per-entity DEVE ser provisionado com o realm/clients da entidade a partir de um template de realm (sem reusar/alterar o `init.sh` legado).
- **FR-008**: O `found` DEVE levantar o stack **NOC** (noc-backend + noc-db + noc-agent + frontend) do spoke, com o agent configurado para os componentes provisionados.
- **FR-009**: Todos os passos novos DEVEM ser **idempotentes** (Check por health/estado) e respeitar a ordem (infra → backend → frontend); health-gate antes de cada dependência.
- **FR-010**: NÃO DEVE haver chave privada em arquivo/env/log; segredos de dev locais seguem o padrão já usado (render local), com `keyProvider`/`certSource` reais adiados para a Fase 4.
- **FR-011**: A rede de referência (`deploy/local/compose.yml`, `make/*.mk`, `backend/docker-compose-backend.*`, `frontend/docker-compose.spoke-*.yml`, `interop/hub-and-spoke/noc/*`, scripts) NÃO DEVE ser modificada; permanece como amostra. Os templates novos vivem no toolkit/provisioning.
- **FR-012**: `cbEndpoint` (samples/bundle) DEVE usar `/api/v1/onboarding/credential-request`.
- **FR-013**: O `image` do manifesto (build vs registry-ref, concat.md §7) DEVE selecionar a origem das imagens de backend/frontend (value-only, sem código).

### Key Entities

- **EntityInfra (novo)**: Postgres + Keycloak + Redis dedicados a uma entidade; nomes/portas/realm derivados de spoke/entidade.
- **EntityBackendStack**: os 4 serviços da entidade, parametrizados, apontando para a EntityInfra e para os contratos do spoke.
- **EntityFrontend**: app(s) da entidade (governance+treasury no CB; bank no banco), com `VITE_API_URL` da entidade.
- **NOCStack**: monitoramento do spoke (subido no found).
- **GovernancePortal**: frontend `governance` + `compliance` (CB-side) — autoriza onboarding; `GOVERNANCE_ROLE` permanece no CB.

## Success Criteria *(mandatory)*

- **SC-001**: Um spoke fundado por manifesto tem CB com **infra dedicada** (Postgres/Keycloak/Redis próprios) — verificável por containers/portas distintos do compartilhado legado.
- **SC-002**: O api-gateway do CB responde `/api/v1/onboarding/credential-request` após o found — habilitando o E2E do join.
- **SC-003**: Um banco que entrou por `join` tem infra+backend próprios e api-gateway apontando para o CB.
- **SC-004**: Dois spokes (BRL/COP) + bancos no mesmo host operam sem colisão de portas/infra.
- **SC-005**: NOC do spoke responde health e reporta os componentes.
- **SC-006**: A rede de referência legada permanece intacta e verde.
- **SC-007**: Os passos novos passam em testes unitários (sem containers) seguindo o padrão TK-5/TK-9.

## Assumptions

- Reuso das **imagens/Dockerfiles** de backend/frontend existentes; o que muda é a **orquestração** (templates parametrizados + render de env), não o código dos serviços.
- O Governance Portal não exige rework on-chain: `GOVERNANCE_ROLE` continua no CB (FR-018 da 033); o portal apenas opera sobre o `compliance`.
- Keycloak dedicado por entidade é aceito apesar do custo de recursos (decisão explícita do time).
- Esta feature é grande e provavelmente será fatiada em sub-PRs (infra per-entity; backend; frontend; NOC), cada um com Constitution Check.
- E2E completo (found CB stack → join banco → liquidação) é o objetivo final, validado após esta feature.

## Out of Scope

- Fase 4 (KMS/CA prod).
- Alterar qualquer artefato legado.
- Liquidação HTLC ponta a ponta entre spokes (depende desta feature; esforço seguinte).
