# Implementation Plan: MLP Path B — Multilateral Liquidity Provider com Gateway Próprio

**Branch**: `005-cooperative-liquidity` | **Date**: 2026-05-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification + análise `/speckit.analyze` de 2026-05-18
**Scope**: Adição incremental ao feature já implementado (T001-T048 concluídos). Este plano cobre exclusivamente os gaps T049-T051 + infraestrutura Path B para o MLP.

**Note**: Path B = MLP com realm Keycloak próprio, gateway próprio (auth + compliance + api-gateway), endereço Ethereum próprio. Path A (MLP no realm de CB-A) foi descartado por violar isolamento de JWKS.

## Summary

O MLP (Multilateral Liquidity Provider — ex.: BID) precisa de identidade técnica própria para:
1. Autenticar com JWT de realm Keycloak exclusivo (JWKS isolado do realm `central-bank-a`)
2. Assinar transações on-chain com endereço Ethereum próprio (não compartilhado com CB-A)
3. Operar via stack de serviços independente (compliance-mlp, auth-mlp, api-gateway-mlp)

**Approach**: Path B — 9 alterações em 7 arquivos + 2 arquivos novos. Nenhuma mudança em código Go (backend já implementado em T020-T021). Todas as mudanças são de infraestrutura (Keycloak, Docker, Makefile, Solidity, Bash).

**Origem**: Análises `/speckit.analyze` de 2026-05-18 identificaram que T049-T051 são necessários para satisfazer FR-004 (MUST) e SC-004. Path B resolve adicionalmente o gap de JWKS realm binding (auth service valida tokens apenas do próprio realm).

---

## Technical Context

**Language/Version**: Go 1.25.5 (backend — sem mudanças), Solidity 0.8.20 (contratos), Bash 5+ (scripts), YAML (Docker Compose)
**Primary Dependencies**: Fiber v2.52.9, GORM + PostgreSQL, go-ethereum v1.17.1, Keycloak (OIDC), Foundry/Forge (Solidity), OpenZeppelin 5.x
**Storage**: PostgreSQL — nova database `cbweb3_mlp` para compliance-mlp; Redis DB 6 (isolamento de noncestore)
**Testing**: Bash E2E tryout (`tryout-scenario-b-e2e.sh`), `forge test` (contratos)
**Target Platform**: Linux Docker (local dev), Besu Spoke-A (Hub em dev)
**Project Type**: Infraestrutura de serviço adicional (não novo serviço — reuso das imagens existentes com nova config)
**Performance Goals**: Mesmos SLOs dos CBs — addLiquidity p95 < 2s (on-chain + DB)
**Constraints**: MLP_ADDRESS deve ser pré-fundido em ETH no genesis Besu local; Redis DB 6 não pode colidir com DBs 0-5 existentes
**Scale/Scope**: 1 nova entidade (MLP) + 4 novos containers Docker + 2 novos arquivos de config

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution está em template em branco — sem MUST principles ativos. Todas as gates passam por padrão.

| Gate | Status | Justificativa |
|------|--------|---------------|
| Não adicionar features além do escopo | ✅ PASS | Apenas infraestrutura MLP — nenhuma nova feature de negócio |
| Não criar helpers para operações únicas | ✅ PASS | Reuso de funções existentes (`create_realm_and_client`, etc.) |
| Sem código de segurança improvisado | ✅ PASS | Keypair de genesis Besu documentado; sem secrets hardcoded em prod |
| Validação apenas em fronteiras do sistema | ✅ PASS | `lp_auth.go` já implementado (T020) |

---

## Project Structure

### Documentation (this feature)

```text
specs/005-cooperative-liquidity/
├── plan.md              # Este arquivo
├── research.md          # Phase 0 — D12-D15 (Path B, ENABLE_MLP, addLiquidity, .env)
├── data-model.md        # Phase 1 — sem novas tabelas (MLP reutiliza liquidity_positions)
├── quickstart.md        # Phase 1 — como subir e usar o MLP (Fluxo 6)
├── contracts/           # Phase 1 — contrato de API MLP adicionado
└── tasks.md             # Existente (T001-T048 concluídos; T049-T052 a adicionar)
```

### Source Code (alterações deste plano)

