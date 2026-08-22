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
| Circuit breaker do AMM | **Convergir** (nova) | Mesmo quórum (2), mas só o A conta instituições distintas |
| Taxa do AMM | **Convergir** (parcial) | Mecanismo igual; padrão 0,3% em A e 0% em B |
| Superfície do AMM | **Intencional** | B tem liquidez cooperativa e saque; A não |
| Proteções de rota | **Convergir** | A comenta rotas; B escolhe conjunto por flag de build |
| Integração da tesouraria | **Decisão** | Duas páginas idênticas, vivas em B e desligadas em A |
| Caminhos de módulo Go | **Convergir** | Três serviços dos dois cenários declaram o mesmo módulo |
| App `dispatcher` | **Decisão** | Existe só em A |

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

## 3. Circuit breaker do AMM — convergir (divergência nova, criada de propósito)

O port do R2-H-2 aconteceu. Os dois contratos têm `pause`, `signResume`, `isPaused` e
`RESUME_QUORUM`, e o quórum numérico é o mesmo:

- `scenario-a/contracts/src/AutomatedMarketMaker.sol:31` — `RESUME_QUORUM = 2`
- `scenario-b/contracts/src/AutomatedMarketMaker.sol:32` — `RESUME_QUORUM = 2`

O breaker assimétrico (pausa 1-de-N, retomada 2-de-N) que a constituição exige está
implementado nos dois. **O que os dois contam é que passou a divergir**, e vale registrar
antes que alguém leia a assimetria como acidente:

| | Cenário A | Cenário B |
| --- | --- | --- |
| Deduplicação da retomada | por **instituição** (`institutionSigned`, via `IdentityRegistry.getInstitutionId`) | por **endereço** (`signed[msg.sender]`) |
| Vinculação à época da pausa | sim (`pauseEpoch`) | não |
| `institutionId` no registro de identidades | sim, obrigatório e não-zero | não existe |

A dedupe por endereço deixa um banco central com duas carteiras de governança formar
o 2-de-N sozinho — exatamente o que o quórum existe para impedir. O cenário A fechou isso
(follow-up do R2-H-2); o cenário B tem a correção equivalente pronta no branch
`fix/amm-resume-quorum-to-require-distinct-institutions` (PR #80), **ainda não integrado**.

Portanto: divergência **temporária e intencional na direção certa**. Ela se resolve
integrando o PR #80, não removendo a checagem do A. As duas implementações usam o mesmo
desenho (`institutionSigned` por proposta + `getInstitutionId` no registro) justamente
para que a integração não precise reconciliar nada.

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
