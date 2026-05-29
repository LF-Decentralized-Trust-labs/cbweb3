# Feature Specification: Fix CB Liquidity (Scenario B)

**Feature Branch**: `008-fix-cb-liquidity`  
**Created**: 2026-05-21  
**Status**: Implemented (2026-05-25)  
**Input**: User description: "Align Scenario B frontend UX and flows with spec-007 backend behavior and updated runbooks/tryouts"

## Contexto e Motivacao

A evolucao do Scenario B introduziu o fluxo soberano de provisao de liquidez para Bancos Centrais via bridge, com bloqueio explicito do padrao G5-cross. Os frontends de banco e governanca ainda precisam ser alinhados para refletir corretamente:

- O fluxo soberano (bridge -> confirmacao -> commit -> execucao) para CBs.
- Erros e estados oficiais usados no ambiente atual de runbook/tryout.
- A separacao de papeis entre Banco Central e Banco Comercial sem regressao no gating de Scenario A.

Sem esse alinhamento, operadores seguem vendo passos obsoletos, mensagens incorretas e comportamentos de UX que nao representam o estado real do sistema.

## Clarifications

### Sessao 2026-05-21

- Q: Quais fases canonicas do fluxo soberano de CB o frontend deve seguir no Scenario B? -> A: Exatamente quatro fases do runbook v6.0 secao 19: (1) `bridge/lock-mint` + polling `bridge_state=ACTIVE` (5s, timeout 120s), (2) `amm/liquidity/commit`, (3) execucao assincrona via watcher ate `commit.status=EXECUTED` (polling 3s, timeout de UI 60s com aviso apos 30s), (4) validacao de `pool_status=ACTIVE`.
- Q: Como tratar tentativa de padrao G5-cross no frontend? -> A: Ao receber HTTP 403 `CROSS_CB_MINT_PROHIBITED` de `mint-and-approve` com `recipient` CB, bloquear continuidade no fluxo legacy, exibir mensagem acionavel orientando uso do fluxo soberano (secao 19) e registrar o erro sem retry automatico.
- Q: Como tratar pre-condicao de bridge nao confirmada? -> A: Ao receber HTTP 422 `BRIDGE_POSITION_NOT_ACTIVE` em `amm/liquidity/commit`, impedir avanco para commit, manter tela em estado de espera e instruir polling de `GET /api/v2/bridge/positions?state=ACTIVE` ate confirmacao do Relayer.
- Q: Qual e o comportamento esperado de swap para banco comercial? -> A: Swap so fica habilitado com `pool_status=ACTIVE`; mapear erros `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY` e `CIRCUIT_BREAKER_HALTED` para mensagens consistentes com acao recomendada, e atualizar cotacao a cada 10-15s.
- Q: Como separar papeis entre apps e cenarios? -> A: App de banco (comercial) exibe apenas jornada de quote/swap/transfer e nunca controles de governanca; app de governanca exibe fluxo de liquidez soberana CB e circuit breaker. Em Scenario A, rotas e menus de Scenario B permanecem ocultos/bloqueados em ambos os apps.

## User Scenarios *(mandatory)*

### User Story 1 - Provisao Soberana de Liquidez para CB (Priority: P1)

Como operador de Banco Central, quero executar a provisao de liquidez no Scenario B pelo fluxo soberano via bridge, sem qualquer dependencia de G5-cross, para manter soberania operacional e aderencia ao fluxo vigente.

**Why this priority**: E o fluxo mais critico para compatibilidade com spec-007; sem isso o UX de CB induz operacao incorreta.

**Independent Validation**: Um operador de CB consegue completar o fluxo de provisao soberana ate o pool ficar ativo, visualizando estados intermediarios e bloqueios esperados sem telas legadas de G5-cross.

**Acceptance Scenarios**:

1. **Given** um CB inicia provisao de liquidez no Scenario B, **When** segue o assistente de liquidez, **Then** o fluxo apresentado usa o caminho soberano e nao exibe passos de mint para signer de outro CB.
2. **Given** o bridge ainda nao foi confirmado, **When** o CB tenta avancar para registro de commit, **Then** o sistema bloqueia a acao e mostra estado de espera com orientacao de aguardar confirmacao do relayer.
3. **Given** ocorrer tentativa de padrao G5-cross, **When** o backend retornar bloqueio por cross-CB mint proibido, **Then** a UI mostra erro explicito e orienta migracao para fluxo soberano.
4. **Given** um commit foi submetido antes de `bridge_state = ACTIVE`, **When** o backend retornar `BRIDGE_POSITION_NOT_ACTIVE`, **Then** a UI bloqueia o avanco, mostra estado "Aguardando confirmacao do Relayer" e inicia/reinicia polling de 5s ate 120s.
5. **Given** ambos os lados do pool forem executados com sucesso, **When** o operador consultar o status, **Then** o pool aparece como `ACTIVE` com indicacao de posicoes de LP dos CBs participantes.

---

### User Story 2 - Swap e Transferencia para Banco Comercial (Priority: P1)

Como operador de Banco Comercial, quero cotar e executar swap/transferencia no Scenario B com feedback consistente de status, slippage, liquidez e governanca, para tomar decisoes operacionais seguras.

**Why this priority**: E o fluxo principal de uso comercial do Scenario B e depende de mensagens corretas de estado/erro para evitar operacoes mal-sucedidas.

**Independent Validation**: Um operador de banco comercial consegue cotar, aprovar e executar swap com retorno de sucesso, e recebe tratamento de erro apropriado para slippage, liquidez insuficiente e circuito pausado.

**Acceptance Scenarios**:

1. **Given** o pool esta ativo, **When** o usuario solicita cotacao e executa swap dentro do limite informado, **Then** a tela exibe confirmacao de execucao com identificadores da operacao.
2. **Given** o preco mudar alem do limite aceito, **When** o swap falhar por slippage, **Then** a UI apresenta mensagem de mercado movido e acao clara para atualizar cotacao.
3. **Given** o pool estiver pausado por governanca, **When** o usuario tentar swap, **Then** o envio fica bloqueado e a interface mostra aviso de interrupcao regulatoria.
4. **Given** houver liquidez insuficiente para o valor solicitado, **When** o backend retornar `INSUFFICIENT_POOL_LIQUIDITY`, **Then** a UI orienta ajuste de valor sem quebrar o fluxo.
5. **Given** o pool nao estiver `ACTIVE`, **When** o usuario abrir a tela de swap, **Then** o botao de envio permanece desabilitado e o formulario nao e submetido.

---

### User Story 3 - Governanca e Observabilidade Operacional (Priority: P2)

Como operador de governanca, quero monitorar estados de pool/liquidez e atuar com mensagens coerentes de erros e transicoes, para reduzir ambiguidade operacional entre bancos centrais e bancos comerciais.

**Why this priority**: Nao substitui o fluxo principal, mas reduz incidentes e tickets causados por inconsistencias de UX entre estados reais e tela.

**Independent Validation**: Um operador de governanca consegue identificar estado do pool, progresso de commits, bloqueios e condicoes de excecao sem depender de logs tecnicos.

**Acceptance Scenarios**:

1. **Given** existem commits pendentes ou em execucao, **When** a tela de governanca carregar, **Then** os estados operacionais ficam claros e com acao recomendada para cada caso.
2. **Given** um erro de pre-condicao ocorrer (ex.: ativo ainda nao confirmado para etapa seguinte), **When** o operador interagir, **Then** a mensagem contextual orienta espera, nova tentativa ou escalonamento.
3. **Given** o ambiente estiver em Scenario A, **When** o usuario navegar, **Then** fluxos e menus de Scenario B permanecem inacessiveis, preservando compatibilidade.

---

### Edge Cases

