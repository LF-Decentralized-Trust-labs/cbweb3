# ADR-003: Privacidade no hub do Scenario B

**Status**: Proposto — mecanismo decidido pela arquitetura (2026-07-28); descope
pendente de sign-off IDB/LNet e de emenda à constituição
**Data**: 2026-07-23 · **Decisão de mecanismo**: 2026-07-28 · **Reverificado**: 2026-08-22
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — decisão exige sign-off IDB/LNet
**Findings relacionados**: 9.2 (feedback LNet, Rodada 2)

---

## Pedido de decisão

Este bloco existe para que a decisão possa ser tomada sem ler o ADR inteiro. O corpo
abaixo continua sendo a fundamentação.

| | |
|---|---|
| **O que se pede** | Aprovar formalmente o **descope da privacidade no hub nesta fase** (Opção C), com os controles compensatórios e o gatilho de reabertura descritos abaixo. Não se pede escolher entre A, B e C: a Opção A já foi descartada pela arquitetura (ver abaixo). |
| **Quem assina** | IDB e LNet — a decisão expõe posições de liquidez soberana ao conjunto de leitores do hub. |
| **Pré-requisito interno** | Emenda de escopo à constituição (Princípio II proíbe valor em claro on-chain). Um ADR não se auto-autoriza esse desvio. |
| **Se aprovado, desbloqueia** | Fechamento de `[R1-9.2]` e `[R2-9.2]` — que são o mesmo finding em dois relatórios e devem ser tratados como um só item. |
| **Se não for aprovado** | A alternativa deixa de ser a Opção A (descartada por design) e passa a ser a Opção D (pseudo-anonimidade por rotação de endereços), que **não** protege valores — apenas atribuição. Cifrar o AMM voltaria a exigir redesenhar a Liquidity Pool. |

### Decisão de mecanismo já tomada (arquitetura, 2026-07-28)

Registrada aqui porque estava apenas no cartão do finding, onde nenhum revisor do ADR
a encontraria:

- **A privacidade no hub não será implementada via Paladin.** Não é possível
  reconciliar o uso do Paladin com o design da Liquidity Pool — o AMM precisa ler
  reservas em claro para calcular a curva. Isso rebaixa a **Opção A de "custosa e
  arriscada" para "incompatível com o design"**, o que é uma diferença material: não é
  um trade-off de esforço, é uma via fechada.
- **Direção alternativa registrada**: um modelo de carteiras em que os endereços podem
  ser gerados por transação, dando **pseudo-anonimidade** — documentada abaixo como
  Opção D. Ela **substitui a Opção B** como caminho de reavaliação preferido.
- **Prioridade**: baixa no momento. Não bloqueia entrega; o que bloqueia o fechamento
  do finding é o sign-off do descope, não engenharia.

**Proveniência**: alinhamento técnico entre a arquitetura CBWeb3 (AH/GL) e a
contraparte técnica do cliente, 2026-07-28, registrado no cartão do finding **R2-9.2**
no tracker do projeto. Não há artefato no repositório por trás dessa decisão além deste
ADR — é exatamente por isso que ela está transcrita aqui: um cartão de tracker não
sobrevive à migração de ferramenta, e esta é a decisão que rebaixa a Opção A de cara
para fechada.

---

## Contexto

O Scenario B (International Hub) tem topologia hub-and-spoke. Hoje as garantias de
privacidade não são uniformes entre as camadas:

- **O hub (chain 1337) opera em texto claro.** Os contratos de liquidez e câmbio
  soberano — `AutomatedMarketMaker` (AMM), `FXAgreement` e
  `LiquidityCommitRegistry` — executam no hub sem camada de privacidade. Valores,
  reservas e posições de liquidez ficam visíveis a quem consegue ler a cadeia do hub.

  **Correção de evidência (2026-08-22).** A redação original apoiava-se em
  `scenario-b/deploy/local/hub-besu/startBesu.sh:5-9`, que listava peers de bancos
  comerciais no hub. Esse arquivo **não existe mais**: o caminho legado `deploy/local`
  foi retirado em `3b14ecaa` e o hub passou a ser provisionado pelo toolkit, onde
  `scenario-b/provisioning/templates/hub.compose.yaml` declara **um único serviço**,
  `hub-validator`. O conjunto de **nós** do hub é, portanto, hoje apenas o validador.

  Isso muda a forma da exposição, **não** a substância: os templates de backend,
  compliance, payment e relayer de **toda** entidade — incluindo bancos comerciais —
  recebem `HUB_BESU_RPC_URL` (`scenario-b/provisioning/templates/entity-{backend,
  compliance,payment,relayer}.compose.yaml`), e a porta RPC do hub é publicada no host
  (`hub.compose.yaml:25`). O leitor deixou de ser um *peer* e passou a ser um *cliente
  JSON-RPC*, com o mesmo acesso de leitura a reservas, posições e swaps. Portanto o
  conjunto de observadores **continua não restrito a autoridades**.

