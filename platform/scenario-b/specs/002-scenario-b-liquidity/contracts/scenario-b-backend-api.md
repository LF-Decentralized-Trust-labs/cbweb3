# Scenario B Backend API Contract (Draft)

## Contract Scope
Contrato funcional exclusivo para Cenario B, substituindo integralmente a API anterior e sem compatibilidade retroativa com fluxos do Cenario A.

## Endpoint Groups

### 1. Quote (Exact Output)
- `GET /api/v2/amm/quote/exact-output`
- **Purpose**: Calcular entrada necessaria para entregar uma saida fixa.
- **Required Inputs**: par de ativos, quantidade de saida alvo, limites operacionais.
- **Output**: quantidade de entrada requerida, impacto de preco, carimbo temporal de cotacao.

### 2. Swap (Exact Output)
- `POST /api/v2/amm/swap/exact-output`
- **Purpose**: Executar troca por saida exata com protecao de limite maximo de entrada.
- **Required Inputs**: referencia da cotacao/parametros equivalentes, identificacao de pagador/beneficiario, `max_amount_in` (obrigatorio, protecao de slippage), referencias de `zk_pointer` dos participantes.
- **Output**: resultado da execucao, volumes finais, estado de sucesso/falha (`EXECUTED`, `FAILED_SLIPPAGE`, `FAILED_COMPLIANCE`, `FAILED_BRIDGE`).
- **Rules**:
  - Execucao MUST falhar com motivo auditavel quando entrada calculada > `max_amount_in`.
  - Execucao MUST exigir ZK-Pointers verificados para pagador e beneficiario.
  - Execucao MUST exigir posicao espelhada (bridging) ativa e suficiente do pagador no Hub.

### 3. Pool Status
- `GET /api/v2/amm/pool/{pair}/status`
- **Purpose**: Expor estado operacional do pool para monitoramento de estabilidade.
- **Output**: body plano (sem objeto `reserves` aninhado) com campos canonicos: `reserve_a` (string), `reserve_b` (string), `current_ratio` (number), `imbalance_flag` (bool), `pool_pair` (string), `updated_at` (RFC3339).
- **Rules**:
  - Nenhum campo `reserves` aninhado deve existir no schema (Decision 23).
  - O tryout MUST verificar disponibilidade do endpoint via selector jq `.reserve_a` (nao `.reserves`).

### 4. Governance Risk Controls (Circuit Breaker assimetrico)
Autorizacao do Circuit Breaker segue modelo assimetrico (FR-030 / Decision 10):
pause = 1-of-N (fail-safe); resume = 2-of-N (quorum minimo). As rotas abaixo
substituem qualquer endpoint unificado anterior.

- `POST /api/v2/amm/governance/circuit-breaker/pause`
- **Purpose**: Acionar pause emergencial do AMM. Qualquer Banco Central autorizado pode pausar (1-of-N) — fail-safe.
- **Required Inputs**: `reason_code`, identificacao do ator autorizado, assinatura institucional.
- **Output**: estado atualizado (`LIVE -> HALTED`), `control_id`, `pause_initiator_bank_id`, referencia auditavel no Hub.
- **Rules**:
  - Autorizacao MUST validar perfil institucional de Banco Central participante.
  - Efeito MUST bloquear novos swaps em ate 1 bloco apos confirmacao no Hub (SC-017), sem interromper execucoes atomicas ja confirmadas.
  - Registro MUST ser append-only em `circuit_breaker_event_history`.

- `POST /api/v2/amm/governance/circuit-breaker/resume-request`
- **Purpose**: Abrir solicitacao de resume; inicia coleta de assinaturas (2-of-N).
- **Required Inputs**: `control_id`, justificativa, identificacao do primeiro signatario.
- **Output**: `resume_request_id`, `signatures_collected=1`, `signatures_required=2`, estado intermediario (`RESUME_PENDING`).

- `POST /api/v2/amm/governance/circuit-breaker/resume-sign`
- **Purpose**: Adicionar assinatura a uma solicitacao de resume aberta; quando quorum 2-of-N e atingido, executa automaticamente a transicao `HALTED -> LIVE` on-chain na mesma resposta.
- **Required Inputs**: `resume_request_id`, identificacao institucional do signatario adicional, assinatura.
- **Output**: `signatures_collected`, estado resultante (`RESUME_PENDING` quando quorum ainda nao atingido, ou `LIVE` quando quorum atingido e transicao executada).
- **Rules**:
  - Signatario adicional MUST ser distinto do iniciador (`signer_bank_id` unico por `resume_request_id`).
  - Quando `signatures_collected >= 2` a transicao `HALTED -> LIVE` e enviada ao AMM do Hub **automaticamente** na mesma chamada (Decision 22); nenhum endpoint `executeResume` separado existe.
  - Tentativa de resume sem quorum MUST emitir evento de disputa auditavel (FR-044).

