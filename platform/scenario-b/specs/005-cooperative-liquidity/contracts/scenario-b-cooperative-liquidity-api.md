# API Contract: Provisão Cooperativa de Liquidez

**Feature**: `005-cooperative-liquidity`
**Date**: 2026-05-14
**Arquivo base**: `backend/services/api-gateway/openapi/v2/scenario-b.yaml`

Este documento descreve os **novos endpoints** e as **modificações nos endpoints existentes** necessários para suportar o modelo de provisão cooperativa.

---

## Novos Endpoints

### `POST /api/v2/amm/liquidity/commit`

Registra a intenção de depósito de um provedor (CB ou MLP) para um lado específico do par. Não move fundos. Inicia o mecanismo de commit-reveal.

**Autorização**: JWT com role `central_bank` ou `mlp` (Keycloak)

**Request Body**:
```yaml
LiquidityCommitRequest:
  type: object
  required: [pool_pair, provider_id, side, amount]
  properties:
    pool_pair:
      type: string
      example: "BRL-USD"
    provider_id:
      type: string
      description: Identificador do banco central ou MLP
      example: "central_bank_a"
    side:
      type: string
      enum: [A, B]
      description: "A = token_a (ex.: BRL); B = token_b (ex.: USD)"
    amount:
      type: string
      description: Quantidade em menor unidade do token (inteiro como string)
      example: "100000"
```

**Response 201 Created**:
```yaml
LiquidityCommitResponse:
  type: object
  properties:
    commit_id:
      type: string
      format: uuid
    pool_pair:
      type: string
    side:
      type: string
      enum: [A, B]
    amount:
      type: string
    status:
      type: string
      enum: [PENDING, MATCHED]
      description: MATCHED se a contraparte já havia commitado previamente
    counterpart_commit_id:
      type: string
      format: uuid
      nullable: true
    expires_at:
      type: string
      format: date-time
    lp_ids:
      type: array
      items:
        type: string
        format: uuid
      description: Preenchido apenas quando status = MATCHED (IDs das LiquidityPositions criadas)
      nullable: true
```

**Response 409 Conflict**:
```json
{ "error": "COMMIT_ALREADY_EXISTS", "message": "Já existe um commit PENDING ou MATCHED para BRL-USD lado A" }
```

**Response 403 Forbidden**:
```json
{ "error": "NOT_AUTHORIZED_LP", "message": "Entidade não autorizada como provedor de liquidez" }
```

---

### `GET /api/v2/amm/liquidity/commits`

Lista commits ativos (PENDING ou MATCHED) para um par.

**Query Parameters**:
```
pool_pair: string (required)
status:    PENDING | MATCHED | EXECUTED | EXPIRED (optional, default: todos)
```

**Response 200 OK**:
```yaml
LiquidityCommitListResponse:
  type: array
  items:
    $ref: '#/components/schemas/LiquidityCommitResponse'
```

---

### `DELETE /api/v2/amm/liquidity/commits/{commit_id}`

Cancela um commit em estado PENDING antes da expiração. Não aplicável a commits MATCHED (já coordenados).

**Response 200 OK**:
```json
{ "commit_id": "uuid", "status": "EXPIRED", "cancelled_at": "2026-05-14T18:00:00Z" }
```

**Response 409 Conflict**:
```json
{ "error": "COMMIT_ALREADY_MATCHED", "message": "Commit já foi pareado; cancele via acordo bilateral" }
```

---

## Endpoints Modificados

### `POST /api/v2/amm/liquidity/add` — Modificação

O endpoint existente **permanece inalterado** para compatibilidade retroativa (MLPs com ambas as moedas, scripts existentes). Adiciona campo opcional `deposit_side` à resposta.

**Response 201 — campos adicionais**:
```yaml
# Campos adicionados ao LiquidityPositionResponse existente
deposit_side:
  type: string
  enum: [A, B, BOTH]
  description: Lado do par depositado. BOTH para depósito duplo (modelo legado).
shares_percentage:
  type: string
  nullable: true
  description: Participação percentual no pool (4 casas decimais). Null para posições BOTH legadas.
```

---

### `POST /api/v2/amm/liquidity/remove` — Modificação

Endpoint existente. Modifica o comportamento de retirada para posições cooperativas (deposit_side = A ou B): retorna fração proporcional das reservas atuais + taxas acumuladas, em vez dos valores originais.

**Request** — sem mudança:
```json
{ "lp_id": "uuid", "pool_pair": "BRL-USD", "provider_bank_id": "central_bank_a" }
```

**Response 200 — campos adicionais**:
```yaml
# Campos adicionados ao response existente
returned_token_a:
  type: string
  description: Quantidade real de token_a retornada (pode diferir de token_a_contributed para posições cooperativas)
returned_token_b:
  type: string
  description: Quantidade real de token_b retornada
fee_claim_paid:
  type: string
  description: Total de taxas acumuladas incluídas na retirada
withdrawal_mode:
  type: string
  enum: [LEGACY, PROPORTIONAL]
  description: LEGACY para deposit_side=BOTH; PROPORTIONAL para deposit_side=A ou B
```

