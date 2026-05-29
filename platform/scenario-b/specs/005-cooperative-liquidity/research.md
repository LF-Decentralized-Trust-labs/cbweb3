# Research: Provisão Cooperativa de Liquidez no AMM

**Feature**: `005-cooperative-liquidity`
**Date**: 2026-05-14
**Status**: Completo — todos os NEEDS CLARIFICATION resolvidos

---

## D1 — Mecanismo de Commit-Reveal para Depósito Cooperativo

### Decisão
O commit-reveal é **gerenciado no backend (off-chain)**, não no contrato Solidity.

### Rationale
O contrato Solidity `AutomatedMarketMaker.addLiquidity(amountA, amountB)` requer que o `msg.sender` detenha **ambos** os tokens. No modelo cooperativo, o BCB detém apenas tCeBMa e o Fed detém apenas tCeBMb — nenhum pode chamar o `addLiquidity` atual com ambos os tokens.

A solução adotada é **adicionar `addSingleSidedLiquidity(bool isTokenA, uint256 amount)` ao contrato AMM**. Cada CB chama este método individualmente com sua própria moeda. O backend aplica a política de commit-reveal: só autoriza a chamada on-chain de cada CB após ambos terem registrado sua intenção no DB.

**Fluxo resultante**:
```
CB_A → POST /api/v2/amm/liquidity/commit { side: "A", amount: 100000 }
       → DB: PoolCommit { side: A, status: PENDING, expires_at: now+72h }
       → resposta: commit_id

CB_B → POST /api/v2/amm/liquidity/commit { side: "B", amount: 100000 }
       → DB: PoolCommit { side: B, status: PENDING, expires_at: now+72h }
       → ambos os lados presentes → STATUS: MATCHED
       → backend chama: AMM.addSingleSidedLiquidity(true, 100000)  [com signer = BCB]
       → backend chama: AMM.addSingleSidedLiquidity(false, 100000) [com signer = Fed]
       → DB: ambos os commits → EXECUTED; pool → ACTIVE
       → resposta: pool_pair, lp_id_cb_a, lp_id_cb_b
```

### Alternativas Consideradas
- **Commit-reveal on-chain**: Adicionaria um `commitDeposit(bytes32 hash)` + `revealDeposit(amount, salt)` ao contrato. Rejeitado — dobra o número de transações, aumenta gas, e introduz timing de reveal que conflita com o SLA de p95 ≤ 6s.
- **Depósitos assíncronos independentes** (opção A): Rejeitado pelo usuário — preferência pelo modelo B (commit-reveal com ativação atômica).
- **Escrow contract intermediário**: Um contrato coleta ambos os lados e chama `addLiquidity`. Rejeitado — adiciona um contrato extra ao deploy com complexidade de autorização duplicada.

---

## D2 — Depósito Unilateral no Contrato AMM (addSingleSidedLiquidity)

### Decisão
Adicionar função `addSingleSidedLiquidity(bool isTokenA, uint256 amount)` ao `AutomatedMarketMaker.sol`.

### Interface
```solidity
/// @notice Adiciona liquidez de apenas um lado do par (modelo cooperativo).
/// @param isTokenA  true → deposita TOKEN_A; false → deposita TOKEN_B
/// @param amount    Quantidade a depositar (deve ser > 0)
/// @dev  Requer que msg.sender seja LP autorizado (onlyLiquidityProvider).
///       Emite LogSingleSidedLiquidityAdded.
function addSingleSidedLiquidity(bool isTokenA, uint256 amount)
    external nonReentrant whenNotPaused onlyLiquidityProvider(msg.sender);
```

**Implementação**:
```solidity
if (isTokenA) {
    TOKEN_A.safeTransferFrom(msg.sender, address(this), amount);
    reserveA += amount;
} else {
    TOKEN_B.safeTransferFrom(msg.sender, address(this), amount);
    reserveB += amount;
}
emit LogSingleSidedLiquidityAdded(msg.sender, isTokenA, amount);
```

O contrato antigo (`addLiquidity`) permanece para compatibilidade retroativa com LPs que queiram depositar ambos os lados de uma vez (ex.: MLP com reservas de ambas as moedas).

### Alternativas Consideradas
- **Remover `addLiquidity` e substituir**: Quebraria posições existentes e scripts de integração. Rejeitado.
- **`addLiquidity(amountA, amountB)` com `amountB = 0` permitido**: Modificar a validação `if (amountA == 0 || amountB == 0)` para `if (amountA == 0 && amountB == 0)`. Tecnicamente mais simples, mas semanticamente ambíguo (intenção não declarada explicitamente). Rejeitado em favor de uma função com nome claro.

---

## D3 — Cálculo de LP Shares com Múltiplos Provedores

### Decisão
LP shares são calculados como **fração percentual da contribuição total do pool** no momento do depósito. Cada vez que um novo depósito é feito, as participações relativas de todos os LPs são recalculadas no backend.

**Fórmula por lado**:
```
valor_pool_A = reserveA × price_ratio   # normalizado em unidade comum
valor_pool_B = reserveB × 1

participação_CB_A = contribuicao_CB_A_em_A / reserveA_total_apos_deposito
participação_CB_B = contribuicao_CB_B_em_B / reserveB_total_apos_deposito

shares_percentage_cb_a = (valor_cb_a) / (valor_pool_A + valor_pool_B) × 100
```

