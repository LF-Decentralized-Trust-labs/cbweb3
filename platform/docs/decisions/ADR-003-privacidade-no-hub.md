# ADR-003: Privacidade no hub do Scenario B

**Status**: Proposto
**Data**: 2026-07-23
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — decisão exige sign-off IDB/LNet
**Findings relacionados**: 9.2 (feedback LNet, Rodada 2)

---

## Contexto

O Scenario B (International Hub) tem topologia hub-and-spoke. Hoje as garantias de
privacidade não são uniformes entre as camadas:

- **O hub (chain 1337) opera em texto claro.** Os contratos de liquidez e câmbio
  soberano — `AutomatedMarketMaker` (AMM), `FXAgreement` e
  `LiquidityCommitRegistry` — executam no hub sem camada de privacidade. Valores,
  reservas e posições de liquidez ficam visíveis para **todos os nós do hub, não
  apenas os validadores**. Hoje o hub tem um **único validador** (`hub-validator`,
  validador único QBFT) e peers não-validadores que incluem **bancos comerciais**
  (`bank-a`, `bank-b`), além dos bancos centrais (`central-bank-a`,
  `central-bank-b`) — ver `scenario-b/deploy/local/hub-besu/startBesu.sh:5-9`.
  Portanto o conjunto de observadores **não** está restrito a autoridades.

- **Paladin/Zeto existe apenas nos spokes.** O runbook de deployment declara
  explicitamente essa fronteira:
  - `scenario-b/docs/runbooks/deployment-runbook.md:9`: "Paladin/Zeto privacy is
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
**restringir a membership do hub a autoridades** (removendo os peers de bancos
comerciais hoje presentes). Tratar privacidade no hub como trabalho futuro
explicitamente fora do escopo atual.

**Prós**
- Reconhece a realidade da implementação e do escopo do D6 v2 sem introduzir risco
  técnico alto no caminho crítico.
- A rede é permissionada (QBFT, membros conhecidos), o que limita a superfície de
  exposição a um conjunto fechado e auditável de nós — **embora esse conjunto não
  seja hoje exclusivamente de autoridades** (ver contexto: há peers de bancos
  comerciais no hub). Fechar a membership do hub a autoridades é **pré-requisito**
  desta opção, não uma premissa já satisfeita.
- Desbloqueia a entrega mantendo a decisão documentada e revisável.

**Contras**
- Posições de liquidez soberana permanecem visíveis ao conjunto de validadores do
  hub; depende da confiança nesse conjunto.
- Exige sign-off explícito e reabertura futura se o conjunto de validadores crescer
  ou a sensibilidade aumentar.

---

## Recomendação

**Adotar a Opção C — descope formal com controles compensatórios nesta fase**,
com um caminho de reavaliação para a Opção B quando o modelo de privacidade
seletiva amadurecer.

Justificativa: a rede do hub é permissionada (QBFT, membros conhecidos), o que
reduz o risco relativo de operar em claro frente à complexidade e ao risco de
regressão de cifrar o AMM (Opção A). A privacidade forte permanece onde mais importa
— entre bancos dentro dos spokes. **Ressalva factual**: hoje o hub inclui peers de
bancos comerciais (`bank-a`, `bank-b`), de modo que o conjunto de observadores não é
exclusivamente de autoridades; **restringir a membership do hub a autoridades é ação
requerida** (ver plano) e condição para a validade do controle compensatório. A
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
3. **Formalizar o modelo de ameaça do hub** — enumerar explicitamente quem são os
   nós do hub (validador único + peers, incluindo hoje bancos comerciais) e que
   dados eles observam, sustentando (ou refutando) a premissa de confiança do
   descope.
4. **Restringir a membership do hub a autoridades** — remover os peers de bancos
   comerciais (`bank-a`, `bank-b`) de
   `scenario-b/deploy/local/hub-besu/startBesu.sh`, confirmar isolamento de rede e
   cobertura de auditoria dos movimentos de liquidez soberana. **Sem isto, o
   controle compensatório (observadores = autoridades) não se sustenta.**
5. **Registrar como limitação conhecida** — atualizar
   `scenario-b/docs/runbooks/deployment-runbook.md` (seção "Known limitations", já
   referenciada na linha 9) e o README do Scenario B.
6. **Definir o alcance hub↔spoke para disclosure** — especificar como o decrypt do
   `OversightService` (ADR-001) aciona a operação spoke-level do Paladin, resolvendo
   a aparente contradição com a fronteira "Paladin só no spoke".
7. **Definir o caminho de reavaliação (Opção B)** — abrir uma spec de investigação
   para privacidade seletiva no `LiquidityCommitRegistry` (commits não-atribuíveis
   com agregados públicos), reutilizando a chave de autoridade de ADR-001.
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
| IDB | Aprovação do descope de privacidade do hub | Pendente | — |
| LNet | Aprovação do descope de privacidade do hub | Pendente | — |

O Status permanece **Proposto** até que as linhas de sign-off acima estejam
preenchidas e a emenda à constituição esteja registrada.
