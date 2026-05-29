# Quickstart - Scenario B Backend + Contracts Rebuild

## Objetivo
Executar a transicao para a nova API backend do Cenario B com corte completo do Cenario A, mantendo somente infraestrutura transversal reaproveitavel, e validar a entrega tecnica atraves de um tryout E2E dedicado.

## 0) Pre-requisitos
- Docker e docker compose instalados
- Go 1.22+, Foundry (`forge`, `cast`), jq, openssl
- Acesso aos docker-compose de `deploy/local/paladin/spoke-a` e `deploy/local/paladin/spoke-b`
- Keycloak, Postgres e Redis em execucao (infraestrutura transversal reaproveitada)
- `k6` opcional para baseline de performance (T105)

## 0.1) Comandos rapidos via Makefile (`make/60-scenario-b.mk`)

Todos os steps podem ser executados via Makefile. Targets principais:

| Target                         | O que faz                                                                 |
|--------------------------------|---------------------------------------------------------------------------|
| `make scenario-b.up-infra`     | Sobe Keycloak + Postgres + Redis + Besu Spoke-A (Hub) + Spoke-B           |
| `make scenario-b.up-relayer`   | Sobe Cacti Relayer (`interop/hub-and-spoke/cacti`)                        |
| `make scenario-b.deploy-contracts` | Build Foundry + deploy AMM (Hub=Spoke-A) + SpokeBridge em ambos spokes |
| `make scenario-b.up-backend`   | Sobe api-gateway v2 + payment-orchestrator + compliance                    |
| `make scenario-b.up`           | Stack completo (infra + relayer + contratos + backend)                    |
| `make scenario-b.down`         | Desliga tudo com ordem correta                                            |
| `make scenario-b.test`         | `forge test` (contratos) + `go test ./...` (backend)                      |
| `make scenario-b.tryout`       | Executa `tryouts/tryout-scenario-b-e2e.sh all` (US1+US2+US3)              |
| `make scenario-b.tryout-usX`   | Roda apenas a user story X (us1/us2/us3)                                  |
| `make scenario-b.perf-baseline`| Roda `tests/performance/scenario-b-perf.js` com k6                        |
| `make scenario-b.validate-openapi` | Valida `scenario-b.yaml` com `@redocly/cli`                            |

Override de endereco do Hub (por padrao usa Spoke-A como Hub em dev):
```bash
BESU_HUB_RPC=http://hub-besu:8545 make scenario-b.deploy-contracts
```

## Escopo desta feature (decisoes consolidadas)
- **Frontend OUT-OF-SCOPE**: nao editar nada sob `frontend/apps/*`. Incompatibilidade pos-cutover e risco aceito.
- **Retencao indefinida** em todas as tabelas operacionais do Cenario B (Decision 14); sem jobs de purga; particionamento por tempo obrigatorio.
- **Audit logs append-only** via triggers Postgres (Decision 15); ver classe supertipo `audit_log` em `data-model.md`.
- **Observabilidade estruturada delegada** (Decision 16): logs ad-hoc em stdout; SC-014/019/023 validados qualitativamente.
- **Seed de liquidez inicial** (`step4b_seed_liquidity`): obrigatorio antes de US1; provisao por Banco Central (central-bank-a) e a unica role autorizada (Decision 19); pool zerado e erro de ambiente, nao cenario de teste valido.
- **Circuit Breaker assimetrico** (FR-030): `pause` 1-of-N, `resume` 2-of-N com coleta de assinaturas.
- **Master Viewing Key** 2-of-3 com timeout 72h (FR-035 / FR-045).
- **Relayer Cacti**: 5 retries idempotentes (backoff 2/4/8/16/32s, cap 60s), fila persistente em Postgres (Decision 11).
- **Performance targets**: quote p95 <= 300ms, swap p95 <= 6s, monitor p95 <= 15s (Decision 13).

## 1) Preparacao do corte
- Confirmar branch ativa: `002-scenario-b-liquidity`
- Popular `LegacyArtifactInventory` listando rotas, handlers, services, jobs, schemas e testes do Cenario A a descontinuar
- Popular `InfrastructureReuseRegister` com Keycloak, Postgres, Redis, observabilidade base e containers de rede (cada item com `legacy_dependency_check = true`)
- Criar `ScenarioBApiCutoverPlan` com `status = PLANNED` e janela de corte definida

