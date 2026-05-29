# Quickstart: Plan Verification for 008 Fix CB Liquidity

**Feature**: `008-fix-cb-liquidity`  
**Scope**: Verificação de alinhamento de frontend (sem mudanças backend)

## 1) Pré-requisitos

1. Stack local Scenario B operacional.
2. Dependências do monorepo frontend instaladas.
3. Tokens/perfis válidos para operar os apps `bank` e `governance`.

## 2) Verificação estática (frontend)

```bash
cd frontend

npm run lint --workspace=bank
npm run lint --workspace=governance

npm run type-check --workspace=bank
npm run type-check --workspace=governance
```

## 3) Verificação de regressão Scenario A

1. Executar build/dev sem `VITE_SCENARIO=b`.
2. Confirmar que rotas e menu de Scenario B não aparecem no `bank`.
3. Confirmar que rotas e menu de Scenario B não aparecem no `governance`.
4. Validar páginas principais de Scenario A (navegação e ações básicas) em ambos apps.

## 4) Verificação funcional Scenario B (manual)

### 4.1 Fluxo soberano CB (governance)

Use o tryout soberano como trilha de referência comportamental:

```bash
bash tryouts/tryout-sovereign-cb-liquidity.sh
```

Conferir no frontend de governança:
1. Exibição das quatro fases canônicas (bridge -> commit -> executed -> pool active).
2. Tratamento de `BRIDGE_POSITION_NOT_ACTIVE` com bloqueio + espera.
3. Acompanhamento de `COMMIT_PENDING` com aviso após 30s e timeout em 60s.
4. Confirmação visual de `pool_status=ACTIVE` ao final.

### 4.2 Fluxo comercial swap (bank)

Use o tryout E2E como referência de estados operacionais:

```bash
bash tryouts/tryout-scenario-b-e2e.sh
```

Conferir no frontend de bank:
1. Swap desabilitado quando `pool_status != ACTIVE`.
2. Mapeamento correto de erros:
   - `POOL_NOT_ACTIVE`
   - `SLIPPAGE_LIMIT_EXCEEDED`
   - `INSUFFICIENT_POOL_LIQUIDITY`
   - `CIRCUIT_BREAKER_HALTED`
3. Atualização de quote em janela de 10-15s.

### 4.3 Anti-G5 cross

No fluxo de governança:
1. Tentativa de caminho legado gera `CROSS_CB_MINT_PROHIBITED`.
2. UI bloqueia continuidade e orienta uso do fluxo soberano.
3. Não ocorre retry automático da mesma operação.

## 5) Critérios de aceite da fase de planejamento

1. Artefatos de plano gerados (`plan.md`, `research.md`, `data-model.md`, `quickstart.md`, `contracts/`).
2. Escopo explicitamente limitado a frontend `bank` e `governance`.
3. Estratégia de regressão de Scenario A documentada.
4. Estratégia de validação com lint/type-check + manual E2E documentada.
