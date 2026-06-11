# Feature Specification: Provisão Cooperativa de Liquidez no AMM

**Feature Branch**: `005-cooperative-liquidity`
**Created**: 2026-05-14
**Status**: Draft — **on-chain mechanism superseded by [013-amm-lp-shares](../013-amm-lp-shares/plan.md)**

> **Update (2026-06-10):** The single-sided `addSingleSidedLiquidity`/`removeSingleSidedLiquidity`
> on-chain primitives and the off-chain `shares_percentage` ledger described here are **replaced** by
> the on-chain ERC20 LP-share model (TASK-12/13). Deposits now use escrow-and-finalize
> (`depositForCommit`/`finalizeCommit`); pool ownership is tracked on-chain as LP-share balances;
> withdrawal is a home-currency zap-out. The cooperative commit-reveal *matching* logic in this spec
> still applies — only the on-chain settlement mechanism changed. See 013 for the authoritative
> design and decisions D1–D6.

## Contexto e Motivação

O modelo anterior de provisionamento de liquidez do AMM do CBWeb3 exigia que **um único provedor depositasse ambas as moedas do par simultaneamente** (ex.: o Banco Central A depositava tanto tCeBMa quanto tCeBMb para formar o pool BRL-USD). Isso viola o princípio de soberania monetária: o Banco Central A não deve precisar deter e controlar a moeda do Banco Central B.

O modelo de **Provisão Cooperativa de Liquidez** permite que cada Banco Central contribua com a sua própria moeda digital de forma independente, formando o pool em conjunto. Além disso, o MLP (Multilateral Liquidity Provider — ex.: BID ou consórcio regional) pode complementar a liquidez quando qualquer CB não tiver capacidade no momento. Os provedores passam a receber uma parcela proporcional das taxas geradas pelos swaps dos bancos comerciais.

> ### ⚠️ VIOLAÇÃO ARQUITETURAL CONHECIDA — G5-cross (MVP single-gateway) — **DEPRECIADO**
>
> O MVP desta feature implementou um padrão denominado **G5-cross** que **viola a soberania monetária ao nível de execução on-chain**:
>
> - CB-B minta TOKEN_B diretamente para o endereço signer de CB-A (`mint-and-approve {"recipient":"<CB-A-addr>"}`)
> - CB-A's signer passa a deter e controlar tokens emitidos por CB-B
> - CB-A executa `addSingleSidedLiquidity(TOKEN_B)` como executor on-chain do TOKEN_B de CB-B
>
> Isso contradiz diretamente o objetivo declarado desta spec. O padrão G5-cross foi aprovado nas Q&As de 2026-05-20 como "limitação de MVP" — **essa aprovação está revogada**.
>
> **O fluxo arquiteturalmente correto** (especificado em `007-bridge-based-cb-liquidity`):
> 1. CB-A minta BRL em Spoke-A (via autoridade CENTRAL_BANK_ROLE no spoke)
> 2. CB-A bloqueia BRL no contrato Bridge do Spoke-A → Bridge Relayer minta W-BRL no Hub
> 3. CB-A deposita W-BRL no AMM do Hub usando o signer **de CB-A**
> 4. CB-B executa o mesmo fluxo para ARS no Spoke-B → W-ARS no Hub → CB-B deposita
>
> Nenhum CB emite tokens para carteira de outro CB em nenhuma etapa.
>
> **Status**: O código de G5-cross nos tryouts e no frontend wizard está marcado como ANTI-PATTERN e deve ser substituído pela implementação da `007-bridge-based-cb-liquidity`.

---

## Clarifications

### Session 2026-05-20

