# Feature Specification: Commercial Cross-Currency Swap

**Feature Branch**: `009-commercial-cross-currency-swap`  
**Created**: 2026-05-26  
**Status**: Draft  
**Input**: User description: "com base nas alterações realizadas de adicionar liquidez no scenário B, sugiro analisar o codigo e se possível atualizar alguma spec, preciso adicionar a funcionalidade de um banco comercial dentro do spoke-a poder trocar BRLs em ARGs ( que está registrado o lp no momento), ele poder trocar BRL e ARGs dentro da poll usando o hub internacional."

## Contexto e Motivação

Com a implementação bem-sucedida do fluxo soberano de provisão de liquidez para Bancos Centrais (specs 007/008), o pool W-BRL-ARS no Hub está ACTIVE e pronto para servir operações comerciais. Bancos comerciais no Spoke-A (Brasil) agora precisam executar trocas cross-currency de BRL por ARS usando esse pool, viabilizando pagamentos internacionais descentralizados.

O fluxo completo envolve:
1. **Bridge Spoke→Hub**: Banco comercial bloqueia BRL no Spoke-A e recebe W-BRL no Hub
2. **Swap Hub**: Troca W-BRL por W-ARS usando o pool de liquidez CB
3. **Bridge Hub→Spoke**: Queima W-ARS no Hub e desbloqueia ARS no Spoke-B

Esta especificação documenta o fluxo end-to-end de um banco comercial realizando swap cross-currency, incluindo pré-condições (pool ACTIVE, circuit breaker status), tratamento de erros (slippage, liquidez insuficiente) e validações de sucesso.

## Clarifications

### Session 2026-05-26

- Q: O endpoint `/api/v2/amm/swap/exact-output` existente (swap direto no Hub) deve coexistir, ser depreciado, ou refatorado como building block interno? → A: **Coexistir** - Bancos Centrais usam `/swap/exact-output` (direct swap no Hub após commitment), Bancos Comerciais usam `/swap/cross-currency` (orchestrador completo)
- Q: Se o bridge Spoke-A→Hub suceder mas o swap Hub falhar (ex: slippage, pool halted), o sistema deve bloquear fundos, fazer rollback automático, ou criar compensação offline? → A: **Rollback automático** - Sistema automaticamente executa bridge reverso (burn W-BRL no Hub → unlock BRL no Spoke-A) e registra tentativa em audit log (SwapRollbackLog) com máximo 3 tentativas automáticas
- Q: A validação de quote expirada (15s TTL) deve ser client-side only, server-side timestamp validation, ou stateless JWT-based? → A: **Server-side timestamp validation** - Backend salva quote em tabela `swap_quotes` com timestamp, valida expiry em `/swap/cross-currency`, retorna HTTP 422 QUOTE_EXPIRED se `NOW() > valid_until`. Background job limpa quotes antigas (>1h)
- Q: Rate limiting (10 swaps/min, 100 swaps/hora) deve usar database-backed counters, Redis, ou in-memory limiter? → A: **Database-backed counters** - Tabela `swap_rate_limit_counters` com registros por (bank_id, window_type, window_start). Background job limpa janelas expiradas (>2h). MVP suficiente, migração para Redis opcional em produção se throughput justificar
- Q: Correlation ID para vincular bridge-in + swap + bridge-out deve usar HTTP header-based, UUID in Context + Explicit Passing, ou Request ID from Gateway? → A: **UUID in Context + Explicit Passing** - Orchestrador gera UUID no início de `/swap/cross-currency`, salva em `CrossCurrencySwapOperation.correlation_id`, passa explicitamente para cada sub-operação e registra em todos os logs
- Q: O endpoint `/swap/cross-currency` deve aceitar apenas W-BRL-ARS hardcoded, descobrir pares dinamicamente via PairRegistry, ou usar config-driven? → A: **Descoberta dinâmica** - Frontend lista pares via `GET /api/v2/amm/pairs` (PairRegistry ACTIVE), usuário seleciona em dropdown, backend valida pool_pair contra PairRegistry. Quando CB-Chile propõe BRL-CLP e CB-Brasil confirma, banco comercial vê novo par automaticamente (sem redeploy)

### Session 2026-05-27

