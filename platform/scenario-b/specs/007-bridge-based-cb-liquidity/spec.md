# Feature Specification: Provisão de Liquidez CB via Bridge (Fluxo Soberano)

**Feature Branch**: `007-bridge-based-cb-liquidity`
**Created**: 2026-05-20
**Status**: Draft
**Depende de**: `005-cooperative-liquidity`, `002-scenario-b-liquidity`
**Substitui (parcialmente)**: padrão G5-cross depreciado de `005-cooperative-liquidity`

## Contexto e Motivação

A feature `005-cooperative-liquidity` introduziu o mecanismo de commit-reveal cooperativo para provisão de liquidez, mas implementou um atalho arquitetural denominado **G5-cross** que viola a soberania monetária:

- CB-B minta TOKEN_B diretamente para o endereço signer de CB-A
- CB-A's signer detém, aprova e deposita tokens emitidos por CB-B
- A execução on-chain de `addSingleSidedLiquidity(TOKEN_B)` é feita pelo signer de CB-A

Isso significa que **a moeda soberana de CB-B é custodiada e movimentada on-chain pela chave privada de CB-A** — violação direta do princípio de soberania monetária.

O fluxo correto para a provisão de liquidez transfronteiriça no hub internacional já existe na arquitetura do sistema — é o mesmo mecanismo de **Lock & Mint Bridge** usado pelos bancos comerciais para swaps no Cenário B:

1. Cada Banco Central **emite sua moeda no seu próprio spoke** (onde tem `CENTRAL_BANK_ROLE`)
2. O Banco Central **bloqueia tokens no Bridge do spoke** → o Bridge Relayer confirma no Hub
3. O Hub minta **tokens espelhados (W-tCeBM)** para o signer do próprio CB no Hub
4. O Banco Central deposita seus **W-tCeBM** no AMM do Hub usando seu **próprio signer**

Nenhum CB emite tokens para carteira de outro CB em nenhuma etapa. O `msg.sender` on-chain de cada depósito é **sempre o signer soberano do CB emissor daquela moeda**.

---

## Clarifications

### Session 2026-05-20

- Q: O mecanismo de Bridge Lock&Mint que os bancos comerciais usam para swaps (spec-002) é o mesmo que os CBs usarão para provisão de liquidez? → A: Sim — o mesmo contrato `SpokeBridge`, o mesmo Relayer Cacti, o mesmo endpoint `POST /api/v2/bridge/lock-mint` e o mesmo fluxo de confirmação assíncrona (polling `GET /api/v2/bridge/positions` até `bridge_state = ACTIVE`). A diferença é que o chamador autenticado é um CB (com `CENTRAL_BANK_ROLE` no spoke) em vez de um banco comercial.

- Q: Os tokens W-tCeBM recebidos no Hub após o bridge são os mesmos usados no AMM (TOKEN_A/TOKEN_B)? Ou são contratos distintos? → A: **São contratos distintos (confirmado na sessão de clarificação de 2026-05-20).** `HUB_TOKEN_A_ADDRESS`/`HUB_TOKEN_B_ADDRESS` são tokens mintados diretamente pelos CBs via `CENTRAL_BANK_ROLE` no Hub sem bridge — representam o mecanismo G5-cross ao nível de hub e devem ser abandonados como par canônico do AMM. Os W-tCeBM mintados pelo Bridge Relayer após lock no spoke são contratos distintos e são os tokens que o AMM do fluxo soberano DEVE usar. O AMM soberano é criado via `PairRegistry.proposePair(W-tCeBM_BRL_addr, W-tCeBM_ARS_addr, newAMM_addr)` — um par diferente do par atual. Os endereços concretos dos W-tCeBM devem ser extraídos de `SpokeBridge.sol` e do deploy script durante a implementação (task obrigatória de pesquisa no `plan.md`).

