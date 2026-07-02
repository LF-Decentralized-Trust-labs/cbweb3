# Implementation Plan: RL-1/RL-2/RL-3 — Relay: Registro Dinâmico de Spokes

**Branch**: `031-relay-spoke-registry` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/031-relay-spoke-registry/spec.md`

## Summary

Substituir a topologia bilateral estática (`spokeA`/`spokeB`) do relay Cacti HTLC por um **registro dinâmico de spokes** (`spokes[]`) carregado de um arquivo YAML via `CACTI_SPOKES_CONFIG`. A resolução de contraparte (`resolveCounterpartContractId`) é substituída por lookup direto no registro por `dest_spoke_id` (campo já presente nos legs de FX Agreement após MD-3). O loop de polling e o mapa de connectors Besu são gerados dinamicamente a partir do registro. Um shim de compatibilidade preserva o comportamento atual com vars legadas `SPOKE_A_*`/`SPOKE_B_*` sem alterar o `docker-compose.yaml` existente.

## Technical Context

**Language/Version**: TypeScript 5.4+ (Node.js 20 LTS — conforme devDependencies `@types/node ^20`)  
**Primary Dependencies**: `js-yaml ^4.1.0` (novo — parsing YAML), `@hyperledger/cactus-plugin-ledger-connector-besu ^2.0.0`, `@grpc/grpc-js ^1.10.0`, `express ^4.18.0`  
**Test Framework**: Vitest ^2.0.0 (novo — não existe test runner no projeto)  
**Storage**: Nenhum novo. `RelayStore` (arquivo JSON existente) é preservado.  
**Target Platform**: Docker container Linux (Node.js 20 LTS); desenvolvido e testado em Linux host.  
**Project Type**: Serviço de relay (long-running process + REST API)  
**Performance Goals**: Lookup de spoke por `dest_spoke_id` O(1) via Map. Sem degradação mensurável em polling com N ≤ 10 spokes.  
**Constraints**: Zero novos arquivos no `docker-compose.yaml` da rede de exemplo. Sem breaking change no `RelayStore`. Sem alteração no fluxo de liquidação HTLC (só muda como o destino é resolvido).  
**Scale/Scope**: 3 arquivos TypeScript modificados (`config.ts`, `htlc-relay.ts`, `index.ts`). Novos arquivos: `src/spokes-config.ts` (loader), `src/htlc-relay.test.ts` (testes). Adição de 2 dependências ao `package.json`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Status | Observação |
|---|---|---|
| I. Isolamento de Cenário | ✅ PASS | Todas as mudanças dentro de `scenario-a/interop/hub-and-spoke/cacti/`. Nenhum arquivo de Scenario B tocado. |
| II. Privacidade | ✅ PASS | Sem mudanças em tokens, contratos, ou fluxos de privacidade. O relay é infraestrutura de roteamento. |
| III. Atomicidade | ✅ PASS | Fluxo HTLC lock + secret reveal preservado integralmente. A única mudança é como o endpoint do destino é resolvido (lookup vs campo estático). |
| IV. Compliance Gate | ✅ PASS | O relay não é um compliance boundary. Nenhum gate de identity/AML é bypassado. |
| V. Test-First | ✅ PASS (com ação requerida) | Vitest adicionado. Testes para `loadSpokesConfig` e `lookupSpokeByDestId` escritos antes da implementação (ver Fase 1). Testes para `resolveCounterpartContractId` escritos antes de ser removido. |
| VI. Observabilidade | ✅ PASS | FR-008 (log de registro no startup), FR-009 (log de erro para dest_spoke_id desconhecido), FR-010 (warning de deprecação do shim) — todos mapeados a chamadas de log explícitas. |

**Re-check pós-design**: Nenhuma violação identificada no design detalhado. O shim de compatibilidade mantém a rede de exemplo verde (Princípio I — sem alteração de arquivos de Scenario A existentes não-cacti).

## Project Structure

### Documentation (this feature)

```text
specs/031-relay-spoke-registry/
├── plan.md              # Este arquivo
├── spec.md              # Especificação de feature
├── research.md          # Fase 0: decisões de tecnologia
├── data-model.md        # Fase 1: schema SpokeConfig, YAML, shim
├── contracts/
│   └── env-vars.md      # Contrato de interface: vars de ambiente + YAML schema
├── checklists/
│   └── requirements.md  # Checklist de qualidade do spec
└── tasks.md             # Fase 2 output (/speckit-tasks — NÃO criado aqui)
```

### Source Code (repository root)

```text
scenario-a/interop/hub-and-spoke/cacti/
├── package.json                  # Adicionar js-yaml, @types/js-yaml, vitest
├── tsconfig.json                 # Sem mudança
├── docker-compose.yaml           # Sem mudança
├── src/
│   ├── config.ts                 # MODIFICAR: spokeA/spokeB → spokes[]; loader; shim
│   ├── htlc-relay.ts             # MODIFICAR: SpokeDep sem counterpartGrpc; lookup por dest_spoke_id
│   ├── index.ts                  # MODIFICAR: connectors map dinâmico; startup log por spoke
│   ├── spokes-config.ts          # CRIAR: loadSpokesConfig(), validateSpokesConfig(), buildLegacyShim()
│   ├── relay-store.ts            # Sem mudança
│   └── htlc-relay.test.ts        # CRIAR: testes Vitest para loadSpokesConfig + HtlcRelay lookup
└── env-sample                    # ATUALIZAR: adicionar CACTI_SPOKES_CONFIG; deprecar SPOKE_A/B_PAYMENT_GRPC
```

**Structure Decision**: Projeto single-package TypeScript. O loader de config é extraído em `spokes-config.ts` (separação de responsabilidades: `config.ts` monta o objeto de configuração, `spokes-config.ts` implementa parsing/validação do YAML e o shim).

## Complexity Tracking

> Nenhuma violação de constituição identificada. Seção preenchida apenas com decisões de design não-óbvias.

| Decisão | Por quê | Alternativa Rejeitada |
|---|---|---|
| Adicionar Vitest ao projeto sem test runner existente | Princípio V (test-first) é NON-NEGOTIABLE; não há como escrever testes sem um runner | Sem testes (bloqueado pela constituição) |
| `spokes-config.ts` como módulo separado de `config.ts` | Isola a lógica de IO (leitura de arquivo YAML) da lógica de configuração; facilita mocking em testes | Tudo em `config.ts` — dificulta unit test do loader sem side effects de processo |
| gRPC client por spoke criado no startup (não por-trade) | Evita overhead de TCP+TLS handshake em cada liquidação | Por-trade: mais simples de implementar mas inaceitável em produção |
| Shim de vars legadas como função separada, não inline | Testável isoladamente; pode ser removido em versão futura sem tocar no loader principal | Inline no loader — acoplamento desnecessário |
