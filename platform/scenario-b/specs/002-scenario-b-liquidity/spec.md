# Feature Specification: Reframe Scenario B Docs

**Feature Branch**: `002-scenario-b-liquidity`
**Created**: 2026-04-23
**Status**: Draft
**Input**: User description: "/speckit-specify crie um resumo do que se trata esse projeto e crie uma espcificacao para o scenario B, para qque eu possa atualizar uma documentação externa. MAs pode fazer isso em um markdown. Analise esses documentos, o foco é o liquidity pool do scenario b@docs-reference LEve em consideracao o que já temos implementado  e sugira nova implementacao. Remova complementamente o scenario A.Adicione a spec que o cenario A deve ser removido e se houver alguma implementacao que possa ser reaproveitada no cenario B destaque"

## Resumo do Projeto

O CBWeb3 e um monorepo de infraestrutura financeira para testes de liquidacao transfronteirica com moeda tokenizada de banco central (tCeBM), conectando redes domesticas soberanas a um contexto internacional de interoperabilidade.

Nesta feature, a documentacao externa deve ser reposicionada para apresentar somente o **Cenario B (Hub-and-Spoke com Liquidity Pool/AMM)**, removendo integralmente o **Cenario A (correspondent banking bilateral)** do pacote externo atualizado.

## Atores e Fluxo de Alto Nivel (Cenario B)

### Atores e papeis

- **Bancos Comerciais nos Spokes (CommBanks)**: atuam como **Liquidity Takers**. Iniciam pagamentos transfronteiricos e, via Payment Orchestrator, movem ativos da rede domestica (Spoke) para o Hub internacional atraves de uma ponte (bridge).
- **Bancos Centrais nos Spokes (BCA/BCB)**: atuam como **Network Operators** e **Issuers** de tCeBM. No cenario experimental, tambem podem atuar como **Liquidity Providers**, injetando tCeBM nos pools do Hub para testes em condicoes controladas. Detem a governanca do **Circuit Breaker** no Hub.
- **Hub Internacional**: ledger neutro e compartilhado que hospeda o contrato inteligente do AMM e os pools de liquidez. E onde a troca efetiva (FX) acontece.
- **Relayer (Hyperledger Cacti)**: espinha dorsal de interoperabilidade. Observa eventos de trava nos Spokes e leva prova criptografica ao Hub para emissao (mint) de tokens espelhados; na direcao inversa sinaliza o Spoke para destravar o ativo original.

### Modelo operacional (Exact-Output Swap)

O fluxo do Cenario B e baseado no modelo de **Exact-Output Swap**, onde o beneficiario recebe um valor fixo e o pagador assume o risco de cambio e de slippage. O pipeline de ponta a ponta e:

1. **Bridging (Lock & Mint)**: o Banco Comercial pagador trava `tCeBMa` em custodia no Spoke A. O Relayer detecta a trava e envia prova ao Hub, que emite tokens espelhados (`W-tCeBMa`) para a carteira do pagador no Hub.
2. **Cotacao e requisicao**: o pagador solicita cotacao ao AMM para entregar um valor exato Z de `tCeBMb` ao beneficiario no Spoke B.
3. **Calculo algoritmico**: o AMM usa a formula de produto constante (x * y = k) para determinar a entrada W necessaria.
4. **Validacao de conformidade (ZK-Pointers)**: os participantes anexam provas de conhecimento zero a transacao; o AMM verifica se os bancos estao "fit to transact" (KYC/AML) sem expor dados sensiveis no ledger internacional.
5. **Execucao atomica**: em uma unica transacao no Hub, o AMM debita `W-tCeBMa` do pagador, credita `Z-tCeBMb` ao beneficiario e atualiza as reservas do pool.
6. **Unbridging (Burn & Unlock)**: os tokens espelhados sao queimados no Hub e o Relayer sinaliza o Spoke B para destravar `tCeBMb` nativo ao beneficiario final.

### Controles de risco e compliance

- **Limite de entrada (maxAmountIn)**: parametro obrigatorio na API de swap; a transacao falha se a volatilidade do pool exceder a tolerancia de slippage definida pelo pagador.
- **Liquidity Monitor**: microservico de backend que observa reservas do pool e emite alertas quando o ratio entre moedas ultrapassa o limite de 70/30 (requisito REQ-FX-008).
- **Circuit Breaker**: pausa de emergencia operada por governanca (Banco Central) para halt/resume do AMM no Hub.
- **Master Viewing Key (Paladin)**: visibilidade institucional do Banco Central para desanonimizacao de transacoes via governanca multi-assinatura em investigacoes AML/CFT.

## Clarifications

### Session 2026-04-23

