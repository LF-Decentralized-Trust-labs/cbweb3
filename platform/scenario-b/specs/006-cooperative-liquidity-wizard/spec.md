# Feature Specification: Scenario B Full Frontend Integration

**Feature Branch**: `006-cooperative-liquidity-wizard`
**Created**: 2026-05-15
**Updated**: 2026-05-20 — scope expanded to Scenario B Full Frontend Integration (FR-018 breaking payload changes + bank app flows)
**Status**: Spec Updated — re-plan required

## Contexto e Motivação

O feature `005-cooperative-liquidity` implementou no backend o modelo de **Provisão Cooperativa de Liquidez via Commit-Reveal**: dois Bancos Centrais independentes depositam cada um a sua própria moeda digital para formar um pool AMM, sem que nenhum deles precise deter a moeda do outro.

O frontend atual (`LiquidityManagementPage`) possui um formulário de "Add Liquidity" **quebrado e conceitualmente incorreto**: ele exige que um único provedor forneça os valores de Token A e Token B simultaneamente — violando o princípio de soberania monetária que o modelo cooperativo resolve. Esse formulário reflete o modelo antigo e precisa ser substituído por uma UX que guie cada Banco Central pelo fluxo cooperativo correto.

A versão inicial deste feature entregou um **wizard multi-etapas** (`CooperativeLiquidityWizard`) embutido na `LiquidityManagementPage`, substituindo o formulário de Add Liquidity quebrado.

**Expansão de escopo (v4.0 do runbook — 2026-05-20)**: O runbook `integracao-scenario-b-2.md` foi atualizado para a versão 4.0, introduzindo mudanças **breaking** nos payloads das APIs de token (`mint-and-approve` e `approve-amm`) — denominadas FR-018 — e detalhando os payloads completos esperados pelos fluxos de depósito fiat, tokenização (escrow) e saque (redeem) no **bank app**. Esses endpoints já estão no ar no backend e retornam HTTP 400 com `DEPRECATED_FIELDS` para payloads antigos. Este feature também resolve a integração completa do bank app com os payloads corretos, e adiciona o sub-fluxo de preparação G5-cross no wizard da aplicação de governança.

---

## Clarifications

### Session 2026-05-15

- Q: O wizard deve suportar operadores de ambos os CBs (Central Bank A e Central Bank B) na mesma instância da aplicação, ou cada instância serve apenas um CB? → A: Cada instância da aplicação de governança serve um único CB (a identidade do provedor é configurada por variável de ambiente ou pelo contexto de autenticação). O campo `provider_id` no Step 2 é pré-preenchido com o ID do CB logado, mas editável para permitir uso em demonstrações multi-tenant (tryouts).

- Q: O wizard deve persistir o estado entre recargas de página (ex.: se o operador fechar e reabrir enquanto o commit está PENDING)? → A: Não — o estado do wizard é sessão-only (Zustand sem persistência), consistente com o padrão atual do projeto. Ao reabrir a página, o usuário parte do Step 1; porém, o Step 3 (Monitor) deve poder ser acessado diretamente se já houver um commit PENDING detectado pela polling de `GET /api/v2/amm/pool/{pair}/status`.

- Q: Quando o commit retorna `status: "EXECUTED"` imediatamente (match instantâneo), o wizard deve saltar Steps 3 e ir direto ao Step 4? → A: Sim — se a resposta do commit já contiver `status: "EXECUTED"` e `lp_ids` populados, o wizard salta diretamente para o Step 4 (Success), exibindo os LP IDs recém-criados.

- Q: Quando `pending_commits` contém múltiplas entradas, como o banner (User Story 5) identifica qual commit pertence ao operador atual? → A: O banner filtra `pending_commits` comparando o campo `provider_id` de cada entrada com o `provider_id` do usuário autenticado (obtido do auth store). Se nenhuma entrada corresponder ao operador atual, o banner não é exibido mesmo que `pool_status` seja `PENDING_COUNTERPART`. Quando uma entrada corresponde, o wizard abre no Step 3 pré-populado com os dados desse commit.

- Q: Quando o wizard avança para o Step 4 via polling (sem ter recebido uma resposta `EXECUTED` direta), e `activeCommit.lp_ids` é null, como a seção de LP IDs deve renderizar? → A: O Step 4 exibe LP IDs de `activeCommit.lp_ids` quando disponíveis (não-null). Se `activeCommit` for null ou `lp_ids` for null, a seção exibe o fallback: "LP IDs not available in current session — check the LP Positions table below." Reservas e estatísticas do pool são sempre exibidas. Isso alinha com o edge case existente sobre `pool_status: "ACTIVE"` sem `pending_commits`.

- Q: O método `listCommits(poolPair, status?)` definido em FR-009 é utilizado por algum componente do wizard ou é reservado para uso futuro? → A: `listCommits` não é utilizado por nenhum step do wizard neste feature — a detecção de commits pendentes para o banner usa `pool_status.pending_commits` retornado por `fetchPoolStatus`. O método é implementado no módulo de API como utilitário mas não é conectado (wired) a nenhum componente do wizard nesta entrega.

- Q: O campo do payload de remoção de liquidez é `provider_id` ou `provider_bank_id` (conforme mencionado no User Story 4, cenário 2)? → A: `provider_id` é o nome canônico, consistente com todos os demais payloads da API. A referência a `provider_bank_id` no User Story 4, cenário 2 é um erro de digitação e foi corrigida na seção correspondente.

- Q: Qual é o mecanismo concreto pelo qual o wizard obtém o valor inicial de `provider_id` para pré-preencher o campo no Step 2? → A: O wizard lê o `provider_id` do auth store existente na aplicação de governança (o mesmo mecanismo usado pelos demais formulários de governança). Variáveis de ambiente configuram URLs de API, não identidade de usuário. O campo permanece editável para cenários de tryout e demonstração.

### Session 2026-05-20 — Expansão FR-018 e Bank App

- Q: O campo `recipient` no `mint-and-approve` do wizard é exibido sempre ou apenas no modo G5-cross? → A: O campo `recipient` permanece no Step 1 como campo opcional visível (comportamento original preservado). O sub-fluxo G5-cross é um bloco colapsável adicional no Step 1, ativado por um toggle "Is this a fresh environment?" — quando expandido, exibe os dois passos G5.1 e G5.2 como ações distintas, antes de prosseguir com o commit.

- Q: No G5-cross (G5.1), qual é o valor do campo `recipient` usado pelo CB-B? → A: O endereço do signer do CB-A — visualmente apresentado como "CB-A Signer Address" e preenchido manualmente pelo operador (não há uma fonte automática no frontend para obter o endereço do signer do CB parceiro). O campo aceita input livre no formato `0x...` com validação de endereço hexadecimal.

- Q: O G5.2 (`approve-amm` com `side: "B"` executado pelo CB-A) reutiliza o mesmo formulário do Step 1 ou é uma ação separada? → A: É uma ação separada dentro do bloco G5-cross colapsável no Step 1 — um botão "Approve TOKEN_B for AMM" que chama a nova action `approveAmm(amount, "B")` do `liquidity.store.ts`. O operador preenche o `amount` do G5-cross uma única vez e usa o mesmo valor para G5.1 e G5.2.

