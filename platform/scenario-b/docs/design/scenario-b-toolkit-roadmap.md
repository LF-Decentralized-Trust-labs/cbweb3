# Toolkit do Cenário B — Roteiro de Alterações

**Status**: Análise / design (nenhum código escrito ainda)
**Última atualização**: 2026-07-10
**Tarefa relacionada (Notion)**: "[Scenario B] Create a Scenario B Toolkit Analogous to the Scenario A Toolkit"
**Referência de implementação**: `scenario-a/toolkit` (binário `cbweb3`) — usado apenas
como referência de *como* construir o toolkit; o Cenário B é independente.

---

## 1. Objetivo e escopo

Este documento é um roteiro do que precisa ser alterado no Cenário B para viabilizar a
construção de um toolkit de provisionamento declarativo, tomando o toolkit do Cenário A
como referência de implementação. É um artefato de planejamento para revisão da equipe,
não um plano de implementação. Espera-se que uma spec numerada do spec-kit (`spec.md` +
`plan.md` com Constitution Check e Complexity Tracking) seja produzida em seguida, uma vez
acordada a direção.

**Não-objetivos (escopo inicial):** upgrade/redeploy de contratos e migração de estado
on-chain ficam fora — como no Cenário A, targets de "redeploy" apenas re-deployam e
re-sincronizam endereços; mudança de fonte de contrato é concern à parte.

### Restrições rígidas

1. **Os Makefiles de deploy do Cenário B não são alterados.** O `scenario-b/Makefile` e
   todos os arquivos sob `scenario-b/make/*.mk` permanecem intactos. Tudo o que o toolkit
   provisiona, o próprio toolkit provisiona. Os Makefiles e o `deploy/local/` passam a
   cumprir o papel de *especificação de referência executável* do que o toolkit precisa
   reproduzir, e continuam servindo como caminho legado de bring-up ao lado do toolkit.
2. **Isolamento de cenário (Constituição v1.0.4).** O toolkit não importa
   `scenario-a/toolkit`. Ele é um módulo Go novo e independente. O toolkit do Cenário A é
   um template *conceitual* a ser reimplementado, nunca um pacote do qual depender.
3. **Privacidade, atomicidade, compliance e observabilidade** da constituição aplicam-se a
   cada caminho provisionado (`tCeBM` apenas na camada de reserva, circuit breaker validado
   antes de swaps, gate de compliance não contornado, logs estruturados). A camada de
   privacidade (Zeto/Noto) é um ponto de escopo a definir com a equipe — ver seção 13.

---

## 2. Arquitetura do Cenário B e o papel do toolkit de referência

O toolkit do Cenário A é a **referência de implementação**: dele reaproveitamos, por
reimplementação, o motor de steps idempotente (`Step{Name,Check,Run}` + estado + lock +
guard de genesis), o mecanismo de bundle como hand-off entre modos, as interfaces
plugáveis (`KeyProvider`, `CertSource`, `RelayRegistrar`), os templates de compose
parametrizados por env e o processo de construção incremental por spec-kit. Nada disso é
importado — é reescrito no módulo do Cenário B.

A **arquitetura do Cenário B é própria e independente**: uma topologia em estrela.

- **Hub neutro** (chain 1337): no `found-hub` implanta `IdentityRegistry`, `tCeBM_BRL` +
  `tCeBM_EUR` (camada de reserva), `FXAgreement`, `PairRegistry`, `CurrencyRegistry` e
  `ManualOracle`. `LiquidityCommitRegistry` e as AMMs por par soberano
  (com tokens `W-tCeBM`) são criados depois, na abertura de cada par (ver seção 6). QBFT com
  `hub-validator` como validador único (bootnode); o hub roda apenas esse nó (eventualmente
  dois para resiliência).
- **N spokes soberanos, dinâmicos** (Brasil, Argentina, Colômbia, …): cada `found-spoke`
  funda **um** spoke novo a partir do manifesto — **não há conjunto fixo `spoke-a`/`spoke-b`**.
  Cada spoke tem `IdentityRegistry`, `tCeBM` doméstico, `SpokeBridge` e `fCeBM`; QBFT com o
  **CB como validador único**, e os bancos entram como **full nodes não-validadores** (modelo do
  Cenário A — ver §6/§13). A identidade do spoke (`id`, `chainId`, `currency`) vem do manifesto;
  `spoke-a`/`spoke-b` aparecem neste documento apenas como exemplo.
- **Acesso ao hub (RPC-only)**: as entidades (CBs e bancos) rodam nó apenas no seu spoke;
  nenhuma roda nó no hub. Os backends acessam o hub exclusivamente por RPC ao `hub-validator`
  (:8845). Consequência: o hub permanece com um único nó (ou dois) e não cresce com os
  spokes.
- **Liquidez da AMM** entra por **commit-reveal cooperativo** (`LiquidityCommitRegistry` +
  relay casando `CommitMatched`), permitindo que cada CB contribua apenas com a própria
  moeda (ver `docs/design/cooperative-liquidity.md`).
- **Circuit breaker** embutido em cada AMM de par soberano: pause 1-de-N, resume com quorum 2.
- **Relay Cacti neutro** observando o hub e o WS de cada spoke; aprende o
  `LiquidityCommitRegistry` dinamicamente (nasce na abertura de par soberano).

A **unidade de provisionamento** do toolkit é o conjunto hub + N spokes + bancos, expresso
por **três modos** (`found-hub`, `found-spoke`, `join`) e **dois bundles** (hub → spoke e
spoke → banco).

---

## 3. Novo módulo e estrutura (net-new)

Criar `scenario-b/toolkit/` como módulo Go isolado (`.../scenario-b/toolkit`), Go 1.26,
dependências mínimas (`github.com/ethereum/go-ethereum`, `gopkg.in/yaml.v3`), usando o
layout de pacotes do toolkit de referência como base de estrutura:

- `cmd/cbweb3b/main.go` — binário único, `apply -f <manifest> [--dry-run] [-o json|yaml]`,
  despacho por modo, report sempre emitido em stdout, `signal.NotifyContext` para reports
  parciais.
- `engine/manifest/` — parse + validação dos kinds do manifesto.
- `engine/apply/` — despacho de modo, resolução de profile/caminhos, dry-run, report.
- `engine/orchestrator/` — o motor de steps e os arquivos `step_*.go`, os conjuntos de deps
  por modo (`found-hub` / `found-spoke` / `join`), estado, portas, nomeação de entidades,
  helpers de QBFT/relay/liquidez.
- `engine/bundle/` — emit/load/validate do hub-bundle e do spoke-bundle.
- `engine/{keyprovider,certsource,genesis,pki,addrs,dockervolume}/` — pacotes equivalentes.
- Assets não-Go sob `scenario-b/provisioning/` — `schema/v1/`, `templates/`, `docs/adr-*`,
  `spikes/`.

---

## 4. Manifesto e modos

Um manifesto específico do Cenário B (`apiVersion: cbweb3b/v1`) com **três modos**:

- **`found-hub`** — operador neutro do hub. Sobe a rede hub (QBFT, validador único),
  implanta os contratos de hub e estabelece a **própria governança do hub** (o operador detém
  `GOVERNANCE_ROLE`/`DEFAULT_ADMIN_ROLE`); os CBs se registram depois, no `found-spoke`
  (auto-registro — ver §13). Ao final **emite o hub bundle**.
- **`found-spoke`** — banco central soberano. Consome o hub bundle, sobe sua rede spoke,
  implanta contratos de spoke, conecta os endereços de hub no backend, registra no relay e
  **emite o spoke bundle**.
- **`join`** — banco comercial anexa ao seu spoke, entrando como **full node não-validador**
  (o CB é o validador único; modelo do Cenário A — ver §6/§13).

Campos específicos do modelo do Cenário B:

- `topology.role: hub | central-bank | commercial-bank`
- `hub{ chainId, currency[] }`
- `hubBundleRef` (consumido no `found-spoke`)
- `pair{ proposerCB, confirmerCB, symbolA, symbolB }` para pares soberanos

O circuit breaker não tem campo de manifesto: `pause` (1-de-N) e `resume` (`RESUME_QUORUM = 2`)
são regras fixas do `AutomatedMarketMaker`, e os signatários são o conjunto de governança
registrado no `IdentityRegistry` (ver seção 9, item 3).

Demais campos do modelo: `environment` (apenas `local` implementado inicialmente),
`node{ advertisedHost, portas rpc/ws/p2p, dataDir }`, `image`, `keyProvider` (`kms://`),
`certSource` (`self-signed` / `ca://`), `relay{ endpoint, advertisedHost }`, `adminUsers[]`.

Os exemplos abaixo assumem um `kind` único `ParticipantDeployment` para os três modos, com
`spec.mode` e `spec.topology.role` como discriminadores. São manifestos de orientação base,
sujeitos ao schema final da fase TK-B1. Observações válidas para todos:

- **Nenhum segredo no manifesto.** Chaves privadas e material de certificado nunca
  aparecem aqui — são mediados por `keyProvider` / `certSource`. Os campos de senha em
  `adminUsers[]` são credenciais de bootstrap apenas de `local` (grant ROPC do Keycloak
  para que o log de auditoria carregue um ator real); em `staging`/`prod` virão de um cofre.
- **Valores de porta e chain id** seguem o que o `deploy/local` usa hoje (hub 1337 / spoke-a
  1338 / spoke-b 1339), para o toolkit conviver com o caminho legado no mesmo host. Para
  spokes além desses, o toolkit aloca `chainId`/portas/rede de forma determinística e sem
  colisão a partir do manifesto (ver seção 13, item 12).
- **Moedas.** O hub implanta os tokens base `tCeBM_BRL` e `tCeBM_EUR` no `found-hub`. Pares
  soberanos como `W-BRL-ARS` (tokens `W-tCeBM`) são criados depois, na abertura de par
  soberano; `ARS` é a moeda doméstica do `spoke-b`.
