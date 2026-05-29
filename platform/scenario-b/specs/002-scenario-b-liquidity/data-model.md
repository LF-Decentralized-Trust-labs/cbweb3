# Data Model - Scenario B Backend

## 1. ScenarioBApiCutoverPlan
- **Purpose**: Representar o plano de corte unico da API antiga para a nova API B.
- **Fields**:
  - `cutover_id` (string, unico)
  - `scheduled_at` (datetime)
  - `executed_at` (datetime, opcional ate execucao)
  - `status` (enum: `PLANNED`, `IN_PROGRESS`, `COMPLETED`, `ROLLED_BACK`)
  - `legacy_artifacts_total` (int)
  - `legacy_artifacts_removed` (int)
  - `infra_components_reused` (list[string])
- **Validation Rules**:
  - `legacy_artifacts_removed` MUST ser igual a `legacy_artifacts_total` quando `status=COMPLETED`.
  - `infra_components_reused` nao pode conter componentes funcionais de dominio do Cenario A.

## 2. ScenarioBEndpointContract
- **Purpose**: Definir o contrato funcional da nova API do Cenario B.
- **Fields**:
  - `endpoint_id` (string, unico)
  - `domain` (enum: `QUOTE`, `SWAP`, `POOL_STATUS`, `GOVERNANCE_RISK`)
  - `path` (string)
  - `method` (enum: `GET`, `POST`)
  - `state` (enum: `ACTIVE`, `DEPRECATED`, `REMOVED`)
  - `acceptance_criteria_ref` (string)
- **Validation Rules**:
  - Nenhum endpoint `ACTIVE` pode ter referencia funcional a fluxos de Cenario A.
  - Endpoints antigos devem estar em `REMOVED` ao final do corte.

## 3. LegacyArtifactInventory
- **Purpose**: Inventariar artefatos backend legados para descontinuacao.
- **Fields**:
  - `artifact_id` (string, unico)
  - `artifact_type` (enum: `ROUTE`, `HANDLER`, `SERVICE`, `JOB`, `TEST`, `DOC`, `SCHEMA`)
  - `source_path` (string)
  - `migration_action` (enum: `REMOVE`, `REPLACE`)
  - `replacement_ref` (string, opcional)
  - `cutover_batch` (string)
- **Validation Rules**:
  - Todo `artifact_id` inventariado deve ter `migration_action`.
  - `REPLACE` exige `replacement_ref`.

## 4. InfrastructureReuseRegister
- **Purpose**: Registrar itens reaproveitados de infraestrutura transversal.
- **Fields**:
  - `component_id` (string, unico)
  - `component_name` (string)
  - `layer` (enum: `IDENTITY`, `DATASTORE`, `CACHE`, `RUNTIME`, `OBSERVABILITY`)
  - `reuse_mode` (enum: `AS_IS`, `RECONFIGURED`)
  - `legacy_dependency_check` (boolean)
- **Validation Rules**:
  - `legacy_dependency_check` MUST ser `true` para liberar reuso.
  - Componentes com dependencia funcional do Cenario A nao podem entrar no registro.

## 5. ScenarioBRiskControlState
- **Purpose**: Modelar estado de controle de risco operacional do Cenario B.
- **Fields**:
  - `control_id` (string, unico)
  - `pool_pair` (string)
  - `imbalance_threshold` (decimal, default 0.70)
  - `current_ratio` (decimal)
  - `imbalance_alert_state` (enum: `NORMAL`, `WARNING`, `BREACHED`)
  - `last_alert_at` (datetime, opcional)
  - `circuit_breaker_state` (enum: `LIVE`, `HALTED`, `RESUME_PENDING`)
  - `pause_initiator_bank_id` (string, opcional)      // BC que acionou o pause
  - `pause_reason_code` (string, opcional)
  - `resume_request_id` (string, opcional)            // vincula a coleta de assinaturas de resume
  - `resume_signatures_required` (int, default 2)     // quorum 2-of-N (FR-030)
  - `updated_at` (datetime)