- Q: No bank app, onde o frontend obtém `requester_besu_address` e `requester_paladin_identity` para os payloads de depósito e saque? → A: `requester_besu_address` é o `wallet_address` armazenado no auth store a partir do resultado do onboarding (campo `wallet_address` da response de `/onboarding/complete`). `requester_paladin_identity` é lido da variável de ambiente `VITE_PALADIN_IDENTITY` (ex.: `"bank-a@node1"`) — configurada por instância, não armazenada em estado de sessão.

- Q: No `EscrowsPage`, de onde o frontend obtém o `deposit_id` para o payload `{deposit_id}`? → A: O frontend lê o `deposit_id` do depósito mais recentemente aprovado. Estratégia: o `payment.store.ts` lista depósitos via `GET /deposit/list` e expõe o ID do primeiro depósito com `status: "APPROVED"` que ainda não possui escrow associado. O campo é pré-preenchido e editável.

- Q: O toggle de `zeto_transfer_tx_hash` no `RedeemsPage` — quando ativado pelo usuário e depois desativado, o campo deve ser limpo? → A: Sim — ao desativar o toggle, o campo `zeto_transfer_tx_hash` é limpo e o campo não é enviado no payload (omitido). Quando ativado, o campo torna-se obrigatório e o botão de submit permanece desabilitado até que um valor seja inserido.

- Q: O `pool_status` gate no `AMMTradingPage` deve desabilitar apenas o botão de swap ou toda a seção (incluindo quote)? → A: Apenas o botão de submit do swap é desabilitado. O formulário de cotação (quote) permanece ativo — o operador pode ver a cotação mesmo com o pool `EMPTY` ou `PENDING_COUNTERPART`, mas ao tentar submeter o swap verá a mensagem inline "Pool is not active. Swaps are currently unavailable."

### Session 2026-05-20 — Rotas de Aprovação de Pagamento (Governance CB, Scenario A)

- Q: Quais rotas backend o módulo `governance/src/services/api/payment.api.ts` chama para aprovar, rejeitar e iniciar a conversão fiat de depósitos, escrows e redeems? → A: O módulo usa 7 rotas `POST /api/v1/payments/*` no api-gateway do CB, autenticadas via cookie (`RequireCookieAuth`). Essas rotas não estavam registradas no `router.go` até 2026-05-20 e retornavam HTTP 404 — adicionadas nesta branch. Os handlers já existiam em `payment.go` (`ApproveDeposit`, `RejectDeposit`, `RequestFiatExchange`, `ApproveEscrow`, `RejectEscrow`, `ApproveRedeem`, `RejectRedeem`); apenas o registro no router estava faltando.
- Q: Essas rotas de aprovação do CB fazem parte do Scenario A ou Scenario B? → A: Scenario A — são rotas `/api/v1/` (não `/api/v2/`). Elas suportam as páginas de governança de pagamento da governance app (`DepositsApprovalPage`, `EscrowsApprovalPage`, `RedeemsApprovalPage`) que estão ativas quando `VITE_SCENARIO=a`. O spec 004 FR-020 menciona que essas páginas ficam inacessíveis no Scenario B, mas não documentou as rotas backend que elas precisam; esta sessão preenche esse gap.
- Q: Os caminhos canônicos das rotas de pagamento do bank app são `/api/v1/payment/deposit/register`, `/api/v1/payment/exchange/request` e `/api/v1/payment/redeem/request`? → A: **Não** — esses caminhos nunca existiram no `router.go`. Os caminhos canônicos são `POST /api/v1/payments/deposits` (plural, sem sufixo `/register`), `POST /api/v1/payments/escrows` (não `/exchange/request`) e `POST /api/v1/payments/redeems` (plural, sem sufixo `/request`). Os User Stories 6 e 7 usavam os caminhos incorretos; foram corrigidos nesta sessão (ver C1 do analyze report de 2026-05-20).
- Q: As 7 rotas de aprovação devem ser também espelhadas no grupo `/internal/v1/`? → A: Sim — para consistência com as demais rotas de pagamento do api-gateway CB, as 7 rotas foram espelhadas no grupo relay `/internal/v1/` (protegido por `RequireRelayAuth` / cabeçalho `X-Relay-Auth`). Caso de uso atual: interno/reservado; sem frontend conhecido que acione aprovação via relay neste momento.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Operador do CB-A Executa o Fluxo Completo (Priority: P1)

Um operador do Banco Central A (BCB — lado BRL) acessa a página de gestão de liquidez, realiza o mint & approve dos seus tokens, registra o commit do lado A, e aguarda o pool ser ativado pelo commit do CB-B. Ao final, o wizard exibe os LP IDs e a opção de remover liquidez.

**Por que esta prioridade**: É o caso de uso principal e o mais frequente. Valida o fluxo completo end-to-end do wizard no ponto de vista do primeiro CB a commitar. Sem este user story funcionar, nenhum outro faz sentido.

**Independent Test**: Pode ser testado em ambiente isolado com mock do backend: o operador preenche Step 1 (mint & approve), avança para Step 2 (commit), recebe resposta `PENDING`, entra no Step 3 (monitor) onde a polling simula o pool tornando-se `ACTIVE` após 5s, e então Step 4 exibe os LP IDs.

**Acceptance Scenarios**:

1. **Given** o operador está na `LiquidityManagementPage` e o pool BRL-USD está em estado `EMPTY`, **When** o usuário clica em "Add Cooperative Liquidity", **Then** o wizard é exibido no Step 1 (Mint & Approve) com um único campo `amount` (o campo `recipient` é opcional), e o Step atual é visualmente destacado no stepper.

2. **Given** o wizard está no Step 1, **When** o operador preenche `amount = 200000` e submete, **Then** o sistema chama `POST /api/v2/amm/token/mint-and-approve` com payload `{amount: "200000"}` e, em caso de sucesso, avança automaticamente para o Step 2.

3. **Given** o wizard está no Step 2 (Commit), **When** o operador preenche `pool_pair = BRL-USD`, `provider_id = central-bank-a`, `side = A`, `amount = 100000` e submete, **Then** o sistema chama `POST /api/v2/amm/liquidity/commit` e, se a resposta retornar `status: "PENDING"`, o wizard avança para o Step 3.

4. **Given** o wizard está no Step 3 (Monitor), **When** a polling de `GET /api/v2/amm/pool/BRL-USD/status` retorna `pool_status: "ACTIVE"`, **Then** o wizard avança automaticamente para o Step 4 (Success) sem intervenção manual do operador.

5. **Given** o wizard está no Step 4 (Success), **Then** o usuário vê os LP IDs associados ao seu depósito, as reservas atuais do pool (reserve_a, reserve_b, current_ratio), e um botão "Add More Liquidity" que reinicia o wizard no Step 1.

6. **Given** o wizard está no Step 2, **When** a chamada a `POST /api/v2/amm/liquidity/commit` falha com HTTP 409 (`SAME_PROVIDER_BOTH_SIDES`), **Then** o wizard exibe uma mensagem de erro explicativa no Step 2 sem avançar de etapa.

---

### User Story 2 — Match Instantâneo (Commit Retorna EXECUTED) (Priority: P1)