- **Acesso ao hub (RPC-only).** As entidades não rodam nó no hub; os backends alcançam o hub
  por RPC ao `hub-validator` (:8845). Por isso os manifestos de `found-spoke`/`join` não
  declaram nó de hub — apenas o nó do próprio spoke.
- **MLP (Market Liquidity Provider)** é um participante/role opcional (`ENABLE_MLP`), não
  mostrado nos exemplos; entraria como um manifesto adicional de role `commercial-bank` com
  a flag de MLP.

### 4.1 Modo `found-hub`

Operador neutro do hub. Emite `bundles/hub-cbweb3.bundle.yaml` ao final.

```yaml
apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata:
  name: hub-cbweb3
spec:
  scenario: "b"
  environment: local
  mode: found-hub
  topology:
    role: hub
  displayName: "CBWeb3 Interoperability Hub"

  hub:
    chainId: 1337
    currency:                       # tokens tCeBM base implantados no hub
      - BRL
      - EUR

  node:
    advertisedHost: host.docker.internal
    rpc:  { port: 8845 }
    ws:   { port: 8855 }
    p2p:  { port: 31503 }
    dataDir: ./.data/hub

  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed           # o hub atua como CA da própria rede

  relay:
    endpoint: http://host.docker.internal:4000

  frontendHost: localhost

  adminUsers:
    - role: GOVERNANCE
      username: hub-governance
      password: <local-only-bootstrap>
    - role: NOC_ADMIN
      username: hub-noc
      password: <local-only-bootstrap>
```

### 4.2 Modo `found-spoke`

Banco central soberano (exemplo: `central-bank-a` no `spoke-a`). Consome o hub bundle e
emite `bundles/spoke-a.bundle.yaml`.

```yaml
apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata:
  name: central-bank-a
spec:
  scenario: "b"
  environment: local
  mode: found-spoke
  topology:
    role: central-bank
  displayName: "Central Bank A"

  spoke:
    id: spoke-a                     # exemplo; cada found-spoke funda um spoke (N, sem conjunto fixo)
    chainId: 1338
    currency: BRL                   # moeda doméstica (tCeBM_BRL / fCeBM_BRL)

  hubBundleRef: ./bundles/hub-cbweb3.bundle.yaml

  node:
    advertisedHost: host.docker.internal
    rpc:  { port: 8645 }
    ws:   { port: 8655 }
    p2p:  { port: 31303 }
    dataDir: ./.data/spoke-a/central-bank-a

  pair:                             # par soberano proposto por este CB,
    proposerCB: central-bank-a      # confirmado pelo CB contraparte (step bilateral, soft)
    confirmerCB: central-bank-b
    symbolA: BRL
    symbolB: ARS

  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed           # este CB atua como CA do seu spoke

  relay:
    endpoint: http://host.docker.internal:4000

  cbEndpoint: http://host.docker.internal:18080   # gateway do CB p/ request de credencial (join)
  frontendHost: localhost

  adminUsers:
    - role: GOVERNANCE
      username: cb-a-governance
      password: <local-only-bootstrap>
    - role: TREASURY
      username: cb-a-treasury
      password: <local-only-bootstrap>
    - role: SUPERVISOR
      username: cb-a-supervisor
      password: <local-only-bootstrap>
```

### 4.3 Modo `join`

Banco comercial anexando ao spoke (exemplo: `bank-a` no `spoke-a`). Consome o spoke bundle.

```yaml
apiVersion: cbweb3b/v1
kind: ParticipantDeployment
metadata:
  name: bank-a
spec:
  scenario: "b"
  environment: local
  mode: join
  topology:
    role: commercial-bank
  bankId: bank-a                    # alimenta CSR CN, IdentityRegistry, BANK_ID
  displayName: "Bank A"

  spoke:
    id: spoke-a
    chainId: 1338
    currency: BRL

  joinBundleRef: ./bundles/spoke-a.bundle.yaml

  node:
    advertisedHost: host.docker.internal
    rpc:  { port: 8646 }
    ws:   { port: 8656 }
    p2p:  { port: 31304 }
    dataDir: ./.data/spoke-a/bank-a
    validator: false                # banco entra como full node não-validador (ver seção 6)

  image: build
  keyProvider: kms://local-emulator
  certSource: ca://central-bank-a   # leaf cert emitido pela CA do CB do spoke

  relay:
    endpoint: http://host.docker.internal:4000

  frontendHost: localhost

  adminUsers:
    - role: BANK
      username: bank-a-admin
      password: <local-only-bootstrap>
```

---

## 5. Bundles (dois níveis)

- **Hub bundle**: endpoint RPC/WS do hub (`advertisedHost` + portas), chainId, endereços dos
  contratos base do hub (IdentityRegistry, tCeBM_BRL, tCeBM_EUR, PairRegistry,
  CurrencyRegistry, FXAgreement, ManualOracle) e cert da CA do hub. Não há "config de circuit
  breaker" a carregar — o estado do breaker deriva do `IdentityRegistry` (cujo endereço já vai
  no bundle). Como as entidades acessam o hub por RPC (não entram na rede P2P), o bundle
  **não precisa** de enode/genesis do hub. Emitido no `found-hub`, consumido no `found-spoke`.
  O `LiquidityCommitRegistry` e as AMMs de par soberano nascem depois (seção 6), então o relay
  aprende esses endereços de forma dinâmica, não pelo bundle.
- **Spoke bundle**: enode/genesis/hash do spoke, endereços de spoke (IdentityRegistry,
  tCeBM doméstico, SpokeBridge, fCeBM) **mais** os endereços de hub reembalados, o cert da
  **CA do CB fundador** (âncora de confiança do spoke, usada para assinar/validar os certs de
  banco — ver seção 15), e o **conjunto de validadores QBFT (só o CB do spoke)**. Emitido no
  `found-spoke`, consumido no `join`.

Ambos permanecem públicos por design (nunca contêm chave privada; rejeitar qualquer
arquivo que contenha `PRIVATE KEY`) e são escritos atomicamente em `<outputDir>/bundles/`. O
**spoke bundle** lê genesis/enode dos named volumes do Besu via o helper `dockervolume`; o
**hub bundle** dispensa isso (RPC-only) e monta-se a partir dos endereços de contrato e do
cert da CA do hub.

---

## 6. Motor de steps — inventário por modo

O motor em si (`Step{Name,Check,Run}` + estado YAML idempotente + `flock` + persistência
por step + guard de genesis não-destrutivo) reaproveita o padrão do toolkit de referência.
O trabalho novo é o **inventário de steps**, derivado dos alvos de Makefile do próprio
Cenário B.

Dois pré-requisitos transversais a todos os modos:

- **Gates de prontidão:** cada step depende da saúde do anterior — esperar o **Besu RPC** no
  ar antes do deploy de contratos (hoje o `wait-rpc.sh`), o **Keycloak** pronto
  (`/realms/master` + marcador `KEYCLOAK_INIT_DONE`) antes de provisionar realms, a **sync** do
  Besu no `join` (`wait-sync`) e a **saúde do relay** antes dos swaps. Os `Check()`/gates devem
  cobrir isso.
- **Build de contratos:** os steps `deploy-*-contracts` assumem artefatos Foundry prontos — o
  toolkit roda `contracts.setup` (soldeer) + `contracts.build` antes, ou assume artefatos
  pré-buildados em `contracts/out/`.

### `found-hub`
`start-besu-hub` (apenas o nó validador/bootnode do hub — eventualmente dois para
resiliência; as entidades não rodam nó no hub, acessam-no por RPC) → `deploy-hub-contracts`
(ordem de dependência:
IdentityRegistry → tCeBM_BRL → tCeBM_EUR → FXAgreement → PairRegistry → CurrencyRegistry →
ManualOracle) → `provision-keycloak-hub`
(realms/clients + **write-back** dos client secrets JWT nos env) → `render-hub-env` →
`start-hub-infra` / `backend` / `frontend` → `start-relay` (serviço neutro compartilhado;
observa o hub e vai registrando o WS de cada spoke e os endereços de `LiquidityCommitRegistry`
dinamicamente — depende da generalização do relay, seção 9, item 1) → `start-noc` (NOC neutro:
`noc-db`/`noc-backend`/`noc-portal`) → emissão do hub bundle.

> **Registro de CBs — no `found-spoke`, não aqui.** O `found-hub` não registra CBs (eles ainda
> não existem). O admin do hub já detém `GOVERNANCE_ROLE`/`DEFAULT_ADMIN_ROLE` pela construção do
> `IdentityRegistry`. Cada CB é registrado no `found-spoke` via `register-cb` (auto-registro
> dirigido por solicitação — ver §13 e Steps bilaterais).

> **Circuit breaker — sem step de configuração.** Os signatários são o conjunto retornado por
> `IdentityRegistry.canGovern()` (participantes `Verified` com papel `CENTRAL_BANK` ou
> `GOVERNANCE`), lido ao vivo pelo modificador `onlyGovernance` do `AutomatedMarketMaker`.
> Registrar um CB no `IdentityRegistry` do hub já o torna signatário de **todas** as AMMs
> de par soberano, sem reconfigurar contrato. `pause` é 1-de-N e `resume` exige
> `RESUME_QUORUM = 2` — logo o caminho de resume só fica operante após ≥2 CBs registrados.
> (Atenção: o `GOVERNANCE_ROLE` do OpenZeppelin, que autoriza `registerParticipant`, é
> distinto de `canGovern()`, que rege o breaker.)

> **A liquidez da AMM não é semeada aqui.** O `seed-hub` fica fora do fluxo default: a
> liquidez entra por **commit-reveal cooperativo** (`LiquidityCommitRegistry`, relay casando
> `CommitMatched`), fluxo bilateral e posterior à existência dos spokes — ver
> `docs/design/cooperative-liquidity.md` e a seção 13.

Mapeia para os alvos de Makefile: `deploy.up-hub-besu`, `contracts.deploy-hub`, `cacti-up`. (Os
alvos de registro/grant de CB — `register-participants-hub`, `contracts.grant-liquidity-providers`,
`contracts.grant-central-bank-role(-b)` — rodam **por CB no `found-spoke`**, via `register-cb`.)