- **Validation Rules**:
  - Quando `current_ratio` ultrapassa `imbalance_threshold` (ex.: 70/30), `imbalance_alert_state` deve transitar para `BREACHED` e gerar alerta auditavel.
  - `pause` MUST ser aceito com 1-of-N (qualquer Banco Central autorizado) e MUST registrar `pause_initiator_bank_id` + `pause_reason_code`.
  - `resume` MUST exigir quorum 2-of-N assinaturas distintas coletadas em `CircuitBreakerSignature` vinculadas por `resume_request_id`; enquanto quorum nao atingir, estado permanece `RESUME_PENDING`.
  - Transicoes de `circuit_breaker_state` sao gravadas como entradas de `audit_log` append-only.

## 5a. CircuitBreakerSignature
- **Purpose**: Registrar assinaturas institucionais parciais de eventos de pause e resume do Circuit Breaker, formalizando o quorum assimetrico (FR-030 / SC-017 / SC-026).
- **Fields**:
  - `signature_id` (string, unico)
  - `control_id` (FK -> `ScenarioBRiskControlState.control_id`)
  - `event_kind` (enum: `PAUSE`, `RESUME`)
  - `request_id` (string)                              // agrupador para resume (quorum 2-of-N)
  - `signer_bank_id` (string)
  - `signer_wallet` (string)                           // endereco EVM que assinou on-chain
  - `signature_payload` (bytes)                        // assinatura agregada on-chain
  - `on_chain_tx_ref` (string, opcional)
  - `signed_at` (datetime)
- **Validation Rules**:
  - `PAUSE` requer exatamente 1 registro para concluir o evento.
  - `RESUME` requer >= 2 registros distintos (`signer_bank_id` unico por `request_id`) antes de publicar transicao no AMM do Hub.
  - Registro e append-only (ver secao 14 Audit).

## 6. LiquidityPosition
- **Purpose**: Representar a contribuicao de um Liquidity Provider ao pool AMM; gerada pelo `POST /api/v2/amm/liquidity/add` e encerrada pelo `POST /api/v2/amm/liquidity/remove`.
- **Fields**:
  - `lp_id` (UUID, PK, gerado pelo backend no momento do Add Liquidity)
  - `pool_pair` (string — ex.: `BRL-USD`)
  - `provider_bank_id` (string)
  - `token_a_amount` (decimal)
  - `token_b_amount` (decimal)
  - `lp_shares` (string — hex on-chain, retornado para referencia; NAO e chave de lookup)
  - `status` (enum: `ACTIVE`, `WITHDRAWN`)
  - `created_at` (datetime)
  - `withdrawn_at` (datetime, opcional — preenchido na transicao para `WITHDRAWN`)
- **Validation Rules**:
  - `lp_id` MUST ser UUID v4 gerado pelo backend; jamais derivado de `lp_shares` on-chain.
  - Handler `POST /api/v2/amm/liquidity/remove` MUST filtrar por `lp_id` AND `status = ACTIVE`; ausencia ou `WITHDRAWN` retorna HTTP 404.
  - Transicao `ACTIVE → WITHDRAWN` e o unico update permitido na tabela; NAO e append-only (diferente das tabelas `audit_log`).
  - Enum canonico: `ACTIVE` (injetada, disponivel no pool) e `WITHDRAWN` (retirada, estado terminal sem reentrada).

## 7. BridgedAssetPosition
- **Purpose**: Modelar a posicao de ativos espelhados no Hub decorrente do bridging Lock&Mint / Burn&Unlock.
- **Fields**:
  - `position_id` (string, unico)
  - `owner_bank_id` (string)
  - `spoke_network` (string)                    // origem do ativo nativo
  - `native_asset` (string)                     // ex.: `tCeBMa`
  - `mirrored_asset` (string)                   // ex.: `W-tCeBMa`
  - `amount_locked` (decimal)
  - `amount_minted` (decimal)
  - `relayer_proof_ref` (string)                // prova entregue pelo Cacti
  - `bridge_state` (enum: `LOCKING`, `ACTIVE`, `BURNED`, `UNLOCKED`, `RECONCILIATION_REQUIRED`, `FAILED`)
  - `created_at` (datetime)
  - `updated_at` (datetime)
