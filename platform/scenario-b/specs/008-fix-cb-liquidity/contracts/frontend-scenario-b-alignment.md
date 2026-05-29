# Frontend Integration Contract: Scenario B Alignment (Spec 008)

## Purpose

Definir o contrato funcional de consumo de APIs e mapeamento de UX para alinhamento dos apps `bank` e `governance` ao comportamento canônico de Scenario B, sem mudança de backend.

## Scope

- Apps: `frontend/apps/bank`, `frontend/apps/governance`
- Sem criação ou alteração de endpoint backend
- Sem mudança em contratos Solidity ou infraestrutura

## Canonical API surfaces consumed

### Governance (soberign CB flow)

1. `POST /api/v2/bridge/lock-mint`
2. `GET /api/v2/bridge/positions?state=ACTIVE`
3. `POST /api/v2/amm/liquidity/commit`
4. `GET /api/v2/amm/liquidity/commits?pool_pair=<pair>&status=EXECUTED`
5. `GET /api/v2/amm/pool/<pair>/status`
6. `POST /api/v2/governance/circuit-breaker/pause`
7. `POST /api/v2/governance/circuit-breaker/resume-request`
8. `POST /api/v2/governance/circuit-breaker/resume-sign`
9. `GET /api/v2/governance/circuit-breaker/status?pair=<pair>`

### Bank (commercial flow)

1. `GET /api/v2/amm/quote/exact-output?pair=<pair>&amount_out=<amount>`
2. `POST /api/v2/amm/swap/exact-output`
3. `GET /api/v2/amm/pool/<pair>/status`
4. `POST /api/v2/amm/token/approve-amm`
5. `GET /api/v2/governance/circuit-breaker/status?pair=<pair>`

## Behavioral contract

### Flow contract: governance sovereign liquidity

1. Fase 1: `lock-mint` + polling de bridge (5s, timeout 120s)
2. Fase 2: submit commit
3. Fase 3: polling commit executado (3s, timeout UI 60s, aviso >30s)
4. Fase 4: validar `pool_status=ACTIVE`

### Flow contract: bank commercial swap

1. Checar `pool_status` antes de habilitar swap
2. Atualizar quote em janela 10-15s
3. Bloquear submissão quando `pool_status != ACTIVE` ou circuito `HALTED`

## Error-to-UX contract

| Error code | App | Mandatory UX behavior |
|---|---|---|
| `CROSS_CB_MINT_PROHIBITED` | governance | Bloqueio definitivo de legado G5-cross + instrução "Use o Fluxo Soberano" |
| `BRIDGE_POSITION_NOT_ACTIVE` | governance | Bloquear avanço de commit + estado de espera + polling bridge |
| `POOL_NOT_ACTIVE` | bank | Manter swap desabilitado + mensagem contextual |
| `SLIPPAGE_LIMIT_EXCEEDED` | bank | Mensagem de mercado movido + ação de atualizar cotação |
| `INSUFFICIENT_POOL_LIQUIDITY` | bank | Mensagem orientando ajuste de valor/aguardar liquidez |
| `CIRCUIT_BREAKER_HALTED` | bank | Bloqueio de envio + aviso regulatório |

## Scenario A compatibility contract

1. Se `VITE_SCENARIO != "b"`, rotas/menus/features de Scenario B ficam ocultos em ambos apps.
2. Nenhuma regressão funcional permitida nas rotas Scenario A existentes.

## Role separation contract

1. `bank` não exibe controles de governança.
2. `governance` não exibe formulário de swap comercial.
3. Componentes compartilhados não devem reintroduzir acoplamento de papéis.

## Out-of-contract

1. Mudanças de payload/semântica de endpoints backend.
2. Novos endpoints ou alterações em autenticação/autorização.
3. Qualquer alteração em contratos on-chain.