> **Diferença deliberada frente ao `deploy/local` atual.** O `hub-besu/startBesu.sh`
> pré-inicia 5 nós (hub-validator + um nó por entidade); o toolkit roda **apenas o
> hub-validator** (eventualmente dois) e as entidades acessam o hub por RPC — sem nós de
> entidade no hub, sem uso do `addNewNode.sh`.

### `found-spoke` (por CB soberano)
`consume-hub-bundle` → `register-cb` (auto-registra o CB no `IdentityRegistry` do hub —
`registerParticipant` CENTRAL_BANK + `grantLiquidityProvider`, dirigido por solicitação;
**automático agora**, ver §13) → `start-besu-spoke` (CB como validador/bootnode) →
`deploy-spoke-contracts` (IdentityRegistry → tCeBM doméstico → SpokeBridge → fCeBM;
grant `GOVERNANCE_ROLE` ao CB) → `wire-hub-addresses` (endereços de hub do bundle → env do
backend) → `provision-keycloak-spoke` (realms/clients + write-back) → `render-spoke-env` /
infra / backend / frontend → `register-relay-spoke` (auto-registro no relay via
`POST /api/v1/spokes`: WS do spoke + URL do gateway — runtime, sem restart) → `add-noc-agent`
(soft: sobe um `noc-agent` ligado ao Besu do spoke e o registra no `noc-backend`;
observabilidade — falha não bloqueia o `found-spoke`)
→ emissão do spoke bundle.

> **Abertura de par soberano — fora do provisionamento (decisão vigente).** A cauda
> `open-sovereign-pair` / `commit-liquidity` / `seed-oracle` foi **removida** do `found-spoke`
> (commit `c90de691`); não existe bloco `spec.pair` no manifesto nem no JSON-Schema, e
> `TestApplyFoundSpokeHasNoSovereignTail` garante que o `apply` nunca planeje esses steps.
> Abrir um corredor é um ato de **runtime**, não de provisionamento: um CB propõe pelo portal
> de governança (`proposePair` no `PairRegistry`) e a contraparte confirma (`confirmPair`);
> em seguida cada CB compromete a **própria** moeda por commit-reveal no
> `LiquidityCommitRegistry`, com o relay casando `CommitMatched`. O `found-spoke` registra o CB
> e a sua moeda soberana no hub, e para por aí — nenhum run detém a chave da contraparte.

> **Camada de privacidade.** O Cenário B, no estado atual, não possui contratos
> `ZetoToken`/`NotoToken` em `contracts/src` nem integração Paladin nos serviços Go. O
> toolkit provisiona exatamente o que o cenário executa (tCeBM/fCeBM/AMM/registries;
> `SpokeBridge` provê lock-and-mint entre spoke e hub). Se a camada de privacidade entrar no
> escopo, os steps correspondentes são acrescentados — decisão registrada na seção 13.

### `join` (por banco comercial)
`write-genesis` → `start-besu-join` → `wait-sync` → `wire-addresses` (do spoke bundle) →
`provision-keycloak-bank` → env / infra / backend / frontend → **cauda diferida**: `gen-csr`
(o **único** step de PKI do toolkit — par de chaves + CSR local). A emissão do cert (o CB assina
o CSR via compliance, gated por aprovação KYC no portal `governance`) e o registro on-chain no
`IdentityRegistry` são **runtime**, não steps do toolkit — ver seção 15.

> **Banco entra como full node não-validador (modelo do Cenário A).** O banco sincroniza e
> transaciona, mas **não** produz blocos; o **CB é o validador único** do spoke (genesis
> `count: 1`), e o `vote-qbft` **não** está no fluxo canônico de `join`. O toolkit do B segue
> esse modelo. A promoção a validador existe como **capacidade diferida** (o `vote-qbft` do A
> promove um nó via `qbft_proposeValidatorVote` na rede viva — usar só se a governança decidir
> adicionar um validador; ADR-002). **Divergência** do `deploy/local` atual do B, cujo
> `startBesu.sh` de spoke gera 2 validadores (CB + banco).

### Steps bilaterais — abertura de par soberano (opção B)

Envolve **três partes** (operador do hub H + CB-A proponente + CB-B confirmador), coordenadas
pela **máquina de estados on-chain** do `PairRegistry` (`PROPOSED→ACTIVE`; eventos
`PairProposed`/`PairRegistered`). Reusa o primitivo request→approve→gated do onboarding. Fases:

1. **`corridor-request`** (CB-A) — declara o par (`symbolA`/`symbolB`/contraparte) via **API no
   gateway do hub**.
2. **scaffolding** (H, admin — **gate leve de governança do hub**) — deploy dos `W-tokens` que
   faltarem, da AMM do par e (reuso do) `LiquidityCommitRegistry`; `setCentralBankOf` mapeando
   cada W-token ao seu CB; grants (LP, relayer). **W-token é por moeda, reutilizável** entre
   corredores — o `SeedNewSovereignPair` hoje deploya por par, então o toolkit deduplica por
   moeda.
3. **`propose-pair`** (CB-A, soberano) — `proposePair(pairId, W-A, W-B, amm)`; emite
   `PairProposed`. `pairId` determinístico (ex.: `W-BRL-ARS`).
4. **descoberta** (CB-B) — o `Check()` do step lê `getPair(pairId).status`; `PROPOSED` = aguardando.
5. **`confirm-pair`** (CB-B, soberano — **gated pela governança do CB-B**) — `confirmPair`;
   `PROPOSED→ACTIVE`; emite `PairRegistered` (só então o `PairRouter` do gateway enxerga o par).
6. **`commit-liquidity`** (CB-A e CB-B) — commit-reveal, cada CB só a sua moeda; o relay casa
   `CommitMatched`. Em seguida **`seed-oracle`** (`setRate`).

**Idempotência:** o estado vive on-chain, então re-rodar qualquer `apply` converge (`ACTIVE` →
pula; `PROPOSED` → pending aguardando a contraparte; inexistente → propõe, se eu for o CB de
`tokenA`). **Soberania:** cada CB assina o seu próprio ato; H nunca assina no lugar de um CB. Em
`local`, o toolkit pode deter as chaves das três partes e completar tudo de uma vez; fora de
`local`, os atos gated ficam `pending` até aprovação.

---

## 7. Templates de compose (net-new, derivados)

Criar `scenario-b/provisioning/templates/` (hub, entity-infra, entity-keycloak,
entity-backend, entity-frontend, relay), **derivados** dos compose atuais de `deploy/local`,
mas com todos os valores discriminantes externados por env (`${VAR:?}`): nomes de
container, redes, bandas de porta determinísticas por entidade, chain id, e named volumes
para estado de nó. Os compose atuais de `deploy/local` **não são movidos nem editados** —
continuam servindo o caminho de Makefile. Aplicar um esquema de porta determinístico por
offset e `host.docker.internal:host-gateway` para alcance cross-stack (backends → hub em
`:8845`).

**Persistência em named volumes (estratégia do Cenário A).** Todo o estado de participante vai
para **named volumes do Docker**, nunca em pastas do host. No Cenário A cada volume tem nome
**determinístico por entidade** — `${SPOKE_ID}_cb_besu_data`, `_cb_genesis`, `_cb_config`,
`_cb_scratch`, `_cb_tls` (mesmo padrão de Paladin `*_data` e Postgres `pg_data`) — declarado no
compose como `volumes: <lógico>: { name: ${SPOKE_ID}_... }`. Os volumes são semeados por
containers de init (owned by root) e o toolkit lê/escreve arquivos dentro deles via o helper
**`dockervolume`** (`docker run --rm -v <vol>:/target alpine` com `cat`/`mkdir`/`chmod`), sem
tocar o FS do host — é assim que o `gen-tls` semeia certs e o bundle emitter lê genesis/enode. O
toolkit do B deve adotar o mesmo esquema: named volumes determinísticos por spoke/entidade
(`<spoke>_<entidade>_besu_data`, `_genesis`, `_config`, `_tls`, DB, …), substituindo os **bind
mounts host** que o `deploy/local` usa hoje (o `startBesu.sh` persiste o nó em
`nodes/<entidade>/data`). **Única exceção de bind mount:** o dir de PKI do banco
(`<dataDir>/pki` — chave + CSR), que precisa ser acessível ao host para o fluxo de
onboarding/KYC (seção 15). O `node.dataDir` do manifesto guarda só o estado do toolkit
(`.provisioning-state.yaml`, `.provisioning.lock`) e esse `pki/` — **não** o estado do nó.

---

## 8. Propagação de endereços cross-network (a parte mais delicada)

Este é o ponto mais delicado do toolkit. Hoje o `deploy/local/tools/sync-contracts.sh` lê o
broadcast JSON do Foundry e faz `sed`-upsert dos endereços em `backend/config/.env.infra.*`
**e** no `.env` do relay Cacti. No toolkit, essa propagação passa a ser **dirigida por
bundle**:

- Endereços de hub fluem para cada spoke via **hub bundle**.
- Endereços de spoke fluem para bancos via **spoke bundle**.
- Escrever endereços nos arquivos `.env` de backend vira responsabilidade dos steps
  `wire-*-env` (via `addrs.AppendAddr`), não de um script bash.

O `sync-contracts.sh` permanece para o caminho legado de Makefile.

Endereços que precisam cruzar redes (hub → cada entidade de spoke + relay):
`HUB_IDENTITY_REGISTRY_ADDRESS`, `HUB_TOKEN_A/B_ADDRESS`, `AMM_CONTRACT_ADDRESS`,
`PAIR_REGISTRY_CONTRACT_ADDRESS`, `CURRENCY_REGISTRY_CONTRACT_ADDRESS`,
`LIQUIDITY_COMMIT_REGISTRY_ADDRESS` (criado na abertura de par soberano, não no `found-hub`;
o relay precisa aprendê-lo dinamicamente) e `SOVEREIGN_*`.

---

