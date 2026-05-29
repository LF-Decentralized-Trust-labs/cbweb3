# Token API — FR-018 Breaking Changes

> **Scope**: Mudanças nos endpoints de token introduzidas pelo FR-018 nesta feature (005-cooperative-liquidity).
> Substitui os payloads definidos em `specs/004-scenario-b-frontend-integration/contracts/governance-api-v2.md`
> para `POST /api/v2/amm/token/mint-and-approve` e `POST /api/v2/amm/token/approve-amm`.

---

## POST /api/v2/amm/token/mint-and-approve

Minta tokens tCeBM e aprova o AMM como spender (setup único por CB).

### Sem recipient — CB minta para si próprio e aprova o AMM

```http
POST /api/v2/amm/token/mint-and-approve
Cookie: access_token=<CB_TOKEN>
Content-Type: application/json

{
  "amount": "200000"
}
```

O gateway detecta automaticamente qual token mintar consultando o `CENTRAL_BANK_ROLE`
do signer nos contratos `HUB_TOKEN_A_ADDRESS` / `HUB_TOKEN_B_ADDRESS`:
- CB-A (emissor de tCeBM-BRL) → minta TOKEN_A e aprova AMM para TOKEN_A
- CB-B (emissor de tCeBM-USD) → minta TOKEN_B e aprova AMM para TOKEN_B
- CB-C (emissor de tCeBM-ARS, se existir) → funciona sem mudança de contrato ou payload

**Response 200**:
```json
{ "status": "ok", "amount": "200000" }
```

### Com recipient — CB minta diretamente na carteira de um banco comercial

```http
POST /api/v2/amm/token/mint-and-approve
Cookie: access_token=<CB_TOKEN>
Content-Type: application/json

{
  "amount": "50000",
  "recipient": "0xABCDEF1234567890abcdef1234567890ABCDEF12"
}
```

O gateway minta o token do CB autenticado na carteira do destinatário.
O approve ao AMM **não** é executado — deve ser feito pelo próprio banco via
`POST /api/v2/amm/token/approve-amm`, pois o approve precisa ser assinado pelo dono dos tokens.

**Response 200**:
```json
{ "status": "ok", "amount": "50000", "recipient": "0xABCDEF..." }
```

### Erros

| HTTP | Condição | Body |
|------|----------|------|
| 400  | Campo depreciado `amount_a` ou `amount_b` presente | `{"error": "use 'amount' instead of 'amount_a'/'amount_b'"}` |
| 400  | `amount` ausente ou inválido | `{"error": "amount is required and must be a non-negative integer string"}` |
| 500  | Signer sem CENTRAL_BANK_ROLE em nenhum token configurado | `{"error": "amm token prepare failed: signer has no CENTRAL_BANK_ROLE on configured tokens"}` |
| 500  | Revert on-chain (ex.: permissão revogada) | `{"error": "amm token prepare failed: ..."}` |

---

## POST /api/v2/amm/token/approve-amm

Autoriza o contrato AMM a gastar tokens tCeBM do signer.

### Banco Central (side auto-detectado)

```http
POST /api/v2/amm/token/approve-amm
Cookie: access_token=<CB_TOKEN>
Content-Type: application/json

{
  "amount": "200000"
}
```

O gateway usa o mesmo `CENTRAL_BANK_ROLE` cacheado para determinar qual token aprovar.
`side` é opcional para CBs — se omitido, o adapter usa o lado detectado no startup.
Quando explicitado, o adapter usa o cliente do token indicado (`tokenA` ou `tokenB`),
independentemente do `CENTRAL_BANK_ROLE` do signer (necessário no padrão G5-cross — ver abaixo).

**Response 200**:
```json
{ "status": "ok", "amount": "200000" }
```

### Banco Central com `side` explícito (padrão G5-cross)

Usado quando o signer de CB-A recebeu TOKEN_B via `mint-and-approve` com `recipient`
(chamada feita por CB-B) e precisa aprovar o AMM para gastar TOKEN_B. Neste caso,
CB-A especifica `side: "B"` explicitamente para sobrescrever a auto-detecção.

```http
POST /api/v2/amm/token/approve-amm
Cookie: access_token=<CB_A_TOKEN>
Content-Type: application/json

{
  "amount": "200000",
  "side": "B"
}
```

**Contexto G5-cross (single-gateway MVP)**: O matching usa lookup DB-local no gateway receptor.
Quando o commit de TOKEN_B é roteado ao gateway CB-A para auto-match, o `executeMatchedCommits`
usa o signer de CB-A para executar ambos os lados on-chain. Para que `addSingleSidedLiquidity(TOKEN_B)`
não reverta, o signer de CB-A precisa ter TOKEN_B balance + aprovação do AMM:

1. CB-B chama `mint-and-approve {"amount":"X","recipient":"<CB-A-signer-addr>"}` → TOKEN_B emitido com a chave de CB-B (soberania de minting preservada).
2. CB-A chama `approve-amm {"amount":"X","side":"B"}` → CB-A's signer (agora holder de TOKEN_B) aprova o AMM via ERC-20 padrão.
3. `executeMatchedCommits` executa `addSingleSidedLiquidity(isTokenA=false, amount)` com sucesso.

O campo `provider_id` no DB (ex.: `"central_bank_b"`) permanece como autoridade canônica de LP ownership;
o `msg.sender` on-chain é o executor técnico (limitação arquitetural do MVP single-gateway — ver D17 em `research.md`).

**Response 200**:
```json
{ "status": "ok", "amount": "200000", "side": "B" }
```

### Banco Comercial (side obrigatório)

```http
POST /api/v2/amm/token/approve-amm
Cookie: access_token=<BANK_TOKEN>
Content-Type: application/json

{
  "amount": "50000",
  "side": "B"
}
```

Bancos comerciais DEVEM informar `side: "A"` ou `side: "B"` para indicar qual token
estão aprovando (o token que vão vender no swap).
Consistente com o padrão `side` do commit-reveal cooperativo.

**Response 200**:
```json
{ "status": "ok", "amount": "50000", "side": "B" }
```

### Erros

| HTTP | Condição | Body |
|------|----------|------|
| 400  | Campo depreciado `amount_a` ou `amount_b` presente | `{"error": "use 'amount' instead of 'amount_a'/'amount_b'"}` |
| 400  | `amount` ausente ou inválido | `{"error": "amount is required and must be a non-negative integer string"}` |
| 400  | Banco comercial omite `side` | `{"error": "'side' required for non-central-bank callers — use 'A' or 'B'"}` |
| 400  | `side` inválido (não `"A"` nem `"B"`) | `{"error": "side must be 'A' or 'B'"}` |
| 500  | Revert on-chain | `{"error": "amm approve failed: ..."}` |

---

## Escalabilidade para CB-C e além

Com o novo payload `{ "amount": "..." }`, adicionar um terceiro corredor (ex.: BRL-ARS com CB-C)
não requer nenhuma mudança nestes endpoints. O CB-C deploya o seu gateway com
`HUB_TOKEN_B_ADDRESS=<tCeBM_ARS_address>`, e o adapter detecta automaticamente
que o signer tem `CENTRAL_BANK_ROLE` em TOKEN_B do par BRL-ARS.