- Q: O commit-reveal cooperativo de spec-005 ainda é necessário no fluxo soberano? → A: Sim — o mecanismo de commit-reveal coordena a intenção de depósito bilateral sem exigir que um CB aguarde o outro indefinidamente. O que muda é a **execução após o match**: em vez de G5-cross (CB-A executa os dois lados), cada CB executa sua própria perna (CB-A deposita W-BRL usando signer de CB-A; CB-B deposita W-ARS usando signer de CB-B). O protocolo de matching precisa ser cross-gateway (ver FR-002 de spec-005 atualizado).

- Q: O que é o protocolo de matching e como ele funciona? → A: **Coordenação on-chain via contrato `LiquidityCommitRegistry` no Hub (confirmado em clarificação 2026-05-20).** Cada gateway de CB submete a intenção de depósito como tx on-chain no contrato; o contrato detecta o match bilateral e emite `CommitMatched(pool_pair, commit_id_a, signer_a, amount_a, commit_id_b, signer_b, amount_b)`; cada gateway escuta esse evento via watcher e executa `addSingleSidedLiquidity` de forma autônoma usando o signer local. Nenhuma comunicação direta entre gateways é necessária. A abordagem é trustless e escala para N CBs futuros sem reconfig de protocolo.

- Q: O campo `provider_id` no DB e nos eventos on-chain precisa mudar para suportar o fluxo soberano? → A: Não — `provider_id` continua como metadado de autoridade de LP. A diferença é que no fluxo soberano o `msg.sender` on-chain do depósito COINCIDE com o CB emissor, eliminando a divergência entre metadado e realidade on-chain presente no G5-cross.

- Q: Qual o impacto no endpoint `POST /api/v2/amm/token/mint-and-approve` com `recipient`? → A: O endpoint com `recipient` deve passar a rejeitar (HTTP 403) qualquer `recipient` que seja o signer de um Banco Central registrado no `IdentityRegistry` — implementando a proteção anti-G5-cross em FR-018 de spec-005. A validação usa `IdentityRegistry.getCentralBankOf(tokenAddress)` para detectar signers CB. Para recipientes que são bancos comerciais, o endpoint continua funcionando normalmente.

- Q: Qual o impacto no frontend wizard de provisão cooperativa (`CooperativeLiquidityWizard.tsx`)? → A: O wizard precisa ser redesenhado para não expor o passo G5-cross (Step G5.1 e G5.2). No fluxo soberano, o CB-A só precisa: (1) chamar bridge lock-mint para seu spoke, (2) aguardar confirmação do Relayer (W-BRL no Hub), (3) submeter commit ao gateway. O CB-B faz o mesmo de forma independente. O wizard de CB-A não exibe nem executa nenhuma ação relacionada ao TOKEN_B de CB-B. Esta mudança é escopo de `008-frontend-sovereign-liquidity`.

### Session 2026-05-20 — Clarificação Formal