## 9. Alterações cirúrgicas em código existente do Cenário B

São as únicas edições reais fora dos Makefiles.

1. **Relay (`interop/hub-and-spoke/cacti/`)** — `src/config.ts` fixa duas spokes (`spokeA` /
   `spokeB`, lendo `SPOKE_A/B_BESU_RPC/WS`); o endereço do `LiquidityCommitRegistry`, o
   `HUB_BESU_RPC/WS` e o `GATEWAY_INTERNAL_URLS` são lidos em `src/liquidity-commit-watcher.ts`
   e `src/index.ts`. Generalizar para N spokes e para aprender o(s) `LiquidityCommitRegistry`
   dinamicamente (os pares soberanos nascem em runtime), sem hardcode — mudança que toca
   `config.ts`, `liquidity-commit-watcher.ts` e `index.ts`. Plano completo na seção 14.B.
2. **Forge scripts que embutem endereços de genesis** — `RegisterParticipants.s.sol` (registro
   de participantes) e `SeedNewSovereignPair.s.sol` (par soberano), além dos scripts de deploy
   que recebem endereços por env. Parametrizá-los para receber endereços/chaves de
   participantes injetados, de modo que o toolkit forneça **chaves derivadas por banco** e uma
   conta fCeBM única por banco (já validado: chave Paladin por banco + zero-gas;
   `ENTITY_BESU_ADDRESS` por banco). Obs.: o `SeedHub.s.sol` está fora do fluxo default (a
   liquidez é cooperativa), então sua parametrização é de baixa prioridade.
3. **Circuit breaker — validação no relay.** Não há contrato separado; a lógica vive em cada
   `AutomatedMarketMaker` de par soberano: pause 1-de-N (`onlyGovernance`),
   resume com `RESUME_QUORUM = 2` (`proposeResume` + `signResume`), `isPaused()` para consulta
   e uma saída bilateral `whenPaused` que evita travar LPs durante a pausa. Os contratos não
   precisam mudar (signatários vêm de `IdentityRegistry.canGovern()`), **mas o relay precisa
   passar a validar `isPaused()` antes de encaminhar swaps** — hoje ele não faz essa checagem
   (nenhuma referência a `isPaused` em `interop/hub-and-spoke/cacti/src/`), o que descumpre o
   Princípio III da Constituição. Esta é a alteração de código a fazer aqui.

---

## 10. Interfaces plugáveis

Reimplementar `KeyProvider` (`kms://`), `CertSource` (`self-signed` / `ca://`) e
`RelayRegistrar` com implementações locais in-memory + stubs de prod, espelhando as
interfaces do toolkit de referência. Sem segredos em manifesto, estado ou bundle.

**Estratégia de chaves (Cenário B) — cada participante tem as suas próprias chaves.** Nada de
chaves compartilhadas entre entidades (hoje o `deploy/local` reutiliza as 12 chaves de genesis
em todas as redes — ver §14.C). O `KeyProvider` deve prover, por participante:

- **Chave própria por banco**, com `ENTITY_BESU_ADDRESS` único e **desacoplado** do signer
  `CENTRAL_BANK_ROLE` — senão a tokenização de um banco debita a conta de outro (já validado).
- **Signer do CB, hub-signer por CB e chave do relayer** distintos (os grants on-chain e o
  `SeedNewSovereignPair` dependem de endereços por entidade).
- **Chaves únicas de genesis por spoke** — o toolkit gera o `alloc` por spoke, em vez de reusar
  o conjunto fixo.
- **Zero-gas / EIP-1559:** `maxFeePerGas=0` nas transações; sem o funding pré-checado do
  EIP-1559, uma chave distinta (não pré-fundada) trava — garantir a config zero-gas ao usar
  chave própria por banco.

As chaves privadas nunca entram em manifesto/estado/bundle (mediadas pelo `KeyProvider`/KMS).

---

## 11. Testes e processo spec-kit

- E2E próprio do Cenário B: hub + spoke + banco, swap pela AMM, checagem do circuit breaker
  (pause/resume) e caminhos de timeout/refund do `SpokeBridge` (lock→mint / burn→unlock).
  Baseline de performance se production-grade.
- Nascer de spec numerada em `specs/` com Constitution Check e Complexity Tracking,
  construída incrementalmente em fases "TK-B", com spikes antes dos steps de rede.

---

## 12. Sequenciamento sugerido (fases TK-B)

A ordem respeita as dependências reais: o **relay generalizado vem antes do `found-spoke`**
(que faz `register-relay-spoke`) e antes do `found-hub` (que faz `start-relay`); cada **bundle
emitter** fica na fase de quem o emite; o **`join`** é separado do fluxo **bilateral de par
soberano** (que exige dois spokes prontos + relay).

1. **TK-B1** — spec + schema de manifesto (três modos) + validação. **[Implementado]**
2. **TK-B2 / B3** — KeyProvider + CertSource. **[Implementado]** — pacotes
   `engine/keyprovider` (secp256k1/EVM via go-ethereum, local determinístico + stub de prod,
   factory `kms://`) e `engine/certsource` (CA P-256 por spoke em memória, emissão de leaf a
   partir de CSR, factory `self-signed`/`ca://`), mais `engine/pki` (`GenerateBankCSR`).
   Chaves privadas nunca em manifesto/estado/bundle; produção é stub.
3. **TK-B4** — templates de compose parametrizados (hub + entity-besu + entity-* + relay + NOC).
   **[Implementado]** — 8 templates sob `scenario-b/provisioning/templates/` (+ `vars/*.env.example`
   e `NAMING.md`), com valores discriminantes por env, named volumes determinísticos (exceção
   `pki/`), portas por offset e cross-stack; validados pelo pacote `engine/composetemplate`
   (interpolação, sem segredos, named volumes, sem colisão) e por `docker compose config`. O
   `entity-besu` foi acrescentado ao resolver o achado C1. `deploy/local` intocado.
4. **TK-B5** — **relay generalizado**: boot neutro (≥0 spokes) + registro em runtime
   (`POST /api/v1/spokes`) + conectores/watchers e roteamento por spoke (§14.B/§14.D). Vem antes
   do `found-hub`/`found-spoke` porque ambos dependem dele. **[Implementado]** — relay Cacti
   generalizado (registry dinâmico, RelayStore JSON persistido, `POST /api/v1/spokes` idempotente,
   roteamento por `spoke_out`, gate `isPaused` on-chain via `amm_address` com falha segura) +
   interface Go `engine/relayregistrar` (local in-memory + stub HTTP + factory por URI). Auth-por-CB
   (§14.D) permanece fora de escopo (segredo compartilhado como fallback). Lógica coberta por
   `node:test` (24) e `go test` (4).
5. **TK-B6** — motor de orquestração + steps `found-hub` (`start-relay`, `start-noc`, Keycloak
   write-back) + **hub bundle emitter** + comando **`apply`** (CLI sobre o motor). **[Implementado]**
   — motor idempotente (`engine/orchestrator`: Step/Check-Run, estado YAML, `flock`, dry-run, report),
   executor injetável (`engine/exec`), steps do `found-hub` (build → **gen-genesis-hub** (QBFT via
   `besu operator generate-blockchain-config`) → besu-hub → deploy via `CBWeb3Hub.s.sol` → keycloak +
   write-back → render-env → infra/backend/frontend → relay → noc → emit), `engine/addrs`
   (broadcast Foundry), `engine/bundle` (hub bundle público) e o subcomando
   `apply -f … [--dry-run] [-o json|yaml]`. Unidade com FakeRunner; suíte E2E sob build tag `e2e`
   (skip-com-aviso). Keycloak mantido (governança + NOC + gate IV).
6. **TK-B7** — `found-spoke` (`register-cb` + contratos de spoke + Keycloak write-back +
   `register-relay-spoke` + `add-noc-agent`) + **spoke bundle emitter**. **[Implementado]** — modo
   `found-spoke` no motor: consume-hub-bundle → register-cb (registerParticipant + grantLiquidityProvider
   automático, idempotentes) → gen-genesis-spoke → start-besu-spoke (captura enode) → deploy via
   `CBWeb3Spoke.s.sol` → wire-hub-addresses → keycloak (+write-back) → infra/backend/frontend →
   register-relay-spoke (via `RelayRegistrar`) → add-noc-agent (**soft**) → emit-spoke-bundle
   (`engine/bundle.SpokeBundle`: genesis + enode + endereços). Extensões do motor: `Step.Soft` +
   captura de enode (`admin_nodeInfo`). Unidade com FakeRunner; suíte E2E sob build tag `e2e`. Par
   soberano/liquidez/oracle e auth-por-CB permanecem fora de escopo (TK-B9/§14.D).
7. **TK-B8** — `join` (full node não-validador). **[Implementado]** — modo `join` no motor (fluxo
   canônico §6): consume-spoke-bundle → write-genesis (copia o genesis do bundle; guard não-destrutivo
   + checagem `sha256`, **não** regenera) → start-besu-join (full node **não-validador**: o genesis
   CB-only garante que o banco não entra no validator set QBFT) → wait-sync (`eth_syncing` até
   sincronizar) → wire-addresses (endereços de spoke do bundle) → provision-keycloak-bank (+write-back)
   → render/infra/backend/frontend → **gen-csr** (cauda diferida; o **único** step de PKI: par de
   chaves + CSR local via `pki.GenerateBankCSR`, chave `0600`, nunca transmitida, **zero** material de
   CA; pré-cria `pki/` como usuário do host). Assinatura do CSR, emissão gated por KYC e registro
   on-chain permanecem **runtime**. Extensão de motor: gate `wait-sync` (`eth_syncing`). **Sem** step
   de relay nem de noc-agent (a cadeia já é observada desde o `found-spoke`). `vote-qbft` (promoção a
   validador) permanece capacidade diferida (ADR-002). Unidade com FakeRunner; suíte E2E sob build tag
   `e2e`.