---

### `GET /api/v2/amm/pool/{pool_pair}/status` — Modificação

Adiciona campos de status cooperativo à resposta existente.

**Response 200 — campos adicionais**:
```yaml
pool_status:
  type: string
  enum: [EMPTY, PENDING_COUNTERPART, ACTIVE]
fee_rate_bps:
  type: integer
  description: Taxa de swap atual em basis points (ex.: 30 = 0.30%)
total_lp_count:
  type: integer
  description: Número de provedores ACTIVE no pool
pending_commits:
  type: array
  items:
    type: object
    properties:
      side: { type: string, enum: [A, B] }
      expires_at: { type: string, format: date-time }
  description: Commits em PENDING que aguardam contraparte
```

---

## Novos Schemas

```yaml
components:
  schemas:

    LiquidityCommitRequest:
      # descrito acima

    LiquidityCommitResponse:
      # descrito acima

    PoolStatus:
      type: string
      enum: [EMPTY, PENDING_COUNTERPART, ACTIVE]

    CommitSide:
      type: string
      enum: [A, B]

    DepositSide:
      type: string
      enum: [A, B, BOTH]
```

---

## Códigos de Erro Adicionados

| Código HTTP | Error Code | Cenário |
|---|---|---|
| 409 | `COMMIT_ALREADY_EXISTS` | Já existe commit PENDING/MATCHED para (pool_pair, side) |
| 409 | `COMMIT_ALREADY_MATCHED` | Tentativa de cancelar commit já pareado |
| 403 | `NOT_AUTHORIZED_LP` | Entidade sem papel `isLiquidityProvider` no IdentityRegistry |
| 422 | `POOL_NOT_ACTIVE` | Swap tentado com pool em EMPTY ou PENDING_COUNTERPART |
| 404 | `COMMIT_NOT_FOUND` | commit_id inválido ou de outro provider |
| 422 | `COMMIT_EXPIRED` | Tentativa de operação sobre commit já expirado |

---

## Compatibilidade Retroativa

Todos os endpoints existentes permanecem funcionais sem alteração de contrato:
- `POST /api/v2/amm/liquidity/add` — sem breaking change; resposta tem campos adicionais opcionais
- `POST /api/v2/amm/liquidity/remove` — sem breaking change; resposta tem campos adicionais opcionais
- `GET /api/v2/amm/pool/{pair}/status` — sem breaking change; campos adicionados são opcionais
- `GET /api/v2/amm/quote/exact-output` — sem alteração
- `POST /api/v2/amm/swap/exact-output` — sem alteração na interface; comportamento interno muda (fee deducted from amountIn)

---

## Novos Endpoints — Multi-Par PairRegistry (Sessão 2026-05-18)

### `POST /api/v2/amm/pairs/propose`

CB proponente (emissor de tokenA) registra proposta de criação de novo par. O gateway verifica que o JWT pertence ao CB emissor de `token_a_address` e submete `proposePair` on-chain ao `PairRegistry`.

**Autorização**: JWT com role `central_bank` — deve ser o CB emissor de `token_a_address`

**Request Body**:
```yaml
PairProposeRequest:
  type: object
  required: [pool_pair, token_a_address, token_b_address, amm_address]
  properties:
    pool_pair:
      type: string
      example: "BRL-ARS"
      description: Identificador legível do par; único no PairRegistry
    token_a_address:
      type: string
      example: "0xabc..."
      description: Endereço do contrato TokenizedCentralBankMoney do tokenA
    token_b_address:
      type: string
      example: "0xdef..."
      description: Endereço do contrato TokenizedCentralBankMoney do tokenB
    amm_address:
      type: string
      example: "0x123..."
      description: Endereço do contrato AutomatedMarketMaker já implantado para este par
```

**Response** `201 Created`:
```yaml
PairProposeResponse:
  type: object
  properties:
    pair_id:
      type: string
      example: "BRL-ARS"
    status:
      type: string
      enum: [PROPOSED]
    proposed_at:
      type: string
      format: date-time
    tx_hash:
      type: string
      description: Hash da transação on-chain de proposePair
```

**Erros**:
| Código HTTP | Error Code | Cenário |
|---|---|---|
| 409 | `PAIR_ALREADY_EXISTS` | `pool_pair` já registrado no PairRegistry |
| 403 | `NOT_CENTRAL_BANK_OF_TOKEN_A` | JWT não pertence ao CB emissor de tokenA |
| 422 | `AMM_ADDRESS_INVALID` | Endereço AMM não responde a interface esperada |

---

### `POST /api/v2/amm/pairs/confirm`