- Q: Os tokens W-tCeBM recebidos no Hub após o bridge são os mesmos endereços de `HUB_TOKEN_A_ADDRESS`/`HUB_TOKEN_B_ADDRESS` usados pelo AMM atual, ou são contratos distintos? → A: **Contratos distintos.** `HUB_TOKEN_A/B` são mintados diretamente pelos CBs via `CENTRAL_BANK_ROLE` no Hub (mecanismo G5-cross ao nível de hub). Os W-tCeBM são mintados pelo Bridge Relayer como contrapartida do lock no spoke — contratos separados. O AMM soberano usa W-tCeBM via novo par registrado no `PairRegistry`. O par antigo (`HUB_TOKEN_A × HUB_TOKEN_B`) é descontinuado para novos depósitos; posições existentes permanecem removíveis. Os endereços concretos dos W-tCeBM são extraídos de `SpokeBridge.sol` e do deploy script na fase de planejamento.
- Q: O mecanismo de coordenação de match entre gateways de CBs distintos deve ser on-chain (contrato no Hub) ou off-chain (gRPC/REST/fila entre gateways)? → A: **On-chain via contrato `LiquidityCommitRegistry` no Hub internacional.** O Hub é o órgão de coordenação neutro do consórcio — trustless, sem autenticação inter-gateway, escalável para N CBs futuros sem alteração de protocolo. Cada gateway registra o commit via tx on-chain; o contrato emite `CommitMatched`; cada gateway ouve o evento e executa sua própria perna de forma autônoma.
- Q: O que acontece se o CB submeter `POST /api/v2/amm/liquidity/commit` antes de o Bridge Relayer confirmar a posição no Hub (`bridge_state` ainda em `LOCKING`)? → A: **O gateway rejeita o commit com HTTP 422** antes de qualquer persistência ou tx on-chain. O handler DEVE verificar que existe uma `BridgedAssetPosition` com `bridge_state = ACTIVE` e `w_token_address` correspondente ao `side` do commit para o CB autenticado. Error body: `{ "error": "no active bridge position found for this side — wait for Relayer confirmation", "code": "BRIDGE_POSITION_NOT_ACTIVE" }`. O CB deve aguardar `bridge_state = ACTIVE` via polling em `GET /api/v2/bridge/positions` antes de submeter o commit.
- Q: O bridge (lock-mint no spoke) é obrigatório para depósitos adicionais em pool já ACTIVE, ou o CB pode depositar W-tCeBM já disponível no Hub sem novo lock? → A: **Bridge é opcional para depósitos adicionais.** Se o CB já possui saldo de W-tCeBM no Hub (de bridge anterior), pode chamar `addSingleSidedLiquidity` diretamente sem lock-mint. O commit-reveal via `LiquidityCommitRegistry` também não é necessário em depósitos adicionais em pool ACTIVE (herdado de spec-005 FR-001). O bridge é necessário apenas se o CB não tiver saldo W-tCeBM disponível. Soberania é garantida porque W-tCeBM só chegam ao Hub via bridge do spoke soberano do CB emissor.
- Q: Como o gateway deve ser configurado para participar de múltiplos pares soberanos simultaneamente (ex: CB-A em W-BRL-ARS e W-BRL-CLP)? → A: **Via mapa `SOVEREIGN_PAIR_AMM_MAP` (JSON)** mapeando `pool_pair` string → endereço AMM (ex: `{"W-BRL-ARS":"0x...","W-BRL-CLP":"0x..."}`). O handler `/internal/amm/execute-matched-commit` faz lookup por `event.pool_pair`. A env var `SOVEREIGN_AMM_ADDRESS` (única) é removida; `SOVEREIGN_POOL_PAIR_ID` (única) é substituída pela lista `SOVEREIGN_PAIR_IDS`. *Remediação de finding C1 — 2026-05-20.*

- Q: O script de deploy do par soberano deve ser reutilizável para novos CBs sem reescrita? → A: **Sim — `SeedNewSovereignPair.s.sol` parametrizado via env vars** (`PAIR_ID`, `TOKEN_SYMBOL_A`, `TOKEN_SYMBOL_B`, `CB_A_HUB_KEY`, `CB_B_HUB_KEY`, `RELAYER_ADDR`). Um único script reutilizável cobre o onboarding de qualquer novo par de CBs. *Remediação de finding C2 — 2026-05-20.*

- Q: O tryout E2E de validação do fluxo soberano deve ser um novo script dedicado ou uma extensão do `tryout-scenario-b-e2e.sh` existente? → A: **Novo script dedicado `tryouts/tryout-sovereign-cb-liquidity.sh`.** O `tryout-scenario-b-e2e.sh` valida o fluxo de swaps do Cenário B (US1/US2 de spec-002); misturar o fluxo soberano de LP cria um script de 800+ linhas difícil de manter e de executar em CI de forma isolada. O script dedicado é executável standalone, valida exatamente os SCs desta feature (incluindo verificação de `msg.sender` on-chain via `cast`) e não quebra o tryout existente durante o desenvolvimento.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — CB Injeta Liquidez Soberana via Bridge (Priority: P0)

O Banco Central A (emissor de BRL) quer adicionar BRL ao pool BRL-ARS no hub internacional, usando exclusivamente sua própria chave privada (signer soberano) em todas as etapas, sem nenhuma dependência de tokens emitidos por CB-B ou execução delegada ao gateway de CB-B.

**Por que esta prioridade**: É o requisito central desta feature — substituir o padrão G5-cross pelo fluxo soberano. Sem este user story, a plataforma não atende ao requisito de soberania monetária real em operações transfronteiriças.