- Tentativa de usar fluxo legado G5-cross em ambiente com bloqueio ativo.
- Commit iniciado antes da posicao de bridge estar realmente pronta para o proximo passo (`BRIDGE_POSITION_NOT_ACTIVE`).
- Pool em estado nao ativo no momento da execucao de swap.
- Timeout de confirmacao operacional sem resposta final dentro da janela esperada.
- Divergencia temporaria entre estado esperado pelo usuario e estado retornado pelo backend.
- Usuario em ambiente Scenario A tentando acessar rotas ou acoes exclusivas de Scenario B.
- Retorno de `CROSS_CB_MINT_PROHIBITED` ao tentar mint com `recipient` de outro CB.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O frontend de governanca DEVE implementar a jornada de provisao soberana de CB no Scenario B em quatro fases canonicas: `LOCK_MINT` -> `BRIDGE_ACTIVE` -> `COMMIT_PENDING/EXECUTED` -> `POOL_ACTIVE`, conforme secao 19 do runbook v6.0.
- **FR-002**: O frontend DEVE tratar HTTP 403 `CROSS_CB_MINT_PROHIBITED` como bloqueio definitivo do fluxo legacy G5-cross: interromper a acao atual, exibir mensagem acionavel "Use o Fluxo Soberano (Secao 19)" e impedir retry automatico da mesma operacao.
- **FR-003**: O frontend DEVE tratar HTTP 422 `BRIDGE_POSITION_NOT_ACTIVE` em `POST /api/v2/amm/liquidity/commit` como pre-condicao nao atendida: bloquear avanco para commit, exibir estado de espera e executar polling de `GET /api/v2/bridge/positions?state=ACTIVE` a cada 5s ate 120s.
- **FR-004**: O frontend do banco comercial DEVE aplicar pre-check de `pool_status` antes do swap e manter botao/formulario de swap desabilitado quando `pool_status != ACTIVE`.
- **FR-005**: O frontend do banco comercial DEVE usar o fluxo quote->approve->swap com refresh de cotacao de 10-15s e mapear erros de swap para UX consistente: `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `CIRCUIT_BREAKER_HALTED`.
- **FR-006**: O frontend de governanca DEVE exibir estados de progresso de provisao/commit/execucao com clareza suficiente para operacao sem dependencia de logs.
- **FR-007**: O escopo primário DEVE limitar-se aos aplicativos de banco e governança. Extensões mínimas de API backend (FR-028, FR-029) SÃO permitidas para suportar observabilidade operacional e reduzir payload do cliente, desde que não alterem lógica de negócio core, contratos Solidity ou infraestrutura.
- **FR-008**: O comportamento de gating de Scenario A DEVE ser preservado integralmente; funcionalidades, menus e rotas de Scenario B permanecem ocultas/bloqueadas quando o ambiente estiver configurado para Scenario A.
- **FR-009**: O frontend DEVE manter consistencia com runbook oficial e tryouts de referencia para nomenclatura de estados e erros de Scenario B.
- **FR-010**: ~~Esta feature DEVE produzir apenas artefatos de especificacao nesta etapa; implementacao de codigo fica condicionada a confirmacao posterior do usuario.~~ *[Superseded 2026-05-25: implementacao aprovada e concluida — ver tasks.md. Pendentes apenas as validacoes manuais T036 e T037.]*
- **FR-011**: Separacao de papeis por aplicacao DEVE ser explicita: app de banco cobre jornada de banco comercial (quote/swap/transferencia) sem controles de governanca; app de governanca cobre fluxo soberano CB e controles de circuito sem formulario de swap comercial.
- **FR-028**: O sistema DEVE expor endpoint `GET /api/v2/amm/liquidity/positions` para consulta de posições LP filtradas por `pool_pair` (obrigatório) e `provider_id` (opcional), retornando lista com `lp_id`, `provider_bank_id`, `pool_pair`, `token_a_contributed`, `token_b_contributed`, `lp_shares`, `status`, `deposit_side`, `added_at`, `commit_id`. Cada CB gateway retorna apenas posições locais (arquitetura descentralizada).
- **FR-029**: Os endpoints `/api/v2/bridge/lock-mint` e `/api/v2/amm/liquidity/commit` DEVEM derivar campos sensíveis (`owner_bank_id`, `provider_id`, `side`, `spoke_network`, `native_asset`, `mirrored_asset`, `w_token_address`) a partir do JWT token e configuração do servidor, reduzindo payload do cliente em 70-80% e eliminando vetores de ataque por manipulação de campos controlados pelo servidor.

### Non-Functional Requirements

- **NFR-001**: As mensagens de status e erro devem ser compreensiveis para operadores nao tecnicos, com acao recomendada explicita em cada falha operacional comum.
- **NFR-002**: O fluxo principal de provisao soberana deve poder ser entendido e executado por um operador em ate 5 minutos, considerando ambiente ja autenticado.
- **NFR-003**: O mapeamento de estados e erros deve ser consistente entre telas de banco e governanca, evitando interpretacoes divergentes para o mesmo evento.
- **NFR-004**: A compatibilidade com Scenario A deve ter regressao zero de navegacao e visibilidade de funcionalidades.
- **NFR-005**: A UX de acompanhamento do fluxo soberano deve refletir a janela operacional do watcher: aviso de latencia quando `COMMIT_PENDING` ultrapassar 30s e timeout de UI em 60s sem perder rastreabilidade do commit.

### Key Entities *(include if feature involves data)*

- **FluxoSoberanoCB**: Jornada operacional de provisao de liquidez de Banco Central com fases `LOCK_MINT`, `BRIDGE_ACTIVE`, `COMMIT_PENDING/EXECUTED` e `POOL_ACTIVE`.
- **EstadoBridgePosicao**: Estado operacional de uma posicao de bridge usada como pre-condicao para avancar no fluxo de liquidez.
- **OperacaoSwapComercial**: Jornada de cotacao, validacao de limite e execucao de swap por banco comercial.
- **ErroOperacionalScenarioB**: Evento de falha padronizado em UX para orientar acao do operador (`CROSS_CB_MINT_PROHIBITED`, `BRIDGE_POSITION_NOT_ACTIVE`, `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `CIRCUIT_BREAKER_HALTED`).
- **ModoCenarioAplicacao**: Estado de habilitacao da aplicacao entre Scenario A e Scenario B para controle de acesso a rotas/fluxos.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% dos fluxos de provisao de liquidez para CB no frontend seguem o caminho soberano e nao exibem o fluxo G5-cross em ambiente Scenario B atualizado. Validação: execução bem-sucedida de `tryouts/tryout-sovereign-cb-liquidity.sh` com pool ACTIVE e LP positions criadas.
- **SC-002**: 100% dos erros `CROSS_CB_MINT_PROHIBITED` e `BRIDGE_POSITION_NOT_ACTIVE` exibem mensagem acionavel correta e acao recomendada sem necessidade de consulta a logs tecnicos.
- **SC-003**: Operadores de banco comercial concluem o fluxo de cotacao + swap com taxa de sucesso de primeira tentativa >= 90% em ambiente de homologacao com pool ativo.
- **SC-004**: Em validacoes de navegacao, 0 rotas exclusivas de Scenario B ficam acessiveis quando o ambiente estiver em Scenario A.
- **SC-005**: Tempo medio para operador de CB identificar motivo de bloqueio de etapa (ex.: aguardando confirmacao do bridge) fica <= 30 segundos apos retorno de erro.
- **SC-006**: 100% das telas de swap bloqueiam submissao quando `pool_status != ACTIVE` e exibem o motivo correspondente (`POOL_NOT_ACTIVE` ou `CIRCUIT_BREAKER_HALTED`).
- **SC-007**: Nenhuma tela do app de banco exibe controles de governanca e nenhuma tela do app de governanca exibe formulario de swap comercial.

