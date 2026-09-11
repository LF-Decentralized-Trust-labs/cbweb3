# Divergência entre os cenários A e B — inventário verificado

**Finding**: R2-M-17 (code review §4 MEDIUM #17, unido ao §9 P2 "scenario convergence")
**Verificado em**: `develop` @ `501129ce`, 2026-08-19
**Status**: inventário completo; a decisão de linhagem canônica está **pendente** (ver
[Decisão pendente](#decisão-pendente))

---

## Por que este documento existe

A constituição do projeto trata `scenario-a/` e `scenario-b/` como produtos separados e
proíbe alcançar de um para o outro. O que ela não diz é **quais** diferenças entre eles
são deliberadas e quais são acidentais. Sem essa distinção, toda divergência parece
intencional para quem chega depois, e toda convergência parece arriscada.

O finding R2-M-17 pede uma de duas coisas: convergir, ou documentar a divergência
intencional. Este documento entrega a segunda metade e prepara a primeira, listando
cada divergência com evidência em arquivo e linha, e classificando-a.

**Método**: cada afirmação abaixo foi verificada contra a `develop`, não herdada do
texto do finding. Duas das cinco áreas que o finding nomeia **não se sustentam mais** —
estão marcadas como tal, para que ninguém gaste tempo nelas.

**Classificação usada**:

| Classe | Significado |
| --- | --- |
| **Intencional** | Diferença de produto. Mantida, e agora documentada. |
| **Convergir** | Divergência acidental. Deveria ser alinhada. |
| **Já convergido** | O finding descreve um estado que não existe mais. |
| **Decisão** | Precisa de escolha humana antes de qualquer ação. |

---

## Resumo

| Área | Classe | Uma linha |
| --- | --- | --- |
| Exports de Dialog na UI | **Já convergido** | Os dois pacotes exportam os mesmos 66 símbolos |
| Auth do NOC | **Já convergido** | O middleware difere apenas no caminho de import |
| Circuit breaker do AMM | **Convergida** (R2-H-4) | Os dois contam instituições distintas e vinculam a assinatura à época da pausa |
| Taxa do AMM | **Convergir** (parcial) | Mecanismo igual; padrão 0,3% em A e 0% em B |
| Superfície do AMM | **Intencional** | B tem liquidez cooperativa e saque; A não |
| Proteções de rota | **Convergir** | A comenta rotas; B escolhe conjunto por flag de build |
| Integração da tesouraria | **Decisão** | Duas páginas idênticas, vivas em B e desligadas em A |
| Caminhos de módulo Go | **Convergir** | Três serviços dos dois cenários declaram o mesmo módulo |
| App `dispatcher` | **Decisão** | Existe só em A |
| Faixas de porta host | **Convergida** | As duas ficavam acima de 32768 em parte dos samples; agora A vive na faixa x645 e B na x145 |
| Escala dos valores | **Intencional** | A usa centavos porque o lock do Zeto trava em 2^64; B usa 18 casas porque seus tokens são ERC-20 |
| Proposta de FX | **Convergir** | A aceita decimais; B segue em unidades inteiras, numa tela que o build de B nem roteia |

---

## 1. Exports de Dialog na UI — já convergido

O finding cita "UI Dialog exports" como divergência. Não é mais.

Os dois pacotes exportam **conjuntos idênticos de 66 símbolos**, incluindo os dez
símbolos de `Dialog`. A única diferença é a **posição** do bloco dentro de
`src/index.ts`: no cenário B ele aparece perto do topo, no A perto do fim.

- `scenario-a/frontend/packages/ui/src/index.ts`
- `scenario-b/frontend/packages/ui/src/index.ts`

O conjunto de arquivos em `src/components/` também é idêntico nos dois pacotes.

**Nada a fazer.** Reordenar o arquivo por reordenar produziria um diff sem efeito.

---

## 2. Auth do NOC — já convergido

O finding cita "NOC auth". O middleware de autenticação dos dois `noc-backend` é
**funcionalmente idêntico**: o diff completo entre os dois arquivos é uma linha, e é o
caminho de import.

```
diff scenario-a/backend/services/noc-backend/internal/middleware/auth.go \
     scenario-b/backend/services/noc-backend/internal/middleware/auth.go
9c9
< 	".../cbweb3-platform/backend/services/noc-backend/internal/keycloak"
---
> 	".../cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
```

`NOC_SKIP_AUTH` aparece seis vezes em cada backend; nenhum dos dois usa cookie de
sessão ou defesa CSRF; os dois frontends usam `withCredentials` em exatamente um
arquivo.

**Nada a fazer neste finding.** Os dois estão no mesmo estado — o que não significa
que o estado seja bom: endurecer o NOC é o escopo de **R2-H-13**, que está *Blocked* e
vale para os dois cenários, não só para o A.

---

## 3. Circuit breaker do AMM — convergida (R2-H-4)

O port do R2-H-2 aconteceu nos dois cenários. Os dois contratos têm `pause`, `signResume`,
`isPaused` e `RESUME_QUORUM`, e o quórum numérico é o mesmo:

- `scenario-a/contracts/src/AutomatedMarketMaker.sol:31` — `RESUME_QUORUM = 2`
- `scenario-b/contracts/src/AutomatedMarketMaker.sol:32` — `RESUME_QUORUM = 2`

O breaker assimétrico (pausa 1-de-N, retomada 2-de-N) que a constituição exige está
implementado nos dois, e desde o R2-H-4 **o que os dois contam também é o mesmo**:

| | Cenário A | Cenário B |
| --- | --- | --- |
| Deduplicação da retomada | por **instituição** (`institutionSigned`, via `IdentityRegistry.getInstitutionId`) | por **instituição** (idem) |
| Vinculação à época da pausa | sim (`pauseEpoch`) | sim (`pauseEpoch`) |
| `institutionId` no registro de identidades | sim, obrigatório e não-zero | sim, obrigatório e não-zero |

Vale registrar por que essa convergência precisou de dois cards em vez de um. O R2-H-2 foi
aplicado ao AMM do cenário A, que **não é implantado por nenhum caminho de deploy em
funcionamento** (ver §5 e o ADR-003 sobre o hub); o do cenário B é o que executa swaps de
verdade. Ou seja: por um período o quórum corrigido era o vestigial e o vulnerável era o de
produção. O R2-H-4 fechou isso. O branch `fix/amm-resume-quorum-to-require-distinct-institutions`
(PR #80) **não será integrado**, por decisão do time (2026-08-22) — o port foi feito por outro
caminho.

### A única diferença que permanece, e é deliberada: a origem do `institutionId`

O desenho é idêntico (`institutionSigned` por proposta, `getInstitutionId` no registro,
id derivado por `keccak256`), mas **a string que entra no hash não vem do mesmo lugar**:

| | Cenário A | Cenário B |
| --- | --- | --- |
| Fonte do código de instituição | `BANK_CODE` (por instituição) | `INSTITUTION_CODE` (renderizado por entidade pelo toolkit) |
| Por que | o `bankCode` do A já é único por instituição | `BANK_CODE` no B é o **papel** da entidade |

No cenário B, `apply.go` deriva a entidade de `spec.topology.role` e o template define
`BANK_CODE: "${ENTITY}"` — então **todo banco central carrega `central-bank`**. Derivar o
`institutionId` disso faria de todos os bancos centrais **uma única instituição**; como o
quórum exige duas distintas, um AMM pausado **nunca mais poderia ser retomado**. É o mesmo
motivo pelo qual `RELAY_KEY_ID` já existia separado de `BANK_CODE`, uma camada acima.

Por isso o B ganhou `INSTITUTION_CODE`, renderizado por entidade nos três modos
(`found-spoke` → prefixo único do manifesto, `join` → id do banco, `found-hub` → `hub`), e
`registry.InstitutionCodeFromEnv` devolve também se o código encontrado é único, para que o
bootstrap de governança avise em vez de registrar um id colidente em silêncio.

**Um código por instituição, nos dois registros.** O mesmo `INSTITUTION_CODE` vale para o
registro do próprio spoke e para o registro do hub: o passo `register-cb` envia esse código
como `bank_code`, e é dele que a compliance do hub deriva o `institutionId`. O registro que o
AMM lê é o do hub, mas manter os dois iguais é o que torna "uma instituição, um id"
verificável por inspeção — pinado por
`TestRegisterCBSendsTheSameInstitutionCodeAsTheSpokeEnv`.

**Não unifique as duas fontes sem antes unificar o que é `BANK_CODE`.** Trocar o B para
`BANK_CODE` reintroduz a colisão descrita acima; trocar o A para `INSTITUTION_CODE` exige
que o toolkit do A passe a renderizá-lo.

### Migração de cadeias já provisionadas

Participantes registrados **antes** dessa mudança foram gravados pela assinatura de quatro
argumentos e têm `institutionId = 0`. O AMM recusa uma assinatura sem instituição
(`AMM__InvalidInstitutionId`), então essas carteiras de governança **não conseguem assinar
uma retomada** até serem registradas de novo com um código de instituição. Em provisionamento
novo (o caminho do toolkit) isso não aparece, porque o registro já nasce com o id.

---

## 4. Taxa do AMM — convergir (parcial)

Aqui há divergência real, e ela é fácil de ler errado.

O **mecanismo** é o mesmo nos dois: taxa em pontos-base, controlada por governança,
teto idêntico (`MAX_FEE_BPS = 1000`, ou 10%), acumulando nas reservas do pool. Nesse
nível, o comentário do cenário A está correto.

O que difere:

| | Cenário A | Cenário B |
| --- | --- | --- |
| Taxa de swap padrão | **30 bps (0,3%)** (`:37`, `:81`) | **0** (`:91`) |
| Taxa de saque | não existe | `withdrawalFeeBps`, padrão 0 (`:92`) |
| Teto | 1000 bps (`:34`) | 1000 bps (`:42`) |

O ponto de atenção é o comentário em
`scenario-a/contracts/src/AutomatedMarketMaker.sol:17-19`:

> Swaps charge a governance-configurable fee (default 0.3%) that accrues to the pool
> reserves. **This mirrors the Scenario B AMM's breaker and fee model**

Lido ao pé da letra, sugere paridade completa. Não há: o padrão do B é zero, e o modelo
do B tem uma segunda dimensão (taxa de saque) que o A não tem — porque o A não tem
caminho de remoção de liquidez nenhum (ver §5).

**Recomendação**: ajustar o comentário para dizer o que é verdade — mecanismo
espelhado, padrões diferentes — em vez de alinhar os padrões. Um pool de correspondente
bancário e um pool soberano de hub não precisam ter a mesma taxa padrão, e mudar
qualquer um dos dois é decisão econômica, não de código.

---

## 5. Superfície do AMM — intencional

Os dois contratos não têm o mesmo tamanho: 304 linhas em A, 558 em B. A diferença é de
produto, não de deriva.

Presente **só no B**: `depositForCommit`, `finalizeCommit`, `cancelCommitDeposit`
(o fluxo de liquidez cooperativa entre bancos centrais), `removeLiquidity`,
`removeLiquidityEmergency`, `setWithdrawalFeeBps`, `getAmountOut`, `getEscrow`,
`_liquidityShares`, `_update`.

Presente **só no A**: `_onlyGovernance`, `_onlyVerified` (verificações internas que no
B estão escritas de outra forma).

Isso reflete o desenho dos dois cenários: no B, dois bancos centrais soberanos abrem um
corredor e depositam cada um o seu lado da liquidez; no A não existe pool soberano
bilateral. Manter.

---

## 6. Proteções de rota — convergir

O finding cita "route protections". A proteção de **autenticação** não divergiu:
`ProtectedRoute` aparece duas vezes no roteador de **todos** os apps dos dois cenários.

O que divergiu é o mecanismo de **desligar uma tela**, e são dois mecanismos diferentes
para o mesmo objetivo:

**Cenário A — rota comentada.** A linha é removida do roteador por comentário:

- `scenario-a/frontend/apps/bank/src/routes/index.tsx` — 4 rotas comentadas
  (`liquidity`, `amm`, `compliance`, `settings`)
- `scenario-a/frontend/apps/treasury/src/routes/index.tsx` — 6 rotas comentadas
  (`funding-requests`, `issuance`, `redemption`, `reconciliation`, `audit`, `settings`)

**Cenário B — conjunto escolhido por flag de build.** O roteador tem duas listas e
escolhe uma em tempo de compilação:

```ts
children: isScenarioB ? scenarioBChildren : scenarioAChildren
```

- `scenario-b/frontend/apps/bank/src/routes/index.tsx:71`
- `scenario-b/frontend/apps/governance/src/routes/index.tsx:64`

`isScenarioB` vem de `VITE_SCENARIO` (`src/config/scenario.ts`), e o toolkit sempre
passa `VITE_SCENARIO=scenario-b`. Consequência prática: as páginas da lista do cenário A
**não entram no bundle** — o Vite as elimina. Verificado inspecionando o asset
construído dentro do container do portal implantado.

**Por que convergir**: os dois mecanismos são invisíveis de formas diferentes. Uma rota
comentada é visível no diff mas fácil de ler como "existe"; uma lista escolhida por flag
faz `grep` encontrar a rota e ainda assim ela não existir no artefato. As duas coisas
já causaram leitura errada: no card R2-M-8 uma correção foi aplicada duas vezes a telas
inalcançáveis, uma por cada mecanismo.

**Recomendação**: um mecanismo único e explícito para "tela desligada" — por exemplo uma
lista nomeada de rotas desabilitadas, com o motivo ao lado — em vez de comentário em um
cenário e flag de build no outro.

---

## 7. Integração da tesouraria — precisa de decisão

O finding cita "Treasury integration". O que existe é assimetria de telas ativas:

| Rota | A | B |
| --- | --- | --- |
| `/` (Dashboard) | ativa | ativa |
| `deposits-approval` | ativa | ativa |
| `escrows-approval` | ativa | ativa |
| `redeems-approval` | ativa | ativa |
| `htlc-monitor` | **ativa** | página não existe |
| `transfer-limits` | **ativa** | página não existe |
| `liquidity`, `liquidity-provisioning` | página não existe | **ativas** |
| `audit` | **desligada** (existe) | **ativa** |
| `settings` | **desligada** (existe) | **ativa** |

As quatro primeiras linhas são o núcleo comum. `htlc-monitor` e `transfer-limits` só
fazem sentido no A (HTLC é o mecanismo de atomicidade do cenário A); `liquidity` e
`liquidity-provisioning` só fazem sentido no B. Essas são **intencionais**.

As duas últimas linhas são o problema: `AuditPage` e `SettingsPage` **existem como
arquivo nos dois cenários**, e estão vivas apenas no B. Não é diferença de produto — é a
mesma tela em dois estados.

**Decisão necessária**: ligar as duas no A, ou registrar por escrito por que a tesouraria
do A não tem auditoria nem configurações. Não é escolha que eu possa fazer sozinho: pode
haver razão de escopo de entregável por trás.

---

## 8. Caminhos de módulo Go — convergir

Achado que não está no finding e importa mais que vários que estão.

| Serviço | Módulo em A | Módulo em B |
| --- | --- | --- |
| `api-gateway` | `.../cbweb3-platform/backend/services/api-gateway` | **idêntico** |
| `auth` | `.../cbweb3-platform/backend/services/auth` | **idêntico** |
| `compliance` | `.../cbweb3-platform/backend/services/compliance` | **idêntico** |
| `noc-backend` | `.../cbweb3-platform/backend/services/noc-backend` | `.../cbweb3-platform/**scenario-b**/backend/services/noc-backend` |

Três serviços de cenários diferentes **declaram o mesmo caminho de módulo**, com código
diferente. Funciona hoje porque cada um é compilado isoladamente e nada importa através
da fronteira. Mas:

1. torna impossível importar os dois no mesmo binário, mesmo via biblioteca compartilhada
   versionada — que é a única forma de compartilhamento que a constituição admite;
2. é internamente inconsistente: no B, só o `noc-backend` carrega o prefixo `scenario-b/`;
3. atrapalha qualquer unificação futura dos dois cenários.

**Recomendação**: prefixar por cenário em todos os módulos, como o `noc-backend` do B já
faz. É mudança mecânica (caminhos de import), sem efeito em runtime, e melhor feita numa
PR isolada por ser ampla e ruidosa.

---

## 9. App `dispatcher` — precisa de decisão

`scenario-a/frontend/apps/dispatcher` existe; não há equivalente no B (procurado por
nome em toda a árvore do cenário B). É o SPA que roteia o usuário por instituição e
papel.

**Decisão necessária**: portar para o B, ou registrar que o B é alcançado de outra forma.

---

## 10. Faixas de porta host — convergida

**Era acidental, e mordia.** Cada porta host que os samples do cenário B fixavam caía
dentro do intervalo efêmero do kernel (`net.ipv4.ip_local_port_range`, cujo padrão é
`32768-60999` há muitos anos), e no cenário A o portal NOC de dois dos quatro bancos
centrais também. Uma porta host publicada nesse intervalo disputa cada conexão de saída
da máquina: o kernel pode já ter entregado aquele número a outro socket quando o Docker
faz o `bind`, e o `bind` falha com `address already in use` contra uma porta que nada
aparenta ocupar. Um `deploy-all.sh --clean` é o pior caso — sobe dezenas de containers,
cada um abrindo conexões próprias — então o deploy é ele mesmo o maior consumidor de
portas efêmeras da máquina, e cada container que sobe é uma nova chance de perder a
corrida. A falha cai num passo aleatório de uma entidade aleatória e se lê como defeito
de produto até alguém olhar o número da porta.

### O que foi medido

| Onde | Exposto antes? |
| --- | --- |
| `scenario-b/samples` | **Sim** — as 33 portas declaradas e ~9 derivadas por entidade, teto em `47947` |
| `deploy-lnet/scenario-b` | Não — portas host até `~22845` |
| `scenario-a/samples` | **Parcialmente** — portal NOC de Argentina (`32845`) e Costa Rica (`32945`) |
| `deploy-lnet/scenario-a` | Não — toda VM usa `rpc: 8645`, logo o portal NOC fica em `32645` |

Os dois ambientes implantados estavam limpos: a exposição era só de desenvolvimento
local. Isso responde por medição a pergunta que o card levantou sobre produção.

**A contagem de 33 subestimava o problema.** Os manifestos declaram três portas por
entidade e o toolkit **deriva** o resto da pilha por offset sobre a porta RPC —
`+5000` Postgres, `+6000` Redis, `+7000` Keycloak, `+8000` api-gateway, `+9000` portal
primário, `+11000` noc-backend, `+12000` portal NOC, `+13000` treasury, `+14000`
supervisor. Com a base em `33645`-`33947`, o teto real era `47947`, e o número de portas
expostas passava de cem, não 33. É por isso que o guard confere as portas **derivadas**,
não só as declaradas.

### As duas faixas — e o teto, que é a regra geral

**Distinga as duas coisas**, porque só uma delas é universal:

- **O teto é da plataforma.** Vale para qualquer manifesto de qualquer árvore, porque
  decorre dos offsets de cada toolkit: `8767` no A (o portal NOC é base `+24000`) e
  `18767` no B (o portal supervisor é base `+14000`). Uma base acima do teto produz
  derivadas dentro da faixa efêmera mesmo quando a base parece segura.
- **A faixa é da topologia de host único dos samples.** Ali os dois cenários rodam lado a
  lado e o launcher os pareia por entidade (as duas árvores compartilham `launcherPort` de
  propósito, `5190`-`5199`). Como os dois derivam por offsets múltiplos de 1000, bases
  separadas por um múltiplo de 1000 colidem por construção; a separação é feita pelos três
  últimos dígitos.

| | Faixa da base (samples) | Teto (plataforma) | Motivo do teto |
| --- | --- | --- | --- |
| Scenario A | `x645`-`x767` | `8767` | portal NOC = base `+24000` |
| Scenario B | `x145`-`x557` | `18767` | portal supervisor = base `+14000` |

Como toda porta derivada preserva os três últimos dígitos da base, nenhuma porta de A
(terminada em `645`-`767`) pode igualar uma de B (terminada em `145`-`557`), em banda
nenhuma. O p2p é declarado, não derivado, e tem banda própria: A em `31303`-`31605`, B em
`30303`-`30605`.

**O `deploy-lnet` separa de outra forma, e está correto.** Lá é uma entidade por VM, e
quando A e B compartilham a VM a disjunção vem de outro arranjo: A no sufixo `645`
(base `8645`), B no sufixo `845` (base `8845`) — documentado na tabela *Coexistence on a
shared VM* do [`deploy-lnet/README.md`](../deploy-lnet/README.md). Verifiquei: 21 portas
do A e 15 do B na mesma VM, interseção **vazia**, máximas `32645` e `30304`. Portanto
**não** alinhe o `deploy-lnet` à faixa dos samples — o que se aplica aos dois é o teto,
não a faixa. Foi por isso que o guard passou a ler as duas árvores, mas só aplica a
verificação de faixa aos samples.

### O que mudou

- **B**: o deslocamento documentado em relação ao A deixou de ser `+25000` e passou a ser
  `+500`. Bases `33645/33745/33845/33945` → `9145/9245/9345/9445`; p2p `563xx`-`566xx` →
  `303xx`-`306xx`. Uma subtração uniforme de `24500` (e `26000` no p2p), o que mantém a
  numeração legível e a revisão verificável.
- **A**: as duas bases que estouravam o teto saíram da faixa. Argentina `8845` → `8665`,
  Costa Rica `8945` → `8685`. Brasil (`8645`) e Colômbia (`8745`) não mudaram, porque
  ambas já cabem na trilha `8645 + 20k`. A numeração deixou de ser monotônica por país —
  é cosmético, e o preço de não tocar em nenhuma porta que o LNET documenta e opera.

**O offset `+24000` do portal NOC do A não foi alterado de propósito.** Baixá-lo para
`+23000` (a única banda livre) corrigiria toda base de uma vez, mas mudaria o `32645` que
o `deploy-lnet` documenta e provavelmente tem regra de firewall — uma mudança
operacional externa, para nenhum ganho em produção, já que o LNET não estava exposto.

### O guard

`ports_ephemeral_test.go`, nos dois toolkits: percorre todo manifesto de sample **e do
`deploy-lnet`** (os `.yaml.tmpl`, que são a fonte versionada; os `.yaml` renderizados são
gitignored), calcula cada porta derivada e falha se alguma alcançar `32768`. Confere também o teto da base
(uma base abaixo de `32768` cujas derivadas estouram é o caso que passaria batido) e a
faixa dos três dígitos — esta só nos samples, pelo motivo acima — para que uma
renumeração futura não quebre a coexistência A/B em silêncio. No cenário B há ainda `TestDerivedOffsetsMatchTheEngineSources`, que lê as
fontes do engine e falha se aparecer um offset que `ports.go` não conhece — o teto vale
o que vale o conjunto de offsets de que ele é calculado.

O guard exclui `cbweb3-data/` e `bundles/` (estado gerado, no `.gitignore`): eles
carregam as portas do último deploy da máquina, e lê-los faria o guard falhar por estado
local, não por defeito do repositório.

**Consequência operacional**: um dataDir de um deploy anterior guarda as portas antigas.
Depois desta mudança, subir sem `--clean` mistura as duas numerações — use `--clean` ou
`make scenario-b.nuke`.

### Duas pendências, deliberadas

1. **Promover o guard a validação de manifesto.** Hoje ele cobre os samples do
   repositório, não o manifesto de um operador. O lugar natural é
   `manifest.Validate()`, mas `validate.go` dos dois cenários é de PRs abertas
   (#224 no A, #229 no B); fica para quando elas entrarem.
2. **Colapsar os literais do B sobre as constantes de `ports.go`.** Os offsets do B são
   literais inline em `step_found_hub.go`, `step_found_spoke.go` e `step_join.go` — os
   três de PRs abertas (#226, #228, #229). O teste anti-drift cobre o intervalo até lá.

---

## 11. Escala dos valores — intencional

Os dois cenários passaram a interpretar um valor inteiro de formas diferentes, e a
diferença é obrigatória, não descuido.

| | Scenario A | Scenario B |
|---|---|---|
| Escala do valor | **centavos (10⁻²)** | **10⁻¹⁸** |
| tCeBM é | **nota Zeto** (token de privacidade, Paladin) | **ERC-20** |
| Teto por movimento | `2⁶⁴ − 1` no circuito de **lock** | nenhum |

### O que foi medido

O Zeto usa circuitos distintos para operações distintas, e as faixas deles não são as
mesmas. Medido em stack ao vivo, com identidade nova a cada ponto:

| Circuito | Teto |
|---|---|
| **mint** | `2¹⁰⁰ − 1` = 1267650600228229401496703205375 |
| **lock** | `2⁶⁴ − 1` = 18446744073709551615 |

A diferença é de **2³⁶ — cerca de 69 bilhões de vezes**. E a nota que excede o teto do
lock **é cunhada normalmente**: ela existe na carteira e só se revela inutilizável quando
algo tenta travá-la, momento em que a transação não é recusada e sim nunca montada
(`Zeto_94`, `CheckHashes_92`, `PD012618`), deixando quem chamou esperando.

A 10⁻¹⁸, o maior valor travável no Scenario A seria **18,45 unidades monetárias**. Daí a
escala de centavos — que coincide com a unidade menor que a ISO 4217 dá às moedas do
piloto e usa 5×10⁻¹⁶ do teto.

O Scenario B **não tem Zeto** — não há adapter, só o relay Cacti, e seus tokens são
ERC-20 sem circuito ZK. Portanto não tem esse teto, e as 18 casas ali são legítimas.

### Não convergir

Igualar as escalas quebraria um dos dois lados: levar A para 18 casas o trava, e levar B
para centavos descartaria precisão que os pools da AMM produzem por construção — um swap
já deixou saldo de `3,959753632757569189`.

O que **deve** permanecer em passo é a regra de entrada, não a escala:
`packages/ui/src/lib/amount.ts` existe nos dois cenários no mesmo caminho, e `diff` entre
os dois arquivos é a forma de conferir. Ver ADR-009.

---

## 12. Proposta de FX — convergir

A proposta de FX de A passou a aceitar valores decimais (convertidos para unidades-base
antes do envio); a de B continua exigindo inteiro em unidades-base.

Hoje é inofensivo: num build de B a página **não é roteada** — ela vive em
`scenarioAChildren`, o caminho vestigial do FXAgreement, que é dead code em B por decisão
anterior e preservado por causa da fusão futura dos cenários.

Converge quando a fusão acontecer, ou antes disso se a tela de B voltar a ser roteada.
Enquanto ninguém a alcança, mexer nela seria alargar um PR sem ganho.

---

## 13. Autenticação do kcadm — convergida por cópia deliberada

Os dois cenários fazem `docker exec … bash -c` num script que começa por
`kcadm.sh config credentials`. B parou de mandar o segredo do administrador em argv; A
fazia o mesmo até a auditoria de guardas encontrá-lo (`docs/guard-parity.md`).

A regra agora é a mesma nos dois — prefixar `KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD"`
e não passar `--password` —, mas o código é **cópia deliberada**, não biblioteca
compartilhada, e mora em lugares diferentes porque os caminhos de provisionamento
diferem:

| | Scenario A | Scenario B |
|---|---|---|
| Helper | `toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go` (`kcadmLogin`) | `toolkit/engine/orchestrator/keycloak_provision.go` (`kcadmLogin`) |
| Guarda | `keycloak_secret_exposure_test.go` (Check e Run do passo de reconcile) | `keycloak_secret_exposure_test.go` (os três modos de provisionamento) |
| Por que difere o local | A provisiona o realm por **importação**; o único script kcadm do caminho de apply é o de reconcile de usuários | B provisiona o realm **por script**, então o login está no provisionamento |

Consequência prática: alterar a regra num cenário não altera o outro, e o par de testes
é o que impede a divergência silenciosa. Se um dia houver biblioteca compartilhada de
toolkit, este é candidato natural.

**Ainda divergente de propósito? Não.** Esta linha está convergida; fica registrada para
que ninguém "conserte" a duplicação apagando um dos lados.

---

## 14. Senha de operador em argv — era lacuna comum, agora convergida por cópia

Registrada originalmente porque uma leitura da seção 13 sugeria que o assunto estava
fechado, e não estava: **os dois cenários** passavam a senha de cada operador em argv, via
`set-password --new-password`. Não era diferença entre cenários — era a mesma exposição nos
dois, que é justamente o que uma comparação de paridade não acha por construção.

**Corrigida nos dois, e a regra é cópia deliberada**, pelo mesmo motivo da seção 13: os dois
toolkits não partilham biblioteca.

| | Scenario A | Scenario B |
|---|---|---|
| Helpers | `orchestrator/keycloak_admin_users_reconcile.go` | `orchestrator/keycloak_operator_password.go` |
| Nomes | `operatorPasswordVar`, `operatorPasswordEnv`, `dockerExecArgs` | os mesmos três |
| Guardas | `keycloak_secret_exposure_test.go` | `keycloak_secret_exposure_test.go` |
| Como o valor chega ao exec | campo `dockerExecCmd(…, env)` do passo | `exec.CommandRunner.RunWithEnv` |

A regra, idêntica nos dois: o script escreve `KC_CLI_PASSWORD="$KC_OP_PW_<n>"` como prefixo
de comando e **não** passa `--new-password`; o valor viaja no ambiente do processo filho, e
ao docker vai só o **nome**, via `docker exec -e NAME` (forma de passagem, não `-e NAME=valor`).

Por que o prefixo resolve: um argumento é legível por qualquer utilizador através do `ps`;
um ambiente não. E por que `-e NAME` e não `-e NAME=valor`: a segunda forma tira o segredo
do script e põe-no no argv do próprio cliente docker — a mesma exposição um passo à esquerda.

**Ainda divergente de propósito? Não.** Convergida. Fica registada para que ninguém "conserte"
a duplicação apagando um dos lados.

### Uma diferença de implementação que não é deriva de comportamento

Em A o passo já injetava o exec por um campo de função, então bastou dar-lhe um parâmetro de
ambiente. Em B o exec passa por `exec.CommandRunner`, cuja interface só tinha `Run`; ganhou
`RunWithEnv` nas três implementações (real, fake, dry). Método na interface em vez de
interface opcional com type assertion: um runner que se esquecesse de implementá-la seguiria
em silêncio pelo caminho do argv, que é o defeito a chegar sem avisar.

Efeito colateral bem-vindo em B: `realRunner` compõe o erro com `strings.Join(args, " ")`, ou
seja, um `docker exec` falhado imprimia o script inteiro — com as senhas dentro. Agora os
argumentos levam só nomes.

---

## Decisão pendente

O finding pede para "decidir a linhagem canônica e convergir, ou documentar a divergência
intencional". Este documento faz a segunda parte e delimita a primeira, mas a escolha de
linhagem é de liderança técnica, não de quem escreve o inventário. Em aberto:

1. **Existe um cenário canônico?** Vários itens acima (breaker, taxa, dialog) chegaram ao
   estado atual por port de B para A. Se B é a linhagem canônica de fato, vale dizê-lo por
   escrito, porque isso decide a direção de todo port futuro.
2. **A tesouraria do A deve ter auditoria e configurações?** (§7)
3. **O `dispatcher` deve existir no B?** (§9)
4. **Os módulos Go devem ser prefixados por cenário?** (§8) — recomendo sim, em PR própria.

Enquanto essas quatro não forem respondidas, este documento é a referência para "isto é
de propósito?".

Quando a pergunta 1 for respondida, a decisão pertence a um **ADR** em
`docs/decisions/` (próximo número livre: ADR-006 — o 005 já está reservado para a
escalabilidade N-spoke), na estrutura daquele diretório: contexto com evidência
`arquivo:linha`, opções com trade-offs, recomendação, plano e tabela de sign-off. Este
inventário serve como o contexto desse ADR; não o substitui, porque não decide nada.

---

## O que este documento não cobre

- **Backends além do NOC e dos módulos.** Não comparei serviço por serviço; o finding
  nomeia áreas específicas e foi isso que verifiquei. Uma comparação exaustiva de
  `backend/services/` é trabalho separado e maior.
- **Contratos além do AMM.** O finding cita o AMM; os demais contratos não foram
  diferenciados.
- **Deriva de documentação.** É o escopo de **R1-11.7** (reconciliar READMEs e docs com o
  estado real da `develop`). Quem pegar aquele card deve ler este antes, para não
  reescrever o mesmo inventário.