**Independent Test**: Pode ser testado em isolamento com um único CB: CB-A emite BRL no Spoke-A, bloqueia no Bridge, recebe W-BRL no Hub, adiciona W-BRL ao pool. O pool terá `reserve_a > 0, reserve_b = 0` após o depósito unilateral de CB-A — exatamente igual ao User Story 1 de spec-005, mas com o signer de CB-A como `msg.sender` on-chain em TODAS as etapas.

**Acceptance Scenarios**:

1. **Given** CB-A possui BRL no Spoke-A (emitido via `CENTRAL_BANK_ROLE` no spoke), **When** CB-A chama `POST /api/v2/bridge/lock-mint` com `amount: "100000"` e aguarda `bridge_state = ACTIVE` via polling, **Then** W-BRL aparece no balanço do signer de CB-A no Hub e `BridgedAssetPosition.bridge_state = ACTIVE`.

2. **Given** CB-A tem W-BRL no Hub (bridge_state ACTIVE), **When** CB-A submete commit `POST /api/v2/amm/liquidity/commit` com `side: "A", amount: "100000"`, **Then** o commit é registrado com `status: PENDING` e nenhum fundo é transferido neste momento.

3. **Given** CB-A tem commit PENDING no `LiquidityCommitRegistry` on-chain e CB-B submete commit `side: "B"` no seu próprio gateway, **When** o `LiquidityCommitRegistry` detecta o match bilateral e emite `CommitMatched`, **Then** o event watcher de cada gateway processa o evento de forma autônoma: CB-A's gateway executa `addSingleSidedLiquidity(W-BRL)` com signer de CB-A; CB-B's gateway executa `addSingleSidedLiquidity(W-ARS)` com signer de CB-B. Nenhuma comunicação entre gateways é necessária. O `msg.sender` on-chain de cada depósito é exclusivamente o CB emissor daquela moeda.

4. **Given** ambos os depósitos on-chain confirmados, **When** qualquer CB consulta `GET /api/v2/amm/pool/{pair}/status`, **Then** `pool_status: ACTIVE`, `reserve_a > 0`, `reserve_b > 0`, e os LPs de ambos os CBs aparecem com `provider_id` correto e `shares_percentage` proporcional.

---

### User Story 2 — CB Remove Liquidez com Unlock no Spoke (Priority: P1)

O Banco Central A quer remover sua participação de liquidez do pool e reaver os tokens W-BRL no Hub, com a opção de fazer o unbridging (burn W-BRL no Hub → unlock BRL no Spoke-A).

**Why this priority**: A remoção de liquidez é a operação simétrica ao depósito. Se o CB pode bloquear e depositar, deve poder remover e desbloquear — fechando o ciclo de custódia soberana.

**Acceptance Scenarios**:

1. **Given** CB-A tem LP position ACTIVE no pool, **When** CB-A chama `DELETE /api/v2/amm/liquidity/{position_id}`, **Then** `removeLiquidity` é executado on-chain pelo signer de CB-A, W-BRL é devolvido ao signer de CB-A no Hub, e a `LiquidityPosition` transiciona para `WITHDRAWN`.

2. **Given** CB-A recebeu W-BRL de volta no Hub após remoção, **When** CB-A chama `POST /api/v2/bridge/burn-unlock` para os W-BRL, **Then** o burn é confirmado no Hub, o Relayer libera BRL no Spoke-A, e `BridgedAssetPosition.bridge_state = BURNED → UNLOCKED`.

---

### User Story 3 — Endpoint Rejeita G5-cross (Anti-Pattern Blocker) (Priority: P1)

O gateway passa a rejeitar qualquer tentativa de usar `mint-and-approve` com `recipient` que seja o signer de outro Banco Central, impedindo a recriação acidental do padrão G5-cross.

**Acceptance Scenarios**:

1. **Given** CB-B tenta chamar `POST /api/v2/amm/token/mint-and-approve` com `recipient: "<CB-A-signer-addr>"`, **When** o gateway valida o recipient via `IdentityRegistry`, **Then** retorna HTTP 403 `{ "error": "recipient is a Central Bank signer — cross-CB minting is prohibited" }`.