Para o protótipo (par 1:1 em valor), simplifica para:
```
shares_percentage = minha_contribuicao / total_pool_valor × 100
```

Armazenado em `LiquidityPosition.shares_percentage` (DECIMAL 18,4). Recalculado a cada novo depósito e a cada retirada.

### Alternativas Consideradas
- **LP tokens ERC-20 on-chain (Uniswap v2 padrão)**: Minting de tokens proporcionais via `sqrt(A * B)` multiplicado pelo supply existente. Explicitamente fora de escopo (spec: "LP tokens transferíveis fora de escopo").
- **Manter `sqrt(A * B)` informacional**: O modelo atual usa este cálculo como valor puramente informacional. Não adequado para fee distribution proporcional — rejeitado.

---

## D4 — Coleta e Distribuição de Taxas de Swap

### Decisão
Taxa de **30 bps (0,30%) por swap**, retida nas reservas do pool (fee-in-reserve model, padrão Uniswap v2).

**Mecanismo on-chain**:
```solidity
uint256 public feeBps = 30; // configurável por governance

function swapExactOutput(...) {
    uint256 amountInWithFee = amountIn × (10000 - feeBps) / 10000;
    // constant product: reserveA × reserveB = (reserveA + amountInWithFee) × (reserveB - amountOut)
    // a taxa fica nas reservas, aumentando o valor proporcional de todos os LPs
}
```

**Distribuição off-chain (backend)**:
- A cada swap, o backend registra um `LPFeeEvent` com a taxa gerada e a distribuição percentual para cada LP ativo naquele momento (snapshot de `shares_percentage`).
- Na retirada, o LP recebe sua fatia do pool atual (que inclui as taxas acumuladas nas reservas) mais qualquer crédito de taxa registrado no `LPFeeEvent`.

**Modelo de retirada proporcional**:
```
valor_retirada_cb_a_em_A = reserveA_atual × shares_percentage_cb_a / 100
valor_retirada_cb_a_em_B = reserveB_atual × shares_percentage_cb_a / 100
```

### Alternativas Consideradas
- **Fee extraída para endereço separado**: Transferir taxa para um treasury contract a cada swap. Rejeitado — aumenta gas por swap e adiciona contrato extra.
- **Taxas por contrato (on-chain LP tracking)**: Registrar créditos por LP no contrato Solidity. Rejeitado — adiciona storage on-chain oneroso e loops por número de LPs.

---

## D5 — Papel de MLP no IdentityRegistry

### Decisão
Adicionar `grantLiquidityProvider(address account)` e `revokeLiquidityProvider(address account)` ao `IdentityRegistry.sol`. O AMM usa um novo modifier `onlyLiquidityProvider(msg.sender)` que chama `registry.isLiquidityProvider(account)`.

**Interface**:
```solidity
function isLiquidityProvider(address account) external view returns (bool);
function grantLiquidityProvider(address account) external onlyAdmin;
function revokeLiquidityProvider(address account) external onlyAdmin;
event LogLiquidityProviderGranted(address indexed account);
event LogLiquidityProviderRevoked(address indexed account);
```

CBs são registrados como LP durante o deploy (script `CBWeb3Hub.s.sol`). MLPs são registrados via governança (transação manual do admin).

### Alternativas Consideradas
- **Reutilizar `canTransact` para LPs**: Bancos comerciais também têm `canTransact = true`; confundiria tomadores e provedores de liquidez. Rejeitado.
- **Role separada no Keycloak**: O Keycloak gerencia autenticação off-chain; a autorização on-chain precisa estar no contrato para ser verificável. Rejeitado como única camada.

---

## D6 — Expiração de PoolCommit (72 horas)

### Decisão
Expiração **off-chain via worker Go** dentro do `api-gateway`. Uma goroutine roda a cada 5 minutos, busca commits com `status = PENDING` e `expires_at < now()`, e os marca como `EXPIRED`. Nenhuma transação on-chain é necessária pois os fundos nunca foram movidos.

### Alternativas Consideradas
- **Expiração on-chain**: Adicionar `commitExpiry` ao contrato com `block.timestamp`. Desnecessário — os commits são entidades de DB, não on-chain. Rejeitado.
- **Tarefa agendada externa (cron job de SO)**: Aumenta dependência operacional. Rejeitado em favor de goroutine interna ao serviço.

---

## D7 — Compatibilidade Retroativa com LiquidityPosition Existentes

### Decisão
Adicionar campo `deposit_side` à tabela `liquidity_positions` com default `BOTH` (valor para posições legadas). Posições com `deposit_side = BOTH` usam o caminho de retirada legado (`removeLiquidity(amountA, amountB)` com os valores originais). Posições com `deposit_side = A` ou `B` usam o novo caminho proporcional (`removeSingleSidedLiquidity`).