- Q: O backend deve remover toda a API atual ou apenas partes ligadas ao Cenario A? → A: Remover toda a API atual e reconstruir a API de backend para o Cenario B.
- Q: A troca da API deve ser corte unico ou faseada? → A: Corte unico (big bang), desligando a API antiga e ativando apenas a nova API do Cenario B na janela de migracao.
- Q: O escopo da substituicao cobre toda a esteira de artefatos backend ou apenas parte dela? → A: Substituir todos os artefatos de backend ligados a API atual.
- Q: No corte da API atual, como tratar dados e historico legado? → A: Descontinuar dados e historico antigos junto com a API atual.
- Q: Qual criterio final de corte para eliminar o Cenario A e o que pode ser reaproveitado? → A: Corte completo do Cenario A implementado, permitindo apenas reaproveitamento de infraestrutura transversal (ex.: Keycloak, Postgres).
- Direcionamento adicional: concentrar o escopo de execucao em backend + contratos Solidity e incluir um tryout E2E dedicado em `tryouts/`.
- Contexto adicional (sessao 2026-04-23): topologia Hub-and-Spoke com settlement centralizado no Hub; bridging por Lock&Mint / Burn&Unlock via Hyperledger Cacti; modelo Exact-Output com `maxAmountIn`; compliance por ZK-Pointers + Master Viewing Key; Liquidity Monitor com threshold 70/30 (REQ-FX-008); Bancos Centrais como governancas do Circuit Breaker e potenciais Liquidity Providers em testes controlados.
- Q: Como resolver falha assimetrica bridging/unbridging (ex.: Burn no Hub ok, Unlock no Spoke B falha)? → A: Reconciliacao automatica idempotente com retry exponencial pelo Relayer; apos N tentativas, posicao entra em `RECONCILIATION_REQUIRED` e e escalada para operador com registro auditavel (sem compensacao on-chain automatica).
- Q: Qual quorum e timeout do Master Viewing Key multi-sig para disclosure AML/CFT? → A: Quorum fixo 2-of-3 entre Bancos Centrais participantes, com timeout de 72h para atingir o quorum; expiracao automatica da solicitacao apos o prazo.
- Q: Quais targets quantitativos de performance para quote, swap e Liquidity Monitor? → A: Quote p95 <= 300ms, Swap p95 <= 6s (inclui confirmacao on-chain em 1 bloco), cadencia de poll do Liquidity Monitor <= 15s.
- Q: Como parametrizar retry/DLQ do Relayer Cacti? → A: 5 tentativas com backoff exponencial (2s, 4s, 8s, 16s, 32s; cap 60s) e fila persistente em Postgres (coluna de estado + worker consumidor), reaproveitando a camada DATASTORE existente.
- Q: Qual o escopo do frontend no cutover da API v2? → A: Nao mexer no frontend. Frontend e declarado OUT-OF-SCOPE explicito desta feature; nenhuma adaptacao, freeze ou pagina de manutencao sera feita. Assume-se que, apos o corte da API v1, o frontend atual ficara incompativel com a nova API v2 ate que uma feature futura dedicada ao frontend v2 seja criada (risco conhecido e aceito, sem mitigacao nesta iteracao).
- Q: Qual o modelo de autorizacao do Circuit Breaker (pause/resume)? → A: Assimetrico — `pause` pode ser acionado por 1-of-N Bancos Centrais participantes (fail-safe: qualquer supervisor pausa em emergencia); `resume` exige quorum 2-of-N (consenso minimo para retomar operacao). Todas as assinaturas sao registradas em log auditavel.
- Q: Qual a politica de retencao para dados operacionais do Cenario B (novos)? → A: Retencao indefinida para TODOS os dados operacionais do Cenario B (`SwapOrderScenarioB`, `BridgedAssetPosition`, `ComplianceZKPointer`, `DisclosureRequest`, audit logs, eventos do Liquidity Monitor, leituras de pool state). Nenhum job de purga e previsto; arquitetura MUST absorver crescimento perpetuo via particionamento e estrategia de storage tiering.
- Q: Qual a garantia tecnica de imutabilidade dos audit logs? → A: Append-only em Postgres com triggers que recusam UPDATE/DELETE nas tabelas de auditoria. Sem replicacao WORM nem anchor on-chain nesta feature — garantia e ao nivel do RDBMS, suficiente para protecao contra alteracao casual; protecoes contra privilegio de sistema (DBA) ficam fora do escopo.
- Q: Qual a observabilidade minima obrigatoria desta feature? → A: Observabilidade estruturada (logs JSON, metricas Prometheus, traces distribuidos) e DELEGADA a feature futura dedicada. Nesta feature aceitam-se logs ad-hoc em formato livre no stdout dos servicos. Os SCs que dependem de observabilidade estruturada (SC-014, SC-019, SC-023) serao validados de forma qualitativa/manual nesta feature e requalificados objetivamente quando a feature de observabilidade existir.
- Q: O projeto usa GORM — por que o spec menciona "migrations" e `.sql`? → A: O spec usa o termo "migration" em dois sentidos distintos, nenhum deles SQL puro: (1) `migration_action` nos arquivos YAML de cutover e um campo de disposicao de artefato legado (REMOVE/REPLACE), sem relacao com banco de dados; (2) o arquivo `migrate.go` e o inicializador GORM Go que chama `db.AutoMigrate()` — nenhum arquivo `.sql` e criado. FR-055 proibe explicitamente `.sql` migration files; todo gerenciamento de schema ocorre via `db.AutoMigrate()` e `db.Exec()` em inicializadores Go. O `quickstart.md` usava "migrations particionadas" como atalho informal e foi corrigido para "inicializadores GORM em `internal/db/init/`".
- Q: Como o api-gateway verifica se uma requisicao vem de um "Banco Central" vs "Banco Comercial"? → A: Realm role Keycloak dedicado (`central_bank` vs `commercial_bank`) presente nos `realm_access.roles` do token JWT; o middleware de autorizacao do api-gateway inspeciona este campo em cada handler que exige perfil institucional (ex.: Circuit Breaker pause/resume, oversight disclosure).
- Q: Quais sao os estados do ciclo de vida de `SwapOrderScenarioB`? → A: 5 estados: `PENDING` (criado, aguardando submissao on-chain) → `SUBMITTED` (transacao enviada ao Hub) → `CONFIRMING` (aguardando confirmacao de bloco on-chain) → `COMPLETED` (bloco confirmado, swap executado) / `FAILED` (erro em qualquer etapa, incluindo violacao de maxAmountIn, rejeicao de ZK-Pointer, revert on-chain); estado `CONFIRMING` e essencial para medir o SLO p95 <= 6s (FR-037/SC-022).
- Q: O que acontece com `W-tCeBMa` ja mintado no Hub quando a validacao de ZK-Pointer falha? → A: Manter `W-tCeBMa` em custodia no Hub (sem consumir nem queimar); retornar HTTP 422 ao cliente com codigo de erro `ZK_VALIDATION_FAILED`; o cliente inicia nova tentativa de swap com ZK-Pointer corrigido sem necessidade de novo Bridging. A posicao `BridgedAssetPosition` permanece no estado `ACTIVE` aguardando novo swap.
- Q: Qual o protocolo de integracao entre o `OversightService` e o Paladin para disclosure do Master Viewing Key? → A: JSON-RPC over HTTP — mesmo protocolo do no Besu/Paladin; cliente Go implementado via `ethclient` ou cliente HTTP customizado em `backend/shared/blockchain/scenariob/paladin/client.go`, reutilizando o padrao ja presente no stack (go-ethereum/ethclient).
- Q: Como o backend distingue "violacao de slippage" de "liquidez insuficiente" nas respostas da API de swap? → A: HTTP 422 com dois codigos de erro distintos: `SLIPPAGE_LIMIT_EXCEEDED` (entrada necessaria ultrapassa `maxAmountIn`) e `INSUFFICIENT_POOL_LIQUIDITY` (reserva do pool zerada ou abaixo do minimo operacional para qualquer swap). Codigos canonicos usados em `error_code` do body da resposta e nas asserções do tryout E2E.

### Session 2026-05-04

