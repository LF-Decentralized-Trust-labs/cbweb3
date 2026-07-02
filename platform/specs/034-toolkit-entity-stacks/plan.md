# Implementation Plan: Toolkit provisiona stacks per-entity (infra isolada + backend + frontend + NOC)

**Branch**: `034-toolkit-entity-stacks` | **Date**: 2026-06-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/034-toolkit-entity-stacks/spec.md`

## Summary

Estender o toolkit (após a rede do spoke já subir — features 026–033) para levantar a **stack operacional** de cada entidade: **Postgres e Redis dedicados**, **Keycloak como estratégia** (shared local / per-entity), **backend** (compliance/auth/payment-orchestrator/api-gateway) e **frontend**, mais o **NOC** (monitoramento) e o **Governance Portal** no `found`. Tudo via **templates novos parametrizados + render de `.env`**, reutilizando as imagens/Dockerfiles existentes como building blocks, **sem alterar a rede de referência legada**. Isso destrava o E2E do join (o api-gateway do CB precisa estar no ar para o `request-cert`).

## Technical Context

**Language/Version**: Go 1.26+ (toolkit/engine); YAML 3.8+ (compose templates); `text/template` (render de `.env` e realm Keycloak).
**Primary Dependencies**: `os/exec` (docker compose v2), `net/http` (health checks), stdlib; sem novas deps no `toolkit/go.mod`. Imagens: `postgres:17-alpine`, `redis:7-alpine`, Keycloak (Dockerfile de `deploy/local/keycloak` ou imagem), imagens de backend/frontend (build ou registry ref via `image`).
**Storage**: Filesystem — `SPOKE_DATA_DIR/<entity>/...` (`.env` renderizado, realm JSON, volumes via bind/named); Postgres/Redis dedicados por entidade.
**Testing**: `go test ./...` com mocks (render de env, derivação de portas, composeEnv, health) sem containers; validação com containers atrás de build tag `integration`.
**Target Platform**: Linux + Docker Compose v2.
**Project Type**: CLI + engine (toolkit standalone do Scenario A).
**Performance/Constraints**: idempotência por passo; health-gate infra→backend→frontend; sem chaves privadas em arquivo/env/log; **não tocar** nos legados; portas determinísticas por entidade/spoke (sem colisão multi-spoke no mesmo host).
**Scale/Scope**: N spokes × N bancos; demonstração 2 spokes × 2 bancos.

## Constitution Check

*GATE: deve passar antes da Fase 0 e reavaliado após Fase 1.*

- **Isolamento de cenário**: ✅ tudo em `scenario-a/toolkit` + `scenario-a/provisioning`; nada em `scenario-b/`.
- **Privacidade**: ✅ não muda o modelo (Zeto/Noto/Pente). Subir backend/infra não expõe PII/valores on-chain. Segredos de dev locais via render (mesmo padrão atual); `keyProvider`/`certSource` reais na Fase 4.
- **Atomicidade / timeouts**: ✅ passos idempotentes com health-gate; falha aborta sem estado parcial inconsistente.
- **Gate de compliance no API gateway**: ✅ esta feature **levanta** o compliance + api-gateway + Governance Portal do CB (reforça o gate; não o contorna). `GOVERNANCE_ROLE` permanece no CB.
- **Test-first, toda camada**: ✅ render de `.env`, derivação de portas, composeEnv e health têm teste falhando antes; integração atrás de tag.
- **Observabilidade**: ✅ logs JSON estruturados por passo (mesmo logger do engine).
- **PR enfraquecendo compliance/segurança**: N/A — reforça (sobe o gate de onboarding).

**Resultado**: PASS, com itens de Complexity Tracking (templates novos vs reuso de composes fixos; Keycloak estratégia).

## Project Structure

```text
specs/034-toolkit-entity-stacks/
├── spec.md        # done
├── plan.md        # this file
├── research.md    # Fase 0 — matriz de env vars por serviço; realm Keycloak template; portas
├── data-model.md  # Fase 1 — EntityInfra, EntityBackendStack, EntityFrontend, NOCStack
├── contracts/     # Fase 1 — assinaturas das funções/steps (render env, start-infra, start-backend, start-frontend, start-noc)
└── tasks.md       # Fase 2
```

```text
scenario-a/provisioning/templates/
├── central-bank/
│   ├── infra-compose.yaml      # NOVO — Postgres + Redis (+ Keycloak se per-entity) do CB
│   ├── backend-compose.yaml    # NOVO — 4 serviços do CB, parametrizado
│   ├── frontend-compose.yaml   # NOVO — governance + treasury
│   ├── env.tmpl                # NOVO — render do .env.infra do CB (aponta p/ infra própria + contratos do spoke)
│   └── keycloak-realm/*.tmpl   # NOVO — realms central-bank-<x> + cbweb3 (NOC)
├── commercial-bank/
│   ├── infra-compose.yaml      # NOVO — Postgres + Redis (+ Keycloak se per-entity) do banco
│   ├── backend-compose.yaml    # NOVO
│   ├── frontend-compose.yaml   # NOVO — bank
│   └── env.tmpl                # NOVO
└── noc/
    └── compose.yaml            # NOVO (ou parametrização do de interop/) — noc-backend+db+agent+frontend

scenario-a/toolkit/engine/orchestrator/
├── entityenv.go        # NOVO — render do .env por entidade (text/template)
├── ports.go            # NOVO/￼estender — derivação determinística de portas por entidade/spoke
├── step_start_infra.go        # NOVO — Postgres/Redis (+ Keycloak) + health
├── step_provision_keycloak.go # NOVO — realms/clients via kcadm (estratégia)
├── step_start_backend_stack.go# NOVO — 4 serviços + health (found CB / join bank)
├── step_start_frontend.go     # NOVO — frontend(s) da entidade
├── step_start_noc.go          # NOVO — NOC (found)
└── *_test.go

# NÃO TOCAR: deploy/local/compose.yml, make/*.mk, backend/docker-compose-backend.*,
#            frontend/docker-compose.spoke-*.yml, interop/hub-and-spoke/noc/*, init.sh
```

**Structure Decision**: novos templates + steps no toolkit; o `start-backend` do join (hoje no-op) passa a usar o `start-backend-stack`. Reuso das imagens/Dockerfiles existentes (build/registry via `image`).

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Templates novos de infra/backend/frontend + render de `.env`, em vez de reusar `docker-compose-backend.*`/`compose.yml`/`init.sh` | Os composes/env/realm legados são **fixos para 6 entidades** e a constituição exige preservar a rede de referência. Para N entidades arbitrárias é preciso parametrizar. | Reusar os arquivos fixos → não serve entidade arbitrária e/ou exigiria editá-los (proibido). |
| Infra **dedicada** (Postgres/Redis) por entidade | Isolamento físico real é a meta de escalabilidade N (remove o gargalo de instâncias compartilhadas). | Manter compartilhado (isolação só lógica) → bloqueia N participantes de verdade. |
| Keycloak como **estratégia** (shared/per-entity) com NOC/Governance sempre no CB | Custo de N JVMs Keycloak no local vs isolamento prod; e NOC/Governance são CB-side por design. | Sempre per-entity → pesado no local; sempre shared → não atende isolamento prod. |
