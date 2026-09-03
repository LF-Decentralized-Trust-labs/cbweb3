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

Os valores abaixo são do **cenário B**, o único com hub. A coluna de origem cita onde o
parâmetro é definido *hoje*: os arquivos `deploy/local/*/config/configTemplate.json` que
registravam esses IDs na época do D12 foram removidos junto com o bring-up legado, e o
manifest do toolkit passou a ser a única definição.

| Parâmetro | D6 v2 | D12 | Onde está definido hoje |
|---|---|---|---|
| Chain ID do hub | `80000` | **`1337`** | `scenario-b/samples/hub/hub-cbweb3.yaml` |
| Chain ID da spoke Brasil | `80001` | **`1338`** | `scenario-b/samples/brazil/central-bank-brazil.yaml` |
| Chain ID da spoke Argentina | `80002` | **`1339`** | `scenario-b/samples/argentina/central-bank-argentina.yaml` |

A alocação segue o padrão hoje nos manifests dos samples — uma chain por spoke, IDs
distintos e contíguos: Brasil `1338`, Argentina `1339`, Colômbia `1340`
(`scenario-b/samples/*/central-bank-*.yaml`, campo `spec.node.chainId`).

**A alocação não é comum aos dois cenários, e isso não está registrado.** O cenário A não
tem hub, e usa a mesma faixa deslocada: Brasil `1337`, Colômbia `1338`, Argentina `1339`
(`scenario-a/samples/*/central-bank-*.yaml`). O mesmo número designa coisas diferentes —
`1337` é o hub no cenário B e a spoke do Brasil no cenário A; `1338` é o Brasil no B e a
Colômbia no A. Pela regra de isolamento de cenários isto não é defeito: são produtos
separados, que não se conectam. Mas é exatamente o tipo de assimetria que
[`../scenario-drift.md`](../scenario-drift.md) existe para classificar como deliberada ou
acidental, e hoje ela não aparece lá — chain ID não é mencionado no arquivo. Registrar a
decisão (faixas separadas de propósito, ou convergir para uma alocação única) fica como
item aberto deste ADR, porque a resposta muda o que um participante precisa presumir se
os cenários vierem a coexistir num mesmo operador.

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

**Fechado — skew de versão do Besu.** A nota de julho sinalizou que os scripts de
bring-up baixavam a distribuição `besu-25.8.0` para gerar o genesis mas subiam os
containers com `hyperledger/besu:latest`, de modo que o genesis era gerado por uma versão
e servido por outra, que avançava sozinha. Quando este ADR foi escrito eram 15 ocorrências
de `:latest`, todas nos scripts `startBesu.sh` dos dois cenários.

Esses scripts eram do caminho `deploy/local`, removido em favor do toolkit como única
forma de subir ambiente. Com a remoção, `startBesu.sh` deixou de existir e nenhum caminho
executável puxa `:latest`: hoje 44 arquivos fixam `hyperledger/besu:25.8.0` e as duas
únicas ocorrências restantes de `:latest` são prosa em
`scenario-b/docs/charts/scenario-b/architecture.md` e na própria nota de migração que este
ADR promove — texto desatualizado, não configuração.

Com isso, a afirmação de [`../TOOLCHAIN.md`](../TOOLCHAIN.md) de que Besu 25.8.0 é
garantido pelo *"`BESU_IMAGE` default in the compose templates"* passou a ser verdadeira
sem ressalva: os templates do toolkit são a única origem da imagem. Continua candidato a
card próprio generalizar para Besu o gate que já existe para Alpine
(`tools/check-alpine-version.sh`, que lê o pino do próprio `TOOLCHAIN.md`) — agora mais
barato, porque há uma origem só a conferir, e é o que impediria a regressão de voltar pela
prosa ou por um template novo.

## Status / Sign-off

| Parte | Decisão | Data |
|---|---|---|
| Time de plataforma | Implementado (valores em vigor no D12) | 2026-07-23 |
| LNET | **Pendente** — itens em aberto listados na nota de migração | — |
| IDB / BID | Não requer sign-off (parâmetro de rede interno) | — |