## 2) Contratos Solidity (Hub + Spokes)
- Revisar `contracts/src/AutomatedMarketMaker.sol` para garantir:
  - Swap Exact-Output com protecao `maxAmountIn`
  - Gatilho de Circuit Breaker autorizado ao perfil de Banco Central
  - Emissao de eventos consumidos pelo Liquidity Monitor
- Revisar `contracts/src/SpokeBridge.sol` e `TokenizedCentralBankMoney.sol` para ciclo Lock&Mint / Burn&Unlock consistente com o Relayer (Cacti)
- Rodar testes: `cd contracts && forge test`
- Publicar deploys locais via `contracts/script/*.s.sol` no Hub e nos Spokes

## 3) Nova API v2 (backend/services/api-gateway)
- Remover rotas e handlers do Cenario A
- Executar inicializadores GORM em `internal/db/init/` (FR-055 — sem arquivos `.sql`): `RunAutoMigrate(db)` para criar tabelas, `CreatePartitions(db)` para particoes mensais (conforme `data-model.md` secao 15) e `CreateAppendOnlyTriggers(db)` para triggers append-only nas tabelas classificadas como `audit_log` (secao 14).
- Implementar grupos da API v2 conforme `contracts/scenario-b-backend-api.md`:
  - Quote Exact-Output (`GET /api/v2/amm/quote/exact-output`)
  - Swap Exact-Output com `maxAmountIn` (`POST /api/v2/amm/swap/exact-output`)
  - Pool Status (`GET /api/v2/amm/pool/{pair}/status`)
  - Governance Circuit Breaker **assimetrico**:
    - `POST /api/v2/amm/governance/circuit-breaker/pause` (1-of-N)
    - `POST /api/v2/amm/governance/circuit-breaker/resume-request`
    - `POST /api/v2/amm/governance/circuit-breaker/resume-sign` (aplica ao atingir 2-of-N)
    - `GET /api/v2/amm/governance/circuit-breaker/status`
  - Liquidity Provisioning (`POST /api/v2/amm/liquidity/add|remove`)
  - Bridging/Unbridging (`POST /api/v2/bridge/lock-mint|burn-unlock`)
  - Compliance ZK-Pointers (`POST /api/v2/compliance/zk-pointer`, `GET .../{id}`)
  - Liquidity Monitor (`GET /api/v2/amm/liquidity/monitor/{pair}`, `GET .../alerts`)
  - Central Bank Oversight (open / sign / disclose / status) com timeout 72h e quorum 2-of-3
- Cobrir cada endpoint com testes de contrato (sucesso, erro funcional, autorizacao, quorum parcial).

## 4) Orquestracao de dominio (backend/services/payment-orchestrator)
- Implementar maquinas de estado:
  - `BridgedAssetPosition`: `LOCKED -> MINTED -> BURNED -> UNLOCKED` (e `FAILED`)
  - `SwapOrderScenarioB`: `PENDING -> EXECUTED | FAILED_SLIPPAGE | FAILED_COMPLIANCE | FAILED_BRIDGE`
  - `ScenarioBRiskControlState`: alerta `NORMAL | WARNING | BREACHED` (threshold 70/30 REQ-FX-008)
- Integrar com o Relayer (Cacti) para refletir eventos de Lock/Mint/Burn/Unlock
- Conectar clientes EVM (`backend/shared/blockchain`) aos contratos Hub/Spoke

## 5) Compliance (backend/services/compliance)
- Implementar ingestao e verificacao de ZK-Pointers antes da execucao do swap
- Integrar hook de Master Viewing Key para fluxos de `POST /oversight/disclosure-request`
- Garantir que nenhum dado sensivel de KYC/AML trafegue alem do digest publico

## 6) Autenticacao e perfis (backend/services/auth)
- Reconfigurar realm/clients Keycloak para perfis do Cenario B: `commbank`, `centralbank`, `hub-operator`
- Nao reaproveitar claims/roles do Cenario A; alinhar aos endpoints v2

## 7) Execucao do corte unico (big bang)
- Mover `ScenarioBApiCutoverPlan` para `IN_PROGRESS`
- Congelar deploys legados e descontinuar dados/historico do Cenario A
- Aplicar remocao/substituicao de todos os artefatos do inventario
- Ativar somente a nova API v2 (nenhuma coexistencia com a API antiga)
- Registrar `executed_at` e transitar `ScenarioBApiCutoverPlan` para `COMPLETED` apos validacao