**Migração de schema**:
```sql
ALTER TABLE liquidity_positions
  ADD COLUMN deposit_side VARCHAR(4) NOT NULL DEFAULT 'BOTH'
    CHECK (deposit_side IN ('A', 'B', 'BOTH')),
  ADD COLUMN shares_percentage DECIMAL(18,4),
  ADD COLUMN fee_claim_accumulated NUMERIC(78,0) NOT NULL DEFAULT 0;
```

### Alternativas Consideradas
- **Migração de todas as posições existentes**: Calcular shares_percentage retrospectivamente. Arriscado sem histórico de swaps; poderia gerar percentuais incorretos. Rejeitado.
- **Tabela separada para posições cooperativas**: Duplicaria lógica de handler e queries. Rejeitado.

---

## D8 — Pool State Machine e Status

### Decisão
O estado do pool é derivado do estado das reservas, não armazenado explicitamente no contrato:

| Estado | Condição | Comportamento |
|--------|----------|---------------|
| `EMPTY` | reserveA = 0 AND reserveB = 0 | Swaps bloqueados: `POOL_NOT_ACTIVE` |
| `PENDING_COUNTERPART` | (reserveA > 0 AND reserveB = 0) OR (reserveA = 0 AND reserveB > 0) | Swaps bloqueados: `POOL_NOT_ACTIVE`; commit pendente |
| `ACTIVE` | reserveA > 0 AND reserveB > 0 | Swaps habilitados |

O backend consulta o pool state antes de executar qualquer swap. O contrato Solidity verifica `reserveA > 0 && reserveB > 0` no início da função `swapExactOutput`.

---

## Resumo das Decisões

| ID | Decisão | Escolhido |
|----|---------|-----------|
| D1 | Mecanismo commit-reveal | Off-chain (backend DB) com chamadas on-chain individuais por CB |
| D2 | Depósito unilateral | Nova função `addSingleSidedLiquidity(bool, uint256)` no AMM |
| D3 | LP shares com múltiplos provedores | Percentual proporcional ao valor total do pool (calculado no backend) |
| D4 | Taxas de swap | 30 bps fee-in-reserve; distribuição registrada por `LPFeeEvent` no backend |
| D5 | Papel MLP | Nova função `isLiquidityProvider` no IdentityRegistry; modifier `onlyLiquidityProvider` no AMM |
| D6 | Expiração de commits | Worker Go interno ao api-gateway, intervalo 5 min, expiração 72h |
| D7 | Retrocompatibilidade | Campo `deposit_side` com default `BOTH` em `liquidity_positions` |
| D8 | Pool state machine | Derivado de reservas; verificado no backend e no contrato antes de swaps |
| D9 | PairRegistry — proposta bilateral | `proposePair` (CB emissor tokenA) + `confirmPair` (CB emissor tokenB); emite `PairRegistered`; sem intervenção admin |
| D10 | Gateway PairRouter | `sync.RWMutex` cache inicializado no startup + goroutine `SubscribeFilterLogs` para `PairRegistered`; sem restart |
| D11 | Persistência PairProposal | Tabela `pair_proposals` (status PROPOSED → ACTIVE); DB local para auditoria e `GET /pairs` sem RPC |

---

## D9 — PairRegistry: State Machine de Proposta Bilateral

**Data**: 2026-05-18

### Decisão

`PairRegistry.sol` gerencia o ciclo de vida de pares com fluxo bilateral obrigatório. CB_A (emissor do tokenA) chama `proposePair(pairId, tokenA, tokenB, ammAddress)`. A autorização é verificada por `getCentralBankOf(tokenA) == msg.sender`. CB_B (emissor do tokenB) chama `confirmPair(pairId)`, verificado por `getCentralBankOf(tokenB) == msg.sender`. Ao confirmar, o par passa para `ACTIVE` e emite `PairRegistered(pairId, ammAddress, tokenA, tokenB)`.

```solidity
/// @notice Propõe um novo par de moedas. Apenas o CB emissor de tokenA pode propor.
function proposePair(
    string calldata pairId,
    address tokenA,
    address tokenB,
    address ammAddress
) external;

/// @notice Confirma uma proposta pendente. Apenas o CB emissor de tokenB pode confirmar.
function confirmPair(string calldata pairId) external;

/// @notice Retorna todos os pares em estado ACTIVE.
function getAllActivePairs() external view returns (PairEntry[] memory);

event PairRegistered(string indexed pairId, address ammAddress, address tokenA, address tokenB);
event PairProposed(string indexed pairId, address proposer, address tokenA, address tokenB);
```

`getCentralBankOf(token)` é uma nova view function no `IdentityRegistry` que retorna o endereço do CB que tem `CENTRAL_BANK_ROLE` para aquele token.

### Alternativa Rejeitada

`onlyAdmin` para confirmação — centraliza poder e contraria soberania monetária. Qualquer CB poderia forçar um par sem consentimento do outro.

---

## D10 — Gateway PairRouter: Cache Local + Event Watcher

**Data**: 2026-05-18

### Decisão

```go
type PairRouter struct {
    mu      sync.RWMutex
    clients map[string]*ammClient // pool_pair → client
}

func (r *PairRouter) ClientFor(pair string) (*ammClient, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    c, ok := r.clients[pair]
    if !ok {
        return nil, fmt.Errorf("unknown pair: %s", pair)
    }
    return c, nil
}
```

