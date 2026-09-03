# Glossary — Scenario B (Hub-and-Spoke Liquidity Pool)

> Termos canonicos utilizados ao longo dos documentos, especificacoes e codigo do
> Cenario B. Uso consistente destes termos e obrigatorio (FR-001, FR-008, SC-004).

## Atores e Papeis

| Termo | Definicao |
|-------|-----------|
| **Banco Central (central_bank)** | Entidade com autoridade regulatoria. No Cenario B atua como Liquidity Provider experimental, signatario de Circuit Breaker e de disclosure via Master Viewing Key. Role Keycloak: `central_bank`. |
| **Banco Comercial (commercial_bank)** | Participante autorizado a operar swaps, bridging e consultar cotacoes. Role Keycloak: `commercial_bank`. |
| **Hub Operator (hub_operator)** | Operador da infraestrutura do Hub (nos Besu e AMM). Nao aprova operacoes de negocio. Role Keycloak: `hub_operator`. |
| **Liquidity Taker** | Cliente consumidor de liquidez (swap contra o AMM). Tipicamente um `commercial_bank`. |
| **Liquidity Provider / Issuer** | Entidade que provisiona liquidez via `liquidity/add`. No Cenario B, Bancos Centrais experimentais. |
| **Network Operator** | Papel de infraestrutura (hub + spokes + relayer + keycloak). Nao e um usuario de API. |

## Arquitetura e Topologia

| Termo | Definicao |
|-------|-----------|
| **Hub** | Rede Besu onde reside o `AutomatedMarketMaker`. Em dev, e o mesmo container do **Spoke-A** (porta `8645`) para reduzir footprint; pode ser separado via env `BESU_HUB_RPC`. |
| **Spoke** | Rede Besu onde residem `SpokeBridge` + `TokenizedCentralBankMoney` de uma jurisdicao. Ha `Spoke-A` (porta `8645`) e `Spoke-B` (porta `8745`). |
| **Relayer (Cacti)** | Servico Hyperledger Cacti que orquestra eventos cross-chain (Lock/Burn). Idempotencia via chave derivada do evento origem. 5 retries, backoff exponencial (2/4/8/16/32s, cap 60s). |
| **API Gateway v2** | Backend Go (Fiber) expondo `/api/v2/*`. Substitui completamente a API v1 do Cenario A. |

## Fluxos de Liquidez

| Termo | Definicao |
|-------|-----------|
| **Lock&Mint** | Usuario trava ativo nativo no Spoke; Relayer propaga; Hub minta representacao espelhada (`BridgedAssetPosition` = ACTIVE). |
| **Burn&Unlock** | Usuario queima representacao no Hub; Relayer propaga; Spoke destrava nativo. |
| **Bridging / Unbridging** | Sinonimos para Lock&Mint / Burn&Unlock. |
| **Swap Exact-Output** | Usuario especifica `amount_out` desejado e `max_amount_in`; AMM calcula input necessario. Se input > `max_amount_in`, rejeitado com `SLIPPAGE_LIMIT_EXCEEDED`. |
| **Price Impact** | Razao entre o preco efetivo do swap e o preco marginal antes da operacao. |
| **Threshold 70/30** | Gatilho do Liquidity Monitor: quando a razao entre reservas sai da faixa 30%-70%, um `LiquidityAlert` e emitido. |

## Governanca

| Termo | Definicao |
|-------|-----------|
| **Circuit Breaker Assimetrico** | Mecanismo on-chain de pause/resume no `AutomatedMarketMaker`. **Pause** requer 1-of-N (fail-safe); **Resume** requer 2-of-N (quorum). |
| **Pause 1-of-N** | Qualquer `central_bank` autorizado pode pausar o AMM unilateralmente em <=1 bloco (SC-017). Emite `CircuitBreakerPaused`. |
| **Resume 2-of-N** | Reativar exige 2 assinaturas distintas. Tentativa com 1 assinatura emite `CircuitBreakerResumeDisputed` (FR-044 / SC-026). Ao atingir 2 assinaturas validas, emite `CircuitBreakerResumed`. |
| **ResumeProposal** | Registro on-chain de uma tentativa de resume com `proposalId`, contador de `signatures`, mapping de assinantes. |
| **Master Viewing Key (MVK)** | Chave de disclosure multi-assinatura 2-of-3 integrada via Paladin JSON-RPC. Libera dados sensiveis apenas quando quorum atingido. |
| **Disclosure Request** | Solicitacao de MVK com `expires_at = now + 72h`. Estados: PENDING, APPROVED, DENIED, EXPIRED. Append-only. |
| **Quorum** | Numero minimo de assinaturas distintas necessario para efetivar uma operacao governada. `RESUME_QUORUM = 2`, `DISCLOSURE_QUORUM = 2`. |