8. **TK-B9** — par soberano + liquidez cooperativa (commit-reveal) + taxa do oráculo.
   **[Implementado como fluxo de runtime; removido do provisionamento]** — a cauda soft
   (`open-sovereign-pair` / `commit-liquidity` / `seed-oracle`) disparada por `spec.pair` foi
   **retirada** do `found-spoke` no commit `c90de691`, junto com o próprio campo `spec.pair`. O
   `found-spoke` passou a registrar o CB e a sua moeda soberana no hub pela API, e nada mais.
   Abrir um corredor é hoje uma sequência de **atos de runtime**, cada um assinado pelo seu
   próprio CB: `proposePair` pelo portal de governança do CB proponente, `confirmPair` pelo da
   contraparte, e depois `registerCommit` por cada lado (o relay casa `CommitMatched`); em
   `local`, o `setRate` no `ManualOracle` fecha o ciclo. A motivação continua sendo a
   **soberania estrita** — nenhum run detém a chave da contraparte, e por isso o
   `SeedNewSovereignPair.s.sol` (exige ambas as chaves) nunca foi reusado. `apply` não planeja
   nenhum desses steps, e `TestApplyFoundSpokeHasNoSovereignTail` trava essa garantia. O fluxo
   de runtime é exercitado por `tests/e2e/sovereign_pair_e2e_test.go` sob o build tag `e2e`.
9. **TK-B10** — E2E + baseline. **[Implementado]** — fase de verificação (testes + doc): um **E2E de
   pipeline completo** (`tests/e2e/pipeline_e2e_test.go`, tag `e2e`) que compõe os modos via
   `apply.Apply` (found-hub → found-spoke ×2 → join → cauda soberana) e exercita o caminho de negócio —
   **swap** (`swapTokensForExactTokens`), **circuit breaker** (`pause`/`signResume` quorum 2/`isPaused`)
   e **`SpokeBridge`** (`lock`/`release`) — mais **idempotência** (re-`apply` converge). Sub-testes
   skip-com-aviso: `TestPipeline_HubMint` (mint relay-mediado, poll de saldo) e `TestPipeline_BridgeRefund`
   (`release` como refund; não há timeout on-chain). **Baseline toolkit-native** em Go
   (`tests/perf/baseline_test.go`, tag `perf`): p95 de quote/swap (informativo, sem gate). Doc
   **`toolkit/E2E-STATUS.md`** (espelho do Cenário A). Todo E2E/baseline faz **skip-com-aviso** sem
   ambiente (nunca falso verde). Sem novos contratos/deps; não altera Makefiles/`deploy/local`.

---

## 13. Decisões, questões em aberto e pontos de atenção

### Recomendações

- **Motor: duplicar vs. lib compartilhada.** *Recomendação: duplicar* o esqueleto do motor
  em `scenario-b/toolkit`. É compatível com o isolamento de cenário sem invocar a exceção
  da "lib compartilhada versionada", condiz com a duplicação histórica do projeto e evita
  acoplar a evolução dos dois toolkits. A extração de uma lib compartilhada fica como
  refactor posterior, se e quando os dois convergirem.
- **Circuit breaker e steps bilaterais** são o maior risco de design (coordenação entre
  dois manifestos de CB). *Recomendação: um spike dedicado* antes de codificar esses steps.

### Decisões tomadas

- **Acesso ao hub — RPC-only.** As entidades (CBs e bancos) não rodam nó no hub; os backends
  alcançam o hub por RPC ao `hub-validator` (:8845). O hub roda apenas o `hub-validator`
  (eventualmente dois). Consequências: o toolkit não usa o `hub-besu/addNewNode.sh` e o hub
  bundle dispensa enode/genesis. (O `deploy/local` atual pré-inicia 5 nós no hub; o toolkit
  diverge deliberadamente.)
- **HTLC fora do escopo deste plano.** O código atual implanta HTLC no hub e no spoke (pernas
  de Scenario A), mas o toolkit não o provisiona neste plano; reabrir caso o escopo mude.
- **N spokes dinâmicos (sem `spoke-a`/`spoke-b` fixos).** Cada `found-spoke` funda um spoke
  novo a partir do manifesto (`spoke.id`/`chainId`/`currency`); o toolkit suporta N spokes
  (Brasil, Argentina, Colômbia, …) sem conjunto fixo. O `CBWeb3Spoke.s.sol` já é parametrizado
  por env e está pronto para isso; o que hoje está fixo em 2 no `deploy/local` e precisa ser
  gerado dinamicamente está detalhado no item 12 dos pontos de atenção.
- **PKI: single-tier, CB-issued (alinhado ao Cenário A).** O banco central fundador é a **CA
  do seu spoke** e assina todo CSR de banco comercial; o banco detém a própria chave e recebe o
  leaf (`OU=ROLE_COMMERCIAL_BANK`). Não há CA por banco no fluxo de produção. O cert da CA do CB
  é a **âncora de confiança do spoke** e vai no spoke bundle. Verificado no código do B (mesmo
  fluxo do A): `auth/.../onboarding.go` valida o CSR e chama `compliance.SignParticipantCSR`;
  `cbclient` faz POST em `/api/v1/onboarding/credential-request`; o login PKI é desafio-resposta
  de nonce contra o cert. Local: raiz self-signed; prod: CA do CB em PKI real — mesmo modelo. O
  `make/05-pki.mk` (CA por banco self-signed) é **atalho de bootstrap local**, não o fluxo de
  produção. Detalhado na seção 15.
- **Abertura de par soberano: fluxo governança-mediado, descoberto on-chain (opção B).** A
  coordenação bilateral usa a cadeia do hub como substrato — a máquina de estados
  `PROPOSED→ACTIVE` do `PairRegistry` + eventos `PairProposed`/`PairRegistered` — e reusa o
  primitivo request→approve→gated do onboarding. Escolhas adotadas: `corridor-request` como
  **API no gateway do hub**; **gate forte de governança no CB-B + gate leve no hub**;
  **W-token por moeda, reutilizável** entre corredores (não por par — reduz o N² da §14.A; hoje
  o `SeedNewSovereignPair` deploya por par, então o toolkit deve **deduplicar** por moeda);
  **`pairId` determinístico** (ex.: `W-BRL-ARS`). Fluxo detalhado em §6 (Steps bilaterais).
- **Registro/admissão de CB no hub: no `found-spoke`, auto-registro (agora).** O registro do CB
  no `IdentityRegistry` do hub (e os grants de LP/roles) acontece no **`found-spoke`** (step
  `register-cb`), dirigido por solicitação e **idempotente** (registra se ainda não houver) —
  **não** é pré-carregado no `found-hub`, que não conhece os endereços dos CBs de antemão. Por
  ora a admissão é **automática** (sem gate). O `DEFAULT_ADMIN_ROLE`/`GOVERNANCE_ROLE` do hub é
  do **operador neutro do hub**. *Nota futura:* a governança do hub poderá **autorizar** a
  admissão (gate de aprovação), como o KYC do onboarding de banco — flip de automático para
  gated, sem mudança de contrato (o `registerParticipant` já é `GOVERNANCE_ROLE`-gated).
- **Banco no spoke = full node não-validador; CB = validador único (modelo do Cenário A).** No
  `found-spoke` o CB é o único validador QBFT (genesis `count: 1`); no `join` o banco sincroniza
  como full node e **não** produz blocos — sem `vote-qbft` no fluxo canônico. A promoção a
  validador fica como **capacidade diferida** (`qbft_proposeValidatorVote` na rede viva, ADR-002
  do A), usada só se a governança decidir adicionar um validador. Diverge do `deploy/local`
  atual do B (spoke com 2 validadores).
- **Circuit breaker com N — política (opção A no toolkit; opção C como correção futura).** Para
  o **escopo do toolkit** fica a **opção A**: o toolkit só registra o conjunto de governança
  correto (os CBs) no `IdentityRegistry` do hub e garante que o relay valide `isPaused` antes de
  swaps (§9 item 3) — **sem mudança na lógica do breaker**. Hoje o breaker é global: pause 1-de-N
  por qualquer CB (mesmo de corredor alheio) e resume por 2 CBs quaisquer (`RESUME_QUORUM` fixo
  em 2). Como **correção futura de contrato** (fora do escopo do toolkit) fica recomendada a
  **opção C**: pause amplo (emergência sistêmica, incl. o operador do hub) + resume restrito aos
  **dois CBs do par** — coerente com a soberania da opção B e tecnicamente limpo (a AMM já tem
  `TOKEN_A/B` + registry para escopar). Ver §14.A.
- **Relay: registro de spokes em runtime (não config+restart).** Um spoke novo é adicionado via
  `POST /api/v1/spokes` (o `found-spoke` auto-registra pelo step `register-relay-spoke`): o relay
  cria connector/watcher + o mapa de roteamento **sem reiniciar** — evita janela de manutenção no
  relay neutro compartilhado e encaixa no modelo declarativo N-spokes. Requer **registro
  persistido** de spokes (RelayStore, `031-relay-spoke-registry`) para sobreviver a reinícios do
  relay, e **auth** (política em aberto — §14.B (5)). MVP possível: config-dinâmica + restart,
  evoluindo para o runtime.
- **Topologia de FX: malha bilateral opt-in (A).** Cada corredor é uma AMM direta entre duas
  moedas, criada só quando os dois CBs concordam (opção B do par). **Não privilegia nenhuma
  moeda** — coerente com o hub neutro + soberania. Aceita-se o custo N² (até N·(N−1)/2 pools) e
  a fragmentação de liquidez como preço da neutralidade, **mitigado por ser opt-in** (só existem
  corredores acordados). *Evolução futura de escala:* roteamento mediado por **numéraire**
  (N pools moeda↔base, liquidez concentrada) — introduz moeda privilegiada + swap multi-hop
  (feature nova no `PairRouter`/relay); é decisão de produto/governança, fora do escopo do
  toolkit.
- **chainId atribuído no manifesto (não auto-atribuído).** O `spec.spoke.chainId` (e a base de
  portas do nó) é **informado pelo operador** no manifesto; o toolkit **valida unicidade e
  colisão** (chainId/porta/rede) e não auto-atribui — alinha com o toolkit de referência e
  permite coexistir com os IDs legados (1337/1338/1339).