- **Validation Rules**:
  - `amount_minted` MUST ser igual a `amount_locked` em `ACTIVE` (sem fee de bridge no modelo base).
  - Transicao para `UNLOCKED` exige `BURNED` previo com mesma quantidade.
  - Valores canonicos do campo `bridge_state`: `LOCKING` (transitorio — aguardando confirmacao do Relayer), `ACTIVE` (mint confirmado no Hub — substitui `MINTED` do modelo anterior), `BURNED` (queima on-chain confirmada), `UNLOCKED` (liberacao no Spoke B confirmada), `RECONCILIATION_REQUIRED` (esgotadas 5 retentativas), `FAILED` (erro irrecuperavel). Os valores `LOCKED` e `MINTED` NAO DEVEM ser usados no backend v2.
  - Toda transicao precisa registrar `relayer_proof_ref` associada.

## 7. ComplianceZKPointer
- **Purpose**: Registrar validacoes "fit to transact" por ZK-Pointer aplicadas a operacoes no AMM.
- **Fields**:
  - `pointer_id` (string, unico)
  - `participant_id` (string)
  - `transaction_ref` (string)                  // swap/add/remove liquidity no Hub
  - `proof_digest` (string)                     // hash/commitment publico da prova
  - `verification_state` (enum: `PENDING`, `VERIFIED`, `REJECTED`)
  - `verified_at` (datetime, opcional)
- **Validation Rules**:
  - Nenhum dado sensivel de KYC/AML pode ser persistido fora do digest publico.
  - Execucao no AMM exige `VERIFIED` para todos os participantes.
  - `REJECTED` bloqueia o debito/credito e registra trilha de auditoria.

## 8. SwapOrderScenarioB
- **Purpose**: Representar uma ordem de swap Exact-Output submetida pelo pagador.
- **Fields**:
  - `order_id` (string, unico)
  - `payer_bank_id` (string)
  - `beneficiary_bank_id` (string)
  - `pool_pair` (string)
  - `exact_output_amount` (decimal)
  - `max_amount_in` (decimal)                   // protecao de slippage
  - `quote_ref` (string, opcional)
  - `zk_pointer_refs` (list[string])
  - `execution_state` (enum: `PENDING`, `EXECUTED`, `FAILED_SLIPPAGE`, `FAILED_COMPLIANCE`, `FAILED_BRIDGE`)
  - `executed_at` (datetime, opcional)
- **Validation Rules**:
  - `max_amount_in` e obrigatorio; execucao deve falhar com `FAILED_SLIPPAGE` quando entrada calculada exceder o limite.
  - Execucao exige `zk_pointer_refs` com todos os `ComplianceZKPointer` em `VERIFIED`.
  - Ordem so pode concluir em `EXECUTED` quando bridging de entrada esta em `MINTED`.

## 9. RelayerQueueItem
- **Purpose**: Persistir trabalhos do Relayer (Hyperledger Cacti) com idempotencia e retries conforme Decision 11 (5 tentativas, backoff 2/4/8/16/32s, cap 60s) — sustenta FR-043 e SC-019.
- **Fields**:
  - `queue_item_id` (string, unico)
  - `event_kind` (enum: `SPOKE_LOCK`, `HUB_MINT`, `HUB_BURN`, `SPOKE_UNLOCK`)
  - `source_ref` (string)                              // hash do evento origem (idempotencia)
  - `idempotency_key` (string, unique)                 // derivado de `source_ref` + `event_kind`
  - `target_position_id` (FK -> `BridgedAssetPosition.position_id`)
  - `attempts` (int, default 0, max 5)
  - `next_attempt_at` (datetime)
  - `last_error` (string, opcional)
  - `state` (enum: `PENDING`, `IN_FLIGHT`, `FAILED`, `ESCALATED`, `COMPLETED`)
  - `created_at` (datetime)
  - `updated_at` (datetime)
- **Validation Rules**:
  - Backoff: `next_attempt_at = now + min(2^attempts seconds, 60s)`.
  - Apos `attempts == 5` sem sucesso, o worker MUST executar dois updates atomicamente em uma unica transacao GORM: (1) `RelayerQueueItem.state = ESCALATED`; (2) `BridgedAssetPosition.bridge_state = RECONCILIATION_REQUIRED`. Os dois estados coexistem intencionalmente: `ESCALATED` reflete o ciclo de vida da tarefa de entrega; `RECONCILIATION_REQUIRED` reflete o estado do ativo. Ambos sao terminais neste ramo de falha.
  - `idempotency_key` garante que reexecucoes nao criam efeito on-chain duplicado.

