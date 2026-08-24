<!-- SPDX-License-Identifier: Apache-2.0 -->

# ADR-006 — Isolamento de rede do hub do Scenario B

| | |
|---|---|
| **Finding** | R1-11.4 (decisão nomeada como "hub co-location", sem ADR) |
| **Status** | **Aceito e implementado** — registro retrospectivo |
| **Substitui** | Arranjo hub-on-Spoke-A, retirado |

## Nota sobre a natureza deste ADR

Este é um **registro retrospectivo**: a decisão já foi tomada e implementada. Não
propõe caminho, documenta o que foi escolhido, contra o que, e o que isso implica. Está
sendo escrito porque o R1-11.4 apontou que decisões estruturais da plataforma não tinham
ADR — e uma decisão implementada sem registro é indistinguível de acidente para quem
chega depois.

Por isso a estrutura difere um pouco da dos ADR-001 a 004, que são propostas aguardando
sign-off: aqui não há "recomendação a aprovar", há decisão em vigor e consequências a
conhecer.

## Contexto

Antes desta mudança, os contratos do hub do Scenario B rodavam **no nó Besu da
Spoke-A**, na chain 1338, porta 8645. O hub não tinha rede própria: era um conjunto de
contratos convivendo com a chain de uma spoke.

Evidência do estado anterior e do corte:
`scenario-b/docs/runbooks/deployment-runbook.md`, seção *"Decommissioning the Legacy
Hub-on-Spoke-A Deployment"*, que declara o arranjo anterior **retirado e
não-autoritativo** e instrui a não usar endereços da chain 1338 como referência de hub.

O problema desse arranjo não é performance — é **soberania e limite de confiança**. O
hub é o ponto neutro onde spokes soberanas se encontram; hospedá-lo dentro da chain de
uma delas dá àquela spoke posição estruturalmente diferente das outras: o validador da
Spoke-A valida os blocos que contêm os contratos do hub. Num consórcio de bancos
centrais, isso é assimetria de governança disfarçada de detalhe de deploy.

## Opções que existiam

### Opção A — Manter o hub na chain da Spoke-A
Menos infraestrutura: uma chain a menos para operar, um conjunto de nós a menos.

Custo: a assimetria acima, permanente. E uma consequência operacional concreta —
derrubar ou reprovisionar a Spoke-A derruba o hub de todos.

### Opção B — Hub em rede própria (escolhida)
O hub passa a ter sua própria rede Besu e sua própria chain, com validador próprio. Cada
spoke tem a sua. O encontro entre elas passa a ser explicitamente cross-chain, via
relay.

Custo: mais uma rede para provisionar e operar; o toolkit ganha um modo `found-hub`
distinto de `found-spoke`; endereços de contrato do hub passam a viver em um `broadcast`
separado (`contracts/broadcast/CBWeb3Hub.s.sol/1337/run-latest.json`).

### Opção C — Hub como contrato replicado em cada spoke
Rejeitada sem implementação: replicar estado de corredor em N chains reintroduz o
problema de consistência que o hub existe para resolver.

## Decisão

**Opção B.** O hub tem rede e chain próprias. O arranjo hub-on-Spoke-A está retirado.

O cutover foi **rebuild limpo, sem migração de estado**, decisão registrada no runbook:
o protótipo não carrega dado de produção, então migrar estado seria custo sem benefício.

## Consequências

**Positivas.** Nenhuma spoke tem posição privilegiada. Derrubar uma spoke não afeta o
hub nem as outras. O limite de confiança fica explícito: o que atravessa é mensagem de
relay, não leitura de estado compartilhado.

**Negativas, e reais.** Passa a existir mais uma rede no caminho crítico — se o RPC do
hub trava, a liquidação para. Isso já se manifestou: um RPC de hub travado estaciona uma
liquidação com `attempt_count=0` e sem escalonamento, porque só falha rápida escalona.
Esse é o preço do isolamento e precisa de tratamento próprio no relay, não de reversão
desta decisão.

**Sobre endereços.** Endereços da chain 1338 como "hub" são históricos e
não-autoritativos. Os válidos vêm do broadcast da chain do hub e são propagados por
`contracts.sync-addresses`.

## Relação com outras decisões

Consequência direta em [`ADR-007`](ADR-007-rebase-de-chain-ids.md): com hub e spokes em
chains distintas, a alocação de chain IDs deixa de ser detalhe local e passa a ser
parâmetro de rede acordado com a LNET.

Interage com [`ADR-003`](ADR-003-privacidade-no-hub.md): o hub em rede própria muda
*onde* o dado do corredor fica visível, o que é justamente o objeto daquele ADR.

## Status / Sign-off

| Parte | Decisão | Data |
|---|---|---|
| Time de plataforma | Implementado (arranjo anterior retirado no runbook) | — |
| LNET | A confirmar: a topologia de rede do hub em ambiente operado pela LNET | — |
| IDB / BID | Não requer sign-off (decisão de topologia interna) | — |