```text
deploy/local/
├── keycloak/init.sh              # [MODIFY] realm mlp + mlp-client + role mlp
├── compose.yml                   # [MODIFY] POSTGRES_DB_MLP env var
└── postgres/init-multi-db.sh     # [MODIFY] create_db_if_missing cbweb3_mlp

contracts/script/
├── CBWeb3Hub.s.sol               # [MODIFY] grantLiquidityProvider(mlpAddress) opcional
└── SeedHub.s.sol                 # [MODIFY] MLP_SIGNER const + register + mint

backend/
├── docker-compose-backend.mlp.yaml     # [NEW] stack: compliance + auth + gateway MLP
└── config/
    └── .env.infra.mlp.example          # [NEW] template de configuração MLP

deploy/local/
├── .env.example                        # [NEW — tracked] Template de feature toggles (defaults seguros)
└── .env                                # [NEW — gitignored] Configuração local: ENABLE_MLP, MLP_ADDRESS

make/
└── 60-scenario-b.mk              # [MODIFY] -include deploy/local/.env + export ENABLE_MLP + targets MLP

tryouts/
└── tryout-scenario-b-e2e.sh      # [MODIFY] MLP vars + step_mlp_us2

.gitignore                        # [MODIFY] adicionar deploy/local/.env
```

---

## Complexity Tracking

Nenhuma violação de constitution — nenhuma justificativa necessária.

---

## Phase 0: Research

*Ver [research.md](research.md) para decisões completas.*

### Unknowns resolvidos

| Unknown | Decisão | Fonte |
|---------|---------|-------|
| Keypair Ethereum do MLP | Besu genesis key #4: `f8f8a2f4...` → `0x22d491Bde2...` | Confirmado: não colide com CB-A (`c87509a1...`) nem CB-B (`ae6ae8e5...`) |
| Ports do stack MLP | api-gw: 68080, auth: 68091, compliance: 68093, payment-orch: 68094 | Não colidem com entidades existentes (38080/60080/48080) |
| Redis DB do MLP | DB 6 | CB-A usa 2, CB-B usa 5; DB 6 disponível |
| PostgreSQL DB | `cbweb3_mlp` | Criado por `init-multi-db.sh` via `POSTGRES_DB_MLP` |
| Keycloak realm | Realm próprio `mlp` (não compartilhar com `central-bank-a`) | Bloqueante: auth service valida JWKS de um único realm configurado no startup |
| PKI do MLP | `mlp-ca.crt` / `mlp-ca.key` via `pki.gen-all` (pattern existente) | Compliance service precisa de CA para governance bootstrap |
| `addLiquidity` vs `addSingleSidedLiquidity` | MLP usa `addLiquidity` (dual-sided) — só requer `onlyVerified`, não `onlyLiquidityProvider` | Analisado em `AutomatedMarketMaker.sol:174-195` |
| `grantLiquidityProvider` para MLP | Necessário apenas para `addSingleSidedLiquidity` — não obrigatório no MVP | MLP pode fazer dual-sided sem ser LP no contrato, mas grant é feito assim mesmo para futuro `addSingleSidedLiquidity` |
| `provider_bank_id` no DB | `"mlp"` — passado no request body; validado pelo middleware, não pelo serviço | `liquidity_provision_service.go:128` aceita qualquer string como `ProviderBankID` |
| Persistência de `ENABLE_MLP` entre sessões | `deploy/local/.env` (gitignored) carregado por `-include` no Makefile | `export` shell não persiste; `.env` elimina configuração repetida por sessão (D15) |

---

## Phase 1: Design & Contracts

*Ver [data-model.md](data-model.md) e [quickstart.md](quickstart.md) para detalhes.*

### Alterações por camada

#### Camada 1 — Keycloak (`deploy/local/keycloak/init.sh`)

**O que muda**: Adicionar bloco condicional `ENABLE_MLP=true` ao final do script que:
- Cria realm `mlp` (novo)
- Cria client `mlp-client` com `serviceAccountsEnabled=true`
- Cria role `mlp` no realm
- Associa role `mlp` ao service account do `mlp-client`
- Grava `.env.infra.mlp` com `KC_CLIENT_SECRET` auto-gerado

**Função reutilizada**: `create_realm_and_client`, `create_platform_roles`, `assign_scenariob_roles_to_service_account` — já existem, só precisam ser chamadas.

**Token resultante**: `realm_access.roles: ["mlp"]` → valida em `lp_auth.go` via `RoleMLPScenarioB = "mlp"` (T020 já implementado).