- Q: Como gateways de banco comercial obtêm `pool_status` e reservas para quote/pre-check do swap cross-currency? → A: **Proxy via BC do spoke** — quando `CENTRAL_BANK_API_URL` está configurado, o api-gateway comercial delega leitura de pool ao api-gateway do BC do mesmo spoke (`GET {CENTRAL_BANK_API_URL}/api/v2/amm/pool/{pair}/status`). O BC é a fonte canônica: lê o **Sovereign AMM no Hub** (`SOVEREIGN_AMM_ADDRESS`) e enriquece com dados locais (LP count, pending commits). Bancos comerciais **não** replicam `SOVEREIGN_AMM_ADDRESS` no próprio `.env`.
- Q: O pool soberano W-BRL-ARS é por spoke ou por Hub? → A: **Contrato no Hub, visão por spoke via BC** — liquidez e reservas on-chain vivem no Hub; cada spoke tem um BC que expõe o status. Mapeamento dev: Spoke-A (`bank-a`, `bank-c`) → `CENTRAL_BANK_API_URL` do CB-A; Spoke-B (`bank-b`, `bank-d`) → CB-B.
- Q: Qual identificador de par usar na consulta ao BC vs na quote cross-currency? → A: **Normalização soberana** — quote pode usar `W-{source}-W-{target}` (ex. `W-BRL-W-ARS`); proxy ao BC converte para o par soberano do Hub `W-{source}-{target}` (ex. `W-BRL-ARS`). Resposta HTTP mantém campo canônico `pool_status` (não `status`).
- Q: Como o gateway comercial executa swap on-chain no Hub sem replicar `SOVEREIGN_AMM_ADDRESS` no `.env`? → A: **Opção B** — CB expõe `GET /api/v2/amm/hub-liquidity-config` (`sovereign_amm_address`, tokens W-tCeBM); banco comercial resolve na subida e configura `ammClient`/`SwapService` para o mesmo contrato usado pelo BC.
- Q: Quando `BANK_CODE` configurado, `side` no payload do `approve-amm` para bancos comerciais deve ser ignorado ou aceito como fallback? → A: **Completamente ignorado** — `side` do payload descartado para callers sem `CENTRAL_BANK_ROLE`; `BANK_CODE` é a única fonte de verdade; campo `side` efetivamente depreciado para bancos comerciais.
- Q: Error code quando `BANK_CODE` não está configurado e `side` ausente no `approve-amm`? → A: **`400 COMMIT_SIDE_NOT_CONFIGURED`** — reutiliza o código já definido em `/liquidity/commit`, mesma semântica de configuração ausente.
- Q: O comportamento de CBs no `approve-amm` (auto-detecção via `CENTRAL_BANK_ROLE` + override explícito de `side` para fluxo G5-cross) deve ser preservado? → A: **Preservado sem alteração** — a mudança afeta exclusivamente callers sem `CENTRAL_BANK_ROLE`; CBs continuam com auto-detecção por `CENTRAL_BANK_ROLE` e podem informar `side` explicitamente para o padrão G5-cross.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Swap Cross-Currency BRL→ARS via Hub (Priority: P1)

Como operador de banco comercial no Spoke-A (Brasil), quero trocar BRL por ARS usando o pool de liquidez CB no Hub, para executar pagamentos internacionais para beneficiários no Spoke-B (Argentina) de forma descentralizada e transparente.

**Why this priority**: Representa o caso de uso principal do Scenario B para bancos comerciais - capacidade de swap cross-currency usando infraestrutura soberana de CB.

**Independent Test**: Um operador consegue converter 1000 BRL em ~2000 ARS (conforme taxa do pool W-BRL-ARS) via interface do banco comercial, visualizando cada etapa do fluxo (bridge, swap, bridge) e confirmando recebimento final no Spoke-B.

**Acceptance Scenarios**:

1. **Given** pool W-BRL-ARS está ACTIVE com reservas suficientes (≥100k BRL, ≥200k ARS), **When** banco comercial inicia swap de 1000 BRL por ARS com slippage tolerance 1%, **Then** sistema executa bridge Spoke-A→Hub, swap W-BRL→W-ARS, bridge Hub→Spoke-B e retorna tx_hash + amount_out dentro do limite de slippage.

2. **Given** banco comercial tem saldo de 5000 BRL no Spoke-A e W-BRL no Hub (via bridge), **When** solicita quote para 2000 ARS, **Then** sistema retorna amount_in requerido (ex: 1020 BRL incluindo fee 3%) e prazo de validade da quote (10-15s).