- **Paladin/Zeto existe apenas nos spokes.** O runbook de deployment declara
  explicitamente essa fronteira:
  - `scenario-b/docs/runbooks/deployment-runbook.md:32`: "Paladin/Zeto privacy is
    available on spokes but not on the hub."
  - Reforçado pela doc de pacote do `OversightService`
    (`scenario-b/backend/services/compliance/internal/services/oversight_service.go:5-7`):
    "Paladin operates exclusively at the spoke level."

**Conclusão do contexto**: as transferências de valor entre bancos dentro de um
spoke têm privacidade (Zeto), mas a **liquidez soberana e o câmbio no hub são
públicos** para o conjunto de validadores do hub. Isso expõe posições de reserva,
paridades e movimentos de liquidez entre bancos centrais — dado de sensibilidade
soberana. O finding 9.2 exige uma decisão explícita: privacidade no hub, sim, não,
ou como; e, se não, descope formal com sign-off.

---

## Opções

### Opção A — Paladin/Zeto no hub para as posições de liquidez soberana

Estender o modelo de privacidade ao hub, cifrando as posições e movimentos do
AMM/`LiquidityCommitRegistry` (por exemplo, comprometimentos de liquidez como UTXOs
privados), mantendo o `FXAgreement` como coordenador.

**Prós**
- Privacidade uniforme fim-a-fim; posições de reserva soberana deixam de ser
  públicas aos validadores do hub.
- Consistente com a direção de privacidade dos spokes.

**Contras**
- **Alta complexidade.** AMM exige matemática de curva sobre valores; cifrar isso
  com preservação de invariantes (por exemplo, produto constante) é problema em
  aberto e caro em ZKP.
- Impacto direto de performance no caminho crítico de swap; exige revalidar
  `make scenario-b.perf-baseline`.
- Reformula os contratos mais ativos do Scenario B — risco alto de regressão em
  atomicidade (lock→mint / burn→unlock) e no circuit breaker.
- Não está no escopo do D6 v2; seria expansão material de escopo.

### Opção B — Privacidade seletiva: cifrar apenas os dados sensíveis, manter o AMM público

Manter o mecanismo do AMM público (preço/curva), mas cifrar os **identificadores e
volumes atribuíveis** por participante — por exemplo, tornar os commits do
`LiquidityCommitRegistry` não-atribuíveis publicamente, revelando apenas agregados.

**Prós**
- Protege o dado mais sensível (quem tem qual posição) sem reescrever a matemática
  do AMM.
- Escopo intermediário, mais viável que a Opção A.

**Contras**
- Privacidade parcial; análise de fluxo ainda pode inferir padrões a partir dos
  agregados públicos.
- Introduz complexidade de modelagem (o que é agregado público vs. atribuível
  privado) que precisa de validação regulatória.

### Opção C — Descope formal com controles compensatórios

Registrar formalmente que o hub opera em texto claro nesta fase, com sign-off
IDB/LNet, apoiado em controles compensatórios: rede permissionada (QBFT, membros
conhecidos), isolamento de rede, trilha de auditoria e — como **ação requerida** —
**restringir a autoridades quem consegue ler o hub**. Desde a migração para o toolkit
isso não é mais remover peers (o hub tem um único nó), e sim restringir o alcance da
RPC do hub, hoje distribuída a todo backend de entidade e publicada no host. Tratar
privacidade no hub como trabalho futuro explicitamente fora do escopo atual.

**Prós**
- Reconhece a realidade da implementação e do escopo do D6 v2 sem introduzir risco
  técnico alto no caminho crítico.
- A rede é permissionada (QBFT, membros conhecidos), o que limita a superfície de
  exposição a um conjunto fechado e auditável — **embora o conjunto de leitores não
  seja hoje exclusivamente de autoridades**. O conjunto de **nós** já é (um único
  `hub-validator`); o de **leitores por RPC** não, porque todo backend de entidade
  recebe `HUB_BESU_RPC_URL` e a porta é publicada no host (ver contexto). Restringir
  o alcance dessa RPC é **pré-requisito** desta opção, não uma premissa já satisfeita.