- Q: A cláusula "mesma transação DB" em FR-006 e a exigência de 100% de distribuição em SC-006 devem ser mantidas como MUST dada a confirmação no tryout de que `fee_claim_paid=0` em ambiente multi-gateway — ou FR-006/SC-006 devem ser relaxados para refletir a limitação arquitetural de isolamento de DB entre gateways? → A: Opção A — relaxar FR-006 para **best-effort síncrono no handler**: a distribuição de taxas ocorre no mesmo handler HTTP do swap, antes de retornar 200, mas é **não-bloqueante** (falhas registradas como warning, não impedem o 200 do swap). Em arquitetura multi-gateway (gateway de swap ≠ gateway LP-owner), a distribuição pode ser omitida silenciosamente pois os LPs não existem no DB local do gateway executante. SC-006 passa a ser garantido apenas em deployments single-gateway ou quando os LPs compartilham o mesmo DB. Bloquear o 200 do swap por falha de fee recording violaria SC-003 (p95 ≤ 6s) e enganaria o swapper sobre o estado da transação on-chain.
- Q: O mecanismo de retry (5 tentativas, backoff exponencial) e estado `RECONCILIATION_REQUIRED` de FR-014 foram implementados no MVP ou devem ser adiados para Fase 2? → A: Adiado — **Fase 2 / T059 (Post-MVP)**. O caminho de falha parcial de commit-reveal não foi exercitado no E2E de 2026-05-20; implementar retry + `RECONCILIATION_REQUIRED` requer novo valor no enum `status` da tabela `pool_commits`, nova goroutine de retry e sistema de alertas ao operador — escopo significativo não coberto por nenhuma task do MVP. FR-014 é mantido como requisito futuro, trackado como T059. O padrão de retry do Relayer Cacti pode ser reutilizado na implementação futura.
- Q: `total_lp_count` deve ser visível com valor correto para qualquer gateway (incluindo bancos comerciais) ou pode retornar 0 quando o gateway consultado não possui LP positions no seu DB local? → A: **Comportamento esperado — documentação de escopo**. Em arquitetura multi-gateway, `total_lp_count` reflete apenas o DB local do gateway consultado. Bancos comerciais consultando seu próprio gateway vão receber 0 se nenhuma LP position foi criada através daquele gateway — isso é correto, não um bug. A fonte canônica de `total_lp_count` é o gateway LP-owner (ex.: CB-A após commit-reveal). O spec deve qualificar esse campo explicitamente.
- Q: SC-003 (p95 ≤ 6s para swaps com múltiplos LPs) tem task de validação automática ou é validado manualmente no MVP? → A: **Validação manual no MVP**. O tryout E2E de 2026-05-20 serve como smoke test de latência (swap retornou em ~1s, muito abaixo do SLA de 6s). Benchmark automatizado com ferramenta de carga (é ex.: `hey`, `k6`) é escopo de Fase 2, trackado como T060. SC-003 é mantido como MUST mas com nota de que a validação é manual neste sprint.
- Q: A mensagem `INFO: fee_claim_paid=0 — may be zero if no swaps occurred` no tryout é enganosa quando um swap de fato ocorreu antes da remoção? → A: Sim — **corrigir a mensagem no tryout**. Após o swap de 2026-05-20, `fee_claim_paid=0` não é por ausência de swap mas por isolamento de DB entre gateways (swap ocorreu via bank-a, LPs vivem no DB do CB-A). A mensagem deve refletir a limitação arquitetural: `fee_claim_paid=0 — esperado em multi-gateway: swap via bank-a gateway não alcança LP positions no DB do CB-A`.
- Q: No modelo MVP de single-gateway, quem é o `msg.sender` on-chain ao executar `addSingleSidedLiquidity` para TOKEN_B quando o match ocorre no gateway de CB-A? A soberania monetária é preservada? → ~~A: CB-A's signer é o `msg.sender` on-chain para **ambos** os lados (TOKEN_A e TOKEN_B). Soberania monetária é preservada ao nível de **minting**: CB-B emite TOKEN_B com sua própria chave privada via `mint-and-approve {"amount":"X","recipient":"<CB-A-signer-addr>"}` (padrão G5-cross). O **depósito on-chain** (`addSingleSidedLiquidity(TOKEN_B)`) é executado pelo signer de CB-A como executor técnico do gateway receptor do match — limitação arquitetural documentada do MVP single-gateway. O campo `provider_id` no DB (ex.: `"central_bank_b"`) é a **fonte canônica de autoridade LP**; o `msg.sender` on-chain é metadado de execução, não de autoridade. O evento on-chain `LogSingleSidedLiquidityAdded(CB-A-signer, isTokenA=false, amount)` reflete o executor, não o titular do LP. Em produção multi-gateway (Fase 2), CB-B's gateway executaria TOKEN_B side com CB-B's signer.~~ **[REVOGADO 2026-05-20 — ANTI-PATTERN]** A resposta original aprova o padrão G5-cross que viola soberania monetária ao nível de execução: CB-A's signer detém e deposita TOKEN_B emitido por CB-B — isso não é soberania, é delegação de custódia ao concorrente. O `provider_id` no DB não tem força legal on-chain; o `msg.sender` do depósito on-chain É o agente econômico registrado no contrato. A análise de arquitetura de 2026-05-20 (findings C1/C3/C4 em `speckit.analyze`) revogou esta aprovação. O fluxo correto está especificado em `007-bridge-based-cb-liquidity`: cada CB usa seu próprio signer para todas as operações, mediadas pelo bridge.
- Q: No MVP, commits de ambos os lados de um par podem ser enviados a gateways diferentes e ainda assim obter auto-match? → A: Não — o matching é **single-DB** no MVP: `FindActiveByPairAndSide` consulta apenas o banco de dados **local** do gateway receptor. Para que o auto-match ocorra, commit B DEVE ser enviado ao **mesmo gateway** que recebeu commit A. O campo `provider_id` no payload do commit é apenas metadado de autoridade de LP — não determina para qual gateway o commit é roteado. Cross-gateway matching (commit B ao gateway de CB-B detecta commit A via chamada inter-gateway) é aspiracional e está fora do escopo do MVP (documentado em `tryout-scenario-b-e2e.sh` como "not yet impl").
- Q: Um CB pode passar `side="B"` explicitamente em `approve-amm` para aprovar o token da contraparte, ou o campo `side` é ignorado/proibido para CBs? → A: `side` é **opcional** para CBs, não proibido, para fins de compatibilidade de ERC-20 padrão. Quando omitido, o adapter auto-detecta via `CENTRAL_BANK_ROLE` (`sideIsA`). Quando explicitado (ex.: `side="B"` em CB-A), o adapter usa o cliente do token indicado. **[NOTA 2026-05-20]** O uso de `side="B"` por CB-A era necessário **exclusivamente** para suportar o padrão G5-cross (agora DEPRECIADO): CB-A's signer aprovava o AMM para TOKEN_B após receber TOKEN_B via mint-to de CB-B. Com o fluxo correto de bridge (`007-bridge-based-cb-liquidity`), cada CB aprova apenas o seu próprio token (TOKEN_A para CB-A, TOKEN_B para CB-B) — `side="B"` em CB-A não deve mais ocorrer em fluxos de produção.
- Q: FR-002 usa a palavra "atômica" para descrever a execução dos dois depósitos após o match. Isso é preciso on-chain? → A: Não — "atômica" é impreciso. As duas chamadas `addSingleSidedLiquidity` são **transações on-chain separadas** (não uma tx única). A garantia de consistência ocorre ao nível do **handler HTTP**: ambas as transferências são executadas em sequência no mesmo handler antes de retornar ao chamador. Se a segunda falhar após a primeira ter sido confirmada on-chain, ambos os commits transitam para `RECONCILIATION_REQUIRED` — há risco de execução parcial documentado em FR-014. FR-002 deve usar "sequencial no mesmo handler" no lugar de "atômica".

### Session 2026-05-19