- **AMM base (BRL/EUR): remover (legado).** Sob a malha opt-in o hub não precisa de um par
  base — cada corredor traz a sua própria AMM + tokens `W-tCeBM`. Decide-se **remover a AMM
  base**; o `found-hub` do toolkit **não a provisiona**. Os tokens base `tCeBM_BRL/EUR`
  permanecem (camada de reserva do hub — `tCeBM` é reserve-layer). A remoção efetiva da AMM é
  **mudança de contrato** (`CBWeb3Hub.s.sol` + testes) — tarefa de contrato fora do escopo
  inicial do toolkit (não-objetivos, §1).

### Pontos de atenção do Cenário B a contemplar no toolkit

Aspectos próprios do Cenário B que o toolkit precisa tratar (verificados na fonte).

**Alto impacto — determinam o modelo de manifesto/modos**

1. **[Revisão futura] Camada de privacidade (Zeto/Noto/Paladin) — decisão de escopo.** O
   Cenário B, hoje, não possui `ZetoToken`/`NotoToken` em `contracts/src`; a transferência de
   valor usa tCeBM/fCeBM/AMM (o `CLAUDE.md` cita esses tokens no inventário, mas está
   desatualizado). *A reconciliar nesta revisão:* existem referências **antecipatórias** a
   Paladin/Zeto/Pente nos serviços Go (`payment-orchestrator`, `noc-*`, `compliance`) — a frase
   "sem integração Paladin no Go" usada em §6/§15 era imprecisa. **Deixado como está por ora**
   (a pedido); definir com a equipe, em revisão futura, se a camada de privacidade entra no
   escopo do toolkit.
2. **Banco como validador — decidido (ver Decisões tomadas).** O banco entra como **full node
   não-validador**; o **CB é o validador único** do spoke; `vote-qbft` é **capacidade
   diferida** (promoção só se a governança decidir). Modelo do Cenário A.

**Impacto médio — subsistemas e steps a cobrir**

3. **Liquidez cooperativa (commit-reveal).** `seed-hub` está fora do fluxo default; a
   liquidez da AMM entra por commit-reveal cooperativo (`LiquidityCommitRegistry`, relay
   casando `CommitMatched`), bilateral e dependente do relay no ar — design central do B.
4. **Ciclo de vida do relay.** O relay é infra neutra compartilhada; precisa estar no ar
   antes do seeding cooperativo e dos swaps. Definido como step `start-relay` no `found-hub`.
5. **NOC — posicionamento nos modos (decidido).** O **`found-hub`** sobe o NOC neutro
   (`start-noc`: `noc-db`/`noc-backend`/`noc-portal` + `noc-agent-hub`); cada **`found-spoke`**
   roda o step **`add-noc-agent`** (soft) que sobe um `noc-agent` ligado ao Besu do spoke e o
   registra no `noc-backend`. Amarrado ao inventário do §6.
6. **[Ajuste futuro] Oracle de FX (baixo impacto).** `mock-fx-feeder` alimenta o `ManualOracle`;
   sem taxas os swaps falham. O essencial — um `setRate` inicial (**seed one-shot**) na abertura
   do par — fica coberto pelo step `seed-oracle` (soft, local-only, no `found-spoke`). A escolha
   **seed único vs. serviço feeder contínuo** (jitter) é detalhe **só de `local`**, sem impacto
   em arquitetura/modos/manifesto/contratos → **ajuste futuro** (seed único vs. serviço feeder,
   só de `local`).
7. **Keycloak bootstrap + write-back de secrets.** `init.sh` provisiona realms/clients e
   escreve os client secrets JWT de volta nos `.env.infra`, além do `init-multi-db.sh` criar
   um DB por entidade — fluxos que o toolkit precisa reproduzir (no `found-hub` e no
   `found-spoke`/`join`).
8. **[Revisão futura] MLP (Market Liquidity Provider).** Role/participante opcional
   (`ENABLE_MLP`, `docker-compose-backend.mlp.yaml`, DB `cbweb3_mlp`, grant de role MLP).
   Ausente do modelo. **Decidir em revisão futura** se entra no escopo do toolkit.
9. **Role on-chain do relayer.** `SeedNewSovereignPair` concede `CENTRAL_BANK_ROLE` ao
   endereço do relayer nos tokens soberanos. Com a remoção da cauda soberana do
   provisionamento (seção 6), esse grant é hoje um ato de **runtime** do CB dono do token —
   mantido aqui como item de checklist, sem step de toolkit correspondente.

**Impacto menor / precisão**

10. **Serviços placeholder.** `fx`, `ledger-gateway`, `payments` têm 0 arquivos `.go`. O
    toolkit provisiona só os implementados (`api-gateway`, `auth`, `compliance`,
    `payment-orchestrator`, `noc-agent`, `noc-backend`).
11. **Mapa frontend → role.** Apps: `bank`, `governance`, `supervisor`, `treasury`, `noc`.
    Falta mapear qual app sobe por role.
12. **N spokes: o que está pronto vs. fixo (verificado no código).** *Pronto para N:* o
    `CBWeb3Spoke.s.sol` é totalmente parametrizado por env (`CENTRAL_BANK_ADDRESS`,
    `TOKEN_NAME/SYMBOL`, `FIAT_*`, chave do deployer) — deploya qualquer spoke. *Fixo em 2
    spokes / 4 entidades hoje:* os diretórios `deploy/local/spoke-besu-a|b` (cada um com
    `configTemplate.json` de `chainId` fixo — 1338/1339); as vars/targets de Makefile
    (`SPOKE_A_ENTITIES`/`SPOKE_B_ENTITIES`, `deploy.up-spoke-a|b`); os alvos de PKI por
    entidade; o `postgres/init-multi-db.sh` (conjunto fixo de DBs); os scripts de Keycloak
    (casos enumerados por entidade); os compose de backend por entidade; e o relay `config.ts`
    (`spokeA`/`spokeB`). O toolkit precisa **gerar esses artefatos por spoke a partir do
    manifesto** (chainId/portas/rede/DB/realm/CA determinísticos, sem colisão) — é o papel dos
    templates (seção 7) e da generalização do relay (seção 9, item 1). Consolidado na seção 14.
13. **Genesis alloc + zeroBaseFee.** O toolkit gera o genesis; precisa decidir quais
    endereços por entidade são fundados no `alloc` e preservar `zeroBaseFee`/zero-gas. Hoje as
    12 chaves do `alloc` são **idênticas** nas três redes — gerar chaves únicas por spoke (ver
    seção 14.C).
14. **Idempotência de steps on-chain.** O `Check()` dos steps bilaterais/de liquidez deve
    consultar estado on-chain (status do `PairRegistry`, commits casados).
15. **Orquestração multi-manifesto.** O bring-up completo (hub → 2 CBs → bancos) precisa de
    um wrapper/sample e de ordem cross-manifesto (`confirmPair` do CB-B depende do CB-A já
    existir).

**Complementos identificados (constituição / PKI)**

16. **PKI e onboarding — ver seção 15.** Modelo decidido (single-tier, CB-issued) e fluxo
    CSR→CB-assina detalhados na **seção 15**. Em resumo: o toolkit gera/segura a CA do CB no
    `found-spoke` e a publica no spoke bundle; o `join` executa a cauda de onboarding; e o
    `make/05-pki.mk` (CA por banco) é apenas local. O exemplo de `join`
    (`certSource: ca://central-bank-a`) está, portanto, correto.
17. **Gate de compliance no API gateway.** A constituição exige `IdentityRegistry` + serviço
    de compliance + Keycloak OIDC no gateway, sem bypass em chamadas serviço-a-serviço, e
    re-checado na iniciação de pagamento (não só no onboarding). Os steps de backend precisam
    fiar essa cadeia; o roteiro cita hoje apenas o Keycloak.
18. **Observabilidade (Princípio VI).** Logs JSON estruturados (request id, serviço,
    severidade, ts ISO-8601) e contexto de trace OTel entre gRPC; o relay deve logar o ciclo
    completo de settlement. Os templates de compose precisam fiar essa configuração.

---

## 14. Suporte a N spokes dinâmicos

**Veredito.** Fundar um spoke novo (Brasil → Argentina → Colômbia → …) é uma operação de
**runtime**, não um redeploy. A camada de contratos do hub já é N-ready; o que está fixo em
2 spokes é a **stack local** (Besu/Makefile/Postgres/Keycloak/backend/PKI/frontend) e o
**relay Cacti**. Análise verificada na fonte, em três frentes.

### 14.A Hub — sem redeploy; onboarding por transações

Adicionar um spoke/moeda **não exige redeploy do hub**. Contratos N-ready (verificados):

- `IdentityRegistry.registerParticipant` — qualquer nº de CBs/bancos (gated por `GOVERNANCE_ROLE`).
- `CurrencyRegistry.registerCurrency` — cada CB registra a própria moeda (auth via `getCentralBankOf`); sem BRL/EUR/ARS fixos.
- `PairRegistry` — `proposePair`/`confirmPair` bilateral geral, nº de pares ilimitado.
- `LiquidityCommitRegistry` — **instância única compartilhada**, keyed por par; serve N pares.
- `ManualOracle` — rates por par (chave order-sensitive); novos pares a qualquer momento.
- `AutomatedMarketMaker` — uma AMM por par.

O `CBWeb3Hub.s.sol` só faz o **bootstrap** BRL/EUR; novas moedas entram por
`SeedNewSovereignPair`, sem re-executar o hub. Checklist por novo spoke (tudo runtime):
registrar o CB no `IdentityRegistry` do hub → deploy dos `W-tCeBM` → `setCentralBankOf` →
`registerCurrency` → abrir corredor (deploy da AMM do par + `proposePair`/`confirmPair`) →
reusar o `LiquidityCommitRegistry` compartilhado → `grantRole(CENTRAL_BANK_ROLE)` ao novo CB
no oracle + `setRate` → grants (LP + relayer nos `W-tokens`) → estender o
`SOVEREIGN_PAIR_AMM_MAP` do gateway do CB (o `PairRouter` é event-driven, sem redeploy).