## 10. DisclosureRequest
- **Purpose**: Formalizar solicitacao de disclosure via Master Viewing Key (Paladin) com quorum 2-of-3 e timeout 72h — sustenta FR-035 e FR-045.
- **Fields**:
  - `request_id` (string, unico)
  - `requested_by_bank_id` (string)                    // BC solicitante
  - `target_transaction_ref` (string)                  // swap ou bridging sob investigacao
  - `reason_code` (string)                             // `AML_INVESTIGATION`, `CFT_INVESTIGATION`, `COURT_ORDER`
  - `state` (enum: `OPEN`, `QUORUM_REACHED`, `DISCLOSED`, `EXPIRED`, `REJECTED`)
  - `quorum_required` (int, default 2)                 // 2-of-3
  - `opened_at` (datetime)
  - `expires_at` (datetime)                            // opened_at + 72h
  - `closed_at` (datetime, opcional)
- **Validation Rules**:
  - Auto-expiracao para `EXPIRED` quando `now > expires_at` e quorum ainda nao alcancado.
  - Transicao para `QUORUM_REACHED` exige >= `quorum_required` entradas em `DisclosureSignature`.
  - `DISCLOSED` so ocorre apos `QUORUM_REACHED` e chamada efetiva ao Paladin.

## 11. DisclosureSignature
- **Purpose**: Registrar assinaturas parciais de Bancos Centrais em uma `DisclosureRequest`.
- **Fields**:
  - `signature_id` (string, unico)
  - `request_id` (FK -> `DisclosureRequest.request_id`)
  - `signer_bank_id` (string)
  - `signer_wallet` (string)
  - `signature_payload` (bytes)
  - `signed_at` (datetime)
- **Validation Rules**:
  - `signer_bank_id` unico por `request_id`.
  - Append-only (secao 14 Audit).

## 12. PoolStateReading
- **Purpose**: Registrar leituras do Liquidity Monitor (cadencia p95 <= 15s conforme FR-037/SC-023) com retencao indefinida para analise historica.
- **Fields**:
  - `reading_id` (string, unico)
  - `pool_pair` (string)
  - `reserve_a` (decimal)
  - `reserve_b` (decimal)
  - `computed_ratio` (decimal)
  - `read_at` (datetime)
- **Validation Rules**:
  - Particionamento mensal por `read_at` (tabela de alta cardinalidade, Decision 14).

## 13. LiquidityAlert
- **Purpose**: Materializar alertas gerados pelo Liquidity Monitor quando `computed_ratio` ultrapassa threshold (REQ-FX-008 / FR-028 / SC-014).
- **Fields**:
  - `alert_id` (string, unico)
  - `pool_pair` (string)
  - `breach_level` (enum: `WARNING`, `BREACHED`)
  - `observed_ratio` (decimal)
  - `threshold` (decimal)
  - `triggered_at` (datetime)
  - `cleared_at` (datetime, opcional)
- **Validation Rules**:
  - `cleared_at` define retorno a `NORMAL`; antes disso, alerta esta ativo.
  - Append-only enquanto nao `cleared_at`; apos cleared, modificacoes seguem secao 14.

## 14. audit_log (supertipo append-only)
- **Purpose**: Classificacao logica de tabelas imutaveis do Cenario B (FR-050 / SC-030). Implementacao via triggers Postgres `BEFORE UPDATE` e `BEFORE DELETE` que levantam excecao `append_only_violation`.
- **Tabelas append-only nesta classe**:
  - `circuit_breaker_signature`
  - `disclosure_request` (apenas transicoes registradas adicionam linhas ao historico; a linha mestre usa padrao event-sourced em `disclosure_event_history`)
  - `disclosure_signature`
  - `liquidity_alert` (historico de alertas)
  - `bridged_asset_position_history` (espelho append-only das transicoes de `BridgedAssetPosition`)
  - `swap_order_event_history` (espelho append-only das transicoes de `SwapOrderScenarioB`)
  - `circuit_breaker_event_history` (pause/resume events)