Quando o CB-B já possui um commit PENDING aguardando, e o CB-A registra o seu commit, o sistema executa o match imediatamente e retorna `status: "EXECUTED"`. O wizard deve tratar este caso saltando o Step 3 e indo diretamente para o Step 4.

**Por que esta prioridade**: Cenário muito comum em tryouts e ambientes de desenvolvimento onde ambos os CBs são operados em sequência rápida. Sem este comportamento, o operador ficaria preso no Step 3 (monitor) mesmo com o pool já ativo.

**Independent Test**: Mockar o endpoint `POST /api/v2/amm/liquidity/commit` para retornar `{ status: "EXECUTED", lp_ids: ["lp-a", "lp-b"] }` e verificar que o wizard salta diretamente do Step 2 para o Step 4 (Success) com os `lp_ids` exibidos corretamente.

**Acceptance Scenarios**:

1. **Given** o wizard está no Step 2 e existe um commit PENDING do lado oposto no pool, **When** o operador submete o commit e a resposta retorna `status: "EXECUTED"` e `lp_ids` populados, **Then** o wizard salta diretamente para o Step 4, sem exibir o Step 3.

2. **Given** o wizard está no Step 4 após match instantâneo, **Then** o Step 4 exibe ambos os LP IDs retornados em `lp_ids[0]` e `lp_ids[1]`, com label indicando qual pertence a cada CB (index 0 = CB-A, index 1 = CB-B).

---

### User Story 3 — Monitoramento e Cancelamento de Commit Pendente (Priority: P2)

Após registrar um commit, o operador aguarda no Step 3. Ele pode ver o tempo restante até a expiração do commit (72h), o status atual do pool, e tem a opção de cancelar o commit caso queira desistir.

**Por que esta prioridade**: Essencial para a operabilidade real — o prazo de 72h pode expirar, os CBs podem mudar de plano, e o operador precisa ter visibilidade e controle sobre o commit pendente.

**Independent Test**: Mockar a polling de pool status retornando `pool_status: "PENDING_COUNTERPART"` com um commit com `expires_at` fixo. Verificar que o countdown exibe a diferença correta. Clicar em "Cancel Commit" e verificar que `DELETE /api/v2/amm/liquidity/commits/:commit_id` é chamado, o wizard volta ao Step 1 e uma mensagem de cancelamento é exibida.

**Acceptance Scenarios**:

1. **Given** o wizard está no Step 3 com um commit PENDING, **Then** a página exibe o `commit_id`, o lado commitado (A ou B), o valor (`amount`), e um countdown formatado (ex.: "71h 42m restantes") calculado a partir do campo `expires_at` do commit.

2. **Given** o wizard está no Step 3, **When** o operador clica em "Cancel Commit", **Then** o sistema chama `DELETE /api/v2/amm/liquidity/commits/:commit_id?provider_id=...` e, em caso de sucesso, exibe uma mensagem de cancelamento e retorna o wizard ao Step 1.

3. **Given** o wizard está no Step 3, **When** a polling retorna `pool_status: "EMPTY"` (indicando que o commit expirou ou foi cancelado pela contraparte), **Then** o wizard exibe uma mensagem informando que o commit expirou e oferece opção de reiniciar o fluxo a partir do Step 1.

4. **Given** o wizard está no Step 3, **When** a polling retorna `pool_status: "PENDING_COUNTERPART"` com `pending_commits` listando o commit do operador, **Then** o wizard exibe os detalhes do commit pendente (lado, valor, expiração) e mantém o operador no Step 3.

5. **Given** o wizard está no Step 3 e o cancelamento falha com erro do servidor, **Then** o wizard exibe o erro inline no Step 3 sem navegar para outro step.

---

### User Story 4 — Operador Vê Pool Ativo e Remove Liquidez (Priority: P2)

No Step 4, após o pool tornar-se ACTIVE, o operador pode visualizar os detalhes do pool (reservas, ratio, taxa) e os seus LP IDs. A partir daí, pode remover liquidez diretamente, ou fechar o wizard e usar a tabela de LP Positions existente.

**Por que esta prioridade**: Fecha o ciclo de vida do LP para o operador — ele precisa saber que o fluxo foi concluído com sucesso e ter acesso imediato à ação de remoção de liquidez.

**Independent Test**: Com Step 4 renderizado com dados mockados (`reserve_a = 100000`, `reserve_b = 100000`, `lp_ids = ["lp-a", "lp-b"]`), verificar que o formulário de "Remove Liquidity" pré-preenche `lp_id` e `provider_id` corretamente, e que ao submeter chama `POST /api/v2/amm/liquidity/remove`.

**Acceptance Scenarios**:

1. **Given** o wizard está no Step 4, **Then** o painel de sucesso exibe `reserve_a`, `reserve_b`, `current_ratio`, `fee_rate_bps` e `total_lp_count` do pool.

2. **Given** o wizard está no Step 4 com LP IDs disponíveis, **When** o operador clica em "Remove Liquidity" para um LP ID específico, **Then** o formulário de remoção é pré-preenchido com `lp_id` e `provider_id`, e ao confirmar o sistema chama `POST /api/v2/amm/liquidity/remove`.

3. **Given** o wizard está no Step 4, **When** o operador clica em "Add More Liquidity", **Then** o wizard reinicia no Step 1 com todos os campos limpos, mantendo o estado do pool atualizado no painel lateral.

---

### User Story 5 — Detecção de Commit Pendente ao Abrir a Página (Priority: P3)

Ao abrir a `LiquidityManagementPage`, se a polling de pool status retornar `pool_status: "PENDING_COUNTERPART"` com commits pendentes, o sistema deve oferecer ao operador a opção de retomar o monitoramento no Step 3 sem precisar refazer os Steps 1 e 2.

**Por que esta prioridade**: Melhora a resiliência da UX — o operador pode ter realizado o commit em uma sessão anterior e reaberto a página para acompanhar. Sem isso, o operador não tem visibilidade sobre o commit pendente ao retornar à página.

**Independent Test**: Renderizar a `LiquidityManagementPage` com pool status mockado retornando `pool_status: "PENDING_COUNTERPART"` e um commit pendente. Verificar que um banner ou prompt é exibido perguntando se o operador deseja monitorar o commit pendente, e que ao confirmar o wizard abre diretamente no Step 3.

**Acceptance Scenarios**:

1. **Given** a `LiquidityManagementPage` é carregada e a polling detecta `pool_status: "PENDING_COUNTERPART"` com commits na lista `pending_commits`, **Then** um banner informativo é exibido indicando que existe um commit pendente, com botão "Monitor Pending Commit" que abre o wizard no Step 3.

2. **Given** o banner de commit pendente está visível, **When** o operador fecha o banner, **Then** o banner desaparece e o operador pode usar a página normalmente (o wizard não abre automaticamente).

---

### User Story 6 — Operador do Banco Comercial Realiza Depósito Fiat com Payload Completo (Priority: P1)

Um operador de banco comercial (Bank A) registra um depósito fiat e solicita a tokenização (escrow) para receber tCeBM. O frontend envia os payloads corretos conforme o runbook v4.0: `{requester_besu_address, requester_paladin_identity, amount}` para o depósito e `{deposit_id}` para o escrow.