## Assumptions

- O runbook de integracao Scenario B v6.0 e os tryouts oficiais representam a fonte de verdade para estados e erros nesta feature.
- Os fluxos soberanos e os bloqueios anti-G5-cross ja estao disponiveis no backend alvo e nao serao reespecificados em detalhe tecnico nesta etapa.
- Os aplicativos de banco e governanca sao os unicos alvos de mudanca funcional para este alinhamento.
- O controle de acesso por perfil de usuario (CB e banco comercial) permanece vigente e consistente com o ambiente atual.
- O mapeamento de erros e estados desta feature segue os codigos canonicamente documentados na secao 16 e na secao 19 do runbook de integracao.
- A implementacao de codigo (frontend + extensões mínimas de backend API) foi confirmada e concluída entre 2026-05-25 e 2026-05-26. Escopo final inclui: frontend alignment (US1/US2/US3), endpoint LP positions (FR-028), API simplificada (FR-029), sincronização de `.env.example` templates e documentação de integração frontend. Pendentes apenas as validações manuais T036 e T037.

## Out of Scope

- Alteracoes de implementacao em microservicos backend, contratos ou infraestrutura.
- Introducao de novos fluxos de negocio fora do Scenario B ja documentado.
- Reprojeto visual amplo das aplicacoes que nao esteja ligado ao alinhamento funcional de fluxo/status/erro.
- Mudancas no comportamento funcional de Scenario A alem da preservacao de compatibilidade e gating.
- Execucao de desenvolvimento de novas funcionalidades alem do escopo de alinhamento definido (FR-001 a FR-011).