#### Camada 2 — PostgreSQL / Redis (`deploy/local/compose.yml`, `init-multi-db.sh`)

**O que muda**:
- `compose.yml`: adicionar `POSTGRES_DB_MLP: cbweb3_mlp` no service `postgres`
- `init-multi-db.sh`: adicionar `create_db_if_missing "$POSTGRES_DB_MLP"`
- Redis: MLP usa `REDIS_DB=6` (configurado no `.env.infra.mlp.example`)

#### Camada 3 — Contratos (`contracts/script/`)

**CBWeb3Hub.s.sol** — Extensão da seção `grantLiquidityProvider`:
```solidity
address mlpAddress = vm.envOr("MLP_ADDRESS", address(0));
if (mlpAddress != address(0)) {
    identityRegistry.grantLiquidityProvider(mlpAddress);
}
```
Condicional: se `MLP_ADDRESS` não definido → comportamento atual preservado.

**SeedHub.s.sol** — Registro on-chain + mint de tokens para MLP:
```solidity
address constant MLP_SIGNER_DEFAULT = 0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b;
// No run():
address mlpSigner = vm.envOr("MLP_ADDRESS", MLP_SIGNER_DEFAULT);
_registerIfNeeded(hubRegistry, mlpSigner, "MLP", ParticipantRole.MLP);
// mint tCeBMa + tCeBMb para mlpSigner
```

#### Camada 4 — Docker Compose (`backend/docker-compose-backend.mlp.yaml`) [NOVO]

**4 serviços** (mesmas imagens existentes, nova config):

| Serviço | Container | Porta |
|---------|-----------|-------|
| `compliance-mlp` | `backend-compliance-mlp` | 68093 |
| `auth-mlp` | `backend-auth-mlp` | 68091 |
| `payment-orchestrator-mlp` | `backend-payment-orchestrator-mlp` | 68094 |
| `api-gateway-mlp` | `backend-api-gateway-mlp` | 68080 |

`auth-mlp` configurado com `KEYCLOAK_REALM=mlp` → valida JWKS de `http://keycloak:8080/realms/mlp/protocol/openid-connect/certs`.

`api-gateway-mlp` com `SIGNER_PRIVATE_KEY=f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315` → endereço `0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b`.

#### Camada 5 — Makefile (`make/60-scenario-b.mk`)

**Carregamento do `.env`** — adicionar no topo do arquivo:
```makefile
# Feature toggles locais — gitignored, nunca falha se ausente
-include deploy/local/.env
export ENABLE_MLP
export MLP_ADDRESS
```
Isso elimina o `export ENABLE_MLP=true` manual: o operador edita `deploy/local/.env` uma vez.

**Novos targets**:
- `scenario-b.up-backend-mlp` → `deploy.up-backend-mlp`
- `scenario-b.down-backend-mlp` → `deploy.down-backend-mlp`
- `scenario-b.tryout-us2-mlp` → executa o tryout (lê `ENABLE_MLP` do `.env`)

`scenario-b.up` e `scenario-b.down` permanecem sem mudança (MLP é opcional).

#### Camada 5b — Arquivos `.env` (`deploy/local/`) [NOVO]

Dois novos arquivos:

**`deploy/local/.env.example`** (rastreado no git — template seguro):
```dotenv
# Feature toggles MLP — copie para deploy/local/.env e ajuste.
ENABLE_MLP=false
MLP_ADDRESS=0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b
```

**`deploy/local/.env`** (gitignored — configuração local do operador):
- Criado pelo operador via `cp deploy/local/.env.example deploy/local/.env`
- Editar `ENABLE_MLP=true` para ativar o MLP
- Adicionado a `.gitignore` (raiz do repo)

#### Camada 6 — E2E Tryout (`tryouts/tryout-scenario-b-e2e.sh`)

Adições:
- Vars `ENABLE_MLP`, `API_GW_MLP_URL`, `KC_MLP_*`, `MLP_TOKEN`
- Função `step6_us2_mlp()` — obtém token MLP, chama `/api/v2/amm/liquidity/add` no gateway MLP, valida `lp_id` e `deposit_side=BOTH`
- Dispatch: se `ENABLE_MLP=true` → usa `step6_us2_mlp`; senão → comportamento atual (fallback CB-A)

#### Camada 7 — Env Example (`backend/config/.env.infra.mlp.example`) [NOVO]

