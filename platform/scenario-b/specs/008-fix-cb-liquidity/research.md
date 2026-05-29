# Research: Fix CB Liquidity (Frontend Alignment)

**Feature**: `008-fix-cb-liquidity`  
**Date**: 2026-05-21  
**Status**: Complete

## 1) Canonical behavior source

**Decision**: Adotar como fonte de verdade primária o runbook `docs/runbooks/integracao-scenario-b.md` v6.0 (especialmente seções 16 e 19), validado pelos tryouts oficiais.

**Rationale**:
- O spec clarificado referencia explicitamente runbook v6.0.
- Seções 16 e 19 trazem códigos de erro, estados, timeouts e recomendações de UX obrigatórias.
- Tryouts refletem comportamento executável do ambiente atual.

**Alternatives considered**:
- Usar apenas implementação atual do frontend: rejeitado (há divergências com o fluxo soberano).
- Usar apenas specs antigas: rejeitado (spec-007 alterou fluxo e bloqueou G5-cross).

## 2) Fluxo canônico de provisão soberana CB

**Decision**: Modelar UX de governança em quatro fases canônicas:
1. `bridge/lock-mint` + polling `bridge_state=ACTIVE` (5s, timeout 120s)
2. `amm/liquidity/commit`
3. execução assíncrona via watcher até `commit.status=EXECUTED` (polling 3s, timeout UI 60s, aviso >30s)
4. validação `pool_status=ACTIVE`

**Rationale**:
- Definido no runbook v6.0 seção 19 e no spec clarificado FR-001.
- Compatível com `tryout-sovereign-cb-liquidity.sh`.

**Alternatives considered**:
- Manter wizard legado de commit-reveal sem fase explícita de bridge: rejeitado.
- Polling único apenas de pool status: rejeitado por baixa rastreabilidade operacional.

## 3) Tratamento do anti-pattern G5-cross

**Decision**: Tratar `CROSS_CB_MINT_PROHIBITED` (HTTP 403) como bloqueio definitivo de fluxo legado, com orientação acionável para fluxo soberano e sem retry automático.

**Rationale**:
- Runbook seção 16 e seção 19 mapeiam esse erro como bloqueio esperado.
- `tryout-scenario-b-e2e.sh` já valida esse cenário como PASS esperado.

**Alternatives considered**:
- Fallback automático para outro endpoint: rejeitado (encobre causa raiz).
- Retry com backoff: rejeitado (erro é de regra de negócio, não transitório).

## 4) Pré-condição de bridge para commit

**Decision**: Tratar `BRIDGE_POSITION_NOT_ACTIVE` (HTTP 422) como estado de espera obrigatório, impedindo avanço para commit e acionando polling de bridge até 120s.

**Rationale**:
- Gate obrigatório no runbook seção 19 e no spec FR-003.
- Evita commits prematuros antes de confirmação do Relayer.

**Alternatives considered**:
- Permitir commit e reconciliar depois: rejeitado (contraria gate explícito).
- Mensagem genérica sem ação recomendada: rejeitado por baixa operabilidade.

## 5) Separação de papéis por app

**Decision**:
- `bank`: somente jornada comercial (quote/swap/transfer), sem controles de governança.
- `governance`: fluxo soberano de CB + circuit breaker, sem formulário de swap comercial.

**Rationale**:
- Especificado em FR-011 do spec 008.
- Reduz erro operacional por confusão de papéis.

**Alternatives considered**:
- Exibir todos os controles em ambos os apps com RBAC visual: rejeitado por risco de UX ambígua.

## 6) Preservação de Scenario A (zero regressão)

**Decision**: Manter gating existente por `isScenarioB` com isolamento total de rotas/menu de Scenario B quando `VITE_SCENARIO != "b"`.

**Rationale**:
- FR-008 exige preservação integral de Scenario A.
- Base já implementada em spec 004 (`routes/index.tsx` e `Sidebar.tsx` em ambos os apps).

**Alternatives considered**:
- Toggle runtime de cenário: rejeitado (fora de escopo e mais propenso a regressão).

## 7) Fluxo comercial de swap (bank)

**Decision**:
- bloquear submissão quando `pool_status != ACTIVE`
- mapear erros `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `CIRCUIT_BREAKER_HALTED`
- atualizar quote em janela 10-15s

**Rationale**:
- Seção 16 + recomendações de UX do runbook.
- FR-004 e FR-005 do spec.

**Alternatives considered**:
- Exibir erro bruto de backend: rejeitado (não atende NFR de mensagem operacional).

## 8) Dependências entre specs

**Decision**:
- `004-scenario-b-frontend-integration`: baseline de estrutura frontend, rotas, stores, serviços e gating.
- `005-cooperative-liquidity`: legado do wizard e tipos de commit/pool; deve ser realinhado ao anti-G5.
- `007-bridge-based-cb-liquidity`: comportamento soberano canônico backend e sequenciamento operacional.

**Rationale**:
- Evita redesenho e reduz risco de incompatibilidade funcional.

**Alternatives considered**:
- Ignorar specs anteriores e redesenhar do zero: rejeitado por alto custo e risco.

## 9) Superfícies impactadas

**Decision**: Concentrar mudanças em páginas/stores/services/types e navegação já existentes, sem criar nova arquitetura de frontend.

**Rationale**:
- Escopo de alinhamento (não feature nova).
- Menor risco para Scenario A.

**Alternatives considered**:
- Novo módulo de orquestração compartilhado entre apps: rejeitado nesta fase por aumento de complexidade.