2. **Given** CB-B chama `mint-and-approve` com `recipient: "<commercial-bank-addr>"` (não é CB), **When** o gateway valida, **Then** o mint é executado normalmente (backward-compat para bancos comerciais).

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE suportar o fluxo de provísão de liquidez soberano onde cada Banco Central usa o mesmo endpoint `POST /api/v2/bridge/lock-mint` (já existente para bancos comerciais — spec-002) para bloquear tokens nativos no seu spoke e receber tokens espelhados (W-tCeBM) no Hub. O CB autenticado deve ter `CENTRAL_BANK_ROLE` no spoke; o Relayer confirma o bridge da mesma forma que para bancos comerciais. Polling via `GET /api/v2/bridge/positions` com timeout 120s / intervalo 5s é obrigatório antes de prosseguir. **Gate de validação obrigatório**: o endpoint `POST /api/v2/amm/liquidity/commit` DEVE rejeitar (HTTP 422, código `BRIDGE_POSITION_NOT_ACTIVE`) qualquer commit cujo CB autenticado não possua `BridgedAssetPosition` com `bridge_state = ACTIVE` e `w_token_address` correspondente ao `side` solicitado. Nenhum commit é persistido no DB local nem submetido ao `LiquidityCommitRegistry` on-chain antes desta validação passar.

- **FR-002**: `HUB_TOKEN_A_ADDRESS`/`HUB_TOKEN_B_ADDRESS` são contratos **distintos** dos W-tCeBM mintados pelo Bridge Relayer (confirmado em clarificação 2026-05-20). O sistema DEVE criar um novo par AMM soberano via `PairRegistry.proposePair(W-tCeBM_BRL_addr, W-tCeBM_ARS_addr, newAMM_addr)` onde os endereços dos W-tCeBM são os contratos que o Bridge Relayer minta no Hub após lock no spoke. O deploy script (`SeedHub.s.sol`) e `SpokeBridge.sol` DEVEM ser inspecionados durante a fase de planejamento para identificar os endereços corretos — isso é task obrigatória de pesquisa bloqueante de implementação. O par antigo (`HUB_TOKEN_A` × `HUB_TOKEN_B`) é descontinuado para novos depósitos de liquidez; os LP positions existentes nesse par permanecem removíveis conforme spec-005 SC-005. Não é aceitável ter dois contratos representando a mesma moeda soberana no Hub como tokens distintos de AMM. **O script de deploy do par soberano (`SeedNewSovereignPair.s.sol`) DEVE ser parametrizado via env vars** (`PAIR_ID`, `TOKEN_SYMBOL_A`, `TOKEN_SYMBOL_B`, `CB_A_HUB_KEY`, `CB_B_HUB_KEY`, `RELAYER_ADDR`) para ser reutilizável sem modificação de código para qualquer par futuro (CB-C, CB-D, etc.).

- **FR-003**: O sistema DEVE implementar um contrato Solidity `LiquidityCommitRegistry` deployado no Hub internacional como mecanismo de coordenação de match on-chain (confirmado em clarificação 2026-05-20). O contrato DEVE: (1) receber registros de intenção de depósito de qualquer CB via `registerCommit(pool_pair, side, amount, w_token_address, cb_signer)` — tx assinada pelo gateway do CB; (2) detectar automaticamente quando ambos os lados de um par estão registrados e emitir o evento `CommitMatched(pool_pair, commit_id_a, signer_a, amount_a, commit_id_b, signer_b, amount_b)`; (3) rejeitar commits duplicados para o mesmo `(pool_pair, side)` com status PENDING; (4) expirar commits após 72h sem contrapartida (herdado de spec-005 FR-013). Cada gateway de CB DEVE manter um **event watcher** para o evento `CommitMatched` no Hub e, ao receber o evento, executar `addSingleSidedLiquidity` usando o signer local soberano para o `side` correspondente ao seu CB. Nenhuma comunicação direta entre gateways é necessária — o Hub é o órgão de coordenação neutro. **O contrato `LiquidityCommitRegistry.sol` não requer alteração para N CBs futuros** (protocolo extensível sem mudança de contrato); entretanto, cada novo par de CBs exige infraestrutura de deploy proporcional: O(N*(N-1)/2) pares para N CBs — novo W-tCeBM, novo AMM, bilateral `proposePair`+`confirmPair` por par. Para suportar que um CB participe simultaneamente de N pares soberanos **o gateway DEVE ser configurado com um mapa `SOVEREIGN_PAIR_AMM_MAP`** (chave: `pool_pair` string, valor: endereço do AMM correspondente) em vez de env vars únicas `SOVEREIGN_AMM_ADDRESS` / `SOVEREIGN_POOL_PAIR_ID`. O handler de `/internal/amm/execute-matched-commit` DEVE fazer lookup de `ammAddress = SOVEREIGN_PAIR_AMM_MAP[event.pool_pair]` e retornar erro se `pool_pair` desconhecido. Em caso de falha de execução após o evento (ex.: gateway offline), o commit transita para `RECONCILIATION_REQUIRED` após timeout configurável (padrão: 300s desde o `CommitMatched`).