Espelho de `central-bank-a.example` sem PKI própria de entidade soberana, com:
- `KC_REALM=mlp`, `KC_CLIENT_ID=mlp-client`
- `SIGNER_PRIVATE_KEY=f8f8a2f4...`
- `REDIS_DB=6`, `DB_NAME=cbweb3_mlp`
- `HUB_TOKEN_A_ADDRESS`, `HUB_TOKEN_B_ADDRESS`, `AMM_CONTRACT_ADDRESS` (preenchidos por `contracts.sync-addresses`)

---

## Sequence: Como o MLP opera (Path B)

### Configuração inicial (uma vez)

```bash
# 1. Criar o arquivo .env local a partir do template
cp deploy/local/.env.example deploy/local/.env

# 2. Ativar o MLP (editar o arquivo)
sed -i 's/ENABLE_MLP=false/ENABLE_MLP=true/' deploy/local/.env
# Ou editar manualmente: ENABLE_MLP=true
```

### Operação (a partir da segunda vez: o .env persiste entre sessões)

```
  make scenario-b.up-infra            # Keycloak provisionado com realm 'mlp' (ENABLE_MLP lido do .env)
  make scenario-b.deploy-contracts    # grantLiquidityProvider(mlpAddress) se MLP_ADDRESS set
  make scenario-b.up-backend-mlp      # stack MLP sobe (porta 68080)
  [source backend/config/.env.infra.mlp]
  curl ... KC_CLIENT_SECRET → MLP_TOKEN  # client_credentials no realm 'mlp'
  POST http://localhost:68080/api/v2/amm/liquidity/add
    Authorization: Bearer {MLP_TOKEN}
    {"pool_pair":"BRL-USD","token_a_amount":"10000","token_b_amount":"10000","provider_bank_id":"mlp"}
  → Response: {lp_id, deposit_side="BOTH", shares_percentage}
```

### Para desativar o MLP

```bash
sed -i 's/ENABLE_MLP=true/ENABLE_MLP=false/' deploy/local/.env
make scenario-b.down-backend-mlp
# Próximo `make scenario-b.up-infra` não provisiona realm 'mlp'
```

---

## Constitution Check (pós-design)

Confirmado: nenhuma violação. Todas as mudanças são mínimas e diretamente necessárias para FR-004 + SC-004.

---

---

# Bug-Fix Plan: I1–I4 (branch fix-005-cooperative-liquidity)

**Branch**: `fix-005-cooperative-liquidity` | **Date**: 2026-05-19 | **Spec**: [spec.md](spec.md)
**Input**: Análise `speckit.analyze` + sessão `speckit.clarify` de 2026-05-19 — bugs identificados no tryout `tryout-scenario-b-e2e.sh`
**Scope**: Correção de 4 bugs (I1–I4) encontrados no E2E. Nenhuma mudança de schema DB, contratos Solidity ou infraestrutura. Todas as alterações estão em 3 arquivos Go + 1 interface.

## Summary

| Bug | Severidade | Arquivo | Causa Raiz | Requisito Violado |
|-----|-----------|---------|-----------|-------------------|
| I1 | CRITICAL | `liquidity_provision_service.go` | `removeProportional` lê `pool_state_readings` estale em vez de `AMM.getReserves()` | FR-007, SC-002 |
| I2 | CRITICAL | `swap_service.go` + `app.go` | `RecordSwapFee` implementado mas nunca chamado | FR-006, SC-006 |
| I3 | HIGH | `pool_status_service.go` | `derivePoolStatus` ignora commits pendentes; usa só reservas on-chain | FR-001, US1 Cenário 4 |
| I4 | MEDIUM | `liquidity_provision_service.go` | `executeMatchedCommits` descarta erro de transação com `_ =`; `Status` não explícito | SC-007, I4 |

---

## Technical Context

**Language/Version**: Go 1.25.5 (api-gateway — sem mudanças de versão)
**Primary Dependencies**: Fiber v2.52.9, GORM + PostgreSQL, go-ethereum v1.17.1
**Storage**: Sem mudanças de schema (nenhuma migration necessária)
**Testing**: Bash E2E tryout (`tryout-scenario-b-e2e.sh`) — re-executar após fixes; assertions existentes validam automaticamente
**Target Platform**: Linux Docker local (mesmo ambiente do tryout)
**Project Type**: Bug fix — 4 alterações cirúrgicas em 3 arquivos + 1 interface
**Performance Goals**: `removeProportional` ganha 1 RPC call extra (`getReserves`) — aceitável; SLA p95 ≤ 6s para swaps mantido (fee recording é síncrono mas leve — apenas SQL INSERTs/UPDATEs em batch)
**Constraints**: `RecordSwapFee` falha NÃO deve bloquear swap (logar como warning) — SC-003 tem precedência sobre SC-006 para user-facing SLA