- Q: Qual deve ser a politica de provisao de liquidez inicial no tryout E2E? → A: Criar `step4b_seed_liquidity()` separado, executado pelo Banco Central (central-bank-a) antes de US1, declarado como pre-condicao obrigatoria do tryout no spec; pool vazio antes desse step e cenario de erro de configuracao de ambiente, nao cenario de teste valido para US1.
- Q: Qual o timeout maximo que o tryout aguarda pela transicao `LOCKING → ACTIVE` de uma `BridgedAssetPosition`? → A: 120 segundos; polling com intervalo de 5s e maximo de 24 tentativas; esgotado o timeout o step e marcado como falha com mensagem de erro explicita.
- Q: Qual campo o endpoint `POST /api/v2/amm/liquidity/remove` deve usar como chave primaria de lookup da posicao de liquidez? → A: `lp_id` (UUID da `LiquidityPosition` no banco); o tryout captura `lp_id` do response de Add Liquidity e o reutiliza no body de Remove; `lp_shares` hex on-chain nao e aceito como chave de lookup.
- Q: O endpoint `resume-sign` deve executar o resume automaticamente ao atingir quorum 2-of-N ou exigir chamada a `executeResume` separado? → A: Executar resume automaticamente — quando `resume-sign` detecta quorum atingido, transiciona estado para `LIVE` na mesma resposta e retorna `state: "LIVE"`; nenhum endpoint `executeResume` separado deve existir na API v2.
- Q: Qual e o campo canonico de nivel superior retornado por `GET /api/v2/amm/pool/{pair}/status` para indicar que o pool esta respondendo? → A: `reserve_a` — campo plano no body raiz; o body do endpoint usa campos planos (`reserve_a`, `reserve_b`, `current_ratio`, `imbalance_flag`, `pool_pair`, `updated_at`), sem objeto `reserves` aninhado. O selector jq no tryout MUST usar `.reserve_a` (ou `.pool_pair`) para verificar disponibilidade do endpoint.
- Q: Qual e o estado terminal de `BridgedAssetPosition` observavel via `GET /api/v2/bridge/positions` apos `POST /api/v2/bridge/burn-unlock` confirmado on-chain no Hub? → A: `BURNED` — o Hub confirma a queima de `W-tCeBM` e transiciona a posicao para `BURNED`; o estado `UNLOCKED` (liberacao do ativo nativo no Spoke B) segue assincronamente via Relayer e pode ser rastreado separadamente; o tryout MUST usar `wait_for_position_state <position_id> BURNED 120 5` apos chamar burn-unlock para confirmar que o Hub processou a queima antes de prosseguir.
- Q: Qual e o enum canonico do campo `bridge_state` da entidade `BridgedAssetPosition` no banco de dados e na API? → A: `LOCKING` (Lock&Mint iniciado, aguardando Relayer), `ACTIVE` (mint confirmado no Hub, disponivel para swap), `BURNED` (Burn no Hub confirmado on-chain), `UNLOCKED` (Relayer confirmou liberacao no Spoke B), `RECONCILIATION_REQUIRED` (esgotadas 5 retentativas), `FAILED` (erro irrecuperavel). Estes sao os valores string retornados por `GET /api/v2/bridge/positions` e usados nos GORM structs Go; os nomes `LOCKED` e `MINTED` do data-model anterior sao substituidos por `LOCKING` e `ACTIVE` respectivamente para alinhar ao vocabulario da spec e dos edge cases. O tryout MUST usar `wait_for_position_state <id> ACTIVE` e `wait_for_position_state <id> BURNED` como valores de estado.
- Q: Qual e o enum canonico do campo `status` da entidade `LiquidityPosition` e qual logica de lookup o handler `POST /api/v2/amm/liquidity/remove` deve aplicar? → A: Enum com 2 valores: `ACTIVE` (posicao injetada, liquidez disponivel no pool) e `WITHDRAWN` (posicao retirada, estado terminal sem reentrada). O handler Remove MUST localizar a `LiquidityPosition` pelo `lp_id` (UUID) E filtrar por `status = ACTIVE`; se nao encontrada ou ja `WITHDRAWN`, retornar HTTP 404 com mensagem `active liquidity position not found`. Apos remocao bem-sucedida, transicionar para `WITHDRAWN` (update GORM — a tabela `liquidity_position` NAO e append-only, diferente das tabelas de audit_log).
- Q: Os estados `ESCALATED` (RelayerQueueItem) e `RECONCILIATION_REQUIRED` (BridgedAssetPosition) devem coexistir ou ser consolidados? → A: Coexistencia intencional — cada entidade tem sua semantica propria: `RelayerQueueItem.state = ESCALATED` reflete o ciclo de vida da tarefa de entrega do Relayer; `BridgedAssetPosition.bridge_state = RECONCILIATION_REQUIRED` reflete o estado do ativo. Ao esgotar as 5 tentativas, o worker Go MUST executar os dois updates atomicamente em uma unica transacao GORM: `queue_item.state = ESCALATED` + `position.bridge_state = RECONCILIATION_REQUIRED`. Ambos os estados sao terminais para suas respectivas entidades neste fluxo de falha.
- Q: Qual e o nome canonico do campo de slippage no body JSON de `POST /api/v2/amm/swap/exact-output`? → A: `max_amount_in` (snake_case) — coerente com a convencao REST da API v2 (todos os campos de request/response usam snake_case: `pool_pair`, `token_a_amount`, `provider_bank_id`); mapeado no GORM struct Go como `MaxAmountIn string` com tag `json:"max_amount_in"`; o contrato OpenAPI v2 MUST usar `max_amount_in` como nome do campo; a linguagem de negocio no spec usa `maxAmountIn` como notacao narrativa, nao como nome de campo de API.
- Q: O `OversightService` no Hub deve invocar o Paladin diretamente ao atingir quorum? → A: NAO. Paladin opera exclusivamente no nivel spoke. O Hub OversightService apenas rastreia quorum (PENDING → QUORUM_REACHED); a operacao real de disclosure do Master Viewing Key e spoke-level, executada pelo no Paladin local do spoke sob demanda do operador. A conexao entre spoke e Hub para este efeito sera via Cacti (quando implementada). O cliente Go em `paladin/client.go` e preservado para uso futuro em features spoke-scoped.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Atualizar pacote externo para Cenario B (Priority: P1)

Como responsavel por documentacao externa, quero publicar um resumo oficial do projeto com foco exclusivo no Cenario B para que parceiros e avaliadores externos compreendam rapidamente o objetivo, o fluxo operacional e o escopo de liquidacao via liquidity pool.

**Why this priority**: Esta historia entrega o valor principal da solicitacao: disponibilizar documentacao externa atualizada e coerente com o foco de negocio atual.

**Independent Test**: Pode ser testada validando se um leitor externo identifica, sem ambiguidade, o objetivo do CBWeb3 e o funcionamento do Cenario B sem encontrar referencias ao Cenario A.

**Acceptance Scenarios**:

1. **Given** um rascunho da documentacao externa, **When** a versao final e publicada, **Then** o conteudo apresenta somente o Cenario B como escopo de settlement externo.
2. **Given** uma revisao textual completa, **When** a auditoria de termos e secoes e executada, **Then** nenhuma secao ativa referencia Cenario A, fluxos bilaterais ou dependencia narrativa de correspondent banking.

---

### User Story 2 - Evidenciar reaproveitamento e lacunas (Priority: P2)

Como lider tecnico, quero uma separacao clara entre o que ja pode ser reaproveitado no Cenario B e o que ainda precisa ser implementado para que o roadmap de evolucao da documentacao e da entrega tecnica seja confiavel.

**Why this priority**: Reduz retrabalho, evita descartar capacidades prontas e acelera a convergencia entre documentacao externa e realidade do produto.

**Independent Test**: Pode ser testada verificando se a documentacao externa inclui uma secao explicita de "capabilidades reutilizaveis" e uma secao de "novas implementacoes propostas" com criterios objetivos.

**Acceptance Scenarios**:

1. **Given** os artefatos atuais do projeto, **When** a matriz de reaproveitamento e preenchida, **Then** as capacidades existentes do Cenario B ficam identificadas com seu status de prontidao.
2. **Given** os gaps identificados, **When** a proposta de nova implementacao e descrita, **Then** cada item possui objetivo de negocio, dependencia e resultado esperado.

---

### User Story 3 - Sustentar governanca e revisao externa (Priority: P3)

Como gestor de governanca e compliance, quero que a especificacao externa deixe explicito o controle de risco operacional do Cenario B para que a narrativa publicada mantenha aderencia a supervisao e seguranca institucional.

**Why this priority**: O Cenario B depende de monitoramento de equilibrio de pool, controle de risco e capacidade de intervencao, pontos criticos para aceitacao institucional.

**Independent Test**: Pode ser testada por revisao de stakeholders de governanca, confirmando que o documento descreve gatilhos de monitoramento e mecanismos de pausa operacional.

**Acceptance Scenarios**:

1. **Given** o fluxo de operacao do Cenario B, **When** o documento externo e revisado por governanca, **Then** os controles de monitoramento e intervencao estao descritos de forma verificavel.
2. **Given** uma condicao de desequilibrio de liquidez, **When** o leitor consulta a documentacao, **Then** ele encontra o comportamento esperado de alerta e resposta operacional.

---

### Edge Cases