Dois pontos de código a parametrizar no onboarding (verificados): o
`contracts/script/RegisterParticipants.s.sol` é um script dev com **5 endereços de genesis
embutidos** (CB-A `0xfe3b55…`, CB-B `0xf17f52…`, Bank-A/B `0xC5fdf4…`, Bank-C/D `0x821aEa…`) —
precisa de um caminho parametrizado que registre qualquer CB/banco por endereço/papel injetado;
e, no gateway do CB, o `HubLiquidityConfigFromEnv` traz default fixo `W-BRL-ARS` e
`SOVEREIGN_HUB_TOKEN_A/B` de par único — devem ser preenchidos por CB/par (o `PairRouter` em si
já é event-driven e N-ready).

Duas considerações de design a alinhar:

- **Escala de pares (decidido: malha opt-in):** cada par = uma AMM dedicada; uma malha completa
  de N moedas dá até **N·(N−1)/2 AMMs** (fragmentação de liquidez + crescimento de estado).
  **Decisão:** malha bilateral **opt-in** (só existe corredor acordado) — ver Decisões tomadas
  (§13); roteamento por numéraire fica como evolução futura de escala.
- **Governança do breaker com N (decidido):** hoje `canGovern()` é global ⇒ pause 1-de-N por
  qualquer CB (mesmo de corredor alheio) e resume por 2 CBs quaisquer (`RESUME_QUORUM` fixo em
  2). **Decisão:** o toolkit segue a **opção A** (só registra a governança + relay valida
  `isPaused`, sem mudar contrato); a **opção C** (pause amplo + resume restrito aos dois CBs do
  par) fica como **correção futura de contrato**, fora do escopo do toolkit. Ver Decisões
  tomadas (§13).

A decisão RPC-only não é afetada por N (o hub segue um único nó).

### 14.B Relay Cacti — precisa de generalização (bloqueador)

Já N-agnostic: o watcher observa **uma** `LiquidityCommitRegistry` no hub e faz **broadcast**
do `CommitMatched` para `GATEWAY_INTERNAL_URLS` (lista), com filtragem no gateway.

Fixo em 2 (bloqueadores):

- `src/config.ts` define objeto fixo `{ spokeA, spokeB }` com `SPOKE_A/B_BESU_RPC` **fatais** se ausentes.
- `src/index.ts` instancia 2 conectores/watchers literais.
- `src/cross-currency-swap-relay.ts` roteia **sempre** para um `CB_B_GATEWAY_URL` fixo —
  `spoke_out`/`beneficiary_bank_id` são validados mas **não usados** no roteamento.

Como quebra hoje: com **1 spoke** o relay não sobe (`requireEnv(SPOKE_B_BESU_RPC)`); com **3
spokes** sobe mas o 3º é invisível e um bridge-out para ele é mal-roteado para o CB-B.

Mudanças necessárias: (1) `config.ts` → **registry dinâmico** de spokes (lista/map, ≥1); (2)
conectores/watchers em **loop** (`Map<spokeId>`); (3) roteamento cross-currency por `spoke_out`
→ **lookup de gateway por spoke id**; (4) **decidido** — endpoint de **registro em runtime**
(`POST /api/v1/spokes`): o `found-spoke` auto-registra o spoke (via `register-relay-spoke`) e o
relay cria connector/watcher + entrada de roteamento **sem restart**; exige um **registro
persistido** de spokes (RelayStore de verdade, feature `031-relay-spoke-registry`) para
sobreviver a reinícios, e auth (item 5). O `cacti-relay-store.json` atual é legado/plano e não
serve; (5) **auth por CB** (recomendação detalhada em §14.D — pendente do project-lead; hoje segredo único compartilhado — sensível
a compliance, requer aprovação de lead); (6) `env-sample`/`docker-compose.yaml` dinâmicos. O
store JSON é legado e não é usado.

### 14.C Stack local — gerar tudo por spoke (hoje sem alocação determinística)

Hoje cada `spoke-a`/`spoke-b` é **constante copiada** — não há fórmula. Bandas de porta
hand-assigned (spoke-a 86xx, spoke-b 87xx, hub 88xx): um **spoke-3 colidiria com o hub**. As
portas de backend quebram o padrão (CB-b salta para 60xxx; `REDIS_DB` 0,1,2,**5**).

**Crítico:** as 12 chaves privadas do `alloc` do genesis são **idênticas** nos três templates
(hub/spoke-a/spoke-b); o gerador de spokes precisa emitir **chaves únicas por spoke/entidade**
(correção de correção/segurança; conecta-se ao trabalho de chave-por-banco já validado).

Checklist que o toolkit gera dinamicamente por spoke, a partir do manifesto: `chainId` único;
genesis + node keys únicas; portas RPC/WS/P2P sem colisão; rede `spoke_<id>_besu_network`;
captura+rewrite do enode + `--bootnodes`; cross-connect a `cbweb3_network`; **estado do nó em
named volumes** (hoje o `startBesu.sh` usa bind `nodes/<entidade>/data` — mover para volumes,
ver seção 7); (Paladin, se em escopo). E, por entidade: DB Postgres, realm/client Keycloak
(+write-back, +client de
supervisor no CB), `.env.infra`, compose de backend, CA/leaf de PKI, índice Redis, portal(is)
de frontend, e as entradas de rede/DNS/container.

**Reuso do toolkit de referência:** o schema de manifesto (`chainId`, `node.rpc.port`), o motor
de **offset de portas** (`ports.go`: base + banda por serviço), os helpers de nome de
rede/container e os steps `render-*-env` — exatamente o que substitui os `startBesu.sh` e
`.env.infra` copiados por geração determinística.

Código fixo adicional identificado (a gerar por spoke, verificado na fonte):

- **NOC:** um `noc-agent` por rede de spoke — hoje `noc-agent-{hub,spoke-a,spoke-b}` fixos em
  `deploy/local/compose.noc.yml`, cada um ligado a uma rede Besu fixa e a um
  `interop/hub-and-spoke/noc/agent-configs/<spoke>/agent.yaml` próprio; gerar agente + config
  por spoke.
- **Paladin (se em escopo):** `deploy/local/paladin/render-configs.sh` define o set de nós por
  spoke num `case` (`spoke-b) NODES="central-bank bank-b bank-d"`, default `bank-a bank-c`);
  gerar por spoke.
- **Propagação de endereços:** o `sync-contracts.sh` mapeia spoke→entidades de forma fixa
  (spoke-a → bank-a/bank-c/central-bank-a; spoke-b → bank-b/bank-d/central-bank-b), com caminhos
  `.env.infra.<entidade>` enumerados; no toolkit isso vira propagação **por bundle** (seção 8),
  gerada por spoke.
- **chainId:** no modelo declarativo vem do manifesto (`spec.spoke.chainId`, atribuído pelo
  operador); o toolkit **valida unicidade e colisão de portas** em vez de auto-atribuir — como
  no toolkit de referência (**decidido** — ver §13).

### 14.D Autenticação por CB no relay (recomendação — pendente do project-lead)

> **Status:** recomendação documentada para **implementação posterior**. É sensível a
> compliance (a constituição exige aprovação do **project-lead** para mudar controles de
> segurança), então **não implementar** antes do sign-off. O segredo compartilhado permanece
> como fallback de `local`/dev.

**Estado atual (verificado).** O relay autentica com **um único segredo compartilhado**,
`INTERNAL_RELAY_AUTH_SECRET`, no header `X-Relay-Auth`, nos dois sentidos (o
`cross-currency-swap-relay` valida na entrada do bridge-out; o watcher e o cross-currency o
enviam ao notificar gateways). Ambos recusam subir sem ele. Não há credencial nem assinatura
por CB.

**Problema com N CBs.** Todos compartilham o mesmo segredo ⇒ (a) um vazamento em qualquer CB
permite **impersonar** qualquer outro; (b) o relay **não distingue** qual CB enviou a
requisição (sem responsabilização/auditoria — contra o Princípio VI); (c) sem não-repúdio. É um
caminho de valor (bridge-out entre CBs soberanos).

**Recomendação: auth por CB via assinatura ancorada no `IdentityRegistry` do hub.**

- **Âncora de identidade = on-chain.** O relay **não** é autoridade de identidade própria: reusa
  o `IdentityRegistry` do hub, onde cada CB já está registrado como `CENTRAL_BANK` com o seu
  endereço EVM (admitido pela **governança do hub** no `register-cb`). Essa é a fonte de verdade
  de "quem é o CB-1".
- **Enrollment (no `register-relay-spoke` / `POST /api/v1/spokes`):** o CB assina um **nonce**
  com a sua chave por-CB (§10); o relay **recupera o endereço** da assinatura e **checa on-chain**
  (`canGovern`/`getCentralBankOf`) que é um CB registrado; se confere, amarra `cbId → pubkey` no
  **registro persistido** (RelayStore de verdade — `031-relay-spoke-registry`; o
  `cacti-relay-store.json` atual é legado/plano e não serve).
- **Por requisição:** o CB assina; o relay verifica contra o pubkey amarrado (ou recupera o
  endereço e re-checa on-chain). **CB-2 não consegue** forjar a assinatura sob a chave de CB-1,
  nem sequestrar o slot de CB-1 (o registro exige provar controle do endereço on-chain, que só
  CB-1 controla).
- **Revogação:** remover/rebaixar o CB no `IdentityRegistry` faz o relay passar a rejeitá-lo.

**Componentes tocados (implicação — transborda o toolkit):**

- **Relay** (`interop/hub-and-spoke/cacti`): troca o check do `X-Relay-Auth` por **verificação
  de assinatura**; **leitura on-chain** do `IdentityRegistry` (o relay já conecta ao hub para o
  LCR — incremental); lógica de enrollment; **registro persistido** de spokes.