**Por que esta prioridade**: Sem os campos completos no payload de depósito, o backend retorna HTTP 400 ou cria um registro incompleto que bloqueia o escrow. É o ponto de entrada da liquidez do banco comercial no sistema.

**Independent Test**: Mockar auth store com `wallet_address = "0xABCD..."` e `VITE_PALADIN_IDENTITY = "bank-a@node1"`. Submeter o formulário de depósito e verificar que o payload enviado inclui `requester_besu_address`, `requester_paladin_identity` e `amount`. Após depósito `APPROVED`, submeter escrow e verificar que o payload é `{deposit_id: "dep-xxx"}`.

**Acceptance Scenarios**:

1. **Given** o operador está em `DepositsPage` e o auth store possui `wallet_address`, **When** o operador preenche o valor do depósito e submete, **Then** o sistema chama `POST /api/v1/payments/deposits` com payload `{requester_besu_address: <wallet_address do store>, requester_paladin_identity: <VITE_PALADIN_IDENTITY>, amount: <valor>}`.

2. **Given** o operador está em `EscrowsPage` e existe um depósito com `status: "APPROVED"` listado, **When** o operador seleciona o depósito e submete a solicitação de tokenização, **Then** o sistema chama `POST /api/v1/payments/escrows` com payload `{deposit_id: <id do depósito selecionado>}` — não mais `{amount}`.

3. **Given** o escrow recebe `status: "APPROVED"` (tCeBM mintados), **Then** a `EscrowsPage` exibe um banner: "Your tCeBM has been issued. Next step: Approve AMM spending" com link navegando para a `AMMTradingPage`.

4. **Given** o auth store não possui `wallet_address` (onboarding não concluído), **When** o operador acessa `DepositsPage`, **Then** o formulário exibe aviso "Onboarding incompleto — wallet address não disponível" e o botão de submit permanece desabilitado.

---

### User Story 7 — Operador do Banco Comercial Realiza Saque (Redeem) com Payload Completo e Toggle Zeto (Priority: P1)

Um operador do banco comercial solicita resgate de tCeBM para fiat com o payload correto: `{requester_besu_address, requester_paladin_identity, amount, zeto_transfer_tx_hash?}`. Para tokens recebidos via transferência privada Zeto, o operador ativa um toggle para incluir o hash da transação Zeto.

**Por que esta prioridade**: Sem os campos obrigatórios no payload, o backend retorna HTTP 400. O campo `zeto_transfer_tx_hash` é condicional — omiti-lo quando necessário causa rejeição com `MISSING_ZETO_PROOF`; incluí-lo sem necessidade é inofensivo mas desconcertante.

**Independent Test**: Mockar auth store com `wallet_address` e `VITE_PALADIN_IDENTITY`. Submeter redeem sem toggle e verificar payload sem `zeto_transfer_tx_hash`. Ativar toggle, preencher hash e verificar que o payload inclui o campo. Desativar toggle após preenchido e verificar que o campo é removido e o valor limpo.

**Acceptance Scenarios**:

1. **Given** o operador está em `RedeemsPage`, **When** preenche o valor do saque e submete sem ativar o toggle Zeto, **Then** o sistema chama `POST /api/v1/payments/redeems` com payload `{requester_besu_address, requester_paladin_identity, amount}` (sem `zeto_transfer_tx_hash`).

2. **Given** o operador ativa o toggle "My tokens came from a Zeto private transfer", **Then** um campo de input `zeto_transfer_tx_hash` é exibido, o botão de submit permanece desabilitado até que o campo seja preenchido, e o campo aceita apenas strings iniciando com `0x`.

3. **Given** o toggle Zeto está ativo e o campo `zeto_transfer_tx_hash` está preenchido, **When** o operador submete, **Then** o payload inclui `zeto_transfer_tx_hash` com o valor informado.

4. **Given** o toggle Zeto está ativo e preenchido, **When** o operador desativa o toggle, **Then** o campo `zeto_transfer_tx_hash` desaparece, seu conteúdo é limpo, e o próximo submit não incluirá o campo.

---

### User Story 8 — Operador do Banco Comercial Aprova AMM com Side Explícito (Priority: P1)

Um operador do banco comercial usa a `AMMTradingPage` para autorizar o contrato AMM a gastar seus tokens tCeBM. O formulário de approve-amm agora exige um único campo `amount` e um seletor de `side` ("A" ou "B"), substituindo os dois campos `approveAmountA` / `approveAmountB` antigos. O pool status `ACTIVE` é verificado antes de habilitar o swap.

**Por que esta prioridade**: O campo `side` é obrigatório para bancos comerciais (FR-018); sem ele o backend retorna HTTP 400 `SIDE_REQUIRED`. Os campos depreciados retornam HTTP 400 `DEPRECATED_FIELDS`. Além disso, permitir swap com pool `EMPTY` geraria erro de backend que é melhor prevenir no frontend.

**Independent Test**: Renderizar `AMMTradingPage` com pool status mockado `EMPTY`. Verificar que o botão de swap está desabilitado com mensagem inline. Mudar mock para `ACTIVE` e verificar que o botão é habilitado. Submeter approve-amm com `amount = "50000"` e `side = "A"` e verificar que o payload é `{amount: "50000", side: "A"}` (sem `amount_a`/`amount_b`).

**Acceptance Scenarios**:

1. **Given** o operador está na seção de approve-amm da `AMMTradingPage`, **Then** o formulário exibe um único campo `amount` e um seletor de `side` (opções "A" e "B"), sem os campos `approveAmountA` / `approveAmountB`.

2. **Given** o operador preenche `amount = "50000"` e seleciona `side = "A"`, **When** submete, **Then** o sistema chama `POST /api/v2/amm/token/approve-amm` com payload `{amount: "50000", side: "A"}`.

3. **Given** a `AMMTradingPage` obtém pool status com `pool_status: "EMPTY"` ou `pool_status: "PENDING_COUNTERPART"`, **Then** o botão de submit do swap é desabilitado e exibe mensagem inline "Pool is not active. Swaps are currently unavailable." O formulário de cotação (quote) permanece ativo.

4. **Given** a `AMMTradingPage` obtém pool status com `pool_status: "ACTIVE"`, **Then** o botão de submit do swap é habilitado normalmente.

5. **Given** o operador submete approve-amm sem selecionar `side`, **Then** o botão de submit permanece desabilitado — o campo `side` é validado como obrigatório no frontend antes de qualquer chamada de API.

---

### User Story 9 — Operador do CB Executa Sub-Fluxo G5-Cross no Wizard (Priority: P2)

Em um ambiente recém-iniciado (fresh environment), antes do commit-reveal, o CB-B precisa mintar TOKEN_B na carteira do signer do CB-A, e o CB-A precisa aprovar o AMM para gastar esse TOKEN_B. O wizard Step 1 oferece um bloco colapsável "G5-cross Setup" para guiar o operador por esses dois passos preparatórios.

**Por que esta prioridade**: Sem o G5-cross em ambientes frescos, o `executeMatchedCommits` no backend reverte com `ERC20: insufficient allowance`. O feature é acionado opcionalmente (toggle "Is this a fresh environment?") para não poluir o fluxo principal em ambientes já configurados.