**Files Modified**:
```
backend/services/api-gateway/internal/
├── app/amm_adapter.go                   [MODIFY] implementar GetPoolReserves
├── services/pool_status_service.go      [MODIFY] override PENDING_COUNTERPART quando pendingCommits>0
├── services/liquidity_provision_service.go  [MODIFY] 3 alterações:
│   ├── removeProportional: usar amm.GetPoolReserves() em vez de pool_state_readings
│   ├── executeMatchedCommits: propagar erro da tx + Status explícito
│   └── AMLiquidityAdder interface: adicionar GetPoolReserves
└── services/swap_service.go             [MODIFY] injetar SwapFeeRecorder + chamar após swap
    └── app/app.go                       [MODIFY] wire liquiditySvc como feeRecorder
```

---

## Constitution Check

Constitution está em template em branco — sem princípios MUST ativos. Gates por padrão:

| Gate | Status | Justificativa |
|------|--------|---------------|
| Não adicionar features além do escopo | ✅ PASS | Apenas correções de bugs identificados — nenhuma feature nova |
| Não criar helpers para operações únicas | ✅ PASS | Interface `SwapFeeRecorder` reutiliza método existente |
| Sem código de segurança improvisado | ✅ PASS | Nenhuma mudança de autenticação/autorização |
| Validação apenas em fronteiras do sistema | ✅ PASS | Sem novas validações de entrada |

---

## Phase 0: Research — Decisões Resolvidas

> Todas as ambiguidades foram resolvidas na sessão `/speckit.clarify` de 2026-05-19 (D13–D16 em `research.md`). Nenhum NEEDS CLARIFICATION remanescente.

| Decisão | Resolução |
|---------|-----------|
| D13 — Fonte de reservas em remoção | `AMM.getReserves()` on-chain (Opção A) |
| D14 — Timing de fee distribution | Síncrono durante swap, não-bloqueante em falha (Opção A) |
| D15 — pool_status com commits pendentes | Override DB-first: `len(pendingCommits) > 0 AND status==EMPTY` → `PENDING_COUNTERPART` |
| D16 — Propagação de erros em executeMatchedCommits | Erro explícito + `Status: LPStatusActive` explícito |

---

## Phase 1: Design & Contracts

### Interface Contract Changes

**`AMLiquidityAdder` — novo método** (`amm_adapter.go`):
```go
// GetPoolReserves returns the current reserves from AMM.getReserves() on-chain.
// Used by removeProportional (FR-007 / D13 / bug I1).
GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, err error)
```

**`SwapFeeRecorder` — nova interface** (`swap_service.go`):
```go
// SwapFeeRecorder accumulates swap fees for LP positions (FR-006 / D14 / bug I2).
// Implemented by LiquidityProvisionService.RecordSwapFee.
type SwapFeeRecorder interface {
    RecordSwapFee(ctx context.Context, poolPair, swapOrderID string, feeAmountA, feeAmountB *big.Int) error
}
```

### Task Decomposition

- **T055** [I3] Fix `pool_status_service.go`: sobrescrever `poolStatus` para `PENDING_COUNTERPART` quando `len(pendingCommits) > 0 AND poolStatus == "EMPTY"` em `GetPoolStatus`, após a chamada ao enricher

- **T056** [I4] Fix `executeMatchedCommits` em `liquidity_provision_service.go`: (a) substituir `_ = s.db.Transaction(...)` por `if err := s.db.Transaction(...); err != nil { return nil, err }`; (b) adicionar `Status: apidomain.LPStatusActive` em `posA` e `posB`

- **T057** [I1] Fix `removeProportional` em `liquidity_provision_service.go`: (a) adicionar `GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, err error)` à interface `AMLiquidityAdder`; (b) implementar em `ammAdapter` (reutilizar binding existente de `getReserves`); (c) substituir bloco de leitura de `pool_state_readings` por chamada `s.amm.GetPoolReserves(ctx, pos.PoolPair)`