- Referencias indiretas ao Cenario A permanecem em anexos, notas ou exemplos antigos e podem passar despercebidas na revisao inicial.
- Secoes reaproveitadas descrevem capacidades "planejadas" como se estivessem "operacionais", causando desalinhamento com o estado real.
- A proposta de nova implementacao mistura escopo de produto com detalhes de tecnologia, reduzindo legibilidade para publico externo.
- Diferentes areas (produto, compliance, arquitetura) divergem sobre quais partes do Cenario B sao prontas versus pendentes.
- Volatilidade de pool causa entrada requerida acima do `maxAmountIn` informado pelo pagador, exigindo falha controlada do swap e comunicacao clara do motivo.
- Pool atinge ou ultrapassa o limite de imbalance 70/30 sem que o alerta do Liquidity Monitor seja escalado para a governanca responsavel.
- Validacao de conformidade via ZK-Pointer falha (participante nao "fit to transact"): a `SwapOrderScenarioB` transiciona para `FAILED`; o handler retorna HTTP 422 com codigo `ZK_VALIDATION_FAILED`; `W-tCeBMa` permanece em custodia no Hub com `BridgedAssetPosition` no estado `ACTIVE` — o cliente pode retentar um novo swap com ZK-Pointer corrigido sem necessidade de novo Bridging.
- Evento de trava no Spoke nao gera emissao correspondente no Hub por falha do Relayer: o Relayer retenta com backoff exponencial (idempotente); esgotadas as tentativas, a posicao entra em `RECONCILIATION_REQUIRED` com alerta operacional e registro auditavel para acao manual.
- Burn no Hub ocorre mas o Unlock no Spoke de destino falha: mesma politica — retry idempotente do Relayer, escalacao para `RECONCILIATION_REQUIRED` apos exaustao, sem reversao automatica on-chain do Burn (preserva invariantes contabeis do Hub).
- Acionamento do Circuit Breaker pelo Banco Central precisa bloquear novos swaps sem perder transacoes ja confirmadas em execucao atomica.
- Swap solicitado com `maxAmountIn` violado (slippage excessivo): handler retorna HTTP 422 com `error_code: SLIPPAGE_LIMIT_EXCEEDED`; `SwapOrderScenarioB` transiciona para `FAILED`; nenhum debito ocorre no pool.
- Swap solicitado com pool zerado ou reservas abaixo do minimo operacional (nenhum `maxAmountIn` resolveria): handler retorna HTTP 422 com `error_code: INSUFFICIENT_POOL_LIQUIDITY`; `SwapOrderScenarioB` transiciona para `FAILED`; cliente deve aguardar reposicao de liquidez pelo provedor antes de retentar.
- Tryout E2E iniciado sem provisao de liquidez inicial: o script MUST executar `step4b_seed_liquidity()` (via Banco Central / central-bank-a) como pre-condicao obrigatoria antes de qualquer step de US1; ausencia dessa provisao faz 100% das chamadas de quote/swap retornarem `INSUFFICIENT_POOL_LIQUIDITY`, invalidando os cenarios de teste de US1 e mascarando erros reais do backend.
- Burn&Unlock chamado imediatamente apos Lock&Mint sem aguardar confirmacao assincrona do Relayer: o tryout MUST usar polling em `GET /api/v2/bridge/positions` com timeout de 120s e intervalo de 5s para aguardar `bridge_state == ACTIVE` antes de prosseguir; chamada prematura resulta em `active bridged position not found` e invalida o cenario de US2.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A documentacao externa MUST conter um resumo executivo do CBWeb3 orientado ao Cenario B, explicando objetivo, atores principais e valor de negocio do modelo de liquidity pool.
- **FR-002**: A documentacao externa MUST remover completamente o Cenario A do escopo publicado, incluindo secoes principais, anexos funcionais, exemplos operacionais e referencias de fluxo.
- **FR-003**: A documentacao externa MUST apresentar o fluxo funcional do Cenario B em linguagem de negocio, cobrindo cotacao de saida exata, execucao de swap, monitoramento de equilibrio e intervencao de governanca.
- **FR-004**: A documentacao externa MUST destacar explicitamente as capacidades ja reaproveitaveis para o Cenario B, incluindo: cotacao para saida exata, troca por saida exata, monitoramento de desequilibrio e pausa de operacao por governanca.
- **FR-005**: A documentacao externa MUST separar as capacidades reaproveitaveis das capacidades pendentes, com classificacao clara de status (disponivel, parcial, nao implementado).
- **FR-006**: A documentacao externa MUST incluir proposta de nova implementacao para o Cenario B com foco em resultado, cobrindo no minimo: ciclo de provisao e retirada de liquidez, consolidacao do fluxo hub-and-spoke de ponta a ponta, e fechamento de trilha de auditoria operacional.
- **FR-007**: Cada proposta de nova implementacao MUST indicar precondicoes de negocio e criterio de pronto verificavel para publicacao futura.
- **FR-008**: O pacote final MUST incluir rastreabilidade entre fontes de referencia analisadas e secoes resultantes do documento externo para facilitar revisao formal.
- **FR-009**: O documento MUST manter linguagem orientada a stakeholders nao tecnicos, sem depender de conhecimento interno para compreender o Cenario B.
- **FR-010**: O plano de backend do Cenario B MUST assumir descontinuidade completa da API atual, com substituicao integral por uma nova API dedicada ao Cenario B.
- **FR-011**: O pacote externo MUST identificar explicitamente os artefatos de backend que serao descontinuados com a API atual e os novos artefatos que deverao compor a API reconstruida do Cenario B.
- **FR-012**: O escopo da nova API MUST excluir qualquer fluxo legado de Cenario A e qualquer dependencia de compatibilidade retroativa com o contrato funcional antigo.
- **FR-013**: A transicao de backend MUST seguir estrategia de corte unico, sem operacao paralela entre API antiga e API nova do Cenario B.
- **FR-014**: O plano de substituicao MUST abranger toda a esteira de artefatos backend vinculados ao contrato atual, incluindo rotas, handlers, servicos, contratos de interface, esquemas de dados operacionais, jobs e testes de backend.
- **FR-015**: O plano de corte MUST incluir descontinuacao dos dados e do historico operacional associados a API atual, sem reaproveitamento do acervo legado no novo backend do Cenario B.
- **FR-016**: O gate de ativacao da nova API MUST exigir evidencia auditavel de que a implementacao funcional do Cenario A (rotas, handlers, servicos, jobs e migrations funcionais) ja foi removida do backend antes do cutover; refina o criterio operacional de FR-010 (descontinuidade) transformando-o em porta de verificacao da janela de corte.
- **FR-017**: Reaproveitamento MUST ser limitado a componentes de infraestrutura transversal (por exemplo, identidade, persistencia base e runtime operacional), sem reaproveitar fluxos funcionais, contratos de API ou artefatos de dominio do Cenario A.
- **FR-018**: A especificacao de implementacao MUST priorizar entregas de backend e contratos Solidity do Cenario B como trilha principal de execucao tecnica.
- **FR-019**: O pacote de entrega MUST incluir um tryout de ponta a ponta em `tryouts/` para validar o fluxo completo do Cenario B apos o corte da API legada. O tryout MUST incluir funcao dedicada `step4b_seed_liquidity()` que provisiona liquidez inicial no pool via Banco Central (central-bank-a) como pre-condicao obrigatoria de ambiente, executada antes dos steps de US1; pool sem liquidez no inicio de US1 e cenario de erro de configuracao do ambiente de teste, nao cenario de teste valido.
- **FR-020**: O tryout E2E MUST validar, no minimo, cotacao, execucao de swap, verificacao de estado de pool e controle de circuito de risco no backend novo.
- **FR-021**: A documentacao externa MUST descrever a topologia Hub-and-Spoke do Cenario B, identificando papeis de Liquidity Taker, Liquidity Provider, Network Operator/Issuer, Hub Internacional e Relayer de interoperabilidade.
- **FR-022**: A documentacao externa MUST explicar o modelo Exact-Output, deixando claro que o beneficiario recebe valor fixo e o pagador assume risco de cambio e slippage.
- **FR-023**: A documentacao externa MUST descrever, em linguagem de negocio, o ciclo de bridging (Lock & Mint) e unbridging (Burn & Unlock) entre Spokes e Hub mediado pelo Relayer de interoperabilidade.
- **FR-024**: A documentacao externa MUST apresentar os controles de risco operacional do Cenario B, incluindo limite maximo de entrada no swap (protecao de slippage), threshold de desequilibrio de pool 70/30 e Circuit Breaker de governanca.
- **FR-025**: A documentacao externa MUST descrever a validacao de conformidade por ZK-Pointers, evidenciando que a verificacao "fit to transact" ocorre sem expor dados sensiveis no ledger internacional.
- **FR-026**: A documentacao externa MUST explicitar a visibilidade institucional do Banco Central via Master Viewing Key para investigacoes AML/CFT sob governanca multi-assinatura com quorum 2-of-3 entre Bancos Centrais participantes e timeout de 72h por solicitacao.
- **FR-027**: A nova API de backend do Cenario B MUST suportar parametro obrigatorio de limite maximo de entrada no swap, falhando a operacao quando a tolerancia de slippage do pagador for violada. O campo canonico no body JSON da requisicao e `max_amount_in` (snake_case); o GORM struct Go MUST mapear como `MaxAmountIn string \`json:"max_amount_in"\``. O handler MUST retornar HTTP 422 com dois codigos de erro distintos conforme a causa: `SLIPPAGE_LIMIT_EXCEEDED` quando a entrada necessaria calculada pelo AMM ultrapassa o `max_amount_in` informado, e `INSUFFICIENT_POOL_LIQUIDITY` quando a reserva do pool esta zerada ou abaixo do minimo operacional para qualquer swap viavel. Ambos os codigos MUST ser documentados no contrato OpenAPI e testados no tryout E2E. O endpoint `POST /api/v2/amm/liquidity/remove` MUST usar `lp_id` (UUID) como campo obrigatorio de lookup da posicao; `lp_shares` hex on-chain NAO DEVE ser aceito como chave de remocao.
- **FR-028**: O backend do Cenario B MUST oferecer capacidade de monitoramento de reservas e razao do pool, emitindo alertas quando o desequilibrio entre moedas ultrapassar o limite de 70/30 (REQ-FX-008). O endpoint `GET /api/v2/amm/pool/{pair}/status` MUST retornar body plano (nao aninhado) com os campos canonicos: `reserve_a` (string), `reserve_b` (string), `current_ratio` (number), `imbalance_flag` (bool), `pool_pair` (string), `updated_at` (RFC3339). Nenhum objeto `reserves` aninhado deve existir no schema; o tryout MUST usar `.reserve_a` como selector jq para verificar disponibilidade do endpoint.
- **FR-029**: O backend do Cenario B MUST observar confirmacoes do Relayer de bridging/unbridging e refletir o estado correspondente dos ativos espelhados no Hub. O tryout E2E MUST aguardar a transicao `LOCKING → ACTIVE` via polling em `GET /api/v2/bridge/positions` com timeout de **120s** (intervalo de 5s, maximo 24 tentativas) antes de executar Burn&Unlock; esgotado o timeout o step MUST falhar com mensagem de erro explicita. Apos `POST /api/v2/bridge/burn-unlock` confirmado on-chain, o tryout MUST aguardar transicao para estado `BURNED` com o mesmo mecanismo de polling (120s / 5s); `BURNED` e o estado terminal observavel do Hub — o estado `UNLOCKED` segue assincronamente via Relayer no Spoke B e e rastreavel mas nao e requisito de polling do tryout.
- **FR-030**: A governanca do Circuit Breaker MUST ser exposta no backend como acao autorizada por perfil institucional (Banco Central) sob modelo **assimetrico**: `pause` exige 1-of-N assinaturas de Bancos Centrais participantes (fail-safe); `resume` exige quorum 2-of-N. Verificacao de perfil MUST ocorrer via realm role Keycloak `central_bank` presente em `realm_access.roles` do token JWT, inspecionado pelo middleware de autorizacao do api-gateway. Cada acionamento e liberacao MUST ser registrado com identidade institucional dos signatarios, timestamp e motivo. O endpoint `POST /api/v2/governance/circuit-breaker/resume-sign` MUST executar o resume automaticamente quando detecta que o quorum 2-of-N foi atingido, retornando `state: "LIVE"` na mesma resposta; nenhum endpoint `executeResume` separado deve ser exposto na API v2. Assinaturas parciais (quorum ainda nao atingido) retornam `state: "RESUME_PENDING"`.
- **FR-031**: O backend MUST reconciliar falhas assimetricas de bridging/unbridging por meio de retentativas idempotentes do Relayer, limitadas a **5 tentativas** com backoff exponencial (intervalos alvo 2s, 4s, 8s, 16s, 32s; cap de 60s). Cada tentativa MUST ser registrada em `BridgedAssetPosition` e nao MUST produzir efeitos duplicados on-chain.
- **FR-032**: Esgotadas as 5 retentativas do Relayer, a posicao afetada MUST transicionar para o estado `RECONCILIATION_REQUIRED` e gerar alerta operacional rastreavel para a governanca, sem reversao automatica on-chain do Burn ou do Lock ja confirmados.
- **FR-033**: O backend MUST expor via API v2 (endpoint dedicado `GET /api/v2/bridge/positions` com filtro por `state`) os dados necessarios para reconciliacao manual de posicoes em `RECONCILIATION_REQUIRED` (ids on-chain, contagem de tentativas do Relayer, erros acumulados, timestamp da primeira e da ultima tentativa), permitindo ao operador registrar acao corretiva auditavel. Nota: "painel" e fora de escopo desta feature por FR-041; consumo visual e delegado a feature futura.
- **FR-034**: O `OversightService` MUST implementar fluxo de disclosure do Master Viewing Key exigindo quorum 2-of-3 entre Bancos Centrais participantes; assinaturas parciais MUST ser armazenadas de forma auditavel ate atingir quorum ou expirar. O estado terminal no Hub e `QUORUM_REACHED`; a operacao real de disclosure via Paladin ocorre no nivel spoke (fora de escopo desta feature). O cliente Go em `backend/shared/blockchain/scenariob/paladin/client.go` e preservado para uso futuro spoke-scoped.
- **FR-035**: Cada solicitacao de disclosure MUST expirar automaticamente apos 72h sem atingir quorum, transicionando para estado `EXPIRED` e recusando assinaturas adicionais; reabertura exige nova solicitacao registrada.
- **FR-036**: Todas as assinaturas, revogacoes, aprovacoes e expiracoes de disclosure MUST ser registradas em log imutavel com identidade institucional do signatario, timestamp e referencia a solicitacao original (trilha de auditoria AML/CFT).
- **FR-037**: A API v2 do Cenario B MUST observar os seguintes alvos de performance em ambiente de homologacao representativo: `GET /api/v2/amm/quote/exact-output` p95 <= 300ms; `POST /api/v2/amm/swap/exact-output` p95 <= 6s (incluindo confirmacao on-chain em 1 bloco do Hub); cadencia de polling do Liquidity Monitor <= 15s entre leituras consecutivas.
- **FR-038**: O pacote de validacao MUST publicar, ao final de T106, um baseline de performance com as medicoes reais de p95 de quote/swap e cadencia do Liquidity Monitor, sinalizando desvios > 20% dos alvos como risco para cutover.
- **FR-039**: O Relayer Cacti MUST persistir sua fila de trabalho em Postgres (reaproveitando a camada DATASTORE existente), com colunas de estado (`PENDING`, `IN_FLIGHT`, `COMPLETED`, `FAILED`, `ESCALATED`), contador de tentativas e timestamp da proxima execucao, de modo que reinicios do servico retomem o processamento sem perda de eventos. Estado `COMPLETED` e terminal e indica mint/burn espelhado confirmado on-chain; `ESCALATED` materializa a transicao para `RECONCILIATION_REQUIRED` apos esgotamento de retries (ver FR-031).
- **FR-040**: Cada item na fila do Relayer MUST carregar um identificador de idempotencia derivado do evento de origem (ex.: hash trava/burn) que impede emissao duplicada on-chain mesmo em cenarios de retry sobreposto.
- **FR-041**: O escopo desta feature EXCLUI explicitamente qualquer alteracao em `frontend/` (apps `bank`, `governance`, `supervisor`): nenhum task MUST editar, congelar, redirecionar ou substituir o frontend atual. Adaptacao do frontend a nova API v2 e deixada para feature futura dedicada.
- **FR-042**: A documentacao externa e o plano de cutover MUST declarar que, apos a janela de corte, o frontend atual permanecera incompativel com a API v2 (sem telas funcionais), assumindo esse efeito como risco aceito — nao como bug ou regressao.
- **FR-043**: O contrato `AutomatedMarketMaker.sol` MUST implementar Circuit Breaker com modelo de autorizacao assimetrico: funcao `pause()` aceita assinatura unica de qualquer Banco Central autorizado; funcao `resume()` exige 2-of-N assinaturas validas agregadas, recusando execucao com quorum incompleto.
- **FR-044**: Tentativas de `resume` sem quorum MUST ser rejeitadas on-chain e registradas off-chain como eventos auditaveis (incluindo assinaturas parciais recebidas), permitindo observabilidade de disputa de retomada sem alterar estado do pool. Nota: "tentativas de resume" aqui sao distintas das retentativas idempotentes do Relayer cobertas por FR-031 (bridging) — nao compartilham contador nem politica de backoff.
- **FR-045**: Todas as entidades operacionais do Cenario B (`SwapOrderScenarioB`, `BridgedAssetPosition`, `ComplianceZKPointer`, `DisclosureRequest`, audit logs, eventos do Liquidity Monitor e leituras historicas de pool state) MUST ser retidas indefinidamente; nenhum job de purga automatica ou retencao maxima MUST ser configurado nesta feature. Crescimento perpetuo MUST ser absorvido exclusivamente por particionamento por tempo (ver FR-046); storage tiering fica fora do escopo e sera endereco em feature futura.
- **FR-046**: As entidades sujeitas a retencao indefinida MUST prever estrategia de particionamento por tempo (ex.: particao mensal ou anual) com indices compativeis com consultas auditoriais por janela. Como o backend usa GORM (FR-055), o DDL de particionamento MUST ser declarado via `db.Exec()` em inicializadores Go (`internal/db/init/`), nao em arquivos `.sql` soltos. A existencia das particoes MUST ser validada em teste de integracao apos startup.
- **FR-047**: O plano de cutover MUST documentar a politica de retencao indefinida no registro operacional (e.g., `runbook` / `cutover-plan.yaml`) e declarar explicitamente a ausencia de jobs de purga, de modo que auditorias futuras compreendam o racional e nao removam dados por engano.
- **FR-048**: Todas as tabelas classificadas como `audit_log` (minimo: registros de Circuit Breaker pause/resume, disclosure lifecycle, assinaturas parciais, transicoes de `BridgedAssetPosition`, alertas do Liquidity Monitor) MUST ser implementadas como **append-only** em Postgres, com triggers `BEFORE UPDATE` e `BEFORE DELETE` que levantam excecao; insercao MUST continuar permitida.
- **FR-049**: Nenhum fluxo aplicacional (API, job, service) MUST emitir SQL `UPDATE` ou `DELETE` contra tabelas de audit log; violacoes desta regra devem ser interceptadas pelos triggers de banco e registradas como incidente operacional.
- **FR-050**: Protecao contra alteracao por privilegio de sistema (superuser, DBA, direct shell access ao Postgres) NAO esta no escopo desta feature e e delegada a controles de infraestrutura (IAM, network policies, backups externos), que serao tratados em feature futura dedicada a hardening de producao.
- **FR-051**: Observabilidade estruturada (logs JSON padronizados, metricas Prometheus, traces OpenTelemetry) NAO esta no escopo desta feature. Servicos MUST apenas emitir logs ad-hoc em stdout suficientes para triagem manual; um sinalizador documental MUST marcar este escopo reduzido para rastreamento em feature futura.
- **FR-052**: A validacao de SCs que dependem de observabilidade estruturada (minimo: SC-014 alerta de desequilibrio, SC-019 alerta de reconciliacao, SC-023 cadencia do Liquidity Monitor) nesta feature MUST ocorrer de forma qualitativa/manual (inspecao de stdout dos servicos durante tryout E2E); revalidacao objetiva MUST ser reagendada quando a feature de observabilidade existir.
- **FR-053**: Rate limiting / throttling de requisicoes na API v2 NAO esta no escopo desta feature. Nenhum middleware de limite de requisicoes MUST ser instalado no api-gateway nesta iteracao; abuso operacional e contido por acesso restrito ao ambiente (autenticacao institucional via Keycloak + segmentacao de rede).
- **FR-054**: A documentacao externa e o plano de cutover MUST declarar a ausencia de rate limiting como escopo conhecido reduzido, incluir o risco operacional na matriz de riscos e enderecar o tema a uma feature futura dedicada a gateway/infra de producao.
- **FR-055**: O backend do Cenario B MUST usar **GORM** (`gorm.io/v2` + `gorm.io/driver/postgres`) como unica camada de acesso ao banco de dados. Raw SQL migration files (arquivos `.sql` em diretorio `migrations/`) NAO DEVEM ser criados nem executados pelo backend desta feature; gerenciamento de schema MUST ocorrer exclusivamente por: (a) `db.AutoMigrate(&Model{})` para criacao e evolucao de tabelas a partir de structs Go anotados com GORM tags, e (b) `db.Exec(rawSQL)` chamado em inicializadores Go dedicados (ex.: `internal/db/init/`) para DDL nao suportado pelo AutoMigrate (triggers PL/pgSQL, particionamento declarativo, funcoes de banco, indices condicionais). Toda dependencia de `pgx` direta MUST ser substituida pelo driver GORM correspondente (`gorm.io/driver/postgres`). Arquivos de seed (dados iniciais) podem usar `db.Exec()` ou GORM Create — nunca arquivos `.sql` soltos.
- **FR-056**: O api-gateway MUST implementar middleware de autorizacao RBAC baseado em realm roles do Keycloak: endpoints que exigem perfil "Banco Central" (minimo: `POST /api/v2/governance/circuit-breaker/pause`, `POST /api/v2/governance/circuit-breaker/resume-sign`, `POST /api/v2/oversight/disclosure-request` e seus subrecursos) MUST rejeitar tokens sem a role `central_bank` em `realm_access.roles` com HTTP 403; endpoints de Banco Comercial (Liquidity Taker) MUST rejeitar tokens com role `central_bank` nesses handlers especificos para evitar escalada de privilegio.
- **FR-057**: O modelo `SwapOrderScenarioB` MUST implementar maquina de estados com 5 estados canonicos: `PENDING` (criado, aguardando submissao on-chain), `SUBMITTED` (transacao enviada ao Hub), `CONFIRMING` (aguardando confirmacao de bloco), `COMPLETED` (bloco confirmado, swap executado com sucesso), `FAILED` (erro em qualquer etapa — inclui violacao de `maxAmountIn`, rejeicao de ZK-Pointer, revert on-chain ou timeout de bloco). Transicoes MUST ser imutaveis na direcao oposta (sem rollback de estado); campos `submitted_at` e `confirmed_at` MUST ser registrados nas transicoes `SUBMITTED` e `COMPLETED` respectivamente, para calcculo de latencia p95 (FR-037/SC-022).
- **FR-058**: Quando a validacao de ZK-Pointer falhar no AMM do Hub, o handler `POST /api/v2/amm/swap/exact-output` MUST retornar HTTP 422 com codigo de erro `ZK_VALIDATION_FAILED`; a `SwapOrderScenarioB` associada MUST transicionar para `FAILED`; o `BridgedAssetPosition` correspondente MUST permanecer no estado `ACTIVE` com `W-tCeBMa` intocado em custodia no Hub — nenhum Unbridging automatico deve ser disparado. O cliente pode retentar o swap com um novo ZK-Pointer valido sem iniciar novo Bridging.
- **FR-059**: O handler `POST /api/v2/amm/swap/exact-output` MUST diferenciar erros de falha de swap usando codigos canonicos no campo `error_code` do body HTTP 422: (a) `SLIPPAGE_LIMIT_EXCEEDED` — entrada calculada pelo AMM ultrapassa `max_amount_in` (campo canonico snake_case) fornecido pelo cliente; (b) `INSUFFICIENT_POOL_LIQUIDITY` — reserva do pool esta zerada ou abaixo do minimo operacional e nenhum `max_amount_in` razoavel seria satisfeito. Ambos os codigos MUST estar documentados no contrato OpenAPI v2 e cobertos por cenarios de assercao distintos no tryout E2E.