- Q: Qual é a fonte de verdade para as reservas do pool ao calcular o montante a devolver numa remoção proporcional — `AMM.getReserves()` on-chain, tabela `pool_state_readings` no DB, ou on-chain com fallback DB? → A: Opção A — `AMM.getReserves()` on-chain no momento exato da remoção. É a fonte canoníca e elimina qualquer risco de estado stale; o custo de 1 RPC extra é aceitável dado que `removeLiquidity` já executa uma tx on-chain logo depois.
- Q: Quando deve `fee_claim_accumulated` ser atualizado nas `LiquidityPosition` — síncrono no fluxo do swap, worker em background, ou lazy na remoção? → A: *(Revisado em 2026-05-20 — ver Session 2026-05-20)* Síncrono no handler do swap, **best-effort**: `LPFeeEvent` persistido e `fee_claim_accumulated` atualizado antes de retornar `200 COMPLETED` quando os LPs vivem no mesmo gateway DB. Em arquitetura multi-gateway, falhas são silenciosas (warning log). A cláusula original "mesma transação DB" foi relaxada após confirmação de limitação arquitetural no tryout de 2026-05-20 (`fee_claim_paid=0`).
- Q: De onde deve ser derivado `total_lp_count` no response de `GET /api/v2/amm/pool/{pair}/status` — DB `liquidity_positions`, eventos on-chain, ou campo `total_LP_shares` do `PoolState`? → A: Opção A — `COUNT(*) FROM liquidity_positions WHERE pool_pair = ? AND status = 'ACTIVE'` no DB **local do gateway consultado**. Em arquitetura multi-gateway, cada instância retorna a contagem do seu próprio DB; bancos comerciais sem LP positions no seu DB vão receber 0 — comportamento esperado documentado em 2026-05-20 (ver Session 2026-05-20).
- Q: `shares_percentage` deve ser recalculado apenas em depósito/retirada ou também após cada swap (já que reservas mudam)? → A: Opção A — `shares_percentage` reflete proporção de shares emitidas e é fixo entre eventos de depósito e remoção; swaps não disparam recalculo. A variação de valor causada por swaps é capturada via `AMM.getReserves()` no momento da remoção (FR-007), sem atualizar o campo `shares_percentage`. Modelo equivalente ao Uniswap v2.
- Q: Como deve `GET /api/v2/amm/pool/{pair}/status` derivar o campo `pool_status` — apenas das reservas on-chain, apenas do DB `pool_commits`, ou DB-first com fallback on-chain? → A: Opção C — derivação DB-first + on-chain: (1) se existir `pool_commit` com `status IN (PENDING, MATCHED)` para o par no DB → retornar `PENDING_COUNTERPART`; (2) senão se `reserve_a > 0 AND reserve_b > 0` via `AMM.getReserves()` → `ACTIVE`; (3) senão → `EMPTY`. O DB é a fonte autoritativa para o estado de commits; a chain é a fonte autoritativa para reservas.
- Q: O payload de `POST /api/v2/amm/token/mint-and-approve` (sem `recipient`) deve continuar usando `{ "amount_a": "...", "amount_b": "0" }` ou simplificar para um campo único? → A: Opção A — simplificar para `{ "amount": "200000" }`. O gateway detecta automaticamente qual token mintar com base na identidade do CB autenticado: cada gateway é deployado com a chave privada do seu CB emissor, que possui `CENTRAL_BANK_ROLE` em exatamente um dos tokens do par. O backend pula o token cujo amount é zero (lógica já existente); com o novo campo `amount`, o gateway lê a configuração de deployment (`HUB_TOKEN_A_ADDRESS`/`HUB_TOKEN_B_ADDRESS`) e a role do signer para determinar qual dos dois tokens mintar. Payload mínimo, sem campo semântico incorreto, escala trivialmente para CB-C sem alteração de contrato. (FR-018)
- Q: Para `mint-and-approve` **com `recipient`**, o mesmo campo único `amount` se aplica, ou manter `amount_a`/`amount_b` para permitir mintar ambos os tokens a um destinatário num único request? → A: Opção A — `{ "amount": "...", "recipient": "0x..." }`. Cada CB minta apenas o token de sua emissão na carteira do destinatário. Se um banco comercial precisar receber ambas as moedas, os dois CBs distintos fazem chamadas separadas — isso reflete corretamente a soberania monetária e elimina o risco de um CB tentar mintar o token do outro.
- Q: O endpoint `POST /api/v2/amm/token/approve-amm` também deve migrar para campo único `amount`, ou manter `amount_a`/`amount_b` pois pode ser chamado por banco comercial que detém ambos os tokens? → A: Opção A — campo único `{ "amount": "..." }` também para `approve-amm`. O gateway usa a mesma lógica de detecção via `CENTRAL_BANK_ROLE` do signer para CBs; bancos comerciais que precisam aprovar o token de entrada do swap sempre aprovam apenas um token por vez (o token que vão vender), portanto o campo único é correto e elimina o aprovisionamento desnecessário do token oposto. (FR-018)
- Q: Esta mudança de payload (`amount_a`/`amount_b` → `amount`) deve ser implementada nesta feature branch (backend handler + adapter + tryouts) ou apenas documentada para feature futura? → A: Opção A — implementar nesta branch (005). Impacto localizado: `token_handler.go`, `amm_adapter.go` e tryouts. Sem alteração de contrato Solidity ou schema de banco de dados. Implementar agora mantém consistência entre documentação e código e evita acúmulo de dívida técnica.
- Q: Para `approve-amm` chamado por banco comercial (sem `CENTRAL_BANK_ROLE`), como o gateway identifica qual token aprovar com campo único `amount`? → A: Opção A — campo opcional `side: "A" | "B"` junto a `amount`, consistente com o padrão commit-reveal. CBs omitem `side` (role detecta automaticamente); bancos comerciais DEVEM informar `side`. Payload CB: `{ "amount": "200000" }`; payload banco comercial: `{ "amount": "200000", "side": "B" }`. Gateway retorna HTTP 400 (`"error": "'side' required for non-central-bank callers"`) se banco comercial omitir o campo. (FR-018)

### Session 2026-05-18

- Q: Arquitetura de contrato por par — múltiplos pares dinâmicos no mesmo gateway: um contrato AMM por par (gateway roteador via PairRegistry) ou um único contrato multi-par? → A: Opção A — um contrato `AutomatedMarketMaker` por par; o gateway consulta um contrato `PairRegistry` on-chain para resolver o endereço AMM correto dado um `pool_pair` string. Nenhum redesenho do contrato AMM existente é necessário.
- Q: Autoridade para criação de um novo par no PairRegistry — unilateral por qualquer CB, apenas admin do consórcio, ou aprovação bilateral dos dois CBs envolvidos? → A: Opção C — aprovação bilateral on-chain: ambos os CBs do par devem propor e confirmar antes do par ser registrado no `PairRegistry`, espelhando o mecanismo de commit-reveal já existente para formação de pool.
- Q: Comportamento do gateway ao rotear requests para o contrato AMM correto — consulta on-chain por request, cache no startup (restart necessário), ou cache no startup com watcher de eventos? → A: Opção B — cache carregado no startup do gateway mais watcher de eventos on-chain `PairRegistered` que invalida e atualiza o cache em tempo real sem necessidade de restart.
- Q: Formato do identificador de par (`pool_pair`) — hash dos endereços de token, string legível definida pelos CBs, ou UUID auto-gerado? → A: Opção B — string legível escolhida pelos CBs na proposta bilateral (ex.: `"BRL-USD"`, `"BRL-ARS"`); o `PairRegistry` garante unicidade. Mantém consistência com o campo `pool_pair` já existente em todos os modelos e tabelas.
- Q: Escopo da API para criação de par — endpoints REST de proposta/confirmação, apenas GET de listagem, ou criação somente via script on-chain? → A: Opção A — dois novos endpoints REST: `POST /api/v2/amm/pairs/propose` (CB proponente abre proposta com `pool_pair`, `token_a_address`, `token_b_address`, `amm_address`) e `POST /api/v2/amm/pairs/confirm` (CB confirmante aprova); mais `GET /api/v2/amm/pairs` para listar pares ativos. O gateway assina e submete on-chain.
- Q: Qual `ParticipantRole` deve ser usado ao registrar o MLP no `hubRegistry` via `_registerIfNeeded` em `SeedHub.s.sol` — `CENTRAL_BANK`, `MLP` (novo valor no enum), ou ausência de registro no hubRegistry (apenas `grantLiquidityProvider`)? → A: Opção B — criar `ParticipantRole.MLP` no enum do `ParticipantRegistry` e registrar o MLP com esse role. FR-004 exige distinção explícita do MLP em relação aos CBs para fins de autorização e relatório; usar `CENTRAL_BANK` violaria esse requisito. A distinção on-chain por `ParticipantRole.MLP` garante auditabilidade nativa.
- Q: No MVP, o MLP pode depositar liquidez de apenas um lado do par (single-sided) ou apenas de ambos os lados simultaneamente (dual-sided)? → A: Opção A — MVP suporta exclusivamente depósito dual-sided via `addLiquidity(amtA, amtB)`. O MLP como provedor suplementar sempre recompleta ambos os lados. Depósito single-sided pelo MLP (via `addSingleSidedLiquidity`) é explicitamente fora do escopo desta feature e deve ser tratado como evolução futura.
- Q: O MLP precisa de CA própria (`mlp-ca.crt` / `mlp-ca.key`) no `.env.infra.mlp.example`, ou é dispensado de PKI neste MVP? → A: Opção A — o MLP possui CA própria gerada por `make pki.gen-all`, seguindo o padrão de todas as entidades existentes. O compliance service requer `CA_CERT_FILE` no startup; `.env.infra.mlp.example` deve incluir `CA_CERT_FILE=/workspace/backend/config/pki/mlp-ca.crt` e `CA_KEY_FILE=/workspace/backend/config/pki/mlp-ca.key`.
- Q: SC-004 deve ser validado como critério de aceite mandatório no E2E (step MLP sempre executado) ou condicional a `ENABLE_MLP=true`? → A: Opção A — SC-004 é condicional: validado apenas quando `ENABLE_MLP=true`. O MLP é uma feature opt-in por design; ambientes de CI sem o stack MLP não devem ser penalizados. SC-004 atualizado para refletir esta condicionalidade.
- Q: Como adicionar `ParticipantRole.MLP` ao enum existente sem introduzir breaking change? → A: Opção A — adicionar `MLP` diretamente ao enum `ParticipantRole` no contrato Solidity onde ele está definido (provavelmente `IParticipantRegistry` ou `ParticipantRegistry.sol`). Em dev local, o redeploy via `make scenario-b.deploy-contracts` resolve o breaking change. Não criar mapping separado; não usar `CENTRAL_BANK` como substituto.