- **Backend chamador** (`payment-orchestrator`/`api-gateway`): passa a **assinar** as requisições
  ao relay — muda serviço de runtime, não só provisionamento.
- **Toolkit:** o `register-relay-spoke` vira **enrollment autenticado** usando a chave por-CB
  (§10) — sem credencial nova, só fiação.
- **Ordenação:** `register-cb` (on-chain) **antes** do `register-relay-spoke` — já é a ordem do
  `found-spoke`.

**Mecanismo (a fixar na implementação):** chave blockchain (secp256k1) + registro on-chain
(recomendado, reusa a identidade que já gate `proposePair`/`canGovern`) **ou** cert PKI X.509 do
CB (§15) como alternativa. **Fallback:** segredo compartilhado em `local` (relay suporta os dois
modos via flag), evitando cutover duro.

---

## 15. PKI e onboarding de identidade

Modelo **decidido: single-tier, CB-issued**, alinhado ao Cenário A e **já implementado no
código do Cenário B**. Esta seção consolida o que os modos `found-spoke`/`join`, os bundles e
o `CertSource` precisam fazer.

### 15.1 Duas identidades por participante

- **TLS/PKI** — cert X.509 ECDSA P-256. Usado no **login PKI** do api-gateway (desafio-resposta
  de nonce: o participante assina o nonce, o gateway valida contra o pubkey do cert —
  `auth/.../server.go`, `pki.ValidateSignature`) e, opcional/scaffolded, mTLS via `CA_CERT_FILE`.
- **Blockchain** — chave secp256k1 (gerida pelo `KeyProvider`/KMS) cujo endereço EVM é
  registrado on-chain no `IdentityRegistry` (whitelist: papel + status `Verified`). No Cenário A
  o papel equivalente é o `PARTICIPANT_REGISTRY_ADDRESS`; no B é o `IdentityRegistry`.

### 15.2 Raiz de confiança: o CB é a CA do seu spoke

- O banco central fundador detém a CA do spoke (`central-bank-<x>-ca.{crt,key}`) e assina
  **todo** CSR de banco comercial; o banco detém a própria chave e recebe só o leaf. **Não há
  CA por banco** no fluxo de produção.
- O **cert da CA do CB é a âncora de confiança do spoke** e viaja no **spoke bundle** (seção 5);
  o `join` o consome para confiar na cadeia.
- **N spokes ⇒ N raízes independentes** — uma CA por spoke (Brasil, Argentina, Colômbia, …),
  sem raiz compartilhada; cada spoke bundle carrega a sua.
- Local: raiz **self-signed**. Prod: a CA do CB é ancorada em **PKI real** (CAs de banco
  central). Mesmo modelo, fonte de raiz diferente. A impl. de produção do `CertSource` é
  **deferida** (stub) — `environment: local` primeiro, como o resto do toolkit (ver seções 4 e 10).

### 15.3 Fluxo de onboarding (CSR → o CB assina) — verificado no código

1. O banco gera **par de chaves + CSR** (`OU=ROLE_COMMERCIAL_BANK`); detém a chave privada.
2. O gateway anexa o CSR + o pubkey blockchain (KMS) e faz `POST
   /api/v1/onboarding/credential-request` ao CB (`auth/.../cbclient/client.go`). O CB **valida a
   assinatura do CSR** (P-256) e persiste o participante como `CREDENTIAL_REQUESTED`
   (`auth/.../grpc/server/onboarding.go`).
3. O **compliance do CB assina o CSR** com a CA do spoke, emitindo o leaf e preservando o pubkey
   do participante (`SignParticipantCSR`; `complianceclient` → `SignedCSR`).
4. **Proof-of-possession:** o participante assina um nonce; o gateway valida contra o pubkey do
   cert e então **registra o endereço EVM on-chain** no `IdentityRegistry`.

Touchpoints: `api-gateway/.../handlers/{onboarding.go,onboarding_proxy.go}`;
`auth/.../grpc/server/{onboarding.go,onboarding_kms.go}`; `auth/.../complianceclient`.

### 15.4 `CertSource` no toolkit

- **`self-signed`** — o CB (ou o hub) atua como CA self-signed da própria rede; usado para
  **gerar a raiz do spoke** no `found-spoke` (e a do hub no `found-hub`). `IssueLeafCert` valida
  `OU=ROLE_COMMERCIAL_BANK` e assina; `GetTrustAnchor` devolve o cert da CA.
- **`ca://<cb-id>`** — no `join`, o cert do banco é emitido **assinando o CSR pela CA do CB**
  (fluxo 15.3). É o caminho de produção.
- **Sem segredos** no manifesto/estado/bundle: a chave da CA do CB nunca é serializada; o bundle
  carrega **apenas o cert** da CA.

### 15.5 Operações de PKI por modo

- **`found-hub`** — o hub é sua própria CA (TLS/transporte); não emite certs de banco.
- **`found-spoke`** — gera/segura a **CA do CB** (raiz do spoke), sobe o serviço de compliance
  (que assina CSRs) e o `cbEndpoint` (`credential-request`), e **publica o cert da CA no spoke
  bundle**.
- **`join`** — o toolkit faz **apenas `gen-csr`** (par de chaves + CSR local em `<dataDir>/pki/`,
  chave `0600`, **nunca transmitida**; pré-criar o dir `pki` como usuário do host antes do compose
  montá-lo — ver 15.6). O resto (`request-credential` → o CB assina via compliance → PoP →
  registro on-chain) é **runtime**, não step do toolkit.
- **Local-only:** o `make/05-pki.mk` (CA por banco self-signed) é atalho de bootstrap; o toolkit
  **não** o replica no caminho de produção (equivalente ao FIX-1 do plano do Cenário A).

### 15.6 Divisão toolkit × stack e armadilhas (padrão do Cenário A, encaixado no B)

**O PKI não força o toolkit a assinar nem registrar.** No Cenário A o toolkit tem escopo de PKI
mínimo e bem delimitado; a assinatura do CSR, a emissão gated por KYC e o registro on-chain ficam
no runtime. O mesmo se aplica ao B:

- **Toolkit (`found-spoke`):** gera a CA self-signed do CB e semeia o material TLS em **named
  volumes** (nunca no FS do host); publica **só o cert** da CA no spoke bundle
  (`spec.trust.caCertPEM`) + `cbEndpoint`; rejeita qualquer arquivo com `PRIVATE KEY`.
- **Toolkit (`join`):** **apenas `gen-csr`** (a chave privada fica em `<dataDir>/pki/{bank}.key`,
  `0600`, nunca transmitida). O toolkit **nunca** assina CSR de banco nem gera CA de banco.
- **Stack/runtime (NÃO o toolkit):** o compliance do CB assina o CSR (`SignParticipantCSR`); a
  emissão é **gated por aprovação KYC** (no B, o app `governance` + o serviço de compliance
  cumprem o papel do Governance Portal do A); o registro on-chain no `IdentityRegistry` é feito
  pelo signatário de governança do CB. Motivo de manter isso fora do toolkit: um CSR submetido
  pelo toolkit criaria um registro de participante keyed na wallet do toolkit em vez da wallet
  KMS de runtime do banco (colisão) — decisão explícita no Cenário A.

**Enforcement do `016-fix-pki-local-ca` (Track 2) a portar:** o caminho de produção do toolkit
emite certs **exclusivamente** via CSR → CB-assina; **nenhum** caminho gera par de CA para banco
comercial; **fail-fast** se a CA do CB estiver inacessível — **sem fallback self-signed**.
Garantia auditável: zero `*-ca.key`/`*-ca.crt` em artefato de join de banco.

**Armadilha a carregar:** qualquer step de usuário-host que escreve num dir montado por bind do
Docker precisa **pré-criar o dir como usuário do host** antes do compose montá-lo, ou o Docker o
cria root-owned e nega acesso (o `gen-csr` do A sofreu esse bug; corrigido pré-criando `pki/`).

**Deltas do B frente ao A (encaixe):**

- O B **não usa Paladin** — então as armadilhas de mTLS do Paladin do A (`CN` do cert = nome do
  nó, `PD030011`; seed de TLS `0644` para uid não-root, `PD020402`) **não se aplicam** ao B. O
  TLS do B é o cert X.509 do login PKI do api-gateway (+ mTLS scaffolded-off).
- O `gen-tls` do A semeava o cert de transporte do Paladin; no B o equivalente é só a **CA do
  CB** (para o compliance assinar) + o material TLS do gateway — mais simples.
- **Interfere no toolkit?** Só de forma delimitada: found-spoke gera a CA do CB + âncora no
  bundle; join só gera o CSR; o toolkit nunca assina/registra e faz fail-fast sem self-sign. A
  **stack do B não precisa de mudança de PKI** — o compliance já assina CSR (`SignParticipantCSR`).

---

## 16. Mapa de referência (repo)

**Referência de implementação (Cenário A — apenas para aprender o padrão do toolkit):**

- `scenario-a/toolkit/cmd/cbweb3/main.go`,
  `scenario-a/toolkit/engine/apply/apply.go`,
  `scenario-a/toolkit/engine/orchestrator/{step.go,orchestrator.go,state.go,deps.go,ports.go}`,
  `scenario-a/toolkit/engine/bundle/{types.go,bundle.go,load.go}`,
  `scenario-a/toolkit/E2E-STATUS.md`.
- Schema do manifesto de referência:
  `scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml`.
- Templates de compose de referência: `scenario-a/provisioning/templates/`.

**Cenário B (alvo do toolkit):**

- Constituição (v1.0.4): `.specify/memory/constitution.md`.
- Deploy: `scenario-b/Makefile`, `scenario-b/make/*.mk`, `scenario-b/deploy/local/`.
- Contratos: `scenario-b/contracts/src/`, `scenario-b/contracts/script/`.
- Backend: `scenario-b/backend/services/`.
- Relay: `scenario-b/interop/hub-and-spoke/cacti/`.
- Liquidez cooperativa: `scenario-b/docs/design/cooperative-liquidity.md`.
</content>