- **T058** [I2] Wire fee distribution no swap service: (a) criar interface `SwapFeeRecorder` em `swap_service.go`; (b) adicionar campo `feeRecorder SwapFeeRecorder` e injeção por `WithFeeRecorder(...) *SwapService`; (c) calcular `feeAmountA = amountIn * feeBps / 10000` e chamar `feeRecorder.RecordSwapFee(...)` após swap COMPLETED (falha loggada como warning, não bloqueia resposta); (d) em `app.go`, chamar `swapSvc.WithFeeRecorder(liquiditySvc)` no bootstrap

### Sequência de Implementação

```
T055 (pool_status_service.go) — independente
T056 (executeMatchedCommits) — independente
T057 (removeProportional + AMLiquidityAdder + ammAdapter) — independente de T055/T056
    ↓ (todos independentes, podem ser feitos em paralelo)
T058 (swap_service.go + app.go) — depende de T056 estar estável (LP positions no DB)
    ↓
Re-run tryout → validar que todos os PASSes continuam + fee_claim_paid > 0 + total_lp_count = 2
```

---

## Constitution Check (pós-design)

Confirmado: nenhuma violação. T055–T058 são alterações cirúrgicas de ≤20 linhas cada, sem adição de features, sem novos endpoints, sem mudança de schema.

---

## Plano Incremental: FR-018 — Payload Redesign (Session 2026-05-19)

**Input**: `/speckit.clarify` de 2026-05-19 identificou que os endpoints `mint-and-approve` e `approve-amm` tinham payload com `amount_a`/`amount_b` (dois campos numéricos ambíguos) que violavam o modelo mental do operador e tornavam a interface frágil.
**Scope**: Simplificar payload para campo único `amount` + `side` opcional; implementar auto-detecção de `CENTRAL_BANK_ROLE` on-chain para determinar qual token o signer pode mintar.
**Approach**: 5 tasks de backend (T061–T065) — sem mudança de contrato Solidity, schema de banco, ou gRPC proto.

### Technical Context (FR-018)

**Mudança central**: `tokenPrepareAdapter` passa a determinar qual token o signer possui `CENTRAL_BANK_ROLE` via chamada `hasRole` on-chain no init. A partir disso, `MintAndApproveForAMM` e `MintToForAMM` operam no token correto automaticamente. `ApproveAMM` aceita `side` explícito ou, para CBs, auto-detecta via `sideIsA`.

**Compatibilidade**: Breaking change intencional — campos `amount_a`/`amount_b` retornam HTTP 400 se detectados. Documentado no tryout e nas contracts.

### Interface Contract Changes (FR-018)

```go
type AMMTokenPreparer interface {
    MintAndApproveForAMM(ctx context.Context, amount string) error
    MintToForAMM(ctx context.Context, recipient, amount string) error
    ApproveAMM(ctx context.Context, amount, side string) error
}
```

**Payloads de API (breaking change)**:
- `POST /api/v2/amm/token/mint-and-approve`: `{"amount":"<int>"}` (+ `"recipient":"<addr>"` opcional)
- `POST /api/v2/amm/token/approve-amm`: `{"amount":"<int>","side":"A"|"B"|""}` (side obrigatório para bancos comerciais; opcional para CBs)

### Task Decomposition (FR-018)

- **T061** [FR-018-A] `tcebm/client.go` — `hasRole` ABI + `HasCentralBankRole(ctx) (bool, error)`
- **T062** [FR-018-B] `amm_adapter.go` — `NewTokenPrepareAdapter(ctx, tA, tB, ammAddr)` com `sideIsA, isCB bool`
- **T063** [FR-018-C] `token_handler.go` — interface atualizada + handlers com detecção de campos depreciados
- **T064** [FR-018-D] `app.go` — bootstrap com `NewTokenPrepareAdapter(ctx, ...)`
- **T065** [FR-018-E] `tryout-scenario-b-e2e.sh` — substituição de payloads

### Sequência de Implementação (FR-018)

```
T061 (tcebm) → T062 (adapter, depende de T061) → T063 (handler, depende de T062 interface)
T064 (app.go, depende de T062)
T065 (tryout, depende de T063/T064 estarem estáveis)
```

**Status**: T061–T065 COMPLETOS (implementados em 2026-05-19/20, validados via `go build ./...`).

---

## Constitution Check (FR-018 pós-design)