3. **Given** quote válida para swap W-BRL→W-ARS, **When** banco comercial aprova e executa swap dentro da janela de validade, **Then** transação on-chain é submetida no Hub AMM, evento LogSwap é emitido, e posição bridge no Spoke-B fica ACTIVE para unlock final.

4. **Given** circuit breaker do pool está HALTED por ação de governança, **When** banco comercial tenta swap, **Then** API retorna HTTP 422 CIRCUIT_BREAKER_HALTED com mensagem "Pool pausado temporariamente - aguarde retomada de governança".

5. **Given** reserva do pool está abaixo do montante solicitado (liquidez insuficiente), **When** banco comercial tenta swap de 150k BRL, **Then** API retorna HTTP 422 INSUFFICIENT_POOL_LIQUIDITY com sugestão de reduzir montante ou aguardar nova provisão de liquidez.

---

### User Story 2 - Quote e Aprovação com Slippage Protection (Priority: P1)

Como operador de banco comercial, quero obter cotação precisa antes de executar swap, com proteção contra slippage (movimento adverso de preço) e prazo de validade claro, para tomar decisões operacionais seguras e previsíveis.

**Why this priority**: Proteção contra slippage é crítica para evitar perdas em mercados voláteis e garantir conformidade com limites operacionais do banco.

**Independent Test**: Operador pode obter quote, aguardar 20s (simular demora interna), tentar executar swap e receber erro de quote expirada, então obter nova quote e executar com sucesso.

**Acceptance Scenarios**:

1. **Given** pool W-BRL-ARS com ratio 1:2 (1 BRL = 2 ARS), **When** banco solicita quote para swap de 1000 BRL, **Then** API retorna amount_out estimado (ex: 1940 ARS após fee 3%) e max_amount_in validado.

2. **Given** quote gerada há 20 segundos, **When** banco tenta executar swap, **Then** API valida timestamp da quote, detecta expiração (prazo padrão 10-15s) e retorna HTTP 422 QUOTE_EXPIRED com orientação para obter nova quote.

3. **Given** pool ratio mudou de 1:2 para 1:1.9 entre quote e execução, **When** banco executa swap com max_slippage 1%, **Then** API detecta movimento adverso >1%, retorna HTTP 422 SLIPPAGE_LIMIT_EXCEEDED e sugere atualização de quote.

4. **Given** banco definiu max_amount_in 1050 BRL para obter 2000 ARS, **When** swap requer 1080 BRL devido a fee e slippage, **Then** transação falha on-chain com revert e API retorna erro SLIPPAGE_LIMIT_EXCEEDED.

---

### User Story 3 - Monitoramento de Pool e Circuit Breaker Status (Priority: P2)

Como operador de banco comercial, quero consultar status do pool (ACTIVE/EMPTY/PENDING_COUNTERPART) e estado do circuit breaker antes de iniciar swap, para evitar tentativas de swap em pool indisponível e reduzir falhas operacionais.

**Why this priority**: Melhora UX operacional ao fornecer feedback preventivo, mas não bloqueia a capacidade core de swap (P1 já valida pool status na execução).

**Independent Test**: Operador acessa dashboard de pools, visualiza W-BRL-ARS com status ACTIVE + reserves 100k/200k + circuit breaker status OK, e recebe alerta visual quando pool transiciona para HALTED.

**Acceptance Scenarios**:

1. **Given** pool W-BRL-ARS ACTIVE no BC do spoke, **When** operador do banco comercial consulta GET /api/v2/amm/pool/W-BRL-ARS/status no gateway do banco, **Then** API retorna pool_status=ACTIVE, reserve_a, reserve_b, current_ratio, fee_rate_bps (dados obtidos via proxy ao BC do spoke, alinhados ao retorno do CB).

2. **Given** governança pausou pool via circuit breaker, **When** operador consulta pool status, **Then** campo circuit_breaker_status retorna HALTED com timestamp da pausa.

3. **Given** pool status é PENDING_COUNTERPART (apenas um CB commitou liquidez), **When** banco comercial tenta swap, **Then** pre-check de pool_status bloqueia envio e exibe mensagem "Pool aguardando commit bilateral de CBs".

---

### Edge Cases

