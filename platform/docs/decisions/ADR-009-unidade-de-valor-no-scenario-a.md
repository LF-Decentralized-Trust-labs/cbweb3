# ADR-009: Unidade de valor no Scenario A

**Status**: **Aceito** (2026-09-09) — Opção A
**Data**: 2026-09-09
**Decisores**: Antonio Souza (liderança técnica)
**Card relacionado**: `[Scenario A] Give amounts a decimals layer — today the unit is a raw base unit`
**Origem**: desdobramento de `[Scenario A] The proposal form rejects an amount with a decimal separator` (PR #232)

---

## Pedido de decisão

Este bloco existe para que a decisão possa ser tomada sem ler o ADR inteiro. O corpo
abaixo é a fundamentação.

| | |
|---|---|
| **O que se pede** | Aprovar a **Opção A** — introduzir uma camada de decimais no Scenario A, convertendo na borda da interface, de modo que a unidade exibida e digitada passe a ser a unidade da moeda — **ou** aprovar a **Opção B**, que mantém a unidade-base crua e registra formalmente que valores fracionários não existem no Scenario A. |
| **Quem assina** | Liderança técnica. Não requer IDB/LNet: nenhuma regra de negócio muda, e nenhum valor liquidado é alterado. |
| **Recomendação a aprovar** | Opção A (ver §Recomendação). |
| **Decidido** | **Opção A, aprovada em 2026-09-09.** A implementação segue em `feat/scenario-a-amount-decimals`, com o escopo de cinco itens de §Opções — sendo o item 5 (migração dos três chamadores) o de maior risco de omissão. |
| **Se aprovado, desbloqueia** | O card acima, e com ele a paridade com o Scenario B, onde `100.20` já é transacionável. |
| **Se não for aprovado** | Precisa ficar registrado como limitação aceita, com sign-off, que **nenhum valor fracionário pode ser transacionado no Scenario A** — nem em emissão, nem em resgate, nem em tokenização, nem em perna de HTLC, nem em proposta de FX. Não como pendência técnica. |
| **Evidência** | Medida contra stack ao vivo em 2026-09-09 (`deploy-all.sh` do Scenario A, Brasil + Costa Rica). Detalhada em §Contexto. |

---

## Contexto: o que existe hoje

O Scenario A não tem camada de decimais. Não é uma omissão pontual — é a convenção
vigente, coerente de ponta a ponta:

| Fato | Como foi verificado |
|---|---|
| O token tem 18 casas | `eth_call decimals()` no fCeBM implantado devolve `18` |
| O backend não escala | `FiatClient.Mint` faz `big.Int.SetString(amount, 10)` e cunha o valor verbatim |
| A interface não divide | `formatCeBM` e `formatFiatUnits` imprimem `BigInt(raw).toLocaleString()` |
| Os formulários são inteiros | emissão, resgate, tokenização e as duas pernas de HTLC guardam com `/^\d+$/` |
| As pernas casam 1:1 | `samples/sample-tryout.sh` trava um HTLC de `1000` contra um `origin_amount` de `1000` |
| Não existe fonte de decimais | a string `decimals` não aparece no gateway nem no frontend do Scenario A |

Portanto um "1.000 fCeBM" exibido são **1000 wei** de um token de 18 casas, e entre 100
e 101 não existe nada. O Scenario B é o contraste: lá `displayToBase` e uma fonte de
decimais (`/token/balance` devolve `{balance, decimals, symbol}`) já existem, e `100.20`
foi transacionado de ponta a ponta em 2026-09-09.

### O que está em jogo

`100.20` não é transacionável em nenhuma tela do Scenario A. Para um piloto cujos
participantes são bancos centrais, a unidade que o operador enxerga não ser a unidade da
moeda é um defeito de produto, ainda que internamente consistente.

---

## Correção de uma afirmação anterior

Ao abrir o card, registrei que a mudança implicaria **revalorização silenciosa de
registros liquidados**. Isso está errado e é importante corrigir, porque superestima o
risco e poderia adiar a decisão sem motivo.

O que está gravado nas colunas `amount`, `origin_amount` e `counter_amount` são
unidades-base, exatamente como estão on-chain. A Opção A **não altera nenhum valor
gravado nem nenhum valor liquidado**. Ela altera duas outras coisas:

1. **O que uma digitação nova produz** — `100.20` passa a virar `100200000000000000000`
   em vez de ser recusado.
2. **Como o histórico é renderizado** — uma linha antiga com `1000` passa a exibir
   `0.000000000000001` em vez de `1.000`.

O item 2 é uma descontinuidade de leitura do histórico, não corrupção de dado. E é, a
rigor, a primeira vez que aquelas linhas serão exibidas corretamente: a exibição antiga
é que era enganosa.

Verifiquei também o risco adjacente — código que compare um valor gravado com um valor
recém-digitado, onde as duas escalas se encontrariam. **Não existe**: não há validação de
escrow contra o valor do depósito, nem checagem de saldo, no orquestrador do Scenario A.

---

## As duas perguntas que esta decisão responde

### 1. Qual é a unidade de cada campo

Há dois tipos de campo, hoje tratados como um só. **A primeira versão deste ADR errou aqui**:
dizia que o campo denominado em token seria "limitado pelas casas do token", o que não
significa nada para o tCeBM — no Scenario A ele **não é um ERC-20**, é uma nota Zeto, e
uma nota guarda um inteiro sem conceito de casas decimais. O erro veio de generalizar a
partir do Scenario B, onde o tCeBM é ERC-20.

A tabela corrigida:

| Tipo | Campos | Token | Limite de casas |
|---|---|---|---|
| **Denominado em moeda** | emissão e resgate de fCeBM | ERC-20 `FiatCentralBankMoney`, 18 casas | o expoente ISO 4217 da moeda |
| **Denominado em token** | tokenização, resgate de tCeBM, pernas de HTLC, pernas de FX | **nota Zeto**, inteiro sem casas | a escala que este ADR define — não há `decimals()` para consultar |

Como a nota Zeto é apenas um inteiro, a escala é uma convenção nossa. Adotamos
**1 unidade de nota = 10⁻¹⁸ tCeBM**, a mesma escala do fCeBM, para que a tokenização
continue sendo 1:1 e a perna de HTLC continue casando com a perna de FX.

A distinção não é acadêmica. Um campo denominado em token carrega resíduo de AMM por
construção — no Scenario B, um swap deixou saldo de `3.959753632757569189`. Limitar a
entrada desse campo às duas casas da moeda tornaria os últimos `0.009753632757569189`
irresgatáveis para sempre, e cada swap acrescentaria mais.

### 1b. A escala de 10⁻¹⁸ cabe na nota Zeto? Medido: cabe, com folga enorme

A escala acima só é viável se o circuito ZK aceitar valores grandes. Circuitos Zeto
aplicam *range check* ao valor da nota, e nada no repositório documentava o limite — nem
aqui, nem no ADR-001. Medido em 2026-09-09 contra o stack ao vivo, cunhando notas direto
pelo Paladin do CB do Brasil (`ptx_sendTransaction`, domínio `zeto`, função `mint`):

| Valor | Resultado |
|---|---|
| 2³², 2⁶³, 2⁶⁴, 2⁷⁰, 2⁸⁰ | aceitos |
| **2¹⁰⁰ − 1** = 1267650600228229401496703205375 | **aceito** |
| **2¹⁰⁰** | **recusado** — `PD210015: Failed to validate function parameters` |
| 2¹²⁸ | recusado |
| `100200000000000000000` (`100,20 × 10¹⁸`) | **aceito** |

O teto por nota é exatamente **2¹⁰⁰ − 1**, um range check de 100 bits. À escala de 10⁻¹⁸
isso são **~1,27 trilhão de tCeBM numa única nota**, e o valor de que a Opção A precisa
(`100,20`) fica 12,6 bilhões de vezes abaixo do teto. E o limite é *por nota*, não por
saldo — um saldo maior se distribui em várias notas.

**Conclusão: a escala de 10⁻¹⁸ é viável e a Opção A sai como está.** Se a medição tivesse
dado 64 bits, a escala teria de cair para centavos (10⁻²) e o Scenario A passaria a
divergir deliberadamente do B, o que exigiria registro em `docs/scenario-drift.md`.

Armadilha registrada para quem repetir a medição: o Paladin do CB do Brasil é a porta
**31648**; a 31748 é a da Costa Rica. Chamar a errada devolve
`PD012230: The from identity must be a valid identity local to the node`, que parece erro
de identidade e é erro de nó.

### 2. Quantas casas cada moeda admite

Pela ISO 4217, e **não é um "2" fixo**. A lista de moedas que o formulário de FX já
oferece inclui duas de expoente zero:

| Moeda | Expoente | Observação |
|---|---|---|
| BRL, ARS, COP, CRC, BOB, DOP, GTQ, HNL, MXN, NIO, PAB, PEN, USD, UYU, VES, CUP | 2 | o caso comum |
| **CLP, PYG** | **0** | não têm subunidade; `1000,50 CLP` não é uma quantia |

Uma regra fixa de duas casas aceitaria `1000,50 CLP` parecendo conforme a norma, o que é
pior do que não aplicar regra nenhuma. A implementação precisa de uma tabela
moeda → expoente.

---

## Opções

### Opção A — camada de decimais, conversão na borda

A unidade exibida e digitada passa a ser a unidade da moeda (ou do token, conforme o
campo). A conversão acontece na fronteira da interface; nada muda no contrato, no
orquestrador ou nos valores gravados.

Escopo, tudo no mesmo PR:

1. Fonte de decimais no Scenario A — o gateway passa a expor `decimals` no balance, como
   o Scenario B já faz.
2. Conversão na entrada — emissão, resgate, tokenização, duas pernas de HTLC, duas pernas
   de FX.
3. Divisão na saída — `formatCeBM`, `formatFiatUnits` e todo saldo ou tabela que os use.
   O Scenario B já tem a forma de referência (commit `95205c0b`), com as duas regras que
   ela carrega: **trunca em vez de arredondar**, e **nunca exibe uma posse não-nula como
   zero** (mostra `< 0.01`).
4. Prefill nunca truncado — qualquer valor devolvido a um campo de valor mantém precisão
   integral. O Scenario B tinha três desses e resolveu com `baseToExactDisplay`.
5. Migração dos chamadores, no mesmo PR — `samples/sample-tryout.sh`,
   `tryouts/tryout-fx-agreement-e2e.sh` e `tests/performance/k6/fx-settlement-throughput.js`
   enviam hoje inteiros crus casados 1:1. Se a interface passar a enviar `100.20 × 10^18`
   e eles não, deixam de exercitar o caminho real.

**Custo**: o item 5 é o que dá trabalho, e é inegociável — sem ele o E2E vira decorativo.

### Opção B — manter a unidade-base crua

Nada muda. Registra-se que o Scenario A opera em unidades-base inteiras e que valores
fracionários não são representáveis.

**Custo**: divergência permanente com o Scenario B, e um piloto em que o banco central
não consegue emitir R$ 100,20.

### Opção C — reduzir as casas do token (descartada)

Redeployar o fCeBM com `decimals() = 2`. Descartada: exige redeploy de contrato e
invalida todo estado on-chain existente, para resolver na camada mais cara um problema
que é de apresentação e de entrada.

---

## Recomendação

**Opção A.**

O argumento decisivo não é paridade com o Scenario B — é que a Opção B pede sign-off
formal numa limitação difícil de sustentar diante de um banco central. E o risco que
parecia justificar a hesitação (migração de dados) não existe, conforme §Correção.

O que resta de risco real é o item 5 do escopo: se os três chamadores não forem migrados
junto, o E2E passa a testar um caminho que ninguém usa. Isso é gerenciável dentro de um
PR e verificável por execução ao vivo.

---

## Consequências se aprovado

- O histórico passa a renderizar valores antigos como frações minúsculas. É correto, mas
  é visível — vale uma nota na release e possivelmente um aviso na tela de histórico.
- O card de auditoria de guardas ganha um caso resolvido: a assimetria A/B mais antiga
  documentada até agora deixa de existir.
- O limite ISO 4217 passa a valer na entrada dos campos denominados em moeda, o que
  recusa `1000.123` em BRL — hoje aceito.
- A validação do orquestrador (`^\d+$` em unidades-base, PR #232) permanece **inalterada**:
  ela descreve o que trafega no fio, e o fio continua em unidades-base.