- **FR-004**: O sistema DEVE bloquear (HTTP 403) qualquer chamada a `POST /api/v2/amm/token/mint-and-approve` com campo `recipient` cujo endereço seja o signer ativo de um Banco Central registrado no `IdentityRegistry`. A validação usa `IdentityRegistry.getCentralBankOf(tokenAddress)` para enumerar os signers de CB conhecidos. O error body DEVE ser `{ "error": "recipient is a Central Bank signer — cross-CB minting is prohibited", "code": "CROSS_CB_MINT_PROHIBITED" }`. Esta validação implementa o bloqueio do padrão G5-cross descrito em FR-018 de spec-005 como "a ser implementado em versão futura".

- **FR-005**: O sistema DEVE suportar que cada Banco Central execute sua perna de `addSingleSidedLiquidity` usando **seu próprio signer** (chave privada soberana). O signer de CB-A NUNCA deve assinar transações de depósito de TOKEN_B; o signer de CB-B NUNCA deve assinar transações de depósito de TOKEN_A. Esta restrição deve ser enforceável via validação no handler de execução de match: o handler verifica que o signer local possui `CENTRAL_BANK_ROLE` no token correspondente ao `side` do commit antes de executar a transação on-chain.

- **FR-006**: O commit-reveal cooperativo de spec-005 (endpoints `POST /api/v2/amm/liquidity/commit`, `GET /api/v2/amm/pool/{pair}/status`) DEVE continuar funcionando sem alteração de API externa. A mudança é exclusivamente interna: após o MATCH, em vez de um único gateway executar os dois `addSingleSidedLiquidity`, o protocolo cross-gateway coordena dois gateways distintos para executar cada um o seu lado.

- **FR-007**: A remoção de liquidez (`DELETE /api/v2/amm/liquidity/{position_id}` ou equivalente) DEVE ser executada pelo signer do gateway que pertence ao CB owner da LP position. CB-A's gateway só remove LP positions de CB-A; CB-B's gateway só remove LP positions de CB-B. O handler DEVE validar que `provider_id` da posição corresponde ao `client_id` do token JWT autenticado — corrigindo o gap de segurança M3 identificado na análise de spec-005.

- **FR-008**: O script de validação E2E desta feature é um **novo script dedicado `tryouts/tryout-sovereign-cb-liquidity.sh`** (confirmado em clarificação 2026-05-20), executável de forma isolada sem dependência do `tryout-scenario-b-e2e.sh`. O script DEVE demonstrar o fluxo completo soberano:
  1. CB-A emite BRL no Spoke-A (`mint-and-approve` sem recipient)
  2. CB-A bloqueia BRL no Bridge (`lock-mint`) → polling `GET /api/v2/bridge/positions` até `bridge_state = ACTIVE`
  3. CB-A submete commit A ao seu gateway → `LiquidityCommitRegistry.registerCommit` on-chain
  4. CB-B emite ARS no Spoke-B → lock-mint → polling até `ACTIVE`
  5. CB-B submete commit B ao **seu próprio gateway** (CB-B gateway, não CB-A) → `LiquidityCommitRegistry.registerCommit` on-chain
  6. Ambos os gateways detectam evento `CommitMatched` on-chain e executam `addSingleSidedLiquidity` de forma autônoma
  7. Pool `ACTIVE` com `msg.sender` de cada depósito sendo o CB emissor correto
  8. Verificação on-chain via `cast logs LogSingleSidedLiquidityAdded` confirmando que signer de CB-A executou TOKEN_A e signer de CB-B executou TOKEN_B — zero ocorrências invertidas