- **Trigger template (referencia)**:
  ```sql
  CREATE OR REPLACE FUNCTION append_only_guard() RETURNS trigger AS $$
  BEGIN
    RAISE EXCEPTION 'append_only_violation: table %% is append-only', TG_TABLE_NAME;
  END;
  $$ LANGUAGE plpgsql;
  -- Aplicado como BEFORE UPDATE e BEFORE DELETE em cada tabela append-only.
  ```

## 15. Particionamento (Decision 14)
Tabelas de alta cardinalidade com retencao indefinida adotam **particionamento por tempo** (mensal) via `PARTITION BY RANGE (<timestamp_col>)` no Postgres:

| Tabela | Coluna de particao |
|--------|---------------------|
| `pool_state_reading` | `read_at` |
| `swap_order_event_history` | `event_at` |
| `bridged_asset_position_history` | `event_at` |
| `liquidity_alert` | `triggered_at` |
| `relayer_queue_item` | `created_at` (particionamento anual, cardinalidade menor) |
| `circuit_breaker_event_history` | `event_at` (particionamento anual) |
| `disclosure_event_history` | `event_at` (particionamento anual) |

Particoes sao criadas automaticamente pelo job de manutencao (nao-destrutivo, sem purge).

## Relationships
- `ScenarioBApiCutoverPlan` 1:N `LegacyArtifactInventory`
- `ScenarioBApiCutoverPlan` 1:N `InfrastructureReuseRegister`
- `ScenarioBEndpointContract` 1:1 ou 1:N com itens `LegacyArtifactInventory` marcados como `REPLACE`
- `ScenarioBRiskControlState` depende de endpoints ativos no dominio `POOL_STATUS` e `GOVERNANCE_RISK`
- `ScenarioBRiskControlState` 1:N `CircuitBreakerSignature`
- `SwapOrderScenarioB` 1:N `ComplianceZKPointer`
- `SwapOrderScenarioB` N:1 `BridgedAssetPosition` (entrada) e pode disparar novo `BridgedAssetPosition` (saida via Burn&Unlock)
- `BridgedAssetPosition` 1:N `RelayerQueueItem`
- `DisclosureRequest` 1:N `DisclosureSignature`
- `LiquidityAlert` N:1 `ScenarioBRiskControlState` (por `pool_pair`)

## State Transitions
- **Cutover**: `PLANNED -> IN_PROGRESS -> COMPLETED` (ou `ROLLED_BACK`)
- **Endpoint**: `ACTIVE -> DEPRECATED -> REMOVED` (para legado) e novo contrato nasce em `ACTIVE`
- **Circuit Breaker**: `LIVE -> HALTED` (via pause 1-of-N) -> `RESUME_PENDING` (coleta 2-of-N) -> `LIVE` (quorum atingido). Retorno direto `HALTED -> LIVE` vedado sem quorum.
- **Imbalance Alert**: `NORMAL -> WARNING -> BREACHED` (e retorno a `NORMAL` apos rebalanceamento)
- **Bridge**: `LOCKING -> ACTIVE -> BURNED -> UNLOCKED` (ramo de falha: `FAILED` ou `RECONCILIATION_REQUIRED` apos exaustao de retries do Relayer; `LOCKING` e `ACTIVE` substituem `LOCKED` e `MINTED` do modelo anterior)
- **RelayerQueueItem**: `PENDING -> IN_FLIGHT -> COMPLETED` ou `PENDING -> IN_FLIGHT -> FAILED -> (retry) PENDING` ate `attempts==5 -> ESCALATED`
- **ZK-Pointer**: `PENDING -> VERIFIED` ou `PENDING -> REJECTED`
- **Swap**: `PENDING -> EXECUTED` ou `PENDING -> FAILED_SLIPPAGE | FAILED_COMPLIANCE | FAILED_BRIDGE | FAILED_CIRCUIT_BREAKER`
- **DisclosureRequest**: `OPEN -> QUORUM_REACHED -> DISCLOSED` ou `OPEN -> EXPIRED` (timeout 72h) ou `OPEN -> REJECTED`
