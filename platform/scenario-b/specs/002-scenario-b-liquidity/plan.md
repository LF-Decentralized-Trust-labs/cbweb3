# Implementation Plan: Reframe Scenario B Docs + Backend & Contracts Rebuild

**Branch**: `002-scenario-b-liquidity` | **Date**: 2026-05-04 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/002-scenario-b-liquidity/spec.md`

## Summary

Entregar documentacao externa focada exclusivamente no **Cenario B (Hub-and-Spoke + AMM/Liquidity Pool)** e, em paralelo, reconstruir o backend + contratos Solidity em big-bang cutover, removendo completamente a implementacao do Cenario A. A nova API v2 cobre Exact-Output Swap com `maxAmountIn`, bridging Lock&Mint / Burn&Unlock via Relayer Cacti, validacao de conformidade por ZK-Pointers, monitor de desequilibrio 70/30, Circuit Breaker com autorizacao assimetrica (pause 1-of-N, resume 2-of-N — executado automaticamente ao atingir quorum no `resume-sign`) e Master Viewing Key multi-sig 2-of-3 com timeout 72h.

Decisoes operacionais consolidadas (sessoes 2026-04-23 e 2026-05-04):

- **Frontend OUT-OF-SCOPE**: nenhum arquivo sob `frontend/` sera modificado; incompatibilidade pos-cutover e risco aceito.
- **Retencao de dados do Cenario B**: indefinida para todas as entidades operacionais; sem purge jobs; particionamento por tempo obrigatorio.
- **Imutabilidade de audit logs**: append-only em Postgres via triggers `BEFORE UPDATE/DELETE` que recusam a operacao; sem WORM, sem anchor on-chain.
- **Observabilidade estruturada**: DELEGADA a feature futura; esta feature usa logs ad-hoc em stdout; SCs dependentes (SC-014/019/023) validados qualitativamente.
- **Rate limiting**: DELEGADO a feature futura; API v2 sem throttling; abuso contido por autenticacao Keycloak + segmentacao de rede.
- **Reconciliacao bridging**: 5 tentativas com backoff exponencial (2s, 4s, 8s, 16s, 32s; cap 60s); fila persistente em Postgres; idempotencia via hash do evento.
- **Performance targets**: quote p95 <= 300ms, swap p95 <= 6s, Liquidity Monitor p95 <= 15s entre leituras.
- **Seed de liquidez inicial no tryout**: `step4b_seed_liquidity()` e funcao obrigatoria executada por Banco Central antes dos steps de US1; pool vazio e erro de ambiente, nao cenario de teste valido (Decision 19).
- **Timeout bridging no tryout**: polling `LOCKING -> ACTIVE` com timeout de 120s, intervalo 5s, 24 tentativas maximas (Decision 20).
- **Lookup de Remove Liquidity**: chave canonico `lp_id` (UUID), nao `lp_shares` hex; tryout captura `lp_id` do response de Add Liquidity (Decision 21).
- **Resume automatico**: `resume-sign` executa transicao `HALTED -> LIVE` automaticamente ao atingir quorum 2-of-N, sem endpoint `executeResume` separado (Decision 22).
- **Schema plano do pool status**: `GET /api/v2/amm/pool/{pair}/status` retorna campos planos (`reserve_a`, `reserve_b`, `current_ratio`, `imbalance_flag`, `pool_pair`, `updated_at`); nenhum objeto `reserves` aninhado (Decision 23).

A validacao final da entrega e feita por um tryout E2E em `tryouts/tryout-scenario-b-e2e.sh` que exercita cotacao, swap, bridging, compliance, Liquidity Monitor, Circuit Breaker assimetrico e Master Viewing Key multi-sig.

## Technical Context

**Language/Version**: Go 1.22+ (backend), Solidity 0.8.20 (contratos), Bash 5+ (tryouts)  
**Primary Dependencies**:

- Backend: Fiber v2 (HTTP), gRPC-Go, go-ethereum/ethclient, abigen, Hyperledger Cacti client Go, gorm.io/v2 + gorm.io/driver/postgres (ORM — substitui pgx direto; FR-055), redigo/go-redis, Keycloak gocloak, OpenAPI 3.1
- Contratos: Foundry (forge, cast), OpenZeppelin contracts, interfaces proprias (`IAutomatedMarketMaker`, `ISpokeBridge`, `ITokenizedCentralBankMoney`)
- Tryout: bash, jq, openssl, curl, forge, cast, docker, docker-compose

**Storage**:

- Postgres (camada `DATASTORE`, reaproveitada da infra Cenario A): novos schemas particionados por tempo para `scenario_b_api_cutover_plan`, `scenario_b_endpoint_contract`, `legacy_artifact_inventory`, `infrastructure_reuse_register`, `scenario_b_risk_control_state`, `bridged_asset_position`, `compliance_zk_pointer`, `swap_order_scenario_b`, `relayer_queue_item`, `disclosure_request`, `disclosure_signature`, `circuit_breaker_signature`, `pool_state_reading`, `liquidity_alert`, `audit_log`, `liquidity_position`
- Redis (camada `DATASTORE`, reaproveitada): cache de cotacoes e estado agregado de pool (nao e fonte de verdade, apenas aceleracao)
- Ledger Besu (Hub Internacional): contratos AMM, tokens espelhados, CommitmentHashRegistry; fonte autoritativa de estado do pool
- Ledgers Besu/Paladin (Spokes): `SpokeBridge`, `TokenizedCentralBankMoney` nativo, `IdentityRegistry`

**Testing**: `go test ./...` (unit + integracao em Go), `forge test -vv` (Solidity via Foundry), `tryouts/tryout-scenario-b-e2e.sh` (E2E orquestrado em Bash)  
**Target Platform**: Linux (servicos Go em containers), EVM Besu (Hub + Spokes), Paladin para Master Viewing Key; desenvolvimento local via Docker Compose  
**Project Type**: web service (backend Go) + smart contracts (Solidity) + integracao multi-ledger; frontend explicitamente out-of-scope nesta feature  

**Performance Goals** (FR-037):

- `GET /api/v2/amm/quote/exact-output`: p95 <= 300ms em ambiente de homologacao representativo
- `POST /api/v2/amm/swap/exact-output`: p95 <= 6s (incluindo confirmacao on-chain em 1 bloco do Hub Besu)
- Liquidity Monitor: cadencia de polling p95 <= 15s entre leituras consecutivas
- Circuit Breaker pause: efeito em <= 1 bloco apos confirmacao (SC-017)
- Bridging LOCKING -> ACTIVE: tryout aguarda com timeout de 120s (intervalo 5s, 24 tentativas)

**Constraints**:

- Cutover big-bang: nenhuma coexistencia operacional entre API v1 (Cenario A) e API v2 (Cenario B)
- Zero reaproveitamento funcional do Cenario A; apenas infraestrutura transversal (camadas `IDENTITY`, `DATASTORE`, `OBSERVABILITY`, `RUNTIME` do `InfrastructureReuseRegister`)
- Frontend (`frontend/apps/*`) permanece intocado; SC-025 valida 0 arquivos modificados
- Observabilidade estruturada nao entregue (delegada); SC-014/019/023 validados qualitativamente nesta feature
- Rate limiting nao entregue (delegado); risco documentado na matriz de riscos do cutover
- Imutabilidade de audit logs limitada a triggers Postgres; hardening contra privilegio de sistema (DBA) fora do escopo
- **ORM**: GORM obrigatorio (`gorm.io/v2` + `gorm.io/driver/postgres`); arquivos `.sql` de migration proibidos (FR-055); schema gerenciado via `db.AutoMigrate(&Model{})` para tabelas padrao e `db.Exec()` em `internal/db/init/` para DDL nao suportado (triggers PL/pgSQL, particionamento declarativo, drop de schemas legados)
- **Schema plano**: endpoint de pool status usa campos planos (`reserve_a`, `reserve_b`, `current_ratio`, `imbalance_flag`, `pool_pair`, `updated_at`); sem objeto `reserves` aninhado
- **Lookup de liquidez**: `POST /api/v2/amm/liquidity/remove` usa `lp_id` (UUID) como chave; `lp_shares` hex nao e campo de lookup
- **Resume automatico**: nenhum endpoint `executeResume` exposto; `resume-sign` finaliza a transicao ao atingir quorum

**Scale/Scope**:

- Retencao indefinida de dados operacionais do Cenario B (sem purge); particionamento por tempo obrigatorio em tabelas de alta cardinalidade
- Tryout E2E cobre minimamente: 2 CommBanks + 1 Banco Central (central-bank-a) + 1 Hub + Relayer Cacti + Paladin mock
- Seed de liquidez inicial obrigatorio (`step4b_seed_liquidity`) executado por Banco Central antes de US1
- Multi-sig: Master Viewing Key 2-of-3, Circuit Breaker pause 1-of-N / resume 2-of-N (auto-executado em `resume-sign`)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constituicao do projeto em `.specify/memory/constitution.md` esta em **estado de template** (principios com placeholders `[PRINCIPLE_N_NAME]`). Sem principios concretos ratificados, nao ha gates automaticos a avaliar nesta fase.

**Resultado do gate**: PASS (por ausencia de regras ratificadas).

**Recomendacao persistente**: ratificar uma constituicao concreta em feature dedicada antes de futuras iteracoes do cutover para que revisoes posteriores possam ser formalmente bloqueadas ou aprovadas por principio.

**Re-check pos-design (2026-05-04)**: PASS — os novos artefatos de design (research.md Decisions 19-23, contracts/scenario-b-backend-api.md atualizado, quickstart.md com step4b) nao introduzem nenhuma contradicao detectavel com as clarificacoes encerradas. Todos os NEEDS CLARIFICATION foram resolvidos.

## Project Structure

### Documentation (this feature)

```text
specs/002-scenario-b-liquidity/
├── plan.md                     # Este arquivo
├── spec.md                     # Especificacao funcional (59 FRs, 30 SCs + clarificacoes 2026-05-04)
├── research.md                 # Decisoes tecnicas consolidadas (Decisions 1-23)
├── data-model.md               # Entidades + state machines + particionamento
├── quickstart.md               # Guia de execucao do rebuild + tryout E2E (step4b incluido)
├── contracts/
│   └── scenario-b-backend-api.md   # Contrato funcional da API v2 (lp_id lookup; schema plano pool; resume auto)
├── checklists/
│   └── requirements.md         # Checklist de qualidade do spec
├── cutover/                    # Fonte autoritativa de registros do corte (YAML)
│   ├── cutover-plan.yaml
│   ├── legacy-artifacts.yaml
│   ├── infrastructure-reuse.yaml
│   ├── docs-editorial-review.md
│   ├── cross-consistency-report.md
│   └── performance-baseline.md
└── tasks.md                    # Gerado por /speckit.tasks
```

### Source Code (repository root)

```text
backend/
├── services/
│   ├── api-gateway/                    # Fiber v2, registro de rotas v2
│   │   ├── cmd/server/                 # main + wire (gates registrados via wiring)
│   │   ├── internal/
│   │   │   ├── http/
│   │   │   │   ├── router/             # router.go (somente rotas v2)
│   │   │   │   └── handlers/           # quote, swap, pool, bridge, compliance, governance, oversight, liquidity
│   │   │   ├── services/               # swap_service.go, swap_gates.go, breaker_gate.go, liquidity_provision_service.go
│   │   │   ├── db/
│   │   │   │   ├── init/               # migrate.go (AutoMigrate), triggers.go, partitions.go, cleanup_scenarioa.go
│   │   │   │   └── seeds/              # endpoint_contract_seed.go
│   │   │   └── audit/                  # audit.go (helper append-only)
│   │   └── openapi/v2/scenario-b.yaml  # OpenAPI 3.1 consolidada (schema plano de pool status)
│   ├── payment-orchestrator/           # Orquestra bridging + swap + unbridging
│   ├── compliance/                     # ZK-Pointers + oversight (Master Viewing Key multi-sig)
│   │   └── internal/services/          # zk_compliance_gate.go, oversight_service.go
│   └── auth/                           # Reconfiguracao Keycloak (perfis CommBank, CentralBank, HubOperator)
└── shared/
    ├── blockchain/
    │   └── scenariob/                  # EVM clients (amm, spokebridge, tcebm, identity), relayer client, paladin client
    └── identity/                       # Reuso de infraestrutura transversal

contracts/
├── src/
│   ├── AutomatedMarketMaker.sol        # pause 1-of-N, resume 2-of-N (auto-execucao on-chain)
│   ├── SpokeBridge.sol
│   ├── TokenizedCentralBankMoney.sol
│   ├── IdentityRegistry.sol
│   ├── CommitmentHashRegistry.sol
│   └── interfaces/
│       ├── IAutomatedMarketMaker.sol
│       ├── ISpokeBridge.sol
│       └── ITokenizedCentralBankMoney.sol
└── test/                               # forge tests AMM, bridge, tCeBM, CB assimetrico, ZK gate

tryouts/
├── tryout-scenario-b-e2e.sh            # E2E story us1|us2|us3|all
│                                       # step4b_seed_liquidity() obrigatorio antes de US1
│                                       # wait_for_position_state() timeout 120s (5s poll)
│                                       # lp_id capturado de Add Liquidity para Remove
└── README.md

deploy/
├── local/besu/hub/
├── local/besu/spoke-a/
├── local/paladin/spoke-a/
└── local/paladin/spoke-b/
```

**Structure Decision**: Monorepo com tres grandes trilhas — `backend/` (Go, Fiber+gRPC), `contracts/` (Solidity, Foundry) e `tryouts/` (Bash E2E). `frontend/` permanece intocado. O rebuild concentra entregas em `backend/services/{api-gateway,payment-orchestrator,compliance,auth}`, `backend/shared/blockchain/scenariob`, `contracts/src/` e tryout E2E com flag `--story`.

## Complexity Tracking

| Complexidade aceita | Por que necessaria | Alternativa rejeitada |
|---------------------|--------------------|-----------------------|
| Dois modelos multi-sig distintos (Master Viewing Key 2-of-3 com timeout 72h vs Circuit Breaker assimetrico 1-of-N/2-of-N) | Master Viewing Key e investigativo; Circuit Breaker e emergencial (fail-safe unilateral no pause) | Unificar em 2-of-3 para ambos rejeitado por tornar pause emergencial dependente de coordenacao |
| Retencao indefinida + particionamento por tempo | Escolha do usuario (sessao 2); maxima defesa regulatoria | Retencao 7/2 anos rejeitada pelo usuario |
| Audit log append-only apenas via triggers Postgres | Escolha do usuario (sessao 2); protege contra alteracao casual com custo minimo | WORM e anchor on-chain rejeitados como superescopo |
| Observabilidade ad-hoc (stdout) | Escolha do usuario (sessao 2); evita lock-in em stack prematura | Stack estruturada (Prom+OTel) rejeitada; SCs afetados marcados como validacao qualitativa |
| Sem rate limiting | Escolha do usuario (sessao 2); abuso contido por auth + rede | Token bucket por tenant rejeitado; risco documentado |
| Frontend intocado apos remocao da API v1 | Escolha do usuario; evita retrabalho duplo | Congelar com pagina de manutencao rejeitado |
| Relayer com fila persistente em Postgres (5 tentativas, backoff exponencial, cap 60s) | Escolha do usuario (sessao 1); reaproveita DATASTORE; sobrevive a restart | Fila in-memory, Redis Streams rejeitados |
| step4b_seed_liquidity() como pre-condicao obrigatoria do tryout | Pool vazio invalida 100% dos cenarios de US1/US2; provisao por Banco Central e a unica role autorizada | Incluir liquidez dentro de US1 rejeitado (mistura setup com cenario de teste) |
| Lookup de Remove Liquidity por lp_id (UUID) | Campo estavel e unico no banco; lp_shares hex pode mudar com rebalanceamentos | lp_shares como chave rejeitado por ambiguidade |
| resume-sign executa transicao automaticamente ao atingir quorum | Comportamento confirmado no tryout; elimina endpoint executeResume desnecessario | executeResume separado rejeitado: adiciona round-trip sem valor |
| Schema plano em pool status (reserve_a, reserve_b) | Alinhamento com response real observado no tryout E2E | Objeto reserves aninhado rejeitado: nao corresponde ao schema implementado |