- **FR-009**: O campo `provider_id` no payload do commit DEVE ser validado contra o `client_id` do token JWT autenticado (finding M3 de spec-005). O gateway DEVE retornar HTTP 403 `{ "error": "provider_id mismatch — must match authenticated CB identity", "code": "PROVIDER_ID_MISMATCH" }` se `provider_id` não corresponder ao CB autenticado.

- **FR-010**: Para depósitos adicionais em pool já `ACTIVE` (após formação inicial via commit-reveal), o bridge (lock-mint) é **opcional** (confirmado em clarificação 2026-05-20). O CB pode chamar `addSingleSidedLiquidity` diretamente se já possui saldo de W-tCeBM no Hub (de bridge anterior). O commit-reveal via `LiquidityCommitRegistry` também não é exigido em depósitos adicionais — herdado de spec-005 FR-001 (commit-reveal é obrigatório apenas na formação inicial do pool). A soberania é garantida pela origem dos W-tCeBM: eles só existem no Hub como resultado do bridge do spoke soberano do CB emissor. O gateway DEVE validar que o CB autenticado é holder do W-tCeBM correspondente antes de autorizar o depósito adicional.

### Non-Functional Requirements

- **NFR-001**: O fluxo completo desde a detecção do evento `CommitMatched` on-chain até a confirmação de ambas as pernas de `addSingleSidedLiquidity` DEVE completar em p95 ≤ 30s em ambiente local (inclui latência de bloco Besu + tempo de execução da tx). O event watcher do gateway DEVE processar o evento `CommitMatched` em até um bloco após a emissão (lag máximo de 1 bloco).

- **NFR-002**: A adição do `LiquidityCommitRegistry` e do event watcher DEVE ser transparente para os chamadores externos dos endpoints existentes — nenhuma mudança de contrato de API REST externo. O endpoint `POST /api/v2/amm/liquidity/commit` continua com a mesma assinatura; internamente o gateway passa a submeter a tx no `LiquidityCommitRegistry` além de persistir no DB local.

- **NFR-003**: A autenticidade das transações no `LiquidityCommitRegistry` é garantida on-chain pelo `msg.sender` da tx (assinada pela chave privada do gateway do CB) — não é necessária autenticação inter-gateway off-chain. O contrato DEVE validar via `IdentityRegistry.getCentralBankOf(w_token_address) == msg.sender` que o registrante é efetivamente o CB emissor daquele token no spoke.

---

## Key Entities

- **SovereignLiquidityDeposit**: Sequência completa de ações de um CB para provisionar liquidez soberana. Estados: `BRIDGE_PENDING` → `BRIDGE_ACTIVE` → `COMMIT_PENDING` → `COMMIT_MATCHED` → `EXECUTED` | `RECONCILIATION_REQUIRED`. Esta é uma agregação lógica para rastreamento; as entidades de persistência existentes (`BridgedAssetPosition`, `PoolCommit`, `LiquidityPosition`) continuam como fonte de verdade.

- **LiquidityCommitRegistry** *(novo contrato Solidity no Hub)*: Contrato de coordenação on-chain que registra intenções de depósito bilateral e detecta match. Funções públicas: `registerCommit(pool_pair, side, amount, w_token_address)` (assinada pelo signer do CB gateway), `cancelCommit(commit_id)`. Eventos: `CommitRegistered(commit_id, pool_pair, side, signer, amount, w_token_addr, expires_at)`, `CommitMatched(pool_pair, commit_id_a, signer_a, amount_a, commit_id_b, signer_b, amount_b)`, `CommitExpired(commit_id)`. Validação interna: `IdentityRegistry.getCentralBankOf(w_token_address) == msg.sender`.