- Desbloqueia a entrega mantendo a decisão documentada e revisável.

**Contras**
- Posições de liquidez soberana permanecem visíveis ao conjunto de validadores do
  hub; depende da confiança nesse conjunto.
- Exige sign-off explícito e reabertura futura se o conjunto de validadores crescer
  ou a sensibilidade aumentar.

### Opção D — Pseudo-anonimidade por rotação de endereços (direção escolhida para reavaliação)

Manter o hub em claro, mas gerar endereços por transação, de modo que posições e
movimentos não sejam trivialmente atribuíveis a uma instituição. Registrada em
2026-07-28 como a alternativa preferida à Opção B.

**Prós**
- Não toca a matemática do AMM nem o caminho crítico de swap — ao contrário das Opções
  A e B, é compatível com o design da Liquidity Pool.
- Incremental: pode ser adotada por fluxo, sem reescrever contratos.

**Contras**
- **Não protege valores.** Reservas, paridades e volumes continuam em claro; o que se
  ganha é dificuldade de atribuição, não confidencialidade. Se a sensibilidade
  soberana estiver no *valor* das posições, esta opção não a endereça.
- Vulnerável a análise de fluxo: a rotação de endereços é desfeita por correlação de
  valores e temporalidade, sobretudo com poucos participantes.
- Exige gestão de chaves/endereços por transação e reconciliação contábil do lado da
  instituição.

---

## Recomendação

**Adotar a Opção C — descope formal com controles compensatórios nesta fase**,
com um caminho de reavaliação para a Opção B quando o modelo de privacidade
seletiva amadurecer.

Justificativa: a rede do hub é permissionada (QBFT, membros conhecidos), o que
reduz o risco relativo de operar em claro frente à complexidade e ao risco de
regressão de cifrar o AMM (Opção A). A privacidade forte permanece onde mais importa
— entre bancos dentro dos spokes. **Ressalva factual** (revista em 2026-08-22): o
conjunto de **nós** do hub já é só o validador, mas o de **leitores** não — todo
backend de entidade, bancos comerciais inclusive, recebe a RPC do hub, que ainda por
cima é publicada no host. O conjunto de observadores, portanto, não é exclusivamente
de autoridades; **restringir o alcance da RPC do hub é ação requerida** (ver plano) e
condição para a validade do controle compensatório. A
decisão precisa de **sign-off IDB/LNet** por envolver visibilidade de liquidez
soberana, e deve ser registrada como limitação conhecida com gatilho de reabertura.

Trade-off central da liquidez soberana: cifrar o AMM (Opção A) protege posições
soberanas, mas o custo/risco no caminho crítico de swap é desproporcional dado que
os observadores são um conjunto pequeno e confiável de autoridades. Se o conjunto
de validadores do hub deixar de ser exclusivamente de autoridades, esta decisão
deve ser reaberta imediatamente.

---

## Relação com a constituição, com ADR-001 e alcance hub↔spoke

- **Desvio da constituição (Princípio II).** O Princípio II proíbe valores em texto
  claro on-chain e admite apenas Zeto/Noto em fluxos de produção. Um ADR **não pode
  se auto-autorizar** esse desvio: esta decisão exige uma **emenda de escopo** à
  constituição (ou uma entrada de Complexity Tracking no plano correspondente),
  enquadrada explicitamente como limitação de **fase-piloto** e com gatilho de
  reabertura definido. O descope só é válido após essa emenda registrada.
- **Dependência de ADR-001.** O caminho de auditoria depende de
  [ADR-001](ADR-001-privacidade-vs-auditabilidade.md) (encryption-to-authority +
  disclosure/decrypt). Enquanto o hub opera em claro, a liquidez soberana não tem o
  mesmo mecanismo criptográfico; a reavaliação (Opção B) deve reutilizar a chave de
  autoridade definida em ADR-001.
- **Alcance hub↔spoke.** ADR-001 posiciona o decrypt no `OversightService` do hub,
  enquanto este ADR afirma que Paladin é exclusivo do spoke. É preciso declarar
  **como o hub alcança o Paladin do spoke** para o disclosure (operação spoke-level
  acionada a partir do hub) — caso contrário as duas decisões se contradizem.

---

## Plano de implementação

1. **Emenda de escopo à constituição** — registrar formalmente o desvio ao
   Princípio II (valores em claro no hub) como limitação de fase-piloto, via emenda
   de escopo ou entrada de Complexity Tracking, com gatilho de reabertura.
   **Bloqueante** para os demais passos.