- `GET /api/v2/amm/governance/circuit-breaker/status`
- **Purpose**: Consultar estado atual e, se houver, `resume_request_id` em aberto com `signatures_collected/required`.

### 5. Liquidity Provisioning
- `POST /api/v2/amm/liquidity/add`
- **Purpose**: Provisionar liquidez no pool (uso experimental por Bancos Centrais como Liquidity Providers).
- **Required Inputs**: par, quantidades de ativos, identificacao do provider, referencias de ZK-Pointer.
- **Output**: `lp_id` (UUID — chave canonico de lookup para Remove), `lp_shares` (hex on-chain — informativo apenas), `pool_pair`, `provider_bank_id`, `token_a_amount`, `token_b_amount`, `added_at`.
- `POST /api/v2/amm/liquidity/remove`
- **Purpose**: Retirar liquidez previamente provisionada.
- **Required Inputs**: `lp_id` (UUID obrigatorio — chave de lookup da `LiquidityPosition`; `lp_shares` NAO e aceito como chave de remocao — Decision 21), `provider_bank_id`.
- **Output**: ativos retornados, reservas atualizadas, `withdrawn_at`.
- **Rules**:
  - O tryout MUST capturar `lp_id` do response de Add Liquidity e reutiliza-lo no body de Remove.
  - Tentativa de Remove com `lp_id` desconhecido ou ja retirado MUST retornar HTTP 404 `active liquidity position not found`.

### 6. Bridging / Unbridging (Hub-and-Spoke)
- `POST /api/v2/bridge/lock-mint`
- **Purpose**: Iniciar ou refletir o fluxo Lock&Mint para disponibilizar ativos espelhados (`W-tCeBM*`) no Hub a partir de trava em Spoke.
- **Required Inputs**: Spoke de origem, ativo nativo, montante, identificacao do proprietario, referencia do Relayer.
- **Output**: estado da posicao espelhada e referencia da prova do Relayer.
- `POST /api/v2/bridge/burn-unlock`
- **Purpose**: Iniciar ou refletir o fluxo Burn&Unlock para devolver ativos ao Spoke de destino.
- **Required Inputs**: posicao espelhada a queimar, Spoke de destino, identificacao do beneficiario.
- **Output**: estado atualizado do unbridging e referencia do Relayer.
- **Rules**:
  - Transicoes MUST refletir eventos confirmados pelo Relayer (Hyperledger Cacti), nao iniciar custodia centralizada no Hub.
  - Falhas em qualquer perna MUST registrar `FAILED` auditavel sem perda de rastreabilidade.

- `GET /api/v2/bridge/positions`
- **Purpose**: Listar posicoes espelhadas com filtro por estado, incluindo suporte direto a reconciliacao manual (FR-033).
- **Required Inputs (query)**: `state` (obrigatorio; aceita `LOCKED`, `MINTED`, `BURNED`, `UNLOCKED`, `FAILED`, `RECONCILIATION_REQUIRED`), `spoke_id` (opcional), `owner_bank_id` (opcional), `cursor`/`limit` (paginacao).
- **Output**: lista de `BridgedAssetPosition` com campos chave (`position_id`, `state`, `spoke_id`, `asset`, `amount`, `relayer_item_id`, `relayer_attempt_count`, `first_attempted_at`, `last_attempted_at`, `last_error_code`, `updated_at`) — suficientes para o operador reconciliar manualmente.
- **Rules**:
  - Quando `state=RECONCILIATION_REQUIRED`, a resposta MUST incluir `relayer_attempt_count`, `last_error_code` e `last_attempted_at` nao nulos, permitindo triagem sem consulta adicional.
  - Autorizacao MUST restringir consulta a operadores autorizados (Banco Central ou operador tecnico do Hub).
  - Endpoint e read-only: acoes corretivas seguem fluxos existentes (`/api/v2/bridge/lock-mint` / `/api/v2/bridge/burn-unlock`) e sao rastreadas via audit log.

### 7. Compliance (ZK-Pointers)
- `POST /api/v2/compliance/zk-pointer`
- **Purpose**: Registrar prova de "fit to transact" associada a uma transacao pendente.
- **Required Inputs**: identificacao do participante, referencia da transacao alvo, digest publico da prova.
- **Output**: estado de verificacao (`PENDING`, `VERIFIED`, `REJECTED`).
- `GET /api/v2/compliance/zk-pointer/{pointer_id}`
- **Purpose**: Consultar estado de verificacao de uma prova.
- **Rules**:
  - Nenhum dado sensivel de KYC/AML pode trafegar na API alem do digest publico.
  - `REJECTED` MUST bloquear execucao de swap/liquidez dependentes.