- **CommitMatchedEvent** *(evento on-chain)*: Emitido pelo `LiquidityCommitRegistry` quando ambos os lados de um par estão registrados. Cada gateway de CB escuta este evento via watcher e executa `addSingleSidedLiquidity` usando o signer local para o side correspondente. Substitui a `CrossGatewayMatchEvent` e `InterGatewayNotification` do design off-chain (ambas removidas desta spec).

---

## Success Criteria *(mandatory)*

- **SC-001**: O tryout E2E demonstra que `msg.sender` dos eventos `LogSingleSidedLiquidityAdded` no Hub corresponde ao signer do CB emissor do token depositado — CB-A's signer para TOKEN_A, CB-B's signer para TOKEN_B. Zero ocorrências de CB-A's signer como executor de TOKEN_B ou vice-versa.

- **SC-002**: O gateway rejeita (HTTP 403) 100% das tentativas de `mint-and-approve` com `recipient` sendo signer de outro CB, antes que qualquer transação on-chain seja submetida.

- **SC-003**: O protocolo cross-gateway conclui o match (ambas as pernas executadas) em p95 ≤ 10s em ambiente local. Falhas parciais (primeira perna ok, segunda falhou) resultam em `RECONCILIATION_REQUIRED` para ambos os commits — nunca em estado parcialmente executado silencioso.

- **SC-004**: `provider_id` no commit é validado contra o JWT — 100% das tentativas de forjar `provider_id` de outro CB são rejeitadas com HTTP 403 antes de qualquer persistência.

- **SC-005**: O fluxo completo lock-mint → commit → match cross-gateway → add-liquidity → pool ACTIVE é demonstrável em ambiente local sem intervenção manual após iniciar o tryout.

---

## Assumptions

- O `SpokeBridge` está deployado e funcional em ambos os spokes (Spoke-A e Spoke-B) com o Relayer Cacti configurado para ambos.
- Os CBs têm `CENTRAL_BANK_ROLE` nos tokens do seu próprio spoke (permissão de mint local). No fluxo soberano, os CBs **não precisam de `CENTRAL_BANK_ROLE` no Hub** para os W-tCeBM — esses tokens são mintados exclusivamente pelo Bridge Relayer (contrato `SpokeBridge`) como contrapartida do lock no spoke. Os CBs recebem W-tCeBM como qualquer holder ERC-20 padrão e podem depositá-los no AMM com `approve` + `addSingleSidedLiquidity`.
- Os gateways de CB-A e CB-B **não precisam de conectividade direta entre si** — a coordenação é feita exclusivamente via eventos on-chain do `LiquidityCommitRegistry`. Cada gateway precisa apenas de acesso ao nó RPC do Hub.
- O `IdentityRegistry` no Hub expõe `getCentralBankOf(tokenAddress)` retornando o endereço do signer do CB emissor — necessário para FR-004.
- O mecanismo de commit-reveal de spec-005 (tabela `pool_commits`, endpoint `/commit`) permanece sem alteração de schema ou API.

---

## Out of Scope

- Interface frontend para o fluxo soberano — tratado em `008-frontend-sovereign-liquidity`.
- Migração de LP positions existentes criados via G5-cross — posições existentes permanecem válidas para remoção (spec-005 SC-005 garantido).
- Multi-hop bridging (spoke → hub → outro hub) — fora do escopo desta versão.
- Suporte a CBs que operam múltiplos spokes com a mesma moeda.
- Mecanismo de governança para rotacião de credenciais inter-gateway — não aplicável nesta feature (coordenação é on-chain; não há segredo compartilhado entre gateways).
- Remoção do código G5-cross dos tryouts e do frontend wizard — a remoção ocorre quando a implementação desta feature estiver validada em ambiente de staging, para não quebrar o ambiente de desenvolvimento antes da substituição estar pronta. O código G5-cross permanece marcado como ANTI-PATTERN e não deve ser replicado.