### Session 2026-05-14

- Q: Quando ambos os commits estão MATCHED e a execução sequencial das transferências falha parcialmente (CB_A ok, CB_B falha), qual é a estratégia de recuperação? → A: Retry automático para CB_B até 5 tentativas com backoff exponencial (2/4/8/16/32s, cap 60s); se esgotar, ambos os commits transitam para `RECONCILIATION_REQUIRED` e um alerta é emitido ao operador — padrão reutilizado do `RelayerQueueItem` existente.
- Q: Qual entidade tem autoridade para registrar ou revogar o papel de MLP no IdentityRegistry? → A: Apenas o admin do contrato (deployer/owner do `IdentityRegistry`) pode chamar `grantLiquidityProvider` e `revokeLiquidityProvider`. A aprovação do MLP ocorre fora da cadeia (consortium agreement); o admin executa o grant após aprovação. Padrão `onlyAdmin` já usado no `IdentityRegistry` existente.
- Q: O sistema deve bloquear ativamente que um único `provider_id` commite os dois lados do mesmo par? → A: Sim — o sistema rejeita um commit se o `provider_id` já tiver um commit PENDING ou MATCHED no lado oposto do mesmo `pool_pair`, retornando erro `SAME_PROVIDER_BOTH_SIDES` (HTTP 409). A restrição é validada na lógica de matching antes de persistir o commit.
- Q: Depósitos adicionais em pool já `ACTIVE` também exigem commit-reveal? → A: Não — o commit-reveal é obrigatório apenas na formação inicial do pool (quando ambas as reservas são zero). Após o pool estar `ACTIVE`, depósitos adicionais são feitos diretamente via `addSingleSidedLiquidity` sem coordenação prévia, já que não existe risco de estado `PENDING_COUNTERPART`.
- Q: O recalculo de `shares_percentage` é síncrono (a cada depósito) ou lazy (calculado na retirada)? → A: Síncrono — a cada depósito o sistema recalcula e persiste `shares_percentage` de **todas** as `LiquidityPosition` ativas do par antes de responder ao request, garantindo `SUM(shares_percentage) = 100%` como invariante persistente no banco de dados.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Depósito Independente por Corredor (Priority: P1)

O BCB (Banco Central do Brasil) deposita tCeBM-BRL no pool BRL-USD sem precisar deter ou controlar tCeBM-USD. Em paralelo (ou posteriormente), o Fed deposita tCeBM-USD no mesmo pool. Ambos passam a ser coproprietários do pool, recebendo LP shares proporcionais às suas contribuições individuais.

**Por que esta prioridade**: É a mudança arquitetural central. Sem este mecanismo, nenhum outro aspecto cooperativo funciona. Resolve diretamente a questão de soberania monetária e é pré-requisito para todos os demais user stories.

**Independent Test**: Pode ser testado em ambiente isolado: BCB deposita apenas BRL → pool tem apenas reserva A; Fed deposita apenas USD → pool agora tem ambas as reservas. Um swap subsequente valida que o pool opera com liquidez bilateral provisionada por dois atores distintos.

**Acceptance Scenarios**:

1. **Given** o pool BRL-USD está vazio, **When** o BCB deposita 100.000 tCeBM-BRL (sem fornecer USD), **Then** o pool registra reserva_A = 100.000, reserva_B = 0, e um LP position do BCB com status ACTIVE é criado com LP shares proporcionais a sua contribuição.

2. **Given** o pool BRL-USD tem reserva_A = 100.000 e reserva_B = 0, **When** o Fed deposita 100.000 tCeBM-USD (sem fornecer BRL), **Then** o pool registra reserva_A = 100.000, reserva_B = 100.000, e um LP position do Fed com status ACTIVE é criado com LP shares proporcionais à sua contribuição.

3. **Given** o pool tem depósitos do BCB e do Fed, **When** um banco comercial executa um swap BRL → USD, **Then** o swap utiliza as reservas combinadas e é executado com sucesso.

4. **Given** o pool BRL-USD está vazio, **When** o BCB registra um commit de 100.000 tCeBM-BRL mas o Fed ainda não commitou sua contraparte, **Then** o pool permanece no estado `PENDING_COUNTERPART`, nenhum fundo é transferido, e swaps são bloqueados com `POOL_NOT_ACTIVE`.

5. **Given** o pool está em estado `PENDING_COUNTERPART` com o commit do BCB registrado, **When** o Fed registra um commit de 100.000 tCeBM-USD, **Then** o sistema executa as duas transferências atomicamente, as reservas do pool são atualizadas, o pool transita para `ACTIVE`, e cada CB recebe LP shares proporcionais à sua contribuição.

6. **Given** o pool está em `PENDING_COUNTERPART` e o prazo de expiração (72h) decorre sem que o segundo CB realize seu commit, **Then** o sistema cancela o commit pendente, nenhum fundo é movido, e o pool retorna ao estado `EMPTY`.

7. **Given** o pool está `ACTIVE` com depósitos do BCB e do Fed, **When** um banco comercial executa um swap BRL → USD, **Then** o swap utiliza as reservas combinadas e é executado com sucesso.

8. **Given** uma entidade não autorizada (sem papel de provedor de liquidez reconhecido), **When** tenta registrar um commit, **Then** o sistema retorna erro de autorização e nenhuma intenção é registrada.

---

### User Story 2 — MLP como Provedor Suplementar (Priority: P2)

O MLP (Multilateral Liquidity Provider — ex.: BID ou consórcio multilateral) pode depositar ambas as moedas de um par simultaneamente (*dual-sided*) quando um Banco Central não tem capacidade de provisionar no momento, garantindo que o corredor permança operacional. **No MVP, o MLP opera exclusivamente via depósito dual-sided** (`addLiquidity`); depósito single-sided pelo MLP é fora do escopo desta feature (ver Out of Scope).

**Por que esta prioridade**: Garante resiliência do mercado mesmo quando CBs não operam ativamente. Essencial para o modelo multilateral descrito no D2.

**Independent Test**: Pode ser testado cadastrando o MLP como provedor autorizado, realizando um depósito de ambas as moedas pelo MLP, e verificando que swaps funcionam normalmente usando as reservas do MLP. O MLP também deve conseguir remover sua liquidez.

**Acceptance Scenarios**:

1. **Given** o MLP está registrado como provedor autorizado no sistema, **When** o MLP deposita tCeBM-BRL e tCeBM-USD no pool BRL-USD, **Then** um LP position do MLP com status ACTIVE é criado e as reservas do pool aumentam.

2. **Given** o MLP tem um LP position ACTIVE, **When** o MLP solicita remoção de liquidez, **Then** o sistema retorna os ativos proporcionais à sua participação atual no pool (ajustada pela atividade de swaps ocorrida), e o LP position transita para WITHDRAWN.

3. **Given** uma entidade registrada apenas como banco comercial, **When** tenta ser registrada como MLP, **Then** o sistema rejeita a solicitação, pois apenas o admin do contrato (`IdentityRegistry` deployer/owner) pode conceder o papel de MLP via `grantLiquidityProvider` — a promoção não pode ser solicitada pela própria entidade.

---

### User Story 3 — Distribuição Proporcional de Taxas por LP (Priority: P2)

Cada swap executado por bancos comerciais no pool gera uma taxa de transação. Essa taxa é acumulada no pool e distribuída proporcionalmente entre todos os provedores de liquidez ativos, de acordo com suas participações no momento do swap.

**Por que esta prioridade**: Cria incentivo econômico para que CBs e MLP mantenham liquidez no pool a longo prazo. Sem isso, o modelo cooperativo funciona tecnicamente, mas sem modelo de negócio.

**Independent Test**: Com BCB (60% das shares) e Fed (40% das shares), execute N swaps de bancos comerciais. Calcule taxas acumuladas. Ao remover liquidez, BCB recebe 60% das taxas + sua reserva proporcional; Fed recebe 40% das taxas + sua reserva proporcional.

**Acceptance Scenarios**:

1. **Given** o pool tem BCB com 60% das LP shares e Fed com 40%, **When** um banco comercial executa um swap gerando taxa de 100 unidades, **Then** 60 unidades são alocadas para o BCB e 40 para o Fed na contabilidade interna do pool.

2. **Given** taxas foram acumuladas para um LP, **When** esse LP solicita remoção de liquidez, **Then** recebe seus ativos proporcionais à participação **mais** as taxas acumuladas em seu favor.

3. **Given** um LP deposita após swaps já terem ocorrido, **When** esse LP solicita remoção de liquidez futuramente, **Then** recebe taxas apenas dos swaps ocorridos **após** seu depósito (não participa de taxas geradas antes de sua entrada).

---

### User Story 4 — Retirada Proporcional ao Saldo Atual do Pool (Priority: P3)

Na remoção de liquidez, o provedor não recebe necessariamente os mesmos tokens na mesma proporção que depositou. Recebe uma **fatia proporcional do saldo atual** do pool (que pode ter mudado devido a swaps), mais as taxas acumuladas.

**Por que esta prioridade**: Alinha o modelo com o funcionamento real de AMMs (Uniswap v2 padrão). Sem isso, a remoção pode deixar o pool em estado inconsistente após muitos swaps.

**Independent Test**: Pool inicia com reserva_A = 1.000, reserva_B = 1.000. Após swaps: reserva_A = 1.200, reserva_B = 833. LP com 100% de participação remove tudo: recebe 1.200 do token A e 833 do token B (não os 1.000/1.000 originais).

**Acceptance Scenarios**:

1. **Given** o pool teve swaps que alteraram as reservas desde o depósito de um LP, **When** esse LP remove sua liquidez, **Then** recebe sua proporção do **saldo atual** das reservas (não o valor original depositado).

2. **Given** o pool tem dois LPs (BCB e Fed) e as reservas mudaram por swaps, **When** ambos removem liquidez, **Then** cada um recebe exatamente sua proporção do saldo atual, sem deixar resíduos no pool.

---

### Edge Cases

- O que acontece se o mesmo `provider_id` tentar commitar ambos os lados do mesmo par (ex.: CB em ambiente de teste)? O sistema retorna `SAME_PROVIDER_BOTH_SIDES` (HTTP 409) e nenhum segundo commit é registrado. A restrição garante o princípio de soberania monetária em nível de validação, não apenas de governança.
- O que acontece se apenas um CB registra commit e o outro nunca o faz? O commit expira após 72 horas sem transferência de fundos; o pool permanece `EMPTY`.
- O que acontece se ambos os CBs tentam remover liquidez simultaneamente e o pool não tiver reservas suficientes para ambos? O sistema deve processar de forma atômica por ordem de chegada e rejeitar o segundo se as reservas forem insuficientes.
- Como o sistema lida com uma entidade que era LP e perde seu papel de provedor autorizado? Seu LP position existente permanece válido para remoção, mas novos depósitos são bloqueados.
- O que acontece se após o MATCH de dois commits a transação de `addSingleSidedLiquidity` do CB_A for confirmada on-chain mas a do CB_B falhar? O backend retenta CB_B até 5 vezes com backoff exponencial (2/4/8/16/32s, cap 60s). Se todas as tentativas falharem, ambos os commits transitam para `RECONCILIATION_REQUIRED`, o operador é alertado, e o pool permanece em `PENDING_COUNTERPART` com reserveA atualizado até resolução manual.
- O que acontece se a taxa de swap configurada for zero? O pool opera normalmente sem distribuição de taxas (compatibilidade retroativa com o modelo atual).
- O que acontece com LP positions criados no modelo antigo (dual-sided por um único provedor)? Devem permanecer válidos e removíveis conforme o modelo antigo durante a migração.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE permitir que um provedor de liquidez autorizado registre uma **intenção de depósito (commit)** para apenas um lado de um par (somente token A ou somente token B), sem transferir fundos neste momento. O mecanismo de commit-reveal é obrigatório **apenas na formação inicial** do pool (estado `EMPTY`). Depósitos adicionais em pool já `ACTIVE` são feitos diretamente via `addSingleSidedLiquidity` sem commit prévio. O endpoint `GET /api/v2/amm/pool/{pair}/status` DEVE derivar `pool_status` usando lógica DB-first + on-chain: (1) se existir `pool_commit` com `status IN (PENDING, MATCHED)` para o par → `PENDING_COUNTERPART`; (2) senão se `reserve_a > 0 AND reserve_b > 0` via `AMM.getReserves()` → `ACTIVE`; (3) senão → `EMPTY`.

- **FR-002**: O sistema DEVE, ao receber commits de ambos os lados de um par (token A e token B) de provedores autorizados **distintos**, executar as duas transferências **de forma sequencial no mesmo handler HTTP** (duas transações on-chain separadas, não uma tx única) e ativar o pool (`ACTIVE`). Em caso de falha parcial (primeira transferência on-chain confirmada, segunda falhou), ambos os commits DEVEM transitar para `RECONCILIATION_REQUIRED` e o pool permanece em estado inativo — ver FR-014 para retry automático (Fase 2). **[REQUISITO DE SOBERANIA — BLOQUEADOR]**: cada transferência on-chain DEVE ser executada pelo signer do CB emissor do token correspondente: CB-A's signer executa `addSingleSidedLiquidity(TOKEN_A)` usando balance de TOKEN_A; CB-B's signer executa `addSingleSidedLiquidity(TOKEN_B)` usando balance de TOKEN_B. Nenhum CB deve deter, aprovar ou depositar tokens emitidos por outro CB. Isso exige arquitetura **multi-gateway com protocolo de matching cross-gateway** (especificada em `007-bridge-based-cb-liquidity`) — o matching single-DB implementado no MVP é um bloqueador de produção, não uma limitação aceitável. Nenhum fundo é transferido enquanto apenas um lado tiver commit registrado. O sistema DEVE rejeitar (`SAME_PROVIDER_BOTH_SIDES`, HTTP 409) qualquer commit cujo `provider_id` já possua um commit PENDING ou MATCHED no lado oposto do mesmo `pool_pair`.