**Independent Test**: Expandir o bloco G5-cross no Step 1. Preencher G5-cross `amount = "200000"` e `cb_a_signer_address = "0xABCD..."`. Clicar "Mint TOKEN_B to CB-A" (G5.1) e verificar que chama `POST /amm/token/mint-and-approve` via gateway CB-B com payload `{amount, recipient}`. Clicar "Approve TOKEN_B for AMM" (G5.2) e verificar que chama `POST /amm/token/approve-amm` com payload `{amount, side: "B"}`.

**Acceptance Scenarios**:

1. **Given** o wizard está no Step 1, **When** o operador ativa o toggle "Is this a fresh environment?", **Then** um bloco colapsável "G5-cross Setup" é exibido abaixo do formulário principal, com campo `amount` e campo `cb_a_signer_address` (endereço blockchain do signer de CB-A), e dois botões de ação: "Mint TOKEN_B to CB-A" (G5.1) e "Approve TOKEN_B for AMM (side B)" (G5.2).

2. **Given** o bloco G5-cross está expandido, **When** o operador preenche `amount` e `cb_a_signer_address` e clica em "Mint TOKEN_B to CB-A" (G5.1), **Then** o sistema chama `POST /api/v2/amm/token/mint-and-approve` com payload `{amount, recipient: <cb_a_signer_address>}`. Em caso de sucesso, o botão G5.1 exibe estado "Done" e o G5.2 fica habilitado.

3. **Given** G5.1 foi executado com sucesso, **When** o operador clica em "Approve TOKEN_B for AMM (side B)" (G5.2), **Then** o sistema chama `POST /api/v2/amm/token/approve-amm` com payload `{amount, side: "B"}`. Em caso de sucesso, o botão G5.2 exibe estado "Done" e o wizard instrui o operador a prosseguir com o Step 1 principal (mint & approve do seu próprio token).

4. **Given** o campo `cb_a_signer_address` contém um valor com formato inválido (não é `0x` + 40 hex chars), **Then** os botões G5.1 e G5.2 permanecem desabilitados e uma mensagem de validação "Invalid Ethereum address" é exibida inline.

5. **Given** o toggle G5-cross está ativo, **When** o operador desativa o toggle, **Then** o bloco G5-cross é recolhido e o estado dos passos G5.1/G5.2 é descartado — o wizard não bloqueia o prosseguimento do fluxo principal por causa do estado G5-cross.

- O que acontece se o operador tentar submeter o Step 1 com `amount = 0` ou `amount` vazio? O campo `amount` é obrigatório e o frontend deve validar que o valor é um número inteiro positivo (string representando inteiro > 0) antes de submeter. O wizard não envia a requisição e exibe validação inline.
- O que acontece se no G5-cross o campo `recipient` no G5.1 contiver um endereço inválido (formato incorreto)? O frontend valida que o endereço segue o padrão hexadecimal `0x[40 hex chars]` antes de habilitar o botão de submit do G5.1. Endereço inválido bloqueia o envio com mensagem inline "Invalid Ethereum address".
- O que acontece se a polling do Step 3 falhar repetidamente (erro de rede)? O wizard exibe um indicador de erro de polling com botão de retry manual, sem sair do Step 3 automaticamente.
- O que acontece se o operador recarregar a página no meio do Step 3? O estado do wizard é perdido (session-only, sem persistência). Ao recarregar, a polling detecta `PENDING_COUNTERPART` e exibe o banner de retomada (User Story 5).
- O que acontece se o wizard estiver no Step 3 e o backend retornar `pool_status: "ACTIVE"` mas sem `pending_commits` para o provedor atual? O wizard deve avançar para o Step 4 mesmo sem `lp_ids` locais — o Step 4 exibe os dados do pool e instrui o operador a consultar a tabela de LP Positions para seus IDs.
- O que acontece se o usuário tentar submeter o Step 2 enquanto já existe um commit PENDING do mesmo provedor no mesmo par? O backend retorna HTTP 409 e o frontend exibe a mensagem de erro com link para o Step 3 (monitorar o commit existente).
- O que acontece se `expires_at` do commit já passou quando o operador está no Step 3? O countdown exibe "Expirado" e o sistema instrui o operador a reiniciar o fluxo — o commit terá sido cancelado automaticamente pelo backend.
- O que acontece no bank app se `DepositsPage` enviar o payload antigo sem `requester_besu_address`? O backend retorna HTTP 400. Após este feature, o frontend sempre inclui os campos completos; caso o auth store não possua `wallet_address` (onboarding incompleto), o formulário exibe um aviso "Onboarding incompleto — wallet address não disponível" e desabilita o submit.
- O que acontece se `AMMTradingPage` tentar submeter swap quando `pool_status` for `EMPTY` ou `PENDING_COUNTERPART`? O botão de swap permanece desabilitado e exibe mensagem inline "Pool is not active. Swaps are currently unavailable." — a chamada de swap nunca é disparada pelo frontend neste estado.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE exibir um wizard multi-etapas (`CooperativeLiquidityWizard`) com 4 steps visualmente distintos e um stepper indicador de progresso: (1) Mint & Approve, (2) Commit, (3) Monitor, (4) Success. O wizard DEVE substituir o formulário "Add Liquidity" atual na `LiquidityManagementPage`.

- **FR-002**: O Step 1 (Mint & Approve) DEVE coletar um único campo `amount` e `recipient` (opcional), chamar `POST /api/v2/amm/token/mint-and-approve` com payload `{amount, recipient?}`, e avançar automaticamente para o Step 2 em caso de sucesso. Os campos `amount_a` e `amount_b` são REMOVIDOS — o gateway auto-detecta qual token mintar via `CENTRAL_BANK_ROLE` on-chain.

- **FR-003**: O Step 2 (Commit) DEVE coletar `pool_pair` (default: "BRL-USD"), `provider_id`, `side` (A ou B, via selector), e `amount`, chamar `POST /api/v2/amm/liquidity/commit`, e: avançar para o Step 4 se a resposta retornar `status: "EXECUTED"`, ou avançar para o Step 3 se retornar `status: "PENDING"`.

- **FR-004**: O Step 3 (Monitor) DEVE realizar polling de `GET /api/v2/amm/pool/{pair}/status` a cada 5 segundos e: avançar automaticamente para o Step 4 ao detectar `pool_status: "ACTIVE"`, ou exibir mensagem de expiração e voltar ao Step 1 ao detectar `pool_status: "EMPTY"` (commit expirado/cancelado).

- **FR-005**: O Step 3 DEVE exibir um countdown legível (formato "Xh Ym restantes") calculado a partir do campo `expires_at` do commit retornado no Step 2. Ao atingir zero, o countdown DEVE exibir "Expirado".

- **FR-006**: O Step 3 DEVE oferecer um botão "Cancel Commit" que chama `DELETE /api/v2/amm/liquidity/commits/:commit_id?provider_id=:provider_id`, exibe confirmação ao sucesso, e retorna o wizard ao Step 1.

