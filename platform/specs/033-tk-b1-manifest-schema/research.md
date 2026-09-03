# Research — TK-B1 (Schema de Manifesto e Validação)

Fase 0. Resolve as incógnitas do Technical Context. Formato: Decisão / Rationale / Alternativas.

## R1 — Abordagem de validação: JSON-Schema vs. validação em Go

- **Decisão**: A **validação programática em Go** é a **fonte de verdade** (regras estruturais +
  semânticas: modos, colisões, segredos, warnings). O **JSON-Schema v1** é publicado como
  **contrato para editores/CI** (autocompletar/lint no editor; check opcional em CI com uma
  ferramenta padrão), **sem** adicionar uma lib de JSON-Schema como dependência de **runtime** do
  toolkit.
- **Rationale**: (a) várias regras (colisão de chainId/porta entre manifestos, ausência de
  segredos, requisitos por modo) não são expressáveis de forma limpa em JSON-Schema; (b) evita
  dependência nova fora do stack (Constituição — "novas dependências exigem justificativa"); (c)
  espelha o toolkit de referência (Cenário A), onde a validação vive em Go e há um `.schema.yaml`
  para editores. FR-009 exige que os dois **concordem** nas regras estruturais.
- **Alternativas**: (i) validar em runtime via lib de JSON-Schema → dependência nova + regras
  semânticas ainda ficariam em Go (duplicação). (ii) só JSON-Schema, sem Go → não cobre colisões
  nem segredos. Ambas rejeitadas.

## R2 — `kind`/`apiVersion` e discriminadores de modo

- **Decisão**: `kind: ParticipantDeployment`, `apiVersion: cbweb3b/v1`; um **único kind** para os
  três modos, discriminados por `spec.mode` + `spec.topology.role`.
- **Rationale**: mantém o despacho de CLI idêntico ao do toolkit de referência e simplifica o
  schema (um documento, regras condicionais por modo). Roadmap §4.
- **Alternativas**: kinds separados (`HubDeployment`/`ParticipantDeployment`) → mais superfície,
  sem ganho nesta fase. Rejeitada.

## R3 — Atribuição de `chainId` e detecção de colisão

- **Decisão**: `chainId` (e portas rpc/ws/p2p) são **informados pelo operador** no manifesto; o
  toolkit **valida unicidade e colisão** ao processar um **conjunto** de manifestos — não
  auto-atribui.
- **Rationale**: decisão registrada no roadmap (§13, §14.C); dá determinismo e coexistência com
  os IDs legados (1337/1338/1339). FR-008.
- **Alternativas**: auto-atribuição pelo toolkit → surpresa/instabilidade entre execuções.
  Rejeitada.

## R4 — Modo de invocação sem execução

- **Decisão**: expor um caminho que **apenas parseia+valida+reporta** — `cbweb3b validate -f <m>`
  (e/ou `apply -f <m> --dry-run`), com `-o json|yaml`. Sem steps/execução nesta fase.
- **Rationale**: entrega o MVP (validação declarativa) sem depender do motor (TK-B5+); FR-010.
- **Alternativas**: só expor via `apply` completo → acopla a validação ao motor inexistente.
  Rejeitada.

## R5 — `node.validator: true` no `join`

- **Decisão**: **aceitar + emitir warning** (não rejeitar) — resolvido na spec (FR-013), pergunta
  de clarificação Q1 → A.
- **Rationale**: mantém o modelo (banco = full node não-validador) sem endurecer contra a
  promoção diferida (`vote-qbft`). Roadmap §6/§13.

**Saída**: todas as incógnitas do Technical Context resolvidas; nenhuma `NEEDS CLARIFICATION`
remanescente.