- **FR-003**: O sistema DEVE calcular e atribuir LP shares de forma dinâmica e proporcional à contribuição de cada provedor em relação ao total do pool. A cada novo depósito (inicial via commit-reveal ou adicional direto em pool `ACTIVE`), o sistema DEVE recalcular e persistir `shares_percentage` de **todas** as `LiquidityPosition` ativas do par de forma síncrona antes de responder ao request, garantindo `SUM(shares_percentage) = 100%` como invariante persistente. Swaps **não** disparam recalculo de `shares_percentage` — apenas eventos de depósito ou remoção de liquidez o fazem. A variação de valor causada por swaps é capturada via `AMM.getReserves()` no momento da remoção (FR-007), sem necessidade de atualizar `shares_percentage` a cada swap.

- **FR-004**: O sistema DEVE suportar o papel de MLP (Multilateral Liquidity Provider) como tipo de provedor autorizado, distinguindo-o de Banco Central e de banco comercial para fins de autorização e relatório. O registro e revogação do papel de MLP só podem ser executados pelo admin do contrato (`IdentityRegistry` deployer/owner) via `grantLiquidityProvider` / `revokeLiquidityProvider`. O MLP DEVE ser registrado no `ParticipantRegistry` com `ParticipantRole.MLP` (valor dedicado no enum — não `CENTRAL_BANK`), garantindo distinção on-chain auditável e alinhada com o requisito de separação de papéis. O valor `MLP` deve ser adicionado diretamente ao enum `ParticipantRole` no contrato Solidity onde ele está definido; em dev local, o redeploy via `make scenario-b.deploy-contracts` resolve o breaking change sem comprometer o ambiente de produção.

- **FR-005**: O sistema DEVE coletar uma taxa percentual configurável por swap executado (ex.: 0,3%) e acumular esta taxa no pool para distribuição posterior entre os LPs.

- **FR-006**: O sistema DEVE distribuir as taxas acumuladas proporcionalmente entre os LPs ativos de acordo com suas participações no momento de cada swap. A distribuição DEVE ocorrer de forma **síncrona no handler HTTP** do swap (best-effort): o `LPFeeEvent` DEVE ser persistido e `fee_claim_accumulated` DEVE ser atualizado em todas as `LiquidityPosition` com `status = ACTIVE` do par antes de retornar a resposta `200 COMPLETED` ao chamador, **quando os LPs existem no DB local do gateway executante**. Falhas de distribuição (ex.: ausência de LP positions no DB local em arquitetura multi-gateway) DEVEM ser registradas como warning e NÃO DEVEM bloquear o `200 COMPLETED` do swap — o estado on-chain da transação é a fonte de verdade. Workers assíncronos ou cálculo lazy na remoção NÃO são o mecanismo primário; a distribuição síncrona best-effort é preferida para minimizar janela de inconsistência.

- **FR-007**: Na remoção de liquidez, o sistema DEVE retornar ao provedor sua fatia proporcional do saldo **atual** das reservas do pool (não o valor original depositado), mais as taxas acumuladas em seu favor desde o depósito. As reservas atuais DEVEM ser obtidas via chamada `AMM.getReserves()` on-chain no momento da remoção — não do valor cacheado em `pool_state_readings` — garantindo que o cálculo reflita o estado canônico da chain. Fórmula: `returnA = getReserves().reserveA * shares_percentage / 100`; `returnB = getReserves().reserveB * shares_percentage / 100`.

- **FR-008**: O sistema DEVE garantir que depósitos de diferentes provedores para um mesmo par sejam tratados como contribuições ao mesmo pool, consolidando as reservas para fins de cálculo de swap. Depósitos adicionais em pool `ACTIVE` são processados diretamente (sem commit-reveal), cada um criando ou atualizando a `LiquidityPosition` do provedor e recalculando todos os `shares_percentage` ativos.

- **FR-009**: O sistema DEVE impedir que qualquer entidade não autorizada como provedor de liquidez (CB ou MLP) deposite ou remova liquidez do pool.

- **FR-010**: O sistema DEVE preservar a operabilidade de LP positions criados no modelo anterior (depósito dual por único provedor), permitindo sua remoção pelo modelo antigo durante período de migração.

- **FR-011**: O sistema DEVE bloquear swaps enquanto o pool não estiver no estado `ACTIVE` (pool `EMPTY` ou `PENDING_COUNTERPART`), retornando `POOL_NOT_ACTIVE`.

- **FR-013**: O sistema DEVE cancelar automaticamente um commit em estado `PENDING` após o período de expiração configurável (padrão: 72 horas) sem que a contraparte registre seu commit, sem mover nenhum fundo.

- **FR-012**: O sistema DEVE registrar, de forma auditável e append-only, cada evento de depósito, remoção e distribuição de taxa por LP, com identificação do provedor e timestamp.

- **FR-014** *(Post-MVP — Fase 2 / T059; não implementado no MVP atual)*: Quando a execução atômica de um par de commits MATCHED falhar parcialmente (primeira transferência on-chain confirmada, segunda falhou), o sistema DEVE retentar a segunda transferência automaticamente até 5 vezes com backoff exponencial (2s, 4s, 8s, 16s, 32s, cap 60s). Ao esgotar todas as tentativas, ambos os commits DEVEM transitar para `RECONCILIATION_REQUIRED` e um alerta DEVE ser emitido ao operador. O pool permanece em `PENDING_COUNTERPART` até resolução manual. **No MVP, falhas parciais resultam em pool travado em `PENDING_COUNTERPART` sem retry automático — intervenção manual é necessária.**

- **FR-015**: O sistema DEVE suportar múltiplos pares de moedas simultâneos num mesmo gateway. Cada par é representado por um contrato `AutomatedMarketMaker` independente. O api-gateway consulta um contrato `PairRegistry` on-chain para resolver o endereço AMM correto dado um identificador de par (`pool_pair`). Nenhum redesenho do contrato AMM existente é necessário — apenas um novo contrato `PairRegistry` é introduzido.

- **FR-016**: O registro de um novo par no `PairRegistry` DEVE exigir aprovação bilateral on-chain: ambos os Bancos Centrais do par (emissor de token A e emissor de token B) devem propor e confirmar o par antes de ele ser ativado. O identificador do par (`pool_pair`) é uma string legível definida pelo CB proponente (ex.: `"BRL-USD"`) e deve ser única no `PairRegistry`. O gateway DEVE manter o mapeamento `pool_pair → endereço AMM` em cache local atualizado no startup e a cada evento `PairRegistered(pairId, ammAddress, tokenA, tokenB)` emitido pelo `PairRegistry`, sem necessidade de restart para reconhecer novos pares.