### Key Entities *(include if feature involves data)*

- **Resumo Externo do Projeto**: narrativa curta que descreve o proposito do CBWeb3 e posiciona o Cenario B como foco da publicacao.
- **Capacidade Reaproveitavel**: funcionalidade ja existente que pode ser aproveitada no pacote externo sem redefinicao conceitual.
- **Lacuna de Implementacao**: funcionalidade necessaria ao Cenario B que ainda exige evolucao antes de ser tratada como pronta.
- **Plano de Evolucao do Cenario B**: conjunto de propostas para transformar lacunas em entregas prontas e auditaveis.
- **Matriz de Rastreabilidade**: mapeamento entre referencia analisada e secao final publicada.
- **Ativo Espelhado no Hub**: representacao do tCeBM original travado em um Spoke como token espelhado (`W-tCeBM*`) disponivel para swap no Hub; tem ciclo de vida vinculado ao par trava/queima e destrava no Spoke de destino.
- **LiquidityPosition**: entidade que representa a contribuicao de um Liquidity Provider ao pool AMM. Chave de lookup canonico: `lp_id` (UUID gerado pelo backend no momento do Add Liquidity). O campo `lp_shares` (hex on-chain) e retornado para referencia, mas NAO e usado como chave de lookup no endpoint `POST /api/v2/amm/liquidity/remove`. Enum canonico de `status`: `ACTIVE` (injetada, disponivel no pool) e `WITHDRAWN` (retirada, estado terminal sem reentrada); handler Remove filtra por `lp_id` AND `status = ACTIVE` e retorna HTTP 404 se nao encontrado.
- **BridgedAssetPosition — estados canonicos**: `LOCKING` (Lock&Mint iniciado, aguardando confirmacao do Relayer) → `ACTIVE` (mint confirmado no Hub, posicao disponivel para swap) → `BURNED` (Burn&Unlock executado on-chain no Hub, queima de `W-tCeBM` confirmada — estado terminal observavel pelo tryout) → `UNLOCKED` (Relayer confirmou liberacao do ativo nativo no Spoke B — estado assincronamente atingido apos `BURNED`) / `RECONCILIATION_REQUIRED` (esgotadas 5 retentativas do Relayer) / `FAILED` (erro irrecuperavel).
- **SwapOrderScenarioB**: entidade central que representa uma ordem de swap Exact-Output no Hub. Estados: `PENDING` (criado, aguardando submissao) → `SUBMITTED` (transacao enviada ao Hub) → `CONFIRMING` (aguardando confirmacao de bloco on-chain) → `COMPLETED` (bloco confirmado, swap executado) / `FAILED` (erro em qualquer etapa — violacao de `maxAmountIn`, rejeicao de ZK-Pointer, revert on-chain, timeout de bloco). Transicao `SUBMITTED → CONFIRMING` registra `submitted_at`; `CONFIRMING → COMPLETED` registra `confirmed_at`, permitindo medicao de p95.
- **ZK-Pointer de Conformidade**: prova anexada a transacao que atesta que um participante e "fit to transact" (KYC/AML) sem expor dados sensiveis no ledger do Hub.
- **Alerta de Desequilibrio de Pool**: evento operacional gerado quando a razao entre reservas ultrapassa o limite de 70/30, enderecado a governanca e monitoramento.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% das referencias funcionais ao Cenario A sao removidas do documento externo publicado.
- **SC-002**: 100% das secoes do documento externo sobre Cenario B possuem classificacao de status (reaproveitavel, parcial ou pendente).
- **SC-003** (*post-launch*, medida apos a publicacao): Pelo menos 90% dos revisores internos conseguem identificar, em ate 5 minutos, o fluxo principal do Cenario B e os controles de governanca correspondentes. Esta metrica nao e construida dentro das Phases 1-6; e validada em rodada de revisao institucional apos T101.
- **SC-004**: A revisao de consistencia entre documento externo e referencias internas reporta zero contradicoes criticas sobre escopo do Cenario B.
- **SC-005**: 100% das novas implementacoes propostas possuem criterio de pronto verificavel e dependencia de negocio explicita.
- **SC-006**: 100% dos endpoints e artefatos da API atual sao classificados como descontinuados ou substituidos no plano de transicao para o backend do Cenario B.
- **SC-007**: A janela de migracao executa um unico evento de ativacao da nova API do Cenario B, sem coexistencia operacional da API antiga apos o corte.
- **SC-008**: 100% dos artefatos backend mapeados no contrato antigo possuem destino definido no corte (descontinuado ou substituido) antes da ativacao da nova API do Cenario B.
- **SC-009**: 100% dos repositórios de dados e historicos operacionais ligados a API antiga sao descontinuados na mesma janela de corte do backend.
- **SC-010**: Apos o corte, 0 artefatos funcionais ativos do Cenario A permanecem operacionais no backend.
- **SC-011**: 100% dos componentes reaproveitados no corte pertencem a camada de infraestrutura transversal previamente classificada como permitida.
- **SC-012**: O tryout E2E em `tryouts/` executa com sucesso o fluxo funcional do Cenario B sem dependencia de endpoints do Cenario A.
- **SC-013**: 100% dos swaps executados respeitam o limite maximo de entrada informado pelo pagador; solicitacoes que excedem o limite retornam falha controlada e auditavel.
- **SC-014** (*validacao qualitativa nesta feature; revalidacao objetiva quando feature de observabilidade existir — ver FR-052*): 100% dos eventos de desequilibrio de pool que atingem o limite 70/30 geram alerta rastreavel em stdout e/ou tabela de alertas durante o tryout E2E.
- **SC-015**: 100% dos fluxos end-to-end validam bridging (Lock & Mint) e unbridging (Burn & Unlock) consistentes entre Spokes e Hub, sem ativos presos apos sucesso declarado.
- **SC-016**: 100% das transacoes executadas no AMM possuem validacao de ZK-Pointers registrada, sem exposicao de dados sensiveis de KYC/AML no ledger internacional.
- **SC-017**: O acionamento do Circuit Breaker (pause) pela governanca com 1-of-N assinaturas bloqueia novos swaps em ate 1 bloco apos confirmacao no Hub, preservando o estado auditavel das operacoes em andamento.
- **SC-018**: 100% das falhas de bridging/unbridging sao retentadas de forma idempotente pelo Relayer antes de serem escaladas para `RECONCILIATION_REQUIRED`; nenhum efeito on-chain duplicado pode ser observado nos tryouts.
- **SC-019** (*validacao qualitativa nesta feature; revalidacao objetiva quando feature de observabilidade existir — ver FR-052*): Em 100% dos casos em que uma posicao entra em `RECONCILIATION_REQUIRED`, um alerta operacional e emitido (stdout e/ou tabela `liquidity_alert`) e os dados de reconciliacao ficam disponiveis via `GET /api/v2/bridge/positions?state=RECONCILIATION_REQUIRED` (FR-033).
- **SC-020**: 100% das solicitacoes de disclosure aprovadas atingem quorum 2-of-3 antes de liberar desanonimizacao; nenhuma desanonimizacao ocorre com apenas 1 assinatura ou com quorum alcancado apos as 72h.
- **SC-021**: 100% das solicitacoes de disclosure sem quorum em 72h transicionam para `EXPIRED` automaticamente e recusam assinaturas posteriores, com registro auditavel do evento de expiracao.
- **SC-022**: Em ambiente de homologacao representativo, `GET /api/v2/amm/quote/exact-output` atinge p95 <= 300ms e `POST /api/v2/amm/swap/exact-output` atinge p95 <= 6s sobre amostra minima de 200 requisicoes consecutivas.
- **SC-023** (*validacao qualitativa nesta feature; revalidacao objetiva quando feature de observabilidade existir — ver FR-052*): O Liquidity Monitor coleta e publica o estado do pool com cadencia p95 <= 15s entre leituras consecutivas durante o tryout E2E, medido por timestamps em `pool_state_reading` e/ou stdout.
- **SC-024**: 100% dos itens na fila do Relayer sobrevivem a restart do servico (validado via tryout que derruba e reinicia o Relayer entre tentativas), sem perda de eventos nem efeitos on-chain duplicados.
- **SC-025**: 0 arquivos sob `frontend/` sao modificados por tasks desta feature (validado por auditoria de diff do branch de cutover contra o commit-base).
- **SC-026**: Tentativas de `resume` do Circuit Breaker com quorum < 2 sao rejeitadas em 100% das execucoes, sem transicao de estado do pool; 100% das execucoes bem-sucedidas de `resume` carregam >= 2 assinaturas validas distintas de Bancos Centrais autorizados.
- **SC-027**: 0 jobs de purga ou TTL expirations afetam entidades operacionais do Cenario B em qualquer janela de execucao (validado por auditoria de DDL e job scheduler pos-deploy).
- **SC-028**: 100% das tabelas operacionais com alta cardinalidade (minimo: `swap_order_scenario_b`, `bridged_asset_position`, `compliance_zk_pointer`, `liquidity_alert`, `pool_state_reading`) possuem estrategia de particionamento por tempo declarada via `db.Exec()` no inicializador GORM e indice compativel com consulta por janela de auditoria (nao em arquivo `.sql` solto).
- **SC-029**: 100% das tabelas classificadas como `audit_log` possuem triggers `BEFORE UPDATE` e `BEFORE DELETE` ativos que recusam a operacao; validado por teste de integracao que tenta UPDATE/DELETE em cada tabela e espera erro de banco.
- **SC-030**: 0 ocorrencias de `UPDATE` ou `DELETE` sobre tabelas de `audit_log` sao observaveis nos logs do Postgres durante o tryout E2E completo.