- **FR-007**: O Step 4 (Success) DEVE exibir: `pool_status`, `reserve_a`, `reserve_b`, `current_ratio`, `fee_rate_bps`, e os LP IDs disponíveis (obtidos de `activeCommit.lp_ids`, armazenado no store durante o Step 2). Quando `activeCommit.lp_ids` for null (ex.: sessão retomada via banner sem novo commit), a seção de LP IDs DEVE exibir o fallback "LP IDs not available in current session — check the LP Positions table below" em vez de falhar. DEVE oferecer botão "Remove Liquidity" por LP ID (somente quando `lp_ids` não for null) e botão "Add More Liquidity" para reiniciar o wizard.

- **FR-008**: A `LiquidityManagementPage` DEVE, ao detectar `pool_status: "PENDING_COUNTERPART"` na polling inicial, filtrar `pending_commits` pelo `provider_id` do usuário autenticado (obtido do auth store). Se um commit correspondente for encontrado, DEVE exibir um banner informativo com botão "Monitor Pending Commit" que abre o wizard diretamente no Step 3 pré-populado com os dados desse commit. Se nenhum commit corresponder ao operador atual, o banner NÃO é exibido.

- **FR-009**: O serviço de API (`liquidity.api.ts`) DEVE expor três novos métodos: `commitLiquidity(payload: CommitRequest): Promise<CommitResult>`, `listCommits(poolPair: string, status?: string): Promise<CommitResult[]>`, e `cancelCommit(commitId: string, providerId: string): Promise<void>`. **Nota**: `listCommits` é um método utilitário implementado mas não conectado a nenhum componente do wizard neste feature — a detecção de commits pendentes para o banner usa `pending_commits` retornado por `fetchPoolStatus`.

- **FR-010**: O store de estado (`liquidity.store.ts`) DEVE ser estendido com: `activeCommit: CommitResult | null`, `commitStatus: "idle" | "loading" | "error"`, `commitError: string | null`, e as actions `submitCommit`, `cancelActiveCommit`, e `clearCommit`.

- **FR-011**: O arquivo de tipos (`liquidity.types.ts`) DEVE ser estendido com: `CommitSide`, `CommitStatus`, `CommitRequest`, `CommitResult`, `PendingCommitSummary`, e o tipo `PoolStatus` existente DEVE ser estendido com os campos `pool_status: PoolLifecycleStatus`, `fee_rate_bps`, `total_lp_count`, e `pending_commits`.

- **FR-012**: O wizard DEVE preservar a navegação direta entre steps via stepper (clique em step anterior permitido), exceto avançar para um step posterior sem ter completado o anterior.

- **FR-013**: Todos os formulários do wizard DEVEM desabilitar o botão de submit enquanto uma requisição estiver em andamento (`status: "loading"`), e exibir mensagens de erro inline (não em toast) ao lado do campo ou abaixo do formulário correspondente.

- **FR-014**: A funcionalidade de Remove Liquidity e a tabela de LP Positions DEVEM ser preservadas inalteradas na `LiquidityManagementPage`, coexistindo com o novo wizard.

- **FR-015**: A polling de pool status existente (intervalo de 15s na `LiquidityManagementPage`) DEVE coexistir com a polling do Step 3 (intervalo de 5s). Quando o wizard estiver no Step 3, a polling do Step 3 é a ativa; ao sair do Step 3, a polling de 15s retoma.

- **FR-016**: O arquivo de tipos da governance app (`liquidity.types.ts`) DEVE ser atualizado: `MintAndApproveRequest` passa de `{amount_a: string, amount_b: string, recipient?: string}` para `{amount: string, recipient?: string}`. DEVE ser adicionado o tipo `ApproveAmmRequest: {amount: string, side?: "A" | "B"}` (o campo `side` é opcional para CBs — obrigatório apenas no padrão G5-cross com `side: "B"` explícito).

- **FR-017**: O serviço de API da governance app (`liquidity.api.ts`) DEVE ser estendido com um método `approveAmm(payload: ApproveAmmRequest): Promise<void>` que chama `POST /api/v2/amm/token/approve-amm`. O método `mintAndApprove` existente DEVE ser atualizado para usar o novo `MintAndApproveRequest`.

- **FR-018**: O store da governance app (`liquidity.store.ts`) DEVE ser estendido com uma action `approveAmm(amount: string, side?: "A" | "B"): Promise<void>`. A action `mintAndApprove` existente DEVE ser atualizada para aceitar `(amount: string, recipient?: string)` em vez de `(amount_a, amount_b, recipient?)`.

- **FR-019**: O `LiquidityManagementPage.tsx` (painel standalone MintAndApprove) DEVE ser atualizado para usar o único campo `amount` em vez dos campos `amount_a` / `amount_b`. A atualização remove os dois inputs e adiciona um único input de `amount`.

- **FR-020**: O Step 1 do `CooperativeLiquidityWizard.tsx` DEVE incluir um bloco colapsável "G5-cross Setup", ativado por toggle "Is this a fresh environment?". Quando expandido, o bloco DEVE exibir: campo `amount` (compartilhado com G5.1 e G5.2), campo `cb_a_signer_address` (endereço blockchain do signer de CB-A, com validação de formato `0x[40 hex chars]`), botão "Mint TOKEN_B to CB-A" (chama `mintAndApprove({amount, recipient: cb_a_signer_address})`) e botão "Approve TOKEN_B for AMM (side B)" (chama `approveAmm(amount, "B")`). Os dois botões são independentes — G5.2 não exige que G5.1 tenha sido executado na sessão atual. Desativar o toggle descarta o estado G5-cross sem bloquear o fluxo principal.

- **FR-021**: O arquivo de tipos do bank app (`amm-v2.types.ts`) DEVE ser atualizado: `ApproveAmmRequest` passa de `{amount_a: string, amount_b: string}` para `{amount: string, side: "A" | "B"}`. O tipo `PoolStatus` DEVE ser estendido com os campos opcionais `pool_status?: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE"`, `fee_rate_bps?: number`, e `total_lp_count?: number`.

- **FR-022**: O store do bank app (`amm-v2.store.ts`) DEVE ter a action `approveAmm` atualizada de `(amount_a: string, amount_b: string)` para `(amount: string, side: "A" | "B")`. A `AMMTradingPage.tsx` DEVE ser atualizada para: (a) exibir um único campo `approveAmount` e um seletor `side` (radio ou select com opções "A" e "B") em substituição a `approveAmountA`/`approveAmountB`; (b) desabilitar o botão de submit do swap quando `pool_status` for `"EMPTY"` ou `"PENDING_COUNTERPART"`, exibindo mensagem inline "Pool is not active. Swaps are currently unavailable.".

- **FR-023**: O arquivo de tipos do bank app (`payment.types.ts`) DEVE ser atualizado: `RegisterDepositRequest` passa de `{amount: string}` para `{requester_besu_address: string, requester_paladin_identity: string, amount: string}`. `RequestEscrowRequest` passa de `{amount: string}` para `{deposit_id: string}`. `RequestRedeemRequest` passa de `{amount: string}` para `{requester_besu_address: string, requester_paladin_identity: string, amount: string, zeto_transfer_tx_hash?: string}`.

- **FR-024**: O serviço de API e o store do bank app (`payment.api.ts`, `payment.store.ts`) DEVEM ser atualizados para propagar os novos tipos de `RegisterDepositRequest`, `RequestEscrowRequest`, e `RequestRedeemRequest` sem quebrar as assinaturas existentes.