| Gate | Status | Justificativa |
|------|--------|---------------|
| Não adicionar features além do escopo | ✅ PASS | Apenas simplificação de payload + auto-detecção no init |
| Não criar helpers para operações únicas | ✅ PASS | `NewTokenPrepareAdapter` reutilizado por `app.go` e testes |
| Sem código de segurança improvisado | ✅ PASS | `hasRole` é leitura on-chain de AccessControl existente |
| Validação apenas em fronteiras do sistema | ✅ PASS | Validação de campos depreciados no handler HTTP |

---

## Plano Incremental: G5-cross — Execução Single-Gateway (Session 2026-05-20)

**Input**: Tryout falhou com `addSingleSidedLiquidity TOKEN_B: transaction reverted` após FR-018. Análise `/speckit.clarify` de 2026-05-20 identificou 4 gaps arquiteturais (I1, U1, U2, A1, I2).
**Scope**: Fix do tryout (G5-cross pattern) + atualização da spec para documentar as restrições arquiteturais. Sem mudança de código Go ou contratos Solidity.
**Approach**: 1 task de tryout (T066) + atualização de spec.md e research.md.

### Technical Context (G5-cross)

**Causa-raiz do revert**: Com FR-018, CB-A's gateway auto-detecta `sideIsA=true` e apenas minta TOKEN_A. Quando commit B é roteado ao gateway CB-A para auto-match, `executeMatchedCommits` executa com o signer CB-A, que não possui TOKEN_B balance nem aprovação do AMM para TOKEN_B.

**Restrição single-gateway**: O matching usa `FindActiveByPairAndSide` no DB local do gateway. Ambos os commits de um par DEVEM ir ao mesmo gateway no MVP (documentado em FR-001/FR-002 de spec.md).

**Padrão G5-cross**: Antes do commit-reveal, CB-B minta TOKEN_B ao endereço do signer CB-A via `recipient` field; CB-A então aprova o AMM para TOKEN_B via `side="B"`. Ambas as chamadas reutilizam a API FR-018 já implementada.

```text
Sequência G5-cross (step4a do tryout, antes do commit-reveal):
  1. CB-A signer addr ← cast wallet address --private-key $SIGNER_PRIVATE_KEY_CBA
  2. CB-B gateway → mint-and-approve {"amount":"200000","recipient":"<CB-A-signer>"}
  3. CB-A gateway → approve-amm {"amount":"200000","side":"B"}
  4. [commit-reveal normal continua]
  5. executeMatchedCommits usa CB-A signer com TOKEN_B balance+approval → sucesso
```

### Task Decomposition (G5-cross)

- **T066** [G5-cross-A] `tryouts/tryout-scenario-b-e2e.sh` — step4a: derivar endereço do signer CB-A via `cast wallet address`; CB-B minta TOKEN_B ao endereço derivado; CB-A aprova AMM para TOKEN_B com `side="B"`

### Sequência de Implementação (G5-cross)

```
T066 — único, independente (depende de FR-018 estar implementado, i.e. T061–T065)
    ↓
Re-run tryout completo → validar addSingleSidedLiquidity TOKEN_B passa
```

**Status**: T066 COMPLETO (implementado em 2026-05-20 como fix para revert no tryout). Revalidação do tryout E2E completo pendente (requer stack completo rodando).

### Decisões de Design (G5-cross)

Todas as decisões foram consolidadas em `research.md` (D17–D20) e documentadas em `spec.md` (FR-001, FR-002, FR-018 + Clarifications Session 2026-05-20):
- **D17**: Restrição single-gateway + padrão G5-cross como workaround MVP
- **D18**: `msg.sender` on-chain ≠ `provider_id` no DB — DB é autoritativo para LP ownership
- **D19**: FR-002 "atômica" = sequencial no mesmo handler (duas txs on-chain separadas)
- **D20**: `side` é opcional (não proibido) para CBs em `approve-amm`

---

## Constitution Check (G5-cross pós-design)

| Gate | Status | Justificativa |
|------|--------|---------------|
| Não adicionar features além do escopo | ✅ PASS | Apenas fix de tryout + documentação de restrição arquitetural |
| Não criar helpers para operações únicas | ✅ PASS | G5-cross usa chamadas inline com variáveis locais no bash |
| Sem código de segurança improvisado | ✅ PASS | `cast wallet address` é derivação pública de chave — sem secrets expostos |
| Validação apenas em fronteiras do sistema | ✅ PASS | Sem novas validações de entrada em Go |