## Assumptions

- A publicacao externa desta iteracao prioriza clareza institucional e alinhamento de escopo, nao detalhamento tecnico de execucao.
- O Cenario B e tratado como trilha principal para comunicacao externa do hub internacional com liquidity pool.
- O backend adotara substituicao integral da API atual por uma nova API do Cenario B, sem obrigacao de manter contrato legado.
- A entrada em producao da nova API do Cenario B ocorrera por corte unico, sem fase de coexistencia operacional com a API anterior.
- A substituicao integral inclui todos os artefatos backend ligados ao contrato funcional atual, nao apenas a camada de rotas HTTP.
- O corte do backend inclui descontinuacao completa dos dados e do historico da API anterior.
- Nenhum artefato funcional do Cenario A permanece ativo apos o corte; apenas infraestrutura transversal podera ser reaproveitada quando nao carregar comportamento legado.
- A validacao final da entrega tecnica sera comprovada por um tryout E2E dedicado ao Cenario B na pasta `tryouts/`.
- O backend usa **GORM** (`gorm.io/v2`) como ORM exclusivo; nenhum arquivo `.sql` de migration sera criado. Gerenciamento de schema ocorre via `db.AutoMigrate()` (tabelas padrao) e `db.Exec()` em inicializadores Go dedicados (triggers, particoes, funcoes PL/pgSQL). `pgx` NAO e usado diretamente — o driver GORM encapsula o acesso ao Postgres.
- A integracao com o Paladin (Master Viewing Key) e **spoke-only**; o Hub OversightService rastreia quorum (PENDING → QUORUM_REACHED) sem invocar Paladin. O cliente Go em `backend/shared/blockchain/scenariob/paladin/client.go` (JSON-RPC over HTTP) e preservado para uso futuro em features spoke-scoped.
- O Cenario B opera em topologia Hub-and-Spoke com settlement centralizado no Hub internacional neutro; liquidez no Hub e condicao necessaria para execucao dos swaps.
- Bridging e unbridging entre Spokes e Hub sao mediados por um Relayer de interoperabilidade (Hyperledger Cacti) com padrao Lock&Mint / Burn&Unlock.
- O modelo padrao de troca e Exact-Output, com risco de cambio e slippage transferido ao pagador via parametro obrigatorio de limite maximo de entrada.
- Bancos Centrais podem atuar como Liquidity Providers em ambiente controlado de teste, alem de seus papeis de Issuer e Network Operator.
- A consolidacao final depende de revisao conjunta entre produto, arquitetura e compliance para fechar a classificacao de prontidao.
- Frontend (`frontend/apps/*`) esta explicitamente fora do escopo desta feature; nenhuma adaptacao a API v2 sera feita aqui e a incompatibilidade pos-cutover e risco aceito.

### Glossario (terminologia canonica)

- **Banco Central (BCA/BCB)**: termo canonico para Network Operator/Issuer de tCeBM em um Spoke. Variacoes como "Central Bank", "Autoridade Monetaria" ou siglas BCA/BCB sao aliases e devem ser resolvidas para "Banco Central" nos artefatos escritos.
- **Liquidity Taker**: Banco Comercial (CommBank) quando origina pagamento transfronteirico e consome liquidez do pool.
- **Liquidity Provider**: em ambiente experimental, papel desempenhado pelo Banco Central quando injeta tCeBM no pool do Hub.
- **Hub Internacional**: ledger neutro que hospeda AMM e pools; distinto dos Spokes domesticos (cada um com seu proprio Banco Central e tCeBM nativo).
- **Relayer**: implementacao de interoperabilidade via Hyperledger Cacti; termo canonico para o componente que observa e repassa provas entre Spokes e Hub.
- **Master Viewing Key**: capacidade institucional do Banco Central sobre o ledger Paladin para desanonimizacao sob multi-sig 2-of-3 com timeout de 72h por solicitacao; nao e chave pessoal de um individuo.