**Inicialização**: no startup, `PairRegistry.getAllActivePairs()` popula o mapa.  
**Event watcher goroutine**:
```go
query := ethereum.FilterQuery{
    Addresses: []common.Address{pairRegistryAddr},
    Topics:    [][]common.Hash{{pairRegisteredEventSig}},
}
logs, _ := client.SubscribeFilterLogs(ctx, query, logsCh)
for log := range logsCh {
    entry := parsePairRegistered(log)
    r.mu.Lock()
    r.clients[entry.PairID] = newAMMClient(entry.AMMAddress)
    r.mu.Unlock()
}
```

### Alternativa Rejeitada

Polling periódico (30s) — overhead para evento raro e janela de inconsistência. RPC por request — latência extra incompatível com SC-003.

---

## D11 — Persistência de PairProposal no DB

**Data**: 2026-05-18

### Decisão

```sql
CREATE TABLE pair_proposals (
    pair_id         VARCHAR(20) PRIMARY KEY,
    proposer_cb     VARCHAR(100) NOT NULL,
    confirmer_cb    VARCHAR(100),
    token_a_address VARCHAR(42) NOT NULL,
    token_b_address VARCHAR(42) NOT NULL,
    amm_address     VARCHAR(42) NOT NULL,
    status          VARCHAR(10) NOT NULL DEFAULT 'PROPOSED'
                    CHECK (status IN ('PROPOSED', 'ACTIVE')),
    proposed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ
);
```

O DB é atualizado pelo event watcher (D10) ao receber `PairRegistered`. O endpoint `GET /api/v2/amm/pairs` consulta o DB sem RPC call. `PairRegistry` on-chain permanece autoritativo; DB é cache persistente.

### Alternativa Rejeitada

Sem DB, apenas consulta on-chain — latência de RPC por `GET /pairs` e impossibilidade de auditoria histórica de propostas off-chain.

---

## D12 — MLP Identity: Path B (Gateway Próprio por Entidade)

**Data**: 2026-05-18

### Decisão

O MLP recebe **stack de serviços independente** (compliance-mlp + auth-mlp + api-gateway-mlp + payment-orchestrator-mlp) configurado com Keycloak realm próprio (`mlp`) e endereço Ethereum próprio.

### Rationale

**Bloqueante de Path A (MLP no realm de CB-A)**: O `auth` service valida JWTs buscando JWKS de `KEYCLOAK_REALM` fixo no startup (`/realms/{REALM}/certs`). Um token emitido pelo realm `mlp` tem `kid` não presente no JWKS do realm `central-bank-a` → 401 em todos os requests do MLP.