- **FR-017**: O sistema DEVE expor três novos endpoints REST para gestão de pares: `POST /api/v2/amm/pairs/propose` (CB proponente registra proposta com `pool_pair`, `token_a_address`, `token_b_address`, `amm_address`), `POST /api/v2/amm/pairs/confirm` (CB confirmante aprova proposta pendente pelo `pool_pair`), e `GET /api/v2/amm/pairs` (lista todos os pares ativos com seus endereços AMM). O gateway assina e submete as transações on-chain após validação de autorização de cada CB chamador.

- **FR-018**: Os endpoints `POST /api/v2/amm/token/mint-and-approve` e `POST /api/v2/amm/token/approve-amm` DEVEM aceitar campo único `amount` (string de inteiro ≥ 0) em lugar de `amount_a`/`amount_b`. Para **`mint-and-approve`** (sem `recipient`): o gateway consulta o `CENTRAL_BANK_ROLE` do signer autenticado nos contratos configurados (`HUB_TOKEN_A_ADDRESS`/`HUB_TOKEN_B_ADDRESS`) e minta/aprova exclusivamente o token em que o signer possui role — o outro é ignorado. Payload: `{ "amount": "200000" }`. Para **`mint-and-approve` com `recipient`**: cada CB minta apenas o token de sua emissão na carteira do destinatário (`{ "amount": "50000", "recipient": "0x..." }`); o uso de `recipient` é restrito a destinatários **não-CB** (ex.: banco comercial) — nenhum CB deve receber tokens emitidos por outro CB via este endpoint. **[ANTI-PATTERN DEPRECIADO]**: o uso de `recipient` com endereço de signer de outro CB (padrão G5-cross) está depreciado e deve ser bloqueado por validação no gateway em versão futura (verificar se `recipient` é um signer de CB via `IdentityRegistry.getCentralBankOf`). Para **`approve-amm`**: CBs omitem `side` (role detecta automaticamente o token a aprovar); bancos comerciais DEVEM informar `side: "A" | "B"`. Gateway retorna HTTP 400 `{ "error": "'side' required for non-central-bank callers" }` se banco comercial omitir o campo. Campos `amount_a`/`amount_b` são **depreciados** e rejeitados com HTTP 400 `{ "error": "use 'amount' instead of 'amount_a'/'amount_b'" }`. Implementado em `token_handler.go` e `amm_adapter.go`.

### Key Entities

- **LiquidityPosition**: Representa a participação de um único provedor em um pool. Passa a ter campos de `shares_percentage` (dinâmico), `fee_claim_accumulated`, `contributed_a` e `contributed_b` (podendo um deles ser zero no modelo unilateral). Status: `ACTIVE` → `WITHDRAWN`.

- **PoolState**: Representa o estado corrente de um pool (reserva_A, reserva_B, total_LP_shares, accumulated_fees_a, accumulated_fees_b, fee_rate_bps). Atualizado a cada depósito, remoção ou swap. O campo `pool_status` exposto em `GET /api/v2/amm/pool/{pair}/status` é derivado por lógica DB-first + on-chain: presença de `pool_commit` com `status IN (PENDING, MATCHED)` → `PENDING_COUNTERPART`; `reserve_a > 0 AND reserve_b > 0` → `ACTIVE`; senão → `EMPTY`. O campo `total_lp_count` DEVE ser calculado como `COUNT(*) FROM liquidity_positions WHERE pool_pair = ? AND status = 'ACTIVE'` **no DB local do gateway que processa a request**. Em deployment multi-gateway, gateways de bancos comerciais que não hospedam LP positions retornarão `total_lp_count = 0` — comportamento esperado; a fonte canônica é o gateway LP-owner (ex.: CB-A após commit-reveal cooperativo).

- **LPFeeEvent**: Registro append-only de cada taxa gerada por swap, com distribuição calculada por LP no momento do evento. Suporte a auditoria e reconciliação.

- **AuthorizedProvider**: Registro de entidades autorizadas como provedores de liquidez. Tipos: `CENTRAL_BANK`, `MLP` (valor dedicado no enum `ParticipantRole` — não sobrepõe `CENTRAL_BANK`). Separado da autorização de bancos comerciais. O MLP é registrado no `ParticipantRegistry` on-chain com `ParticipantRole.MLP`; adicionalmente recebe `grantLiquidityProvider` no `IdentityRegistry`.

- **PoolCommit**: Intenção de depósito registrada por um provedor para um par específico, antes da transferência de fundos. Campos: `commit_id`, `provider_id`, `pool_pair`, `side` (A ou B), `amount`, `status` (`PENDING` → `MATCHED` → `EXECUTED` | `EXPIRED`), `expires_at`. Expira automaticamente após 72 horas sem contraparte.

- **PairRegistry** *(novo)*: Contrato on-chain que mapeia identificadores de par (`pool_pair` string legível, ex.: `"BRL-USD"`) para endereços de contratos AMM independentes. Garante unicidade da string `pool_pair`. Suporta fluxo de aprovação bilateral (proposta pelo CB_A + confirmação pelo CB_B). Emite evento `PairRegistered(pairId, ammAddress, tokenA, tokenB)` ao ativar um par. O api-gateway usa este contrato como fonte autoritativa de roteamento, mantendo cache local atualizado via event watcher.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Dois Bancos Centrais distintos conseguem formar um pool operacional BRL-USD — cada um depositando apenas a sua própria moeda via mecanismo de commit-reveal — sem que nenhum deles precise deter a moeda do outro, em até 4 interações totais com o sistema (commit A + commit B + ativação + confirmação).

- **SC-008**: Um commit registrado sem contraparte é automaticamente cancelado e nenhum fundo é movido após 72 horas, sem necessidade de intervenção manual.

- **SC-002**: Após N swaps de bancos comerciais, cada LP ativo recebe ao remover sua liquidez exatamente sua proporção calculada das reservas atuais e das taxas acumuladas, com desvio máximo de 0,01% devido a arredondamentos. O `shares_percentage` de cada LP é recalculado de forma síncrona a cada depósito, mantendo `SUM(shares_percentage) = 100%` como invariante persistente — desvio acimá de 0,01% deve ser tratado como falha de cálculo.

- **SC-003** *(Validação manual no MVP — tryout E2E de 2026-05-20 confirmou ~1s por swap, dentro do SLA; benchmark automatizado previsto em T060 / Fase 2)*: Swaps de bancos comerciais continuam sendo executados dentro do SLA de p95 ≤ 6s mesmo com o pool formado por múltiplos provedores independentes.

- **SC-004**: *Condicional a `ENABLE_MLP=true`.* Quando o stack MLP está ativo, um MLP consegue depositar e remover liquidez dual-sided de forma indistinguível de um CB do ponto de vista operacional (mesmos endpoints REST, mesmo modelo de resposta, mesmas garantias de auditabilidade). O script E2E valida SC-004 apenas quando `ENABLE_MLP=true`; ambientes sem o stack MLP ativo ficam isentos deste critério.

- **SC-005**: LP positions criados no modelo anterior (dual-sided por único provedor) permanecem removíveis sem erros durante o período de migração, sem exigir migração de dados manual.