### 8. Liquidity Monitor
- `GET /api/v2/amm/liquidity/monitor/{pair}`
- **Purpose**: Expor estado consolidado de reservas, razao atual e alertas de desequilibrio.
- **Output**: reservas, razao, `imbalance_alert_state` (`NORMAL`/`WARNING`/`BREACHED`), timestamp.
- `GET /api/v2/amm/liquidity/alerts`
- **Purpose**: Listar alertas de desequilibrio gerados conforme REQ-FX-008 (threshold 70/30).
- **Output**: alertas com par, razao, estado e metadados de auditoria.

### 9. Central Bank Oversight (Master Viewing Key, 2-of-3, 72h)
Fluxo multi-sig FR-035/FR-045: quorum fixo 2-of-3 entre BCs participantes; timeout de 72h para atingir quorum; expiracao automatica apos o prazo.

- `POST /api/v2/oversight/disclosure-request`
- **Purpose**: Abrir solicitacao de disclosure. Cria `DisclosureRequest` em estado `OPEN` com `expires_at = opened_at + 72h`.
- **Required Inputs**: `target_transaction_ref`, `reason_code` (`AML_INVESTIGATION`/`CFT_INVESTIGATION`/`COURT_ORDER`), identificacao do BC solicitante.
- **Output**: `request_id`, `state=OPEN`, `expires_at`, `signatures_collected=1`, `quorum_required=2`.

- `POST /api/v2/oversight/disclosure-request/{request_id}/sign`
- **Purpose**: Adicionar assinatura a uma solicitacao aberta. Quando `signatures_collected >= 2`, transita para `QUORUM_REACHED`.
- **Required Inputs**: identificacao institucional do co-signatario, assinatura.
- **Output**: `signatures_collected`, `state`.
- **Rules**:
  - Co-signatario MUST ser distinto do solicitante e de outros signatarios (`signer_bank_id` unico por `request_id`).
  - Tentativa apos `expires_at` MUST retornar `EXPIRED` sem efeito.

- `POST /api/v2/oversight/disclosure-request/{request_id}/disclose`
- **Purpose**: Executar disclosure apos `QUORUM_REACHED`. Chama Paladin com Master Viewing Key e retorna dados sensiveis apenas em canal autorizado.
- **Rules**:
  - MUST recusar se `state != QUORUM_REACHED`.
  - Transita para `DISCLOSED` e registra em audit log.

- `GET /api/v2/oversight/disclosure-request/{request_id}`
- **Purpose**: Consultar estado, contador de assinaturas e tempo restante ate expiracao.

## Non-Compatibility Rules
- Nenhum endpoint de Cenario A permanece ativo apos corte.
- Nenhuma rota legado deve responder sob prefixos antigos apos ativacao da API v2 do Cenario B.
- Contratos de request/response do legado nao sao aceitos como fallback.

## Operational Exclusions (Decisions 16-17)
- **Sem rate limiting** nesta iteracao: nenhum middleware de throttling nos endpoints acima. Protecao depende de autenticacao Keycloak + segmentacao de rede (FR-053/54; risco aceito).
- **Sem observabilidade estruturada**: logs sao ad-hoc em stdout; SCs dependentes (SC-014/019/023) sao validados qualitativamente ate a feature de observabilidade existir (FR-051/52).

## Acceptance Rules
- Cada endpoint ativo deve possuir teste de contrato cobrindo sucesso, erro funcional e autorizacao.
- Endpoint antigo somente considerado removido quando nao estiver mais registrado no roteador e na especificacao de API ativa.
- Mudancas de circuito de risco devem gerar registro auditavel no backend novo, seguindo fluxo assimetrico (pause 1-of-N, resume 2-of-N). Tentativa de resume sem quorum MUST emitir evento de disputa auditavel e nao propagar ao Hub.
- Swaps devem ser recusados de forma auditavel quando `max_amount_in` for excedido, quando ZK-Pointers nao estiverem `VERIFIED`, quando a posicao espelhada de entrada nao estiver `MINTED` com saldo suficiente, ou quando `circuit_breaker_state != LIVE`.
- Alertas do Liquidity Monitor devem refletir de forma ponta-a-ponta o threshold 70/30 (REQ-FX-008) e expor trilha auditavel de transicoes; cadencia p95 <= 15s entre leituras (validacao qualitativa nesta feature).
- Bridging/Unbridging deve ser consistente com eventos do Relayer; endpoints backend refletem estado observado, nao criam custodia paralela no Hub. Exaustao de 5 tentativas transita posicao afetada para `RECONCILIATION_REQUIRED` e emite escalacao auditavel.
- Divulgacoes via Master Viewing Key exigem 2-of-3 assinaturas registradas antes de qualquer resposta com dados sensiveis; solicitacoes expiram automaticamente em 72h sem quorum.
- Targets de performance: `quote/exact-output` p95 <= 300ms, `swap/exact-output` p95 <= 6s, `liquidity/monitor/{pair}` cadencia p95 <= 15s (FR-037 / Decision 13). Baseline registrado em `cutover/performance-baseline.md`.