- **Quote expirada**: Sistema valida timestamp de quote e rejeita execução após 10-15s, exigindo refresh.
- **Slippage excede limite**: Preço move adversamente entre quote e execução além do max_slippage configurado → transação reverte on-chain.
- **Pool transiciona para EMPTY**: CB remove liquidez entre quote e execução → swap falha com POOL_NOT_ACTIVE.
- **Circuit breaker ativa durante execução**: Governança pausa pool enquanto transação está em mempool → tx reverte on-chain com erro POOL_HALTED.
- **Bridge Spoke-A falha**: Lock de BRL no Spoke-A falha por saldo insuficiente ou approve não concedido → rollback sem mint no Hub.
- **`BANK_CODE` ausente no gateway comercial**: Gateway de banco comercial sem `BANK_CODE` configurado ao chamar `approve-amm` → retorna `400 COMMIT_SIDE_NOT_CONFIGURED`; operador deve verificar variável de ambiente do gateway.
- **Bridge Hub→Spoke-B timeout**: Burn no Hub sucede mas unlock no Spoke-B falha por Relayer offline → posição bridge fica stuck em BURNING, requer intervenção manual.
- **BC do spoke indisponível**: Gateway comercial com `CENTRAL_BANK_API_URL` configurado retorna erro ao consultar pool status ou quote (HTTP 502/422 `POOL_STATUS_UNAVAILABLE`) se o api-gateway do BC não responder — operador deve aguardar recuperação do CB, não há fallback para AMM legado local.
- **Múltiplos swaps simultâneos**: Pool reserves são atualizados atômicamente por contrato AMM, garantindo que swaps concorrentes não violem invariante k.
- **Montante mínimo/máximo**: Swap de montantes muito pequenos (<0.01 BRL) ou muito grandes (>50% da reserve) pode falhar por limites on-chain ou gas insuficiente.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE expor endpoint `POST /api/v2/amm/swap/cross-currency` que aceita `source_currency` (ex: BRL), `target_currency` (ex: ARS), `pool_pair` (ex: W-BRL-ARS, selecionado dinamicamente pelo usuário via lista de pares ACTIVE do PairRegistry), `amount_out` (montante desejado em target), `max_amount_in` (limite de slippage), `payer_bank_id`, `beneficiary_bank_id` e executa o fluxo completo: bridge Spoke→Hub, swap W-tokens, bridge Hub→Spoke.

- **FR-002**: O sistema DEVE validar pré-condições antes de executar swap: (a) `pool_pair` existe no PairRegistry com status=ACTIVE (query via `PairService.ListActivePairs`), (b) pool com `pool_status=ACTIVE` — no gateway do **BC** via leitura on-chain do Sovereign AMM no Hub; no gateway do **banco comercial** via proxy HTTP ao BC do spoke (`CENTRAL_BANK_API_URL`), (c) circuit breaker não em estado HALTED, (d) reserves suficientes para amount_out solicitado (reservas obtidas pela mesma fonte que (b) no gateway emissor da requisição), (e) se `quote_id` fornecido, quote existe na tabela `swap_quotes` e `NOW() ≤ valid_until` (validação server-side obrigatória para prevenir replay attacks).

- **FR-012**: Gateways de banco comercial com `CENTRAL_BANK_API_URL` configurado DEVE expor `GET /api/v2/amm/pool/{pair}/status` e DEVE usar o mesmo cliente HTTP (`CentralBankPoolClient`) para pré-check de swap e geração de quote cross-currency (`GET /api/v2/amm/quote/cross-currency`), delegando ao BC do spoke. Na inicialização, DEVE chamar `GET {CENTRAL_BANK_API_URL}/api/v2/amm/hub-liquidity-config` e configurar o cliente AMM de execução de swap (`SwapService`) com `sovereign_amm_address` e tokens soberanos retornados pelo BC. Gateways de BC DEVEM expor `hub-liquidity-config` quando `SOVEREIGN_AMM_ADDRESS` está configurado. Falha de comunicação com o BC DEVE retornar erro acionável, sem fallback silencioso para `AMM_CONTRACT_ADDRESS` legado.

- **FR-003**: O sistema DEVE calcular amount_in requerido usando fórmula `x·y=k` do AMM com fee (3% padrão): `amount_in = (reserve_in · amount_out) / ((reserve_out - amount_out) · (1 - fee_bps/10000))` e validar que `amount_in ≤ max_amount_in` (slippage protection).