- **SC-006**: Em deployments **single-gateway** (gateway de swap e gateway LP-owner compartilham o mesmo DB), 100% das taxas coletadas em swaps são distribuídas entre os LPs ativos, sem taxas perdidas ou acumuladas sem destinatário. Em deployments **multi-gateway** (padrão de produção com CB-A, CB-B e bank-a em gateways isolados), a distribuição ocorre apenas para LPs cujas posições existem no DB do gateway executante do swap; LPs em outros gateways não recebem crédito de taxa via mecanismo síncrono — limitação arquitetural documentada (ver Session 2026-05-20). A resolução completa de SC-006 em multi-gateway requer store compartilhado de `liquidity_positions` ou chamada gRPC inter-gateway (T059, fora do escopo desta feature).

- **SC-007**: O histórico completo de quem depositou quanto, quando, e quanto recebeu em taxas é auditável via consulta sem necessidade de reconstrução de estado a partir de eventos on-chain.

- **SC-009**: Dois Bancos Centrais distintos conseguem propor, confirmar e ativar um novo par (ex.: BRL-ARS) via `POST /api/v2/amm/pairs/propose` + `POST /api/v2/amm/pairs/confirm` em até 2 interações REST totais, sem restart do gateway. Após ativação, o gateway roteia swaps e depósitos para o AMM correto do novo par sem necessidade de reconfig manual. O par fica visível em `GET /api/v2/amm/pairs` imediatamente após a confirmação on-chain.

---

## Assumptions

- O modelo de taxa de swap de **0,3% por operação** (padrão de mercado para AMMs de produto constante) é assumido como valor default configurável, podendo ser ajustado por governança dos CBs via Circuit Breaker.
- A coordenação entre os Bancos Centrais para formação do pool utiliza um **mecanismo de commit-reveal em duas fases**: na Fase 1, cada CB registra sua intenção de depósito (commit) com o valor e o par pretendidos, sem transferir fundos; na Fase 2, após ambos os lados terem commitado, o sistema executa as transferências atomicamente e ativa o pool. O pool só é considerado **ACTIVE** após ambas as transferências confirmadas. Um commit sem contraparte expira após período configurável (padrão: 72 horas), devolvendo a intenção sem movimentação de fundos.
- O MLP não tem autoridade de governança (não pode pausar o AMM via Circuit Breaker); apenas CBs mantêm esse poder.
- O MLP possui CA própria (`mlp-ca.crt` / `mlp-ca.key`) gerada pelo mesmo mecanismo de `pki.gen-all` usado para as demais entidades. O `compliance-mlp` requer `CA_CERT_FILE` no startup, por isso o MLP não é dispensado de PKI mesmo sendo um consórcio (não entidade soberana).
- O registro do papel de MLP no `IdentityRegistry` é executado pelo admin do contrato (deployer/owner) após aprovação off-chain pelo consórcio dos Bancos Centrais participantes. Não há mecanismo de auto-solicitação on-chain por parte do MLP.
- LP positions com `contributed_b = 0` (depósito apenas do lado A) são válidos e registrados, mas swaps que requerem saída do lado B são bloqueados até que o lado B tenha reservas suficientes.
- A taxa de swap é calculada sobre o `amount_in` (input do swap) antes da execução, e a taxa é retida nas reservas do pool (não transferida a um endereço separado).
- LP shares existentes no modelo anterior (calculados como `sqrt(A × B)`) são tratados como 100% da participação do único provedor e permanecem válidos durante a migração; novos depósitos coexistem com LP shares do modelo antigo sem conflito.
- O escopo desta feature **não inclui** a interface de usuário — as mudanças são exclusivamente no contrato inteligente, nos microserviços backend e na API REST/gRPC.

---

## Out of Scope

- Interface frontend para gestão de LP positions cooperativos.
- Mecanismo de leilão de liquidez (auction-based liquidity provision).
- Rebalanceamento automático de pool por algoritmo externo.
- LP tokens transferíveis (ERC-20 padrão) como shares negociáveis em mercado secundário — a participação é não-transferível neste modelo.
- Integração com oráculos externos para ajuste dinâmico de taxa de swap.
- Interface frontend para proposta e confirmação de novos pares (os endpoints REST são expostos, mas a UI não faz parte desta feature).
- Depósito single-sided pelo MLP via `addSingleSidedLiquidity` — o MLP opera exclusivamente com depósito dual-sided no MVP. O suporte single-sided para o MLP fica adiado para evolução futura (o `grantLiquidityProvider` já está concedido, permitindo habilitar sem redeploy).
- **Retry de commit parcialmente executado e estado `RECONCILIATION_REQUIRED` (FR-014)**: o mecanismo de retry com backoff exponencial (5 tentativas, 2/4/8/16/32s) e transição para `RECONCILIATION_REQUIRED` após esgotar tentativas não estão implementados no MVP. Falhas parciais de commit-reveal deixam o pool em `PENDING_COUNTERPART` até intervenção manual. Previsto para Fase 2 (T059) reutilizando o padrão de retry do Relayer Cacti.
- **Corredor BRL-ARS / CB-C (Argentina)**: o PairRegistry foi projetado para suportar pares adicionais (ex.: `"BRL-ARS"`) envolvendo um terceiro Banco Central (CB-C, emissor de `tCeBM_ARS`). O script de deploy de referência (`DeployArs.s.sol`) e o target `make contracts.deploy-ars` foram removidos do repositório por estarem fora do escopo do MVP validado (corredor `BRL-USD` com CB-A e CB-B é suficiente para demonstrar o fluxo bilateral do PairRegistry). Para habilitar o corredor BRL-ARS em versão futura: (1) criar `script/DeployArs.s.sol` deployando `tCeBM_ARS` + `AutomatedMarketMaker` + `grantLiquidityProvider(cbcAddress)`, (2) registrar `setCentralBankOf(tokenARS, cbcAddress)` no `IdentityRegistry`, (3) subir o stack `backend/docker-compose-backend.central-bank-c.yaml`, (4) chamar `POST /amm/pairs/propose` (CB-A) + `POST /amm/pairs/confirm` (CB-C) para ativar o par.
- **Padrão G5-cross** *(DEPRECIADO — anti-pattern de soberania monetária)*: o mecanismo de CB-B mintar TOKEN_B para o endereço signer de CB-A de modo que CB-A execute o depósito on-chain foi implementado no MVP desta feature mas viola o princípio de soberania monetária. Está depreciado e será substituído pelo fluxo de bridge-based CB liquidity provisioning especificado em `007-bridge-based-cb-liquidity`. Nenhuma nova funcionalidade deve ser construída sobre o padrão G5-cross.
- **Protocolo de matching multi-gateway** *(pré-requisito bloqueador de produção)*: o matching cross-gateway (commit A no gateway de CB-A, commit B no gateway de CB-B, coordenação de execução entre os dois signers soberanos) é requisito obrigatório para que FR-002 respeite a soberania monetária. Não foi implementado no MVP — toda a lógica de matching usa lookup DB-local, obrigando ambos os commits ao mesmo gateway. O protocolo multi-gateway é escopo da `007-bridge-based-cb-liquidity`.
