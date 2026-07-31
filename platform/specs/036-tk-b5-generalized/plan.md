# Implementation Plan: Toolkit do Cenário B — Relay generalizado (TK-B5)

**Branch**: `036-tk-b5-generalized` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/036-tk-b5-generalized/spec.md`

## Summary

Generalizar o relay Cacti (`scenario-b/interop/hub-and-spoke/cacti/`, TypeScript) de **dois spokes
fixos** para **N spokes dinâmicos**: registry dinâmico + boot neutro (≥0), um conector/watcher por
spoke, roteamento cross-currency por **lookup de gateway por `spoke_out`**, endpoint
**`POST /api/v1/spokes`** para registro em runtime (sem restart) com **registro persistido** (JSON)
recarregado no boot, e **validação de `isPaused()`** no AMM antes de encaminhar swaps (corrige uma
violação atual do Princípio III). Em paralelo, entregar no toolkit Go a interface **`RelayRegistrar`**
(interface + local in-memory + stub de produção HTTP + factory por URI), consumida pelo motor
(TK-B6). A **autenticação por CB (§14.D) fica fora de escopo** (pendente do project-lead); o segredo
compartilhado permanece como fallback local/dev. Estratégia: extrair a **lógica pura** (registry,
store, roteamento, checagem de breaker, validação de payload) em módulos testáveis, mantendo finos
os pontos que dependem do Cacti/Besu.

## Technical Context

**Language/Version**: TypeScript 5.4 (relay Cacti, Node.js 20 LTS) + Go 1.26 (`RelayRegistrar` no
módulo `scenario-b/toolkit`).
**Primary Dependencies**: relay — **nenhuma nova** de runtime: `express` (endpoint), `ethers` v6
(chamada `isPaused()` no AMM), `@hyperledger/cactus-plugin-ledger-connector-besu`, `socket.io` (já
presentes); persistência via `fs` (stdlib Node). Testes do relay: **`node:test`** (nativo do Node
20) + `ts-node` (já é devDependency) — sem novo framework. Go — **stdlib apenas** (`net/http`,
`encoding/json`, `net/url`, `sync`); nenhuma dep nova.
**Storage**: relay — **arquivo JSON persistido** (RelayStore) no volume do relay, lido no boot e
atualizado a cada registro; substitui o `cacti-relay-store.json` legado/plano. Go local — registry
**in-memory**.
**Testing**: `node --test` (lógica pura do relay: registry, store, roteamento por `spoke_out`,
validação de payload, decisão de breaker com provider injetável) + `go test` (RelayRegistrar local +
factory + stub prod).
**Target Platform**: serviço relay (contêiner Node) + biblioteca Go do toolkit.
**Project Type**: edição cirúrgica de serviço TypeScript existente (roadmap §9) + novo pacote Go
(`engine/relayregistrar`).
**Performance Goals**: N/A (relay orientado a eventos; sem meta de throughput nesta fase).
**Constraints**: **não** implementar auth-por-CB (§14.D — pendente de lead); **não** alterar a
lógica de circuit breaker dos contratos (só validar `isPaused` no relay); não importar `scenario-a/`;
`isPaused` indisponível ⇒ **falha segura** (não encaminhar); sem segredos no `RelayRegistrar`.
**Scale/Scope**: N spokes dinâmicos (testado com N = 1 e 3); relay + 1 interface Go plugável.

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B5) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Mudanças restritas a `scenario-b/` (relay + toolkit); **não** importa `scenario-a/`. |
| **II. Privacy by Design** | ✅ O relay não coloca PII/valores em plaintext on-chain nem muda tokens; o `RelayRegistrar` não persiste segredos. Neutro ao princípio. |
| **III. Atomic Settlement** | ✅ **Reforça**: FR-008 adiciona a validação `isPaused()` **antes** de encaminhar swaps — corrige uma violação atual (o relay hoje não checa o breaker). Falha segura quando a consulta é indisponível. |
| **IV. Compliance Gate** | ✅ O relay não contorna o gate. Auth-por-CB fica fora de escopo e o controle atual (segredo compartilhado) é **preservado, não enfraquecido** (FR-010) — logo, sem necessidade de aprovação de lead nesta fase. |
| **V. Test-First** | ✅ Lógica pura extraída e coberta por `node:test` (relay) e `go test` (registrar), com teste falhando antes da implementação. |
| **VI. Observability** | ✅ FR-009: log estruturado do ciclo (spoke registrado, roteamento, recusa por breaker); sem swallow de erros. |

**Novas dependências:** nenhuma (runtime). Testes do relay usam `node:test` (nativo) + `ts-node` (já
presente); o Go usa só stdlib. Não há justificativa de dependência a registrar. **Nota de
compliance:** FR-010 mantém o controle de auth atual — não é um enfraquecimento (não requer
sign-off); a introdução de auth-por-CB (fortalecimento) é que fica adiada até aprovação do lead.

**Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações (corrige uma).

## Project Structure

### Documentation (this feature)

```text
specs/036-tk-b5-generalized/
├── plan.md, spec.md
├── research.md          # Fase 0 — testes do relay, persistência, isPaused via ethers, extração de lógica
├── data-model.md        # Fase 1 — Spoke, SpokeRegistry, RelayStore, GatewayRoute, RelayRegistrar
├── quickstart.md        # Fase 1 — subir relay neutro, registrar spoke, validar breaker
├── contracts/           # Fase 1 — POST /api/v1/spokes + API Go do RelayRegistrar
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/interop/hub-and-spoke/cacti/          # RELAY (TypeScript — edição cirúrgica §9)
├── src/
│   ├── config.ts                 # registry dinâmico (remove spokeA/spokeB fatais)
│   ├── spoke-registry.ts         # NEW — Map<spokeId,Spoke>, add/get/list idempotente
│   ├── relay-store.ts            # NEW — persistência JSON (load no boot, save no registro)
│   ├── spokes-api.ts             # NEW — POST /api/v1/spokes (valida + registra + hidrata runtime)
│   ├── circuit-breaker.ts        # NEW — isPaused(ammAddr, provider) + falha segura
│   ├── cross-currency-swap-relay.ts  # roteamento por lookup de gateway por spoke_out
│   ├── index.ts                  # loop de conectores/watchers a partir do registry; boot neutro
│   └── liquidity-commit-watcher.ts   # ajuste p/ N spokes (aprende registry/AMM dinâmico)
│   └── __tests__/                # node:test — registry, store, routing, breaker, payload
├── env-sample                    # sem SPOKE_A/B_* fatais
└── docker-compose.yaml           # modelo dinâmico (boot neutro)

scenario-b/toolkit/engine/relayregistrar/         # RelayRegistrar (Go)
├── relayregistrar.go             # interface + erros tipados
├── local.go                      # in-memory (registry idempotente)
├── prod.go                       # stub HTTP → POST /api/v1/spokes
├── factory.go                    # New(uri): local | relay://… → prod; else erro
└── *_test.go
```

**Structure Decision**: o relay é **editado no lugar** (é código existente do Cenário B; roadmap §9
autoriza). A generalização é isolada em módulos novos e **puros** (`spoke-registry`, `relay-store`,
`circuit-breaker`, validação de payload) para permitir teste sem Cacti/Besu; `index.ts` fica fino
(fiação). O `RelayRegistrar` entra como novo pacote Go do toolkit, espelhando `keyprovider`/
`certsource`. Sem CLI/motor nesta fase (o motor de TK-B6 consome o `RelayRegistrar`).

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. A dupla-linguagem (TS + Go) **não** é um desvio: o relay já é TypeScript (código
existente) e a interface do toolkit é Go — cada peça na sua camada natural, sem ponte nova. Nenhuma
dependência nova a justificar.