- **FR-025**: A `DepositsPage.tsx` DEVE ler `requester_besu_address` do auth store (`wallet_address` do perfil de onboarding) e `requester_paladin_identity` da variável de ambiente `VITE_PALADIN_IDENTITY`, injetando-os automaticamente no payload sem exibir campos editáveis para o operador. Se `wallet_address` não estiver disponível no auth store, o formulário DEVE exibir aviso "Onboarding incomplete — wallet address unavailable" e desabilitar o submit.

- **FR-026**: A `EscrowsPage.tsx` DEVE solicitar tokenização via `{deposit_id}` em vez de `{amount}`. O `deposit_id` DEVE ser pré-preenchido com o ID do depósito mais recente com `status: "APPROVED"` disponível no store (field editável para correção manual). Após escrow com `status: "APPROVED"`, DEVE exibir banner: "Your tCeBM has been issued. Next step: Approve AMM spending" com link para `AMMTradingPage`.

- **FR-027**: A `RedeemsPage.tsx` DEVE incluir o toggle "My tokens came from a Zeto private transfer". Quando inativo: payload de redeem é enviado sem `zeto_transfer_tx_hash`. Quando ativo: campo `zeto_transfer_tx_hash` (string, validado como iniciando em `0x`) torna-se obrigatório e o submit permanece desabilitado até ser preenchido. Ao desativar o toggle, o campo é limpo. Os campos `requester_besu_address` e `requester_paladin_identity` são injetados automaticamente (mesma lógica de `DepositsPage`).

- **FR-028**: O api-gateway do Banco Central DEVE expor as seguintes 7 rotas de ação de pagamento (Scenario A), autenticadas via `RequireCookieAuth` no grupo `/api/v1` e espelhadas no grupo relay `/internal/v1` (`RequireRelayAuth`), suportando as operações de supervisão do Banco Central sobre depósitos fiat, tokenizações (escrows) e saques (redeems) iniciados pelos bancos comerciais:
  - `POST /api/v1/payments/deposits/approve` — payload: `{deposit_id: string}` → `{status: "approved"}`
  - `POST /api/v1/payments/deposits/reject` — payload: `{deposit_id: string, reason: string}` → `{reason: string}`
  - `POST /api/v1/payments/deposits/fiat-exchange` — payload: `{deposit_id: string}` → `FiatExchangeResponse`
  - `POST /api/v1/payments/escrows/approve` — payload: `{escrow_id: string}` → `ApproveEscrowResponse`
  - `POST /api/v1/payments/escrows/reject` — payload: `{escrow_id: string, reason: string}` → `{reason: string}`
  - `POST /api/v1/payments/redeems/approve` — payload: `{redeem_id: string}` → `ApproveRedeemResponse`
  - `POST /api/v1/payments/redeems/reject` — payload: `{redeem_id: string, reason: string}` → `{reason: string}`

  Os handlers correspondentes (`ApproveDeposit`, `RejectDeposit`, `RequestFiatExchange`, `ApproveEscrow`, `RejectEscrow`, `ApproveRedeem`, `RejectRedeem`) já estão implementados em `backend/services/api-gateway/internal/http/handlers/payment.go`. O registro das rotas no `router.go` foi realizado nesta branch em 2026-05-20 (ver Session 2026-05-20 — Rotas de Aprovação de Pagamento). Estas rotas são consumidas exclusivamente por `frontend/apps/governance/src/services/api/payment.api.ts`.

### Key Entities

#### Governance App (`frontend/apps/governance/`)

- **CommitSide**: Tipo literal `"A" | "B"` — identifica qual lado do par o provedor está commitando.

- **CommitStatus**: Tipo literal `"PENDING" | "EXECUTED"` — estado do commit no momento da resposta da API.

- **PoolLifecycleStatus**: Tipo literal `"EMPTY" | "PENDING_COUNTERPART" | "ACTIVE"` — estado do ciclo de vida do pool AMM.

- **MintAndApproveRequest** *(atualizado — FR-016)*: Payload para mint & approve pelo Banco Central. Campos: `amount: string`, `recipient?: string`. Os campos depreciados `amount_a` e `amount_b` são removidos.

- **ApproveAmmRequest** *(novo — FR-016)*: Payload para approve-amm pelo Banco Central. Campos: `amount: string`, `side?: "A" | "B"`. O campo `side` é opcional para CBs (auto-detectado via `CENTRAL_BANK_ROLE`) mas exigido explicitamente no padrão G5-cross (`side: "B"` para CB-A aprovar TOKEN_B recebido).

- **CommitRequest**: Payload para registrar um commit. Campos: `pool_pair: string`, `provider_id: string`, `side: CommitSide`, `amount: string`.

- **CommitResult**: Resposta do backend ao criar um commit. Campos: `commit_id: string`, `pool_pair: string`, `side: CommitSide`, `amount: string`, `status: CommitStatus`, `expires_at: string` (ISO 8601), `lp_ids: string[] | null`.

- **PendingCommitSummary**: Representação resumida de um commit pendente retornada dentro de `PoolStatus.pending_commits`. Campos: `commit_id: string`, `provider_id: string`, `side: CommitSide`, `amount: string`, `expires_at: string`.

- **PoolStatus** (estendido): Acrescenta aos campos existentes `pool_status: PoolLifecycleStatus`, `fee_rate_bps?: number`, `total_lp_count?: number`, `pending_commits?: PendingCommitSummary[]`. Os campos `reserve_a`, `reserve_b`, `current_ratio`, `imbalance_flag`, `updated_at` são mantidos.

- **WizardStep**: Enumeração interna dos steps do wizard: `MINT_APPROVE = 1`, `COMMIT = 2`, `MONITOR = 3`, `SUCCESS = 4`. Controla o estado de navegação interno do componente.

#### Bank App (`frontend/apps/bank/`)

- **ApproveAmmRequest** *(atualizado — FR-021)*: Payload para approve-amm pelo banco comercial. Campos: `amount: string`, `side: "A" | "B"`. O campo `side` é obrigatório para bancos comerciais (não possuem `CENTRAL_BANK_ROLE`). Os campos depreciados `amount_a` e `amount_b` são removidos.

- **PoolStatus** *(estendido — FR-021)*: Os campos `pool_status?: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE"`, `fee_rate_bps?: number`, e `total_lp_count?: number` são adicionados aos campos existentes.

- **RegisterDepositRequest** *(atualizado — FR-023)*: Campos: `requester_besu_address: string`, `requester_paladin_identity: string`, `amount: string`.

- **RequestEscrowRequest** *(atualizado — FR-023)*: Campos: `deposit_id: string`. O campo `amount` é removido — o backend recupera o valor a partir do depósito referenciado.

- **RequestRedeemRequest** *(atualizado — FR-023)*: Campos: `requester_besu_address: string`, `requester_paladin_identity: string`, `amount: string`, `zeto_transfer_tx_hash?: string`.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um operador do banco central consegue completar o fluxo end-to-end do wizard (Mint & Approve → Commit → Monitor → Success) em menos de 5 interações com a interface (4 submits: Step 1, Step 2, e 2 confirmações no Step 4), sem precisar conhecer os endpoints da API.