## 8) Validacao pos-corte via tryout E2E
Rodar `tryouts/tryout-scenario-b-e2e.sh [--story us1|us2|us3|all]`, que deve:
1. Subir Spoke A, Spoke B e Hub (docker compose) e fazer deploy dos contratos
2. Registrar participantes e preparar ZK-Pointers dos bancos
3. **[step4b — OBRIGATORIO]** Provisionar liquidez inicial via Banco Central (central-bank-a) em `step4b_seed_liquidity()` antes de qualquer step de US1 (Decision 19); pool zerado antes deste step e erro de ambiente, nao cenario de teste valido
4. Executar `lock-mint` no Spoke A; **aguardar** `BridgedAssetPosition = ACTIVE` via polling (timeout 120s, intervalo 5s — Decision 20) antes de prosseguir com swap ou burn-unlock
5. Solicitar quote Exact-Output para o beneficiario
6. Executar swap com `maxAmountIn` (feliz) e validar debito/credito + reservas atualizadas
7. Repetir swap com `maxAmountIn` deliberadamente baixo e validar `FAILED_SLIPPAGE`
8. Consultar `pool/{pair}/status` via selector jq `.reserve_a` (campo plano — Decision 23); forcar desequilibrio para validar alerta 70/30
9. Acionar Circuit Breaker:
   - `pause` com 1 assinatura e validar bloqueio de novos swaps em ate 1 bloco (SC-017)
   - Abrir `resume-request` + enviar apenas 1 assinatura adicional -> estado permanece `RESUME_PENDING` e tentativa de resume on-chain sem quorum MUST emitir evento de disputa (FR-044 / SC-026)
   - Enviar segunda assinatura distinta via `resume-sign` -> transita automaticamente para `LIVE` (Decision 22; nenhum `executeResume` separado necessario)
10. Executar `burn-unlock`; capturar `lp_id` do response de Add Liquidity e usar em Remove Liquidity (Decision 21); simular exaustao de retries do Relayer e validar transicao para `RECONCILIATION_REQUIRED` (SC-019); depois rodar fluxo feliz e confirmar unlock no Spoke B
11. Abrir `disclosure-request`, adicionar 1 assinatura (insuficiente), validar que sem quorum 2-of-3 a operacao nao libera dados; adicionar 2a assinatura dentro de 72h e concluir disclosure (SC-020)

Criterios de sucesso do tryout:
- Nenhuma chamada a endpoints do Cenario A (SC-012)
- Transacoes registram ZK-Pointers verificados (SC-016)
- Bridge ponta-a-ponta consistente (SC-015) e reconciliacao escalada apos 5 falhas (SC-019)
- Alerta 70/30 rastreavel (SC-014, qualitativo nesta feature)
- Circuit Breaker: pause unilateral efetivo em <= 1 bloco (SC-017); resume somente com 2-of-N (SC-026); tentativa sem quorum gera evento de disputa (FR-044)
- Disclosure: quorum 2-of-3 enforced com timeout 72h (SC-020)
- Tabelas append-only recusam `UPDATE`/`DELETE` (SC-030)

## 9) Criterios de pronto operacionais
- 0 artefatos funcionais ativos do Cenario A (SC-010)
- 100% dos endpoints antigos `REMOVED` ou `DEPRECATED` ja baixados do roteador (SC-006/SC-008)
- 100% dos dados/historicos legados descontinuados (SC-009)
- 100% de reuso em infraestrutura transversal registrada (SC-011)
- 0 arquivos modificados sob `frontend/apps/*` (SC-025)
- Tabelas `audit_log` com triggers ativos recusando `UPDATE`/`DELETE` (SC-030)
- Baseline de performance registrado em `specs/002-scenario-b-liquidity/cutover/performance-baseline.md` com p95 dentro dos alvos (FR-037 / Decision 13) ou desvio documentado como risco
- Tryout E2E verde (SC-012) com cobertura de slippage, alerta 70/30, bridge/unbridge + reconciliacao, ZK, Circuit Breaker assimetrico e Master Viewing Key 2-of-3