2. **Documento de descope** — redigir a limitação formal ("hub opera em claro"), os
   controles compensatórios e o gatilho de reabertura; obter sign-off IDB/LNet.
3. **Formalizar o modelo de ameaça do hub** — enumerar explicitamente quem observa o
   hub: o nó (validador único) **e os clientes JSON-RPC**, que hoje incluem o backend
   de toda entidade, mais quem alcançar a porta publicada no host. Registrar que dados
   cada um observa, sustentando (ou refutando) a premissa de confiança do descope. A
   distinção importa: o modelo de ameaça de um peer e o de um leitor por RPC coincidem
   em leitura, e é a leitura que está em questão aqui.
4. **Restringir quem lê o hub a autoridades** — *reformulado em 2026-08-22*. A parte
   de **membership de nós** já está satisfeita por construção: o hub provisionado pelo
   toolkit tem um único serviço (`hub-validator`), sem peers de bancos comerciais. O
   que resta é o **acesso de leitura por RPC**: todo backend de entidade recebe
   `HUB_BESU_RPC_URL` e a porta RPC do hub é publicada no host. Fechar isto significa
   restringir o alcance de rede da RPC do hub (por exemplo, expor a RPC apenas às
   redes das autoridades e dar aos bancos apenas os endpoints de que dependem) e
   confirmar a cobertura de auditoria dos movimentos de liquidez soberana. **Sem isto,
   o controle compensatório (observadores = autoridades) não se sustenta** — a
   retirada dos peers, por si só, não o sustenta.
5. **Registrar como limitação conhecida** — atualizar
   `scenario-b/docs/runbooks/deployment-runbook.md` (a afirmação "Paladin/Zeto privacy
   is available on spokes but not on the hub" está hoje na **linha 32**, não na 9: o
   banner da retirada do caminho legado deslocou o texto) e o README do Scenario B.
6. **Definir o alcance hub↔spoke para disclosure** — especificar como o decrypt do
   `OversightService` (ADR-001) aciona a operação spoke-level do Paladin, resolvendo
   a aparente contradição com a fronteira "Paladin só no spoke".
7. **Definir o caminho de reavaliação (Opção D)** — abrir uma spec de investigação
   para o modelo de endereços por transação, incluindo o que ele **não** cobre
   (valores permanecem em claro) e o limite de análise de fluxo com poucos
   participantes. A Opção B (privacidade seletiva no `LiquidityCommitRegistry`,
   reutilizando a chave de autoridade de ADR-001) permanece registrada como
   alternativa, mas deixou de ser a direção preferida em 2026-07-28.
8. **Fechar o finding 9.2** — registrar a decisão e o sign-off como resolução do
   finding.

---

## Esforço e cronograma (estimativa preliminar — a confirmar pelo time)

| Fase | Escopo | Esforço estimado |
|------|--------|------------------|
| Emenda + descope + modelo de ameaça (passos 1–3) | Documentação + sign-off | ~1 semana-dev + sign-off (externo) |
| Restringir membership + isolamento (passo 4) | Ajuste de topologia do hub + validação | ~0,5–1 semana-dev |
| Alcance hub↔spoke + docs (passos 5–6) | Especificação + runbook/README | ~1 semana-dev |
| Spec de reavaliação (passo 7) | Investigação de privacidade seletiva | ~1 semana-dev (quando priorizada) |

Estimativa total: **~3–4 semanas-dev** (excluindo a Opção B, futura), bloqueadas
pela emenda à constituição e pelo sign-off IDB/LNet.

---

## Status / Sign-off

| Parte | Papel | Decisão | Data |
|-------|-------|---------|------|
| Time de arquitetura CBWeb3 (AH/GL) | Autor | Proposto | 2026-07-23 |
| Time de arquitetura CBWeb3 (AH/GL) | Mecanismo: Paladin no hub descartado (incompatível com o design da Liquidity Pool); direção alternativa = Opção D | **Decidido** | 2026-07-28 |
| IDB | Aprovação do descope de privacidade do hub | Pendente | — |
| LNet | Aprovação do descope de privacidade do hub | Pendente | — |
| Constituição (Princípio II) | Emenda de escopo registrando o desvio de fase-piloto | Pendente | — |

O Status permanece **Proposto** até que as linhas de sign-off acima estejam
preenchidas e a emenda à constituição esteja registrada. A decisão de mecanismo
(2026-07-28) **não** substitui o sign-off: ela fecha a escolha técnica, o sign-off
autoriza a exposição de liquidez soberana que dela resulta.