- **SC-002**: Quando o commit retorna `status: "EXECUTED"` imediatamente, o wizard exibe o Step 4 em menos de 1 segundo após a resposta da API, sem exibir o Step 3.

- **SC-003**: O countdown no Step 3 é atualizado a cada segundo e exibe a diferença correta entre o momento atual e `expires_at`, com margem máxima de 2 segundos de defasagem visual.

- **SC-004**: A polling do Step 3 detecta a transição `PENDING_COUNTERPART → ACTIVE` e avança o wizard em até 10 segundos após a mudança de estado no backend (2 ciclos de polling de 5s).

- **SC-005**: O cancelamento de commit no Step 3 completa (sucesso ou erro exibido) em menos de 5 segundos após o clique do botão.

- **SC-006**: A tabela de LP Positions existente e o formulário de Remove Liquidity continuam funcionando sem regressão após a introdução do wizard — 100% dos casos de uso de remoção de liquidez existentes permanecem operacionais.

- **SC-007**: Ao recarregar a `LiquidityManagementPage` com pool em estado `PENDING_COUNTERPART`, o banner de retomada é exibido em até 15 segundos (um ciclo de polling).

- **SC-008**: Todas as mensagens de erro da API (HTTP 4xx e 5xx) são exibidas de forma legível (sem stack traces ou mensagens de erro raw) dentro do step correspondente, sem navegar para outro step ou quebrar a interface.

- **SC-009**: O Step 1 do wizard não envia nenhuma requisição HTTP com os campos depreciados `amount_a` ou `amount_b` — o payload de `mint-and-approve` sempre contém exclusivamente `amount` (e opcionalmente `recipient`). Verificável inspecionando as requisições de rede no browser ou nos testes unitários do store.

- **SC-010**: A `AMMTradingPage` nunca chama `POST /amm/token/approve-amm` sem o campo `side` preenchido — o botão de submit do formulário de approve-amm permanece desabilitado enquanto `side` não estiver selecionado.

- **SC-011**: A `AMMTradingPage` não envia o payload de swap enquanto `pool_status !== "ACTIVE"` — o botão de swap fica visualmente desabilitado e nenhuma chamada HTTP ao endpoint de swap é disparada nesses estados.

- **SC-012**: O formulário de depósito (`DepositsPage`) envia `requester_besu_address` e `requester_paladin_identity` em todos os submits bem-sucedidos — verificável via testes unitários do store e inspeção de rede.

- **SC-013**: O formulário de escrow (`EscrowsPage`) envia `{deposit_id}` (não mais `{amount}`) e exibe o banner de next-step após status `APPROVED`.

- **SC-014**: O formulário de redeem (`RedeemsPage`) inclui `zeto_transfer_tx_hash` no payload apenas quando o toggle estiver ativo e o campo preenchido — nunca inclui o campo com valor vazio ou `undefined`.

- **SC-015**: O painel standalone MintAndApprove na `LiquidityManagementPage` exibe apenas o campo `amount` após a atualização — os campos `amount_a` e `amount_b` não existem mais na interface.

---

## Assumptions

- O projeto já possui o hook `usePolling` (atualmente usado na `LiquidityManagementPage` com intervalo de 15s) que pode ser reutilizado no Step 3 com intervalo de 5s.
- O sistema de design `@cbweb3/ui` (shadcn/ui) já expõe os componentes `Badge`, `Button`, `Card`, `Input`, `Label`, e primitivos de tabela necessários para o wizard e os formulários atualizados. Nenhum novo pacote npm precisa ser instalado.
- A autenticação e o contexto do usuário (incluindo o `provider_id` do CB logado e o `wallet_address` do banco comercial) são gerenciados pelo auth store existente de cada aplicação — o wizard lê o `provider_id` do auth store para pré-preencher campos de identidade; as páginas do bank app leem `wallet_address` do auth store para o payload de depósito/redeem.
- O campo `side` no Step 2 (commit) é explicitamente selecionado pelo operador — o wizard não tenta inferir automaticamente o lado (A ou B) a partir do `provider_id`, pois a associação entre CB e lado pode variar por corredor.
- O backend valida a autorização do provedor e o estado do pool — o frontend confia nas respostas HTTP da API para determinar se o fluxo pode avançar, sem duplicar validações de negócio no cliente.
- O estado do wizard (`activeCommit`, `currentStep`) e dos formulários do bank app é armazenado em Zustand sem persistência entre sessões, consistente com o padrão `Browser session state only` do projeto.
- O campo `lp_ids` na resposta do commit pode ser `null` quando `status: "PENDING"`. O wizard usa `lp_ids[0]` como o LP ID do CB-A e `lp_ids[1]` como o LP ID do CB-B quando `status: "EXECUTED"`, conforme documentação da API.
- O endpoint `POST /api/v2/amm/liquidity/add` (legacy) é mantido no `liquidity.api.ts` para compatibilidade com o formulário de Remove Liquidity e sessões de tryout, mas não é exposto no wizard.
- A variável de ambiente `VITE_PALADIN_IDENTITY` está configurada em cada instância do bank app com o valor correto (ex.: `"bank-a@node1"`). Não há fallback — se a variável estiver ausente, o campo `requester_paladin_identity` será uma string vazia e o erro será visível na response da API.
- O backend já está deployado com a versão v4.0 do runbook: payloads `mint-and-approve` com campo único `amount` (campos `amount_a`/`amount_b` retornam HTTP 400 `DEPRECATED_FIELDS`) e payloads `approve-amm` com `amount` + `side` obrigatório para bancos comerciais (omissão retorna HTTP 400 `SIDE_REQUIRED`). Os payloads de depósito/escrow/redeem também esperam os campos completos conforme documentado.
- O G5-cross é necessário apenas em ambientes frescos (sem setup prévio). O toggle "Is this a fresh environment?" é informacional — o wizard não detecta automaticamente se o ambiente precisa do G5-cross; cabe ao operador julgar.

---

## Out of Scope

- Mudanças no backend, nos endpoints da API Gateway ou na lógica de negócio do serviço de liquidez.
- Fluxo H (PairRegistry) — registro bilateral de par de moedas; ocorre uma única vez na configuração de rede e está fora deste feature.
- Interface de gestão de papel MLP (concessão/revogação de autorização de provedor) — esta é uma função administrativa separada.
- Fluxo de depósito adicional em pool já `ACTIVE` (sem commit-reveal) — o wizard é específico para formação inicial do pool via commit-reveal.
- Bridging cross-spoke ou integração com múltiplos nós de rede na mesma tela.
- Notificações push ou e-mail ao CB-B quando CB-A registra commit — comunicação off-chain está fora do escopo da interface.
- Histórico persistente de commits anteriores — apenas o commit da sessão corrente é rastreado no estado do wizard.
- Internacionalização (i18n) — os textos são em inglês, consistente com o padrão atual do frontend de governança.
- Testes automatizados end-to-end com Playwright ou Cypress — os testes são de responsabilidade de um feature separado de QA.
- Instalação de novos pacotes npm — todas as mudanças usam exclusivamente as dependências já presentes no workspace.
- Persistência de estado Zustand (localStorage/sessionStorage) — o padrão do projeto é session-only (sem middleware de persistência).