**Path B resolve**:
- `auth-mlp` configurado com `KEYCLOAK_REALM=mlp` → valida JWTs do realm `mlp` corretamente
- MLP tem endereço Ethereum `0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b` (Besu genesis key #4), isolado do CB-A (`0x627306...`)
- `grantLiquidityProvider(mlpAddress)` no IdentityRegistry garante identidade on-chain auditável separada

**Código backend já implementado (T020)**: `lp_auth.go` valida `realm_access.roles: ["mlp"]` via `RoleMLPScenarioB = "mlp"`. Path B é o único caminho que faz esse código ser exercitado — em Path A o token nunca carrega role `mlp`.

### Parâmetros Técnicos

| Parâmetro | Valor |
|-----------|-------|
| Ethereum private key | `f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315` |
| Ethereum address | `0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b` |
| Keycloak realm | `mlp` |
| Keycloak client | `mlp-client` |
| Keycloak role | `mlp` |
| Redis DB | `6` (CB-A=2, CB-B=5) |
| PostgreSQL DB | `cbweb3_mlp` |
| api-gateway port | `68080` |
| auth port | `68091` |
| compliance port | `68093` |
| payment-orchestrator port | `68094` |

### Alternativa Rejeitada

**Path A (MLP como client no realm de CB-A)**:
- Requer `auth` de CB-A validar tokens de `mlp` → impossível sem modificar código Go
- Conflate identidade do MLP com a do CB-A na auditoria
- `grantLiquidityProvider` no mesmo endereço que o CB-A → impossibilidade de rastrear separadamente

---

## D13 — Ativação Condicional do MLP (ENABLE_MLP)

**Data**: 2026-05-18

### Decisão

O MLP é um ator **opcional** controlado pela variável de ambiente `ENABLE_MLP=true|false`:
- `deploy/local/keycloak/init.sh`: bloco condicional — realm `mlp` só é criado se `ENABLE_MLP=true`
- `contracts/script/CBWeb3Hub.s.sol`: `grantLiquidityProvider(mlpAddress)` só executado se `MLP_ADDRESS != address(0)`
- `contracts/script/SeedHub.s.sol`: registro on-chain + mint do MLP com `vm.envOr("MLP_ADDRESS", MLP_SIGNER_DEFAULT)` — padrão é endereço pré-fundido no genesis
- `backend/docker-compose-backend.mlp.yaml`: arquivo separado; o usuário faz `docker compose -f ... up` explicitamente
- `make/60-scenario-b.mk`: targets `scenario-b.up-backend-mlp` e `scenario-b.down-backend-mlp` separados de `scenario-b.up`

### Rationale

Permite ambientes sem MLP (ex.: testes isolados de US1) sem overhead operacional. O MLP é necessário apenas para US2 e E2E-UC02-04 do D6.

---

## D14 — `addLiquidity` vs `addSingleSidedLiquidity` para o MLP

**Data**: 2026-05-18

### Decisão

O MLP usa **`addLiquidity(amtA, amtB)`** (dual-sided) — não `addSingleSidedLiquidity`.

### Rationale

- `addLiquidity` requer apenas `onlyVerified(msg.sender)` — basta estar no IdentityRegistry como participante registrado
- `addSingleSidedLiquidity` requer `onlyLiquidityProvider(msg.sender)` — papel mais restrito
- O MLP tem capacidade de provisionar ambas as moedas (é um consórcio multilateral, não um banco nacional com soberania de uma única moeda)
- `grantLiquidityProvider` ainda é feito no `CBWeb3Hub.s.sol` para habilitar operação futura de `addSingleSidedLiquidity` sem redeploy

### Fluxo de Autorização

```
MLP → POST /api/v2/amm/liquidity/add { pool_pair, token_a_amount, token_b_amount }
  → lp_auth.go: verifica role "mlp" no JWT → passa
  → liquidity_service.go: chama AMM.addLiquidity(amtA, amtB)
  → AMM.sol: onlyVerified(msg.sender) → mlpAddress está no IdentityRegistry → passa
```

---

## D15 — Persistência de ENABLE_MLP via Arquivo `.env`

**Data**: 2026-05-18

### Decisão

Criar `deploy/local/.env.example` (rastreado no git) e `deploy/local/.env` (gitignored) como fonte persistente de `ENABLE_MLP` e `MLP_ADDRESS`. O `make/60-scenario-b.mk` carrega o arquivo via `-include deploy/local/.env` no topo.

```
deploy/local/
├── .env.example   # [NEW — tracked] Template com defaults seguros
└── .env           # [NEW — gitignored] Configuração local do operador
```

### Conteúdo de `deploy/local/.env.example`

```dotenv
# Multilateral Liquidity Provider (MLP) — Feature toggle
# Copie este arquivo para deploy/local/.env e ajuste para o seu ambiente.
# O .env é gitignored — nunca commite secrets aqui.

# Ativar o MLP (realm Keycloak 'mlp' + stack backend MLP)
ENABLE_MLP=false

# Endereço Ethereum do MLP (dev: Besu genesis key #4, pré-fundido)
# Produção: substituir pelo endereço real do consórcio MLP.
MLP_ADDRESS=0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b
```

### Mecanismo de carregamento no Makefile

```makefile
# make/60-scenario-b.mk — topo do arquivo
# Carrega feature toggles locais se o arquivo existir (gitignored, nunca falha)
-include deploy/local/.env
export ENABLE_MLP
export MLP_ADDRESS
```

O prefixo `-` em `-include` faz o Make ignorar silenciosamente quando o `.env` não existe — nenhuma quebra em CI/CD ou em ambientes sem o arquivo.

A diretiva `export` propaga o valor para subprocessos (scripts Bash e Docker Compose).

### Leitura em `deploy/local/keycloak/init.sh`

O script já usa `${ENABLE_MLP:-false}` — sem mudança no script. A diferença é que o valor agora pode vir de `deploy/local/.env` carregado pelo Make, em vez de exigir `export ENABLE_MLP=true` manual no shell.

### Gitignore

Adicionar entrada em `.gitignore` (raiz do repo):
```gitignore
deploy/local/.env
```

### Rationale

- **Problema atual**: `export ENABLE_MLP=true` só persiste na sessão de terminal corrente. Em novo terminal ou CI, o valor é perdido.
- **Solução**: O operador edita `deploy/local/.env` uma vez; todos os targets `make scenario-b.*` lêem automaticamente.
- **Segurança**: `.env` é gitignored — MLP_ADDRESS (dev key) não vaza para repositório.
- **Backward compatibility**: Ambientes sem `deploy/local/.env` continuam funcionando com `ENABLE_MLP=false` como padrão (`?=` e `:-false` garantem isso).

### Alternativas Rejeitadas

- **`export` em `.bashrc`**: Poluição global de shell; conflito entre projetos.
- **Docker Compose `env_file:`**: Não funciona para scripts Bash (init.sh, tryout.sh) fora de containers.
- **Variável hardcoded no Makefile**: Impede habilitar/desabilitar sem editar código versionado.

---

## Session 2026-05-19 — Bug Fixes I1–I4 (branch fix-005-cooperative-liquidity)

> Contexto: análise do tryout `tryout-scenario-b-e2e.sh` revelou 4 bugs. As decisões abaixo derivam da sessão `/speckit.clarify` de 2026-05-19 e da inspeção direta do código.

---

### D13 — Fonte de Reservas em `removeProportional` (Bug I1)

**Decisão**: Substituir a leitura de `pool_state_readings` (DB cache estale) por chamada on-chain `AMLiquidityAdder.GetPoolReserves(ctx, pair)` no momento da remoção.

**Rationale**: A tabela `pool_state_readings` é semeada em `executeMatchedCommits` com os valores do commit-reveal (100000/100000) e nunca atualizada após swaps. No tryout, um swap alterou as reservas para 101015/99000, mas `removeProportional` leu 100000/100000 do cache → retornou 50000/50000 em vez de 50507/49500. `AMM.getReserves()` on-chain é a única fonte canônica. Custo: 1 RPC extra por remoção — aceitável dado que `removeSingleSidedLiquidity` já executa uma tx on-chain logo depois.

**Impacto no código**:
- `AMLiquidityAdder` interface: adicionar `GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, err error)`
- `ammAdapter` em `amm_adapter.go`: implementar o método (reutiliza lógica de `GetPoolReserves` do `PoolStatusService`)
- `removeProportional` em `liquidity_provision_service.go`: substituir bloco `var latest apidomain.PoolStateReading / s.db...First(&latest)` por `rA, rB, _ = s.amm.GetPoolReserves(ctx, pos.PoolPair)`

**Alternativas Rejeitadas**:
- Manter `pool_state_readings` e atualizar em cada swap: aumenta coupling entre swap service e pool state, e exige propagação de estado extra — desnecessário dado que on-chain já é a fonte canônica.

---

### D14 — Integração de `RecordSwapFee` no Fluxo de Swap (Bug I2)

**Decisão**: Injetar uma interface `SwapFeeRecorder` no swap service e chamar `RecordSwapFee` de forma **síncrona** após cada swap COMPLETED, dentro do mesmo contexto de request.

**Rationale**: `RecordSwapFee` está implementado em `LiquidityProvisionService` mas nunca é chamado. O swap service não tem referência ao serviço de liquidez. A solução limpa é criar uma interface mínima `SwapFeeRecorder` com o método `RecordSwapFee(ctx, poolPair, orderID string, feeA, feeB *big.Int) error` e injetá-la no swap service. O fee deve ser calculado como `amountIn * feeBps / 10000` usando o `feeBps` retornado pelo AMM on-chain (ou o valor configurado, 30).

**Impacto no código**:
- `services/swap_service.go`: adicionar campo `feeRecorder SwapFeeRecorder` + construtor com injeção opcional
- `app/app.go`: passar `liquiditySvc` como `feeRecorder` ao construir o swap service
- Calcular `feeAmountA = amountIn * feeBps / 10000` e chamar `feeRecorder.RecordSwapFee(...)` após swap COMPLETED
- Falha em `RecordSwapFee` não deve bloquear o swap — logar como warning (SC-003 > SC-006 em termos de user-facing SLA)

**Alternativas Rejeitadas**:
- Worker assíncrono: risco de perda de fee em crash entre swap e atualização (viola SC-006)
- Lazy compute na remoção: viola SC-007 (auditabilidade em tempo real) e FR-012

---

### D15 — Derivação de `pool_status` com Commits Pendentes (Bug I3)

**Decisão**: Após chamar `derivePoolStatus(reserveA, reserveB)`, o método `GetPoolStatus` deve sobrescrever o resultado para `PENDING_COUNTERPART` se `len(pendingCommits) > 0` AND `poolStatus == "EMPTY"`.

**Rationale**: `derivePoolStatus` usa apenas reservas on-chain. Durante a fase de commit-reveal (após Commit A, antes de Commit B), as reservas são 0 on-chain porque nenhum fundo foi transferido ainda — este é o comportamento correto do protocolo. O enricher já busca `pendingCommits` do DB via `ListPendingCommits`. Basta usar essa informação para corrigir o status derivado. A lógica de override é minimal e local ao `GetPoolStatus`.

**Impacto no código**:
- `pool_status_service.go` em `GetPoolStatus`: adicionar bloco `if poolStatus == "EMPTY" && len(pendingCommits) > 0 { poolStatus = "PENDING_COUNTERPART" }` após `derivePoolStatus` e após a chamada ao enricher

**Alternativas Rejeitadas**:
- Modificar `derivePoolStatus` para receber `pendingCommits` como parâmetro: aumenta a assinatura da função sem necessidade — a lógica de override é responsabilidade do orquestrador (`GetPoolStatus`), não da função de derivação pura.

---

### D16 — Propagação de Erros em `executeMatchedCommits` e Status Explícito em LP (Bug I4)

**Decisão**: (a) Substituir `_ = s.db.Transaction(...)` por `if err := s.db.Transaction(...); err != nil { return nil, err }` em `executeMatchedCommits`. (b) Adicionar `Status: apidomain.LPStatusActive` explicitamente em `posA` e `posB` para não depender do comportamento de default do GORM com zero-value strings.

**Rationale**: O `_ =` descarta silenciosamente qualquer erro na transação que cria posA/posB. Se o `recalculateSharesInTx` ou os `tx.Create` falharem, os LP positions não são criados no DB, mas a função ainda retorna LPIDs válidos — causando `total_lp_count=0` quando o enricher consulta o DB. A adição do `Status` explícito elimina dependência no comportamento de default do GORM v2 com zero-value strings (comportamento documentado como ambíguo quando o campo tem constraint `not null;default:ACTIVE`).

**Impacto no código**:
- `executeMatchedCommits` em `liquidity_provision_service.go`: `_ =` → `if err := ...; err != nil { return nil, fmt.Errorf(...) }`
- Adicionar `Status: apidomain.LPStatusActive` em posA e posB

**Alternativas Rejeitadas**:
- Usar `gorm:"<-:create"` tag para forçar o campo: intrusivo no domain struct; o fix com `Status` explícito é mais legível e sem side-effects.

---

## FR-018: Payload Redesign — mint-and-approve / approve-amm (Session 2026-05-19)

### R-018-1: CENTRAL_BANK_ROLE e hasRole on-chain

**Decision**: Adicionar `hasRole(bytes32 role, address account) returns (bool)` ao ABI do cliente tCeBM e usá-lo para detectar qual token o signer pode mintar.

**Rationale**: `TokenizedCentralBankMoney.sol` (linha 17) declara `bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE")`. O contrato herda de `AccessControl` do OpenZeppelin, que expõe `hasRole` como view — sem custo de gas em modo leitura via `eth_call`. O gateway pode chamar `hasRole` em ambos os tokens na inicialização e cachear o resultado, evitando latência por request.

**Alternatives considered**: (a) Novo env var `CENTRAL_BANK_TOKEN_SIDE=A|B` — funciona mas acrescenta config manual propensa a erro; (b) Try-first (tentar TOKEN_A, fallback TOKEN_B se revert) — perigoso: falha parcial em caso de revert inesperado; (c) Lookup no IdentityRegistry on-chain — mais pesado e já disponível via hasRole no próprio token.

---

### R-018-2: Estratégia de detecção de side no tokenPrepareAdapter

**Decision**: Campo `sideIsA bool` e `isCB bool` populados no construtor de `tokenPrepareAdapter` via chamadas `hasRole` em TOKEN_A e TOKEN_B.

**Rationale**: A detecção ocorre uma vez no startup (ou lazy na primeira chamada), sem overhead por request. Se o signer tem role em TOKEN_A → `sideIsA=true, isCB=true`. Se em TOKEN_B → `sideIsA=false, isCB=true`. Se em nenhum → `isCB=false` (banco comercial ou gateway de read-only). O campo `isCB` permite que `ApproveAMM` retorne HTTP 400 quando o caller é banco comercial e omite `side`.

**Alternatives considered**: Detecção lazy na primeira chamada ao invés do construtor — elimina falha de startup mas atrasa o erro; construtor explícito é mais seguro e compatível com os padrões de init já usados em `app.go`.

---

### R-018-3: Compatibilidade de breaking change

**Decision**: HTTP 400 com `{"error": "use 'amount' instead of 'amount_a'/'amount_b'"}` quando `amount_a` ou `amount_b` aparecem no body.

**Rationale**: A detecção se dá via `json.RawMessage` (desserializar em `map[string]json.RawMessage` e verificar presença de chaves depreciadas). Alternativa de aceitar os campos e ignorar seria confusa e ocultaria bugs de cliente. O 400 explícito força atualização imediata de todos os callers (tryouts, frontend).

**Alternatives considered**: Silent ignore — descartado pois cria incompatibilidade silenciosa e dificulta depuração; `X-Deprecated` header — não suficiente para forçar migração.

---

### R-018-4: Interface AMMTokenPreparer — novas assinaturas

**Decision**:
```go
type AMMTokenPreparer interface {
    // MintAndApproveForAMM minta o token que o signer possui CENTRAL_BANK_ROLE
    // e aprova o AMM. amount é o valor inteiro como string.
    MintAndApproveForAMM(ctx context.Context, amount string) error
    // MintToForAMM minta o token que o signer possui CENTRAL_BANK_ROLE ao recipient.
    MintToForAMM(ctx context.Context, recipient, amount string) error
    // ApproveAMM aprova o AMM para gastar o token indicado por side ("A"|"B").
    // side pode ser "" para CBs (auto-detectado pelo adapter).
    // side é obrigatório para bancos comerciais (isCB=false no adapter).
    ApproveAMM(ctx context.Context, amount, side string) error
}
```

**Rationale**: Assinaturas mínimas que refletem o modelo mental correto. `side` vazio é válido para CBs (o adapter auto-detecta); obrigatório para bancos comerciais — a validação fica no handler (HTTP 400) e no adapter (fallback seguro).

**Alternatives considered**: Passar dois parâmetros separados `isTokenA bool` e `amount` — funciona mas menos expressivo; usar `SideEnum` em vez de `string` — over-engineering para dois valores.

---

### R-018-5: Arquivos afetados (escopo total do FR-018)

| Arquivo | Mudança |
|---------|---------|
| `backend/shared/blockchain/scenariob/tcebm/client.go` | Adicionar `hasRole` ao ABI JSON + método `HasCentralBankRole(ctx) (bool, error)` |
| `backend/services/api-gateway/internal/app/amm_adapter.go` | `tokenPrepareAdapter`: campo `sideIsA, isCB bool`; detectar no construtor; atualizar `MintAndApproveForAMM`, `MintToForAMM`, `ApproveAMM` |
| `backend/services/api-gateway/internal/http/handlers/token_handler.go` | Atualizar `AMMTokenPreparer` interface + handlers: novo struct de request, rejeição 400 de campos depreciados |
| `tryouts/tryout-scenario-b-e2e.sh` | Substituir todos `amount_a`/`amount_b` por `amount`; adicionar `side` em chamadas `approve-amm` de bancos comerciais |
| `specs/005-cooperative-liquidity/contracts/governance-api-v2.md` | Atualizar payloads dos dois endpoints |

**Não afetados**: contratos Solidity, schema de banco de dados, gRPC proto, frontend.

---

## G5-Cross Pattern e Clarificações Arquiteturais (Session 2026-05-20)

> Originado na análise `/speckit.analyze` + sessão `/speckit.clarify` de 2026-05-20.
> Resolve os findings I1, U1, U2, A1, I2 documentados em `spec.md`.

### D17 — Single-Gateway Matching Constraint e Padrão G5-cross

**Contexto**: Após a mudança FR-018, o tryout falhou com `addSingleSidedLiquidity TOKEN_B: transaction reverted` quando o commit B era roteado ao gateway CB-A para auto-match. CB-A's signer não possuía balance de TOKEN_B nem aprovação do AMM.

**Causa-raiz**: `executeMatchedCommits` executa ambas as chamadas `AddSingleSidedLiquidity` com o signer do gateway receptor. O matching usa lookup DB-local (`FindActiveByPairAndSide`) — portanto ambos os commits de um par DEVEM ir ao mesmo gateway. No cenário E2E, o commit B vai ao gateway CB-A.

- **Decision**: Padrão G5-cross — CB-B minta TOKEN_B diretamente ao endereço do signer CB-A (`recipient` field) antes do commit-reveal; CB-A então aprova o AMM para TOKEN_B usando `side="B"`.
  - **Rationale**: CB-B possui `CENTRAL_BANK_ROLE` no TOKEN_B, portanto a minting é válida. CB-A's signer é `isLiquidityProvider=true` (registrado no deploy de IdentityRegistry via `grantLiquidityProvider(centralBankAddress)`). O padrão preserva a soberania de minting: cada CB mantém controle do seu token.
  - **Implementation**: 
    1. CB-B → `POST /api/v2/amm/token/mint-and-approve {"amount":"200000","recipient":"<CB-A-signer-addr>"}`
    2. CB-A → `POST /api/v2/amm/token/approve-amm {"amount":"200000","side":"B"}`
    3. Ambas as chamadas usam a API FR-018 já implementada (sem novas rotas).
  - **Alternatives considered**: (a) Rotear commit B ao gateway CB-B — não funciona porque o matching DB-local exigiria commit A também em CB-B; (b) Matching cross-gateway via inter-gateway API — Fase 2 (T059-class work); (c) Usar uma conta intermediária de custódia — viola separação de chaves.

- **Constraint documentada**: No MVP, o auto-match usa lookup DB-local. Ambos os commits de um par DEVEM ser enviados ao **mesmo gateway**. Documentado em FR-002 e FR-001 de `spec.md`.

### D18 — Separação entre Executor On-chain e Autoridade Canônica no DB

- **Decision**: `msg.sender` on-chain em `addSingleSidedLiquidity(TOKEN_B)` é o signer do gateway CB-A; `provider_id = "central_bank_b"` no DB é a autoridade canônica de LP ownership.
  - **Rationale**: A execução single-gateway exige um único signer. O contrato AMM apenas valida `isLiquidityProvider(msg.sender)`; não armazena nem expõe o `provider_id` de negócio. O DB é autoritativo para o mapeamento LP-owner → posição.
  - **Alternatives considered**: Exigir que o signer on-chain corresponda ao `provider_id` — viola a restrição single-gateway do MVP; requer coordenação cross-gateway.

### D19 — FR-002 "Atômica" vs Execução Real

- **Decision**: "Atômica" em FR-002 é impreciso. A execução correta é: **duas transações on-chain sequenciais no mesmo handler HTTP**. Rollback de status DB é garantido por DB transaction; não há rollback on-chain real.
  - **Rationale**: `addSingleSidedLiquidity(TOKEN_A)` e `addSingleSidedLiquidity(TOKEN_B)` são duas txs EVM distintas. Em caso de falha parcial (A ok, B falha), o handler retorna erro e os commits entram em `RECONCILIATION_REQUIRED` (Fase 2: T059 implementará retry automático).
  - **Alternatives considered**: Usar Atomic Broadcast ou batching de txs — requer mudança de contrato; fora do escopo MVP.

### D20 — `side` Opcional vs Obrigatório para CBs em `approve-amm`

- **Decision**: O parâmetro `side` em `approve-amm` é **opcional para CBs** (não proibido). CBs podem especificá-lo explicitamente para sobrescrever a auto-detecção do adapter (use case: G5-cross, onde CB-A aprova TOKEN_B via `side="B"`).
  - **Rationale**: O adapter usa `sideIsA` como padrão quando `side == ""`; ao receber `side="B"`, respeita o override. Isso é necessário para o padrão G5-cross sem adicionar novos endpoints.
  - **Alternatives considered**: Exigir `side` sempre — breaking change desnecessário; endpoint separado `/approve-counterpart-token` — over-engineering para um único use case.