CB confirmante (emissor de tokenB) aprova proposta pendente. O gateway verifica o JWT e submete `confirmPair` on-chain. Após confirmação, o event watcher D10 atualiza o cache local do gateway.

**Autorização**: JWT com role `central_bank` — deve ser o CB emissor de `token_b_address` do par

**Request Body**:
```yaml
PairConfirmRequest:
  type: object
  required: [pool_pair]
  properties:
    pool_pair:
      type: string
      example: "BRL-ARS"
```

**Response** `200 OK`:
```yaml
PairConfirmResponse:
  type: object
  properties:
    pair_id:
      type: string
      example: "BRL-ARS"
    status:
      type: string
      enum: [ACTIVE]
    amm_address:
      type: string
    confirmed_at:
      type: string
      format: date-time
    tx_hash:
      type: string
      description: Hash da transação on-chain de confirmPair
```

**Erros**:
| Código HTTP | Error Code | Cenário |
|---|---|---|
| 404 | `PAIR_NOT_FOUND` | `pool_pair` não tem proposta PROPOSED no PairRegistry |
| 403 | `NOT_CENTRAL_BANK_OF_TOKEN_B` | JWT não pertence ao CB emissor de tokenB |
| 409 | `PAIR_ALREADY_ACTIVE` | Par já está em estado ACTIVE |

---

### `GET /api/v2/amm/pairs`

Lista todos os pares em estado `ACTIVE` registrados no PairRegistry. Resposta servida do cache DB local (sem RPC call).

**Autorização**: JWT válido (qualquer role autenticada)

**Response** `200 OK`:
```yaml
PairListResponse:
  type: object
  properties:
    pairs:
      type: array
      items:
        type: object
        properties:
          pool_pair:
            type: string
            example: "BRL-USD"
          amm_address:
            type: string
          token_a_address:
            type: string
          token_b_address:
            type: string
          proposer_cb:
            type: string
          confirmer_cb:
            type: string
          confirmed_at:
            type: string
            format: date-time
```

---

## Seção MLP — Path B (Gateway Dedicado)

> Os endpoints abaixo são servidos pelo `api-gateway-mlp` na porta `68080`.
> Mesmos endpoints dos CBs — diferenciados pelo **gateway de entrada** e pelo **JWT realm**.

### `POST /api/v2/amm/liquidity/add` (MLP dual-sided)

**Gateway**: `http://localhost:68080`
**Autorização**: JWT do realm `mlp` com role `mlp`

**Request Body**:
```yaml
AddLiquidityMLPRequest:
  type: object
  required: [pool_pair, token_a_amount, token_b_amount, provider_bank_id]
  properties:
    pool_pair:
      type: string
      example: "BRL-USD"
    token_a_amount:
      type: string
      description: Quantidade de token A em menor unidade
      example: "50000"
    token_b_amount:
      type: string
      description: Quantidade de token B em menor unidade
      example: "50000"
    provider_bank_id:
      type: string
      description: Identificador do MLP
      example: "mlp"
```

**Response `200 OK`**:
```yaml
AddLiquidityMLPResponse:
  type: object
  properties:
    lp_id:
      type: string
      format: uuid
    pool_pair:
      type: string
    deposit_side:
      type: string
      enum: [BOTH]
      description: MLP sempre deposita ambos os lados
    shares_percentage:
      type: number
      format: float
      description: Percentual do pool pertencente ao MLP após o depósito
    tx_hash:
      type: string
      description: Hash da transação on-chain AMM.addLiquidity()
```

**Erros**:
| Code | Body | Condição |
|------|------|----------|
| 401 | UNAUTHORIZED | Token ausente ou role `mlp` não presente |
| 403 | FORBIDDEN | Identidade on-chain não é `isVerified` no IdentityRegistry |
| 409 | POOL_NOT_ACTIVE | Pool não está ACTIVE (sem liquidez bilateral) |
| 422 | INSUFFICIENT_ALLOWANCE | MLP não fez `approve` no token antes de chamar `addLiquidity` |

---

### Fluxo de Autorização MLP (Detalhado)

```
Client (ENABLE_MLP=true)
  │
  ├─ KC realm 'mlp': client_credentials → JWT (roles: ["mlp"])
  │
  ├─ POST :68080/api/v2/amm/liquidity/add
  │     Authorization: Bearer {JWT}
  │
  └─ api-gateway-mlp
        │
        ├─ auth-mlp (KEYCLOAK_REALM=mlp)
        │     └─ JWKS: /realms/mlp/protocol/openid-connect/certs ✓
        │
        ├─ lp_auth.go: role == "mlp" → autorizado
        │
        ├─ compliance-mlp: participant lookup (DB: cbweb3_mlp)
        │
        └─ Besu Hub: AMM.addLiquidity(amtA, amtB)
              └─ onlyVerified(0x22d491...) ✓
```