- **FR-004**: O endpoint de quote `GET /api/v2/amm/quote/cross-currency` (e `GET /api/v2/amm/quote/exact-output` onde aplicável) DEVE persistir quote na tabela `swap_quotes` com `created_at` (server UTC timestamp), calcular `valid_until` (created_at + 15s), e retornar `quote_id`, `amount_in`, `amount_out`, `effective_rate`, `fee_bps`, `created_at`, `valid_until`, `time_remaining_seconds` (computed: valid_until - now). Em gateways comerciais, `amount_in` e snapshots de reserva DEVEM refletir dados do BC do spoke (proxy), não do AMM legado local.

- **FR-005**: O sistema DEVE mapear erros de swap para códigos HTTP + error_code consistentes: `POOL_NOT_ACTIVE` (422), `CIRCUIT_BREAKER_HALTED` (422), `SLIPPAGE_LIMIT_EXCEEDED` (422), `INSUFFICIENT_POOL_LIQUIDITY` (422), `QUOTE_EXPIRED` (422), cada um com mensagem acionável em UX.

- **FR-006**: O sistema DEVE registrar evento de swap com campos: `swap_id` (UUID único), `correlation_id` (UUID vinculando bridge-in/swap/bridge-out), `tx_hash` (transação Hub AMM), `payer_bank_id`, `beneficiary_bank_id`, `amount_in`, `amount_out`, `pool_pair`, `timestamp`, `bridge_position_id_in` (Spoke-A), `bridge_position_id_out` (Spoke-B), `status` (COMPLETED/FAILED), `failure_reason` (se aplicável), permitindo auditoria end-to-end via query por correlation_id.

- **FR-007**: O frontend de banco comercial DEVE listar pares disponíveis via `GET /api/v2/amm/pairs` (retorna apenas pares ACTIVE do PairRegistry), exibir dropdown de seleção (ex: "BRL-ARS", "BRL-CLP"), permitir usuário escolher par antes de obter quote, e implementar fluxo de 3 etapas com feedback visual: (1) Quote & Approve (seleção de par + refresh countdown 15s), (2) Bridge & Swap (polling de tx_hash até confirmação on-chain, timeout 60s), (3) Unlock Final (polling de bridge_state=UNLOCKED no Spoke-B, timeout 120s).

- **FR-008**: O sistema DEVE aplicar rate limiting por banco comercial usando tabela `swap_rate_limit_counters` (database-backed counters): máximo 10 swaps/minuto e 100 swaps/hora por `payer_bank_id`, incrementando contadores a cada swap iniciado e validando limites antes de aceitar requisição. Retornar HTTP 429 TOO_MANY_REQUESTS com header `Retry-After` quando limite excedido. Background job limpa janelas expiradas (window_start < NOW() - INTERVAL '2 hours').

- **FR-009**: O sistema DEVE expor endpoint `GET /api/v2/amm/pool/{pair}/circuit-breaker-status` que retorna `status` (OK/HALTED), `halted_by` (bank_id de quem pausou), `halted_at` (timestamp), `reason` (mensagem de governança), permitindo UI mostrar alerta preventivo.

- **FR-010**: O sistema DEVE suportar rollback parcial automático: se bridge Spoke-A→Hub suceder mas swap falhar (ex: slippage, circuit breaker halted), deve automaticamente executar bridge reverso Hub→Spoke-A (burn W-BRL → unlock BRL), registrar tentativa em `SwapRollbackLog`, e realizar até 3 tentativas automáticas com backoff exponencial (5s, 15s, 45s) antes de escalar para intervenção manual.

- **FR-011**: O sistema DEVE executar background job (cron ou scheduler) que limpa quotes expiradas da tabela `swap_quotes` a cada hora, deletando registros com `created_at < NOW() - INTERVAL '1 hour'` para evitar acúmulo de dados históricos desnecessários.

- **FR-013**: O endpoint `POST /api/v2/amm/token/approve-amm` DEVE derivar `side` automaticamente da variável de configuração `BANK_CODE` para callers sem `CENTRAL_BANK_ROLE` (bancos comerciais), alinhando-se ao padrão já adotado em `POST /api/v2/amm/liquidity/commit`. O campo `side` no payload DEVE ser completamente ignorado para esses callers (não aceito como fallback). Quando `BANK_CODE` não estiver configurado no gateway, retornar `HTTP 400` com `code: "COMMIT_SIDE_NOT_CONFIGURED"`. O comportamento de CBs (callers com `CENTRAL_BANK_ROLE`) DEVE ser preservado sem alteração: auto-detecção via `CENTRAL_BANK_ROLE` em startup e override explícito de `side` no payload permitido para o padrão G5-cross (CB-A aprovando TOKEN_B recebido de CB-B).

