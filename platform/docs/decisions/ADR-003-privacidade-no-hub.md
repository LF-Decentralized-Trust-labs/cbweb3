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
  reservas e posições de liquidez ficam visíveis para todos os validadores do hub.

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
IDB/LNet, apoiado em controles compensatórios: rede permissionada (QBFT, validadores
conhecidos e limitados a bancos centrais/autoridade), isolamento de rede e trilha de
auditoria. Tratar privacidade no hub como trabalho futuro explicitamente fora do
escopo atual.

**Prós**
- Reconhece a realidade da implementação e do escopo do D6 v2 sem introduzir risco
  técnico alto no caminho crítico.
- Os validadores do hub já são um conjunto restrito e confiável (banco central /
  autoridade), o que limita a superfície de exposição — a privacidade
  intra-validadores tem valor marginal menor que nos spokes multi-banco.
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

Justificativa: o conjunto de validadores do hub é restrito e permissionado (QBFT,
bancos centrais/autoridade), o que reduz materialmente o risco relativo de operar
em claro no hub frente à complexidade e ao risco de regressão de cifrar o AMM
(Opção A). A privacidade forte permanece onde mais importa — entre bancos dentro
dos spokes. A decisão precisa de **sign-off IDB/LNet** por envolver visibilidade de
liquidez soberana, e deve ser registrada como limitação conhecida com gatilho de
reabertura.

Trade-off central da liquidez soberana: cifrar o AMM (Opção A) protege posições
soberanas, mas o custo/risco no caminho crítico de swap é desproporcional dado que
os observadores são um conjunto pequeno e confiável de autoridades. Se o conjunto
de validadores do hub deixar de ser exclusivamente de autoridades, esta decisão
deve ser reaberta imediatamente.

---

## Plano de implementação

1. **Documento de descope** — redigir a limitação formal ("hub opera em claro"),
   os controles compensatórios (QBFT permissionado, validadores restritos a
   autoridades, isolamento de rede, auditoria) e o gatilho de reabertura; obter
   sign-off IDB/LNet.
2. **Formalizar o modelo de ameaça do hub** — enumerar explicitamente quem são os
   validadores do hub e que dados eles observam, sustentando a premissa de
   confiança do descope.
3. **Reforçar controles compensatórios** — confirmar que a lista de validadores do
   hub está restrita a autoridades, que a rede está isolada e que a trilha de
   auditoria cobre os movimentos de liquidez soberana.
4. **Registrar como limitação conhecida** — atualizar
   `scenario-b/docs/runbooks/deployment-runbook.md` (seção "Known limitations", já
   referenciada na linha 9) e o README do Scenario B para refletir a decisão e seu
   status.
5. **Definir o caminho de reavaliação (Opção B)** — abrir uma spec de investigação
   para privacidade seletiva no `LiquidityCommitRegistry` (commits não-atribuíveis
   com agregados públicos), a ser priorizada se/quando o conjunto de validadores ou
   a sensibilidade exigir.
6. **Fechar o finding 9.2** — registrar a decisão e o sign-off como resolução do
   finding.
