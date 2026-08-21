<!-- SPDX-License-Identifier: Apache-2.0 -->

# ADR-007 — Rebase dos chain IDs (80000/80001/80002 → 1337/1338/1339)

| | |
|---|---|
| **Finding** | R1-11.4 (decisão nomeada como "chain-ID change", sem ADR) |
| **Status** | **Aceito e implementado**; itens de coordenação com a LNET em aberto |
| **Promove** | [`../../scenario-b/docs/runbooks/besu-chainid-migration-notes.md`](../../scenario-b/docs/runbooks/besu-chainid-migration-notes.md) (nota informativa, Lucas Campelo e Samuel Venzi, 2026-07-23) |

## Nota sobre a natureza deste ADR

A decisão **já estava documentada** — a nota de migração acima registra os valores, a
racionalidade e as verificações de compatibilidade. O que ela declara sobre si mesma é
*"Status: Informational"*: descreve o delta entre os deliverables D6 v2 e D12, sem
estrutura de opções, recomendação ou sign-off.

Este ADR **promove** aquela nota à estrutura de decisão deste diretório, do mesmo modo
que o índice já prevê para o ADR-005. A nota permanece como referência técnica detalhada;
este documento é o registro da decisão e do que ainda depende de acordo.

## Contexto

Entre D6 v2 e D12, três parâmetros de rede mudaram:

| Parâmetro | D6 v2 | D12 | Onde está definido |
|---|---|---|---|
| Chain ID do hub | `80000` | **`1337`** | `deploy/local/hub-besu/config/configTemplate.json` |
| Chain ID da Spoke-A | `80001` | **`1338`** | `deploy/local/spoke-besu-a/config/configTemplate.json` |
| Chain ID da Spoke-B | `80002` | **`1339`** | `deploy/local/spoke-besu-b/config/configTemplate.json` |

A alocação segue o padrão hoje nos manifests dos samples — uma chain por spoke, IDs
distintos e contíguos: Brasil `1338`, Argentina `1339`, Colômbia `1340`
(`scenario-b/samples/*/central-bank-*.yaml`, campo `spec.node.chainId`).

Isto é consequência direta de [`ADR-006`](ADR-006-isolamento-de-rede-do-hub.md): com o
hub em rede própria, a alocação de chain ID deixa de ser detalhe local de cada stack e
passa a ser parâmetro de rede — cada participante precisa saber em que chain assina.

## Por que o chain ID importa aqui

Não é rótulo. O chain ID entra na assinatura da transação (EIP-155), então é o que
impede que uma transação assinada para uma spoke seja reexecutada em outra. Com hub e N
spokes em chains distintas, IDs distintos são a proteção contra replay entre corredores,
e um ID duplicado entre duas spokes seria falha de segurança, não inconveniência.

## Opções que existiam

### Opção A — Manter a faixa 80000+
Nenhum trabalho de migração.

Custo: a faixa era arbitrária e não coincide com convenção alguma; a nota de migração
registra o alinhamento com a stack D12 como motivação para sair dela.

### Opção B — Rebase para a faixa 1337+ (escolhida)
Alinha com a convenção de redes de desenvolvimento EVM (1337 é o valor histórico de
devnet), o que reduz atrito com ferramental que assume essa faixa.

Custo: é mudança de parâmetro de rede — exige rebuild do genesis, e endereços de
contrato anteriores deixam de ser válidos. Sem estado de produção, o custo é operacional
e não de migração de dados.

### Opção C — IDs derivados de identificador soberano
Rejeitada por ora: atrelaria o ID a um esquema de numeração externo que ainda não existe.
Continua defensável quando a rede sair do piloto, e é o gatilho natural de reabertura
deste ADR.

## Decisão

**Opção B.** Hub `1337`, e uma chain por spoke a partir de `1338`, contígua e distinta.

## Consequências

**Positiva.** Alinhamento com ferramental EVM e proteção de replay explícita por spoke.

**Aberta — coordenação com a LNET.** A nota de migração lista itens aguardando acordo com
a LNET, porque a rede operada por eles precisa dos mesmos parâmetros. Enquanto isso não
fechar, o parâmetro é decidido do lado da plataforma e **presumido** do lado da operação.

**Risco vivo, verificado nesta data — skew de versão do Besu.** A nota de julho sinalizou
que os scripts de bring-up baixam a distribuição `besu-25.8.0` para gerar o genesis mas
sobem os containers com `hyperledger/besu:latest`. Conferido hoje: são **15 ocorrências
de `hyperledger/besu:latest`** nos scripts `startBesu.sh` dos dois cenários, enquanto os
templates de provisioning do toolkit fixam `hyperledger/besu:25.8.0` corretamente (8
ocorrências).

Consequência prática: o genesis é gerado por uma versão e servido por outra, que muda
sozinha quando a tag `latest` avança. Isso não é problema de chain ID, mas está no mesmo
parágrafo de risco porque é o mesmo material — parâmetro de rede que se presume fixo e
não está.

Vale notar que [`../TOOLCHAIN.md`](../TOOLCHAIN.md) afirma que Besu 25.8.0 é garantido
pelo *"`BESU_IMAGE` default in the compose templates"*, o que é verdade para os templates
do toolkit e **falso para os scripts legados**. Existe hoje um gate para o mesmo problema
em imagens Alpine (`tools/check-alpine-version.sh`, que lê o pino do próprio
`TOOLCHAIN.md`); generalizá-lo para Besu fecharia esta lacuna e é candidato a card
próprio.

## Status / Sign-off

| Parte | Decisão | Data |
|---|---|---|
| Time de plataforma | Implementado (valores em vigor no D12) | 2026-07-23 |
| LNET | **Pendente** — itens em aberto listados na nota de migração | — |
| IDB / BID | Não requer sign-off (parâmetro de rede interno) | — |