### Non-Functional Requirements

- **NFR-001**: Latência end-to-end (quote → unlock final no Spoke-B) DEVE ser ≤90s em p95, considerando 3 operações on-chain + 2 ciclos de Relayer (Spoke-A→Hub e Hub→Spoke-B).

- **NFR-002**: O sistema DEVE manter disponibilidade de swap ≥99.5% durante horário comercial (08:00-18:00 BRT), exceto durante manutenção programada ou pause de governança.

- **NFR-003**: Mensagens de erro DEVEM ser compreensíveis para operadores não-técnicos, incluindo ação recomendada explícita (ex: "Liquidez insuficiente - reduza montante para até 50k BRL ou aguarde nova provisão CB").

- **NFR-004**: O frontend DEVE implementar retry automático com backoff exponencial para erros transientes (network timeout, Relayer busy) mas NÃO para erros definitivos (SLIPPAGE_LIMIT_EXCEEDED, INSUFFICIENT_LIQUIDITY).

- **NFR-005**: Logs de swap DEVEM incluir `correlation_id` único (UUID gerado pelo orchestrador no início de cada operação) vinculando as 3 sub-operações (bridge in, swap, bridge out). O `correlation_id` DEVE ser salvo em `CrossCurrencySwapOperation.correlation_id`, passado explicitamente para cada serviço (BridgeLockMintService, SwapService, BridgeBurnUnlockService) e registrado em todos os log statements (formato: `[correlation_id=<uuid>]`) para facilitar troubleshooting de falhas parciais via grep/query.

### Key Entities

- **CrossCurrencySwap**: Agregado representando operação end-to-end com estados `QUOTING`, `BRIDGE_IN_PROGRESS`, `SWAP_IN_PROGRESS`, `BRIDGE_OUT_PROGRESS`, `COMPLETED`, `FAILED`. Atributos: `swap_id`, `payer_bank_id`, `beneficiary_bank_id`, `source_currency`, `target_currency`, `amount_in`, `amount_out`, `pool_pair`, `tx_hash_swap`, `bridge_position_in`, `bridge_position_out`, `created_at`, `completed_at`, `failure_reason`.

- **SwapQuote**: Entidade de quote com validade temporal. Atributos: `quote_id`, `pool_pair`, `amount_out` (desejado), `amount_in` (requerido), `effective_rate`, `fee_bps`, `max_slippage_pct`, `timestamp`, `valid_until`, `expired` (computed).

- **PoolStatus**: Snapshot de estado do pool. Atributos: `pool_pair`, `pool_status` (ACTIVE/EMPTY/PENDING_COUNTERPART), `reserve_a`, `reserve_b`, `current_ratio`, `total_lp_count`, `circuit_breaker_status` (OK/HALTED), `fee_rate_bps`, `updated_at`. **Fonte de verdade on-chain**: Sovereign AMM no Hub (via gateway do BC). **Fonte para banco comercial**: mesma resposta, obtida por proxy ao BC do spoke.

- **CircuitBreakerEvent**: Evento de pause/resume de pool. Atributos: `event_id`, `pool_pair`, `action` (HALT/RESUME), `triggered_by` (bank_id de CB), `timestamp`, `reason` (descrição de governança).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% dos swaps cross-currency bem-sucedidos completam o ciclo completo (bridge in, swap Hub, bridge out) e são registrados com tx_hash verificável on-chain nos 3 contratos (SpokeBridge-A, SovereignAMM, SpokeBridge-B).

- **SC-002**: Operadores de banco comercial completam swap de 1000 BRL→ARS em tempo médio ≤60s (p50) e ≤90s (p95), medido via tryout `tryout-commercial-swap-e2e.sh` com pool pré-provisionado.

- **SC-003**: Taxa de falha de swap por slippage (SLIPPAGE_LIMIT_EXCEEDED) é ≤5% em ambiente de teste com volatilidade simulada (ratio mudando ±2% durante quote validity window).

- **SC-004**: 100% dos erros de swap exibem mensagem user-friendly no frontend com ação recomendada, validado via checklist manual de error scenarios (pool not active, circuit breaker halted, insufficient liquidity, quote expired).