## Compliance e Privacidade

| Termo | Definicao |
|-------|-----------|
| **ZK-Pointer** | Apontador Zero-Knowledge para evidencia de KYC/AML validada off-chain. Campos: `pointer_id`, `commitment_hash`, `proof_cid`, `expires_at`. Estados: VALID, EXPIRED, REVOKED. |
| **Commitment Hash Registry** | Contrato que ancora commitments publicos on-chain. Nao contem dados pessoais. |
| **ZK Validation Failure** | Swap rejeitado por `ZK_VALIDATION_FAILED` mantem `BridgedAssetPosition.bridge_state = ACTIVE`; cliente pode retentar com novo pointer (FR-058). |

## Estados Canonicos

### `SwapOrderScenarioB` (FR-057)
`PENDING` -> `SUBMITTED` -> `CONFIRMING` -> (`COMPLETED` | `FAILED`)

### `BridgedAssetPosition`
`LOCKING` -> `ACTIVE` -> `BURNING` -> `RELEASED`; ou `RECONCILIATION_REQUIRED` apos 5 falhas do Relayer.

### `ScenarioBRiskControlState.circuit_breaker_state`
`LIVE` -> `HALTED` -> `RESUME_PENDING` -> `LIVE`.

### `DisclosureRequest.state`
`PENDING` -> (`APPROVED` | `DENIED` | `EXPIRED`).

## Codigos de Erro (API v2)

| Codigo | HTTP | Quando ocorre |
|--------|------|---------------|
| `SLIPPAGE_LIMIT_EXCEEDED` | 422 | Input calculado > `max_amount_in` (FR-059) |
| `INSUFFICIENT_POOL_LIQUIDITY` | 422 | Reserva do par zerada/abaixo do minimo (FR-059) |
| `ZK_VALIDATION_FAILED` | 422 | ZK-Pointer invalido/expirado/revogado (FR-058) |
| `INSUFFICIENT_ROLE` | 403 | JWT sem `realm_access.roles` exigido pelo endpoint (FR-056) |
| `CIRCUIT_BREAKER_HALTED` | 503 | Tentativa de swap com AMM pausado |
| `DISCLOSURE_EXPIRED` | 422 | Tentativa de assinar disclosure apos 72h |

## Infraestrutura

| Termo | Definicao |
|-------|-----------|
| **AutoMigrate** | Metodo GORM usado para criar/sincronizar schemas. Nao ha arquivos `.sql` (FR-055). |
| **Append-Only Audit Log** | Tabela com triggers PL/pgSQL `BEFORE UPDATE/DELETE` que lancam excecao (FR-048/FR-049/SC-030). Aplica-se a: `circuit_breaker_signatures`, `disclosure_signatures`, `liquidity_alerts`, tabelas `*_event_history`. |
| **Time-Partitioning** | Particionamento PostgreSQL por mes em tabelas de alta cardinalidade: `swap_order_scenario_b`, `pool_state_readings` (FR-046). Substitui jobs de purge (retencao indefinida). |
| **Idempotency Key** | Hash deterministico derivado de `(event_type, source_chain, tx_ref, block_number)` usado pelo Relayer para evitar efeitos duplicados. |

## Abreviacoes

| Sigla | Significado |
|-------|-------------|
| **AMM** | Automated Market Maker |
| **CB** | Circuit Breaker (ou Central Bank, conforme contexto) |
| **LP** | Liquidity Provider / Liquidity Position |
| **MVK** | Master Viewing Key |
| **RBAC** | Role-Based Access Control |
| **tCeBM** | Tokenized Central Bank Money |
| **ZK** | Zero-Knowledge |

## Termos Proibidos (legacy — Cenario A)

Os seguintes termos NAO devem aparecer em documentos do Cenario B (SC-001 / SC-010):

- `scenario.a`, `scenarioA`, `scenario_a`, `CenarioA`
- `fx-agreement` (substituido por `swap-exact-output`)
- `htlc` fora de contexto historico
- `trade_order`, `trade_id` (substituidos por `swap_order_scenario_b`, `swap_id`)
- Roles antigas (`banker`, `regulator`) sem sufixo claro de legado