- **SC-005**: Em validação manual, operador consegue obter quote, aguardar expiração (>15s), receber erro QUOTE_EXPIRED, obter nova quote e executar swap com sucesso — ciclo completo documentado em `specs/009-commercial-cross-currency-swap/quickstart.md`.

- **SC-006**: Latência p95 de cada etapa medida via tryout: bridge Spoke-A→Hub ≤30s, swap Hub ≤10s (on-chain confirmation), bridge Hub→Spoke-B ≤30s, total ≤90s.

## Assumptions

- Pools de liquidez no Hub (ex: W-BRL-ARS, W-BRL-CLP) já estão ACTIVE com liquidez provisionada pelos CBs (specs 007/008) antes de bancos comerciais iniciarem swaps. Bancos comerciais **consultam** esse estado via api-gateway do BC do seu spoke (`CENTRAL_BANK_API_URL`), não via leitura direta do contrato no gateway comercial. Sistema suporta **descoberta dinâmica de pares** via PairRegistry on-chain — quando CB de novo país (ex: Chile) propõe par e CB contraparte confirma, o par fica disponível automaticamente para bancos comerciais (sem redeploy).
- Cada banco comercial já possui `CENTRAL_BANK_API_URL` apontando para o BC do spoke (ex.: `bank-a` → `api-gateway-central-bank-a`; `bank-b`/`bank-d` → `api-gateway-central-bank-b`), seguindo o mesmo padrão de proxy já usado em onboarding e pagamentos.
- A infraestrutura de bridge (specs 007/008) e Cacti Relayer já estão operacionais e validados em tryouts CB.
- O endpoint POST /api/v2/amm/swap/exact-output existente será **mantido** e usado por Bancos Centrais para swaps diretos no Hub (quando W-tokens já estão presentes via liquidity commit). O novo endpoint POST /api/v2/amm/swap/cross-currency é **adicional** e específico para bancos comerciais que precisam do fluxo completo bridge→swap→bridge.
- Banco comercial já possui saldo de BRL nativo no Spoke-A (via mint CB ou depósito prévio) e não precisa adquirir BRL como parte desta feature.
- O beneficiário no Spoke-B já possui wallet registrado no IdentityRegistry do Spoke-B e pode receber ARS (onboarding de beneficiário é out-of-scope).
- A governança CB já configurou fee_rate_bps padrão (3%) no contrato SovereignAMM e esse valor é imutável durante swaps (mudança de fee requer pausa de pool).
- Circuit breaker é controlado exclusivamente por CBs via endpoints de governança (banco comercial não pode pausar/resumir pool).
- O sistema usa exact-output semantics: banco comercial especifica amount_out desejado e sistema calcula amount_in requerido (inverso de exact-input também está disponível mas não é escopo primário desta spec).
- Rate limiting usa database-backed counters (tabela `swap_rate_limit_counters`) para MVP. Migração para Redis em produção é opcional se throughput justificar complexidade adicional (ver research.md Q4).

## Out of Scope

- Implementação de swap exact-input (usuário especifica amount_in e recebe amount_out variável) — exact-output é suficiente para MVP.
- Suporte a múltiplos hops (ex: BRL→USD→ARS via 2 pools) — apenas swaps diretos via pool único (um par por operação).
- Integração com oráculos externos de preço (ex: Chainlink) para validação de fair rate — ratio é determinado puramente por reserves on-chain (x·y=k).
- Limite por usuário/beneficiário de quantidade de swaps ou montante total diário — rate limiting é apenas por banco comercial (payer_bank_id).
- Reversão de swap completado (undo) — uma vez unlock final no Spoke-B suceder, operação é irreversível (beneficiário deve iniciar swap reverso se desejar).
- Dashboard de analytics de swap para CBs (volume total, fees acumulados, etc) — apenas logs de auditoria individuais são registrados.
- Otimização de gas via batching de múltiplos swaps em uma transação — cada swap é uma transação independente.
- Suporte a stable-swap curve (algoritmo otimizado para stablecoins) — usa constant-product AMM padrão (x·y=k) mesmo que BRL e ARS sejam fiat.
- Propagação de `SOVEREIGN_AMM_ADDRESS` para `.env` de bancos comerciais — fora de escopo; comerciais resolvem via `hub-liquidity-config` do BC (FR-012 / T054).
