# Paridade de guardas entre os cenários

Inventário das guardas do Scenario B, o que existe de equivalente no Scenario A, e o
que foi **verificado** em cada caso. Companheiro de [`scenario-drift.md`](scenario-drift.md),
que registra diferenças deliberadas; este documento registra diferenças de *proteção*.

Card de origem: *\[Scenario A/B\] Audit Scenario B's guards and port the missing ones to
Scenario A*.

---

## Por que este documento existe

Toda lacuna de guarda entre os cenários foi encontrada **à mão, depois do fato**. O card
lista quatro pares de PR em poucas semanas, cada um descoberto porque alguém reparou.
Este inventário existe para substituir isso por uma passagem única.

E, tão importante quanto: para registrar **como** cada guarda foi verificada. Uma guarda
que ninguém quebrou de propósito é uma guarda que ninguém sabe se funciona.

---

## Método, e por que o óbvio não funciona

A primeira tentativa foi comparar nomes de função de teste entre os dois toolkits. **Não
serve.** O toolkit de A tem 476 testes, o de B tem 356, e 335 dos nomes de B não existem
em A — não porque faltem guardas, mas porque são produtos diferentes: B tem hub, pares
soberanos e agentes NOC que A não tem.

Diff por nome mede semelhança de código, não cobertura de risco. O inventário abaixo é
por **guarda nomeada**: uma proteção contra uma falha específica, localizada pelo que ela
protege e não pelo nome que recebeu.

### Níveis de verificação

| Nível | Significa |
|---|---|
| **fonte** | a guarda existe e foi lida; ninguém a quebrou |
| **mutação** | o que ela protege foi quebrado de propósito e a guarda falhou |
| **ao vivo** | exercitada contra um stack em execução |

Só o nível *mutação* autoriza dizer que uma guarda guarda.

---

## Inventário — as sete guardas nomeadas no card

| Guarda | Scenario B | Scenario A | Situação |
|---|---|---|---|
| Limite de log de container | `composetemplate/log_policy_test.go` | `orchestrator/provisioning_log_policy_test.go` | **convergida** (caminhos diferentes) |
| Segredo em argv | `orchestrator/keycloak_secret_exposure_test.go` | `orchestrator/keycloak_secret_exposure_test.go` | **convergida** — o defeito estava presente em A e foi corrigido; ver abaixo |
| Deriva de literal de imagem | `orchestrator/image_pins_test.go` | `apply/image_pins_test.go` + `orchestrator/step_start_frontend_stack_test.go` | **convergida** — B tinha o pin copiado em quatro passos; ver abaixo |
| Pré-requisitos do Keycloak | `orchestrator/keycloak_provision_prereq_test.go` | — | **lacuna em A**, mas por um motivo estrutural — ver abaixo |
| Portas efêmeras | 4 testes | os mesmos 4 | **convergida** |
| Limite DNS de 63 caracteres | `apply/container_name_dns_test.go` + `orchestrator/proxy_dns_label_test.go` | os mesmos dois arquivos | **convergida** — ver abaixo, o port não foi linha a linha |
| Recusas codificadas | 3 arquivos | — | **lacuna em A**, com ressalva importante abaixo |

As linhas acima descrevem *onde* cada guarda está. Todas as sete foram desde então
verificadas por mutação — a tabela do passo 2, logo abaixo, é o que autoriza dizer que
elas guardam.

---

## Passo 2 — as sete verificadas por mutação

Cada linha quebra o que a guarda protege, restaurando o comportamento *anterior* à
correção, e confirma que a guarda falha. Todas conferidas por código de saída.

| Guarda | Mutação aplicada | Resultado |
|---|---|---|
| Limite de log de container | teto de log removido do template renderizado | **detectou** nos dois cenários |
| Portas efêmeras | porta fixa reintroduzida no range efêmero | **detectou** nos dois cenários |
| Limite DNS de 63 caracteres | nome de container acima do rótulo DNS | **detectou** nos dois cenários |
| Segredo em argv (B) | `kcadmLogin` volta a interpolar `--password <segredo>` | **detectou** — as duas asserções, nos três modos (`found-hub`, `found-spoke`, `join`) |
| Literal de imagem (A) | `hyperledger/besu:25.8.0` plantado em `engine/apply` | **detectou** |
| Pré-requisitos do Keycloak (B) | o caminho `join` deixa de emitir `update realms -s sslRequired=NONE` | **detectou**, e só o subteste `join` falhou — exatamente a regressão histórica |
| Recusas codificadas (B) | `trustRejectionCodes` reduzido a `RELAY_SIGNATURE_INVALID` | **detectou** (5 testes) — mas ver a ressalva abaixo |

Nenhuma das sete é decorativa. Isso responde à pergunta que precedia qualquer port:
copiar para A uma guarda de B não copia algo quebrado.

### Ressalva: a guarda de recusas codificadas é mais estreita do que aparenta

A mutação foi detectada por `__tests__/trust-errors.test.ts`. O outro arquivo,
`__tests__/auth.interceptor.test.ts`, **passou** com a classificação reduzida — ou seja,
o teste do interceptor não cobre os códigos irmãos, só o caminho principal. A guarda que
efetivamente guarda é a do classificador; a do interceptor não a substitui.

---

## O defeito de segredo em argv está presente em Scenario A

Este é o achado mais concreto da auditoria, e muda o enquadramento da linha "Segredo em
argv": não é só que A não tem o teste — é que A faz hoje o que B corrigiu.

`scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go` interpola o
segredo de administrador do Keycloak em `--password` **duas vezes**, no caminho de leitura
(linha 44, o `Check` do passo) e no de escrita (linha 59):

```go
fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password %[2]s …",
    kc, adminPassword)
```

O script inteiro é um argumento de `docker exec … bash -c`, então o valor cai em três
listas de processos: a do container, a do host e a do próprio toolkit. É a mesma exposição
que B removeu.

**A correção de B era portável sem adaptação, e foi portada.** O motivo pelo qual B pôde
parar de mandar o valor é que ele já está dentro do container, em
`KC_BOOTSTRAP_ADMIN_PASSWORD`. Em A esse mesmo nome já era definido —
`step_provision_keycloak.go:146` o passa ao container, e
`provisioning/templates/entity-keycloak/keycloak-compose.yaml:44` o exige. Os dois cenários
também fixam a mesma imagem, `quay.io/keycloak/keycloak:26.0`, então a verificação que B
fez contra ela vale para A.

### O que foi feito

1. **A guarda primeiro, contra o código com defeito.** `keycloak_secret_exposure_test.go`
   foi portado para A e falhou nas quatro asserções — segredo embutido e `--password`
   presente, nos dois scripts. Essa execução vermelha é a verificação por mutação desta
   linha: o alvo não precisou ser mutado, já estava no estado anterior.
2. **A correção.** Um `kcadmLogin(kc)` em A, com a mesma forma de B, e os dois construtores
   de script (`adminUsersReadScript`, `adminUsersReconcileScript`) deixaram de receber o
   segredo — a assinatura mudou, então o compilador impede o retorno do padrão.
3. **O campo `kcAdminPass` ficou no passo, de propósito.** A asserção de valor só significa
   algo se o passo tiver um segredo para embutir; removê-lo tornaria o teste vago.

Suíte completa do toolkit de A verde por código de saída. A cópia deliberada está
registrada em [`docs/scenario-drift.md`](scenario-drift.md) §13.

**Uma agravante que o inventário inicial não tinha:** em A a exposição acontecia também no
`Check` do passo, que roda em **todo** apply. Um apply que converge e não muda nada pagava
a exposição do mesmo jeito.

### Uma exposição que nenhum dos dois cobre

Ao verificar isso apareceu um terceiro caso, e ele não é lacuna de paridade — é lacuna nos
dois: a senha de *cada operador* também vai em argv, via `set-password --new-password`,
em `scenario-a/.../keycloak_admin_users_reconcile.go:69` e
`scenario-b/.../step_found_spoke.go:568`. A guarda de B verifica só o segredo de
administrador, então ela passa apesar disso. Fica registrado aqui porque é o tipo de coisa
que uma auditoria de paridade não encontra por construção: comparar dois lados não revela
o que falta em ambos.

---

## O sentido inverso: o pin do Besu em B

A linha de literal de imagem era a única lacuna em B, e era real. A centralizou o pin numa
constante (`DefaultBesuImage`) e guardou contra literais reaparecerem; B tinha
`"hyperledger/besu:25.8.0"` escrito em quatro passos — `step_found_hub`, `step_found_spoke`,
`step_join` e `step_genesis` — sem nada ligando as cópias.

**Nenhum gate de CI cobria isso.** O workflow `toolchain-pins` existe e tem self-test, mas
verifica só a versão do Alpine. O pin do Besu não era verificado em lugar nenhum.

Este projeto já pagou por uma cópia esquecida: o pin chegou ao toolkit e não à antiga
árvore `deploy/local`, nada falhou, e os dois stacks rodaram versões diferentes do Besu até
alguém reparar. Quatro cópias num só toolkit são a mesma armadilha, mais perto.

O que foi feito em B: `engine/orchestrator/image_pins.go` passa a ser o único lugar
autorizado a escrever a referência, os quatro passos leem a constante, e
`image_pins_test.go` faz a varredura do pacote isentando esse arquivo pelo nome.
Verificado por mutação — um literal plantado no pacote derruba a guarda.

**Uma diferença deliberada no port:** A guarda dois pins, Besu e Paladin. B guarda só o
Besu, porque B não roda Paladin — seu template `entity-besu` declara um serviço que o
toolkit nunca sobe (`docs/scenario-drift.md`). Um needle de Paladin em B não guardaria nada
e sugeriria que B o executa.

---

## Limite DNS: o que A não tinha não era o teste de B

A linha dizia "4 testes contra 1". Errado nos dois números: A já tinha dois dos três
(`TestGeneratedContainerNamesFitDNSLabel` e `TestEntityNameHeadroomIsStated`), e o terceiro
de B vive noutro arquivo (`proxy_dns_label_test.go`), que a contagem não alcançou.

O port também **não foi linha a linha, de propósito**. A versão de B afirma o limite para
nomes de entidade que ninguém implantou — o nome de país ISO mais longo incluído. Rodar essa
forma em A **falha**: `cbweb3-central-bank-saint-vincent-and-the-grenadines-governance-frontend`
tem 72 octetos. Mas isso não relata nada novo: `TestEntityNameHeadroomIsStated` de A já diz,
pelo outro lado, que `metadata.name` cabe até **35 octetos**, e esse nome tem 45. Um segundo
teste vermelho sobre uma hipótese que o primeiro já precifica é ruído — e um teste vermelho
sobre o qual ninguém pode agir acaba desativado.

O que A realmente não tinha é o **elo entre o proxy e os containers**. O proxy resolve cada
upstream pela rede da entidade, e são duas condições, nenhuma verificada:

1. todo upstream cabe num rótulo DNS, para as entidades **de fato declaradas** nos
   manifestos versionados;
2. todo upstream nomeia um container que os templates realmente criam — um rename deixa o
   proxy discando um nome que não resolve, com exatamente o mesmo sintoma (502 contra um
   stack saudável).

A segunda é a que pega um rename, que é como este defeito costuma chegar. As duas foram
verificadas por mutação: renomear um upstream para `-governance-spa` e alongar um nome de
container derrubam a guarda correspondente.

Para isso as duas listas de rota saíram de literais em `orchestrator.go` para
`centralBankProxyRoutes` / `commercialBankProxyRoutes` em `proxy.go` — sem mudança de
comportamento; é o que torna a guarda possível.

---

## Pré-requisitos do Keycloak: a contagem 18 × 6 era a métrica errada

A versão anterior desta tabela dizia "18 testes contra 6". Contar testes compara volume,
não cobertura, e aqui esconde a única diferença que importa: **os dois cenários provisionam
o Keycloak por caminhos diferentes.**

- **A provisiona por realm importado.** O grosso dos seus testes gira em torno de
  `RenderRealmJSON` — origens sem curinga, `sslRequired` conforme o ambiente, papéis e
  segredos no documento. São guardas sobre um *documento*.
- **B provisiona por script kcadm.** Seus testes giram em torno de um *script* montado por
  concatenação: mensagens de asserção, predicados contra saída real do kcadm, e o elo entre
  os dois que `keycloak_provision_prereq_test.go` fecha.

Consequência: a maioria das guardas de B não tem sentido em A, porque A não monta esse
script no caminho de provisionamento. `TestKeycloakProvisioning_SetsWhatItsOwnAssertionsDemand`
protege o script contra afirmar um estado que ele próprio não estabelece — e o script de
provisionamento de A não afirma estado nenhum: não há um único `exit 1` de asserção em
`step_provision_keycloak.go`.

Isso não é ausência de risco, é risco de outra forma. A pergunta certa para A não é
"faltam 12 testes de Keycloak", e sim: **um apply de A que deixe o realm incompleto
reporta sucesso?** Pelo caminho de importação, provavelmente sim. Isso merece um card
próprio, e não é o port de nada.

O caminho onde A *sim* monta script é `keycloak_admin_users_reconcile.go` — o mesmo da
seção anterior, e ali a paridade é direta: A tem 7 testes contra 6 de B para o mesmo passo,
com nomes quase idênticos. Essa metade já está convergida.

---

## O que o card não previa: a deriva tem dois sentidos

O card enquadra o trabalho como *portar de B para A*. A linha de **literal de imagem**
mostra que isso é metade da história: essas três guardas existem **só em A**, e B não tem
equivalente.

Consequência para o inventário: procurar apenas o que falta em A garante fechar metade
das lacunas e declarar paridade.

---

## Recusas codificadas — a lacuna mais séria, e menos grave do que parece

Em B, um `401` sem código fazia o portal do banco tratá-lo como sessão expirada:
refresh → retry → `forceLogout()`, e o operador voltava para a tela de login sem
explicação. Corrigido em duas PRs (#227 para identidade de chamador, #231 para relay-auth).

Verificado em A, no nível fonte:

- **A tem a mesma cadeia no interceptor.** `bank/src/services/api/interceptors/auth.interceptor.ts`
  trata **todo** `401` que não seja retry como sessão: refresh, retry, `forceLogout()`.
  Não existe classificação por código — A não tem equivalente a `trust-errors.ts`.
- **As recusas de relay-auth de A são sem código.** `middleware/internal_relay_auth.go`
  tem três: *relay auth not configured on server*, *X-Relay-Auth header is required*,
  *invalid relay auth secret*.
- **Mas o proxy de A não repassa verbatim.** `handlers/payment_proxy.go:197` converte
  qualquer não-200 do banco central em `fmt.Errorf("central bank returned %d: %s", …)`.

**Portanto o sintoma de B não se reproduz igual em A.** O `401` do banco central não
chega ao navegador como `401`; chega como erro do próprio gateway. O operador
provavelmente vê uma falha opaca em vez de ser ejetado — o que é ruim, mas é outra coisa.

A lacuna é real em dois níveis: A não tem a guarda de teste, e as recusas são sem código.
A **consequência** é que ainda não foi medida. Portar a correção de B às cegas descreveria
um defeito que A não tem.

### O que o operador de A vê — medido

A cadeia inteira é determinística e foi percorrida no código, com o único elo incerto
medido de fato:

1. O banco central recusa: `401 {"error":"invalid relay auth secret"}`
   (`middleware/internal_relay_auth.go`).
2. O gateway do banco **não repassa**. `getInternalJSON` converte qualquer não-200 em
   `fmt.Errorf("central bank returned %d: %s", …)`
   (`handlers/payment_proxy.go:199`).
3. O handler de extrato converte esse erro em **`502 Bad Gateway`** com o corpo
   `{"error":"statement: load deposits: central bank returned 401: …"}`
   (`handlers/statement.go:135`).
4. O interceptor do portal só ramifica em `401`
   (`interceptors/auth.interceptor.ts:15`), então **nada de refresh, retry ou
   `forceLogout()`**.
5. A store guarda `error.message`, não `response.data`
   (`stores/statement.store.ts:24`).

O elo 5 era o único que dependia de comportamento de biblioteca, então foi medido contra o
axios instalado no próprio app, com um servidor devolvendo exatamente o 502 acima:

```
error.message   = "Request failed with status code 502"
response.status = 502
response.data   = {"error":"statement: load deposits: central bank returned 401: …"}
```

**Conclusão: o sintoma de B não existe em A, e portar a correção de B descreveria um
defeito que A não tem.** O operador de A não é ejetado para a tela de login. Ele vê, na
página de extrato:

> Request failed with status code 502

E só isso. A explicação que o gateway montou — inclusive o `401` do banco central e o
motivo — está em `response.data` e é descartada pela store.

O defeito de A é, portanto, **outro**, e menor em consequência e maior em opacidade: não há
perda de sessão, mas também não há nenhuma pista. Nem o operador nem o suporte conseguem
distinguir "credencial de relay divergente" de "banco central fora do ar" a partir do que a
tela mostra.

O que isso implica para o port: **não portar `trust-errors.ts`**. A correção que A precisa
é de outra natureza — apresentar `response.data.error` em vez de `error.message` — e não
depende de códigos. Codificar as recusas de relay-auth de A continua desejável, mas é a
segunda etapa, e o ganho é bem menor do que o inventário sugeria.

**E não é local à store do extrato.** O mesmo `error instanceof Error ? error.message : …`
aparece **31 vezes em 13 arquivos** do portal do banco de A — todas as stores, sete páginas
e o fluxo de onboarding. Ou seja: *nenhum* erro de API nesse portal mostra a mensagem que o
servidor mandou; todos mostram "Request failed with status code NNN". A forma da correção já
existe no próprio app, em `stores/payment.store.ts:29` (`getErrorMessage`), que só precisa
consultar `response.data` antes de `message` e ser usada em toda parte.

Isso é maior do que esta auditoria e muda o que **toda** página do portal exibe, então fica
como card próprio e não entra aqui — mas foi esta linha que o encontrou, e a medição acima é
a evidência.

*Nível de verificação:* fonte para a cadeia (elos 1–4, todos leitura direta e sem ramificação
condicional), medição real para o elo 5. Não foi um stack de ponta a ponta: a única coisa que
um stack ao vivo acrescentaria é confirmar a mesma string na tela, e ela já está determinada
pelo elo 5.

A codificação de erros em A não é inexistente: o caminho de login já usa
`CodeInvalidRequest`, `CodeMissingCredentials` e `CodeAuthServiceUnavailable`
(`handlers/auth.go`). É a superfície de relay que ficou de fora.

---

## Regras de verificação, tiradas de erros reais

Duas regras que o passo 2 do card deve seguir, ambas vindas de enganos cometidos neste
projeto e não de teoria:

**Mutar para o comportamento anterior, nunca para uma deleção.** Ao verificar uma guarda
nova do orquestrador, apagá-la derrubou a suíte — mas por *panic* de ponteiro nulo, não
por detecção. A guarda só se provou quando a mutação restaurou a validação anterior.
Uma deleção pode derrubar a suíte por motivos que nada têm a ver com a guarda.

**Conferir por código de saída, nunca por linha de resumo.** O wrapper de CLI do
repositório imprimiu `PASS (11) FAIL (0)` numa execução cujo JSON trazia
`"success": false` e uma suíte que não carregou. Uma auditoria que lê resumos não
certifica nada, inclusive a si mesma.

E uma terceira, específica do Zeto, registrada no ADR-009: **use identidade nova a cada
ponto de medição**, ou a seleção de notas contamina o resultado.

O passo 2 acrescentou mais quatro, todas vindas de falsos "a guarda NÃO detectou" que
custaram tempo nesta mesma auditoria. As quatro têm a mesma forma: **a mutação não chegou
ao alvo, e o verde foi lido como veredito.**

**Provar que a mutação se aplicou, antes de ler o resultado.** O harness usado aqui
(`mutate.py`) aborta com `MUTACAO-NAO-APLICADA` quando o padrão não casa. Sem isso, uma
substituição que não casou é indistinguível de uma guarda que detectou nada.

**Mutar o código, não o comentário.** A primeira tentativa contra
`keycloak_secret_exposure_test.go` trocou a primeira ocorrência de `config credentials` no
arquivo — que está num comentário, 29 linhas acima do código. A guarda passou, corretamente,
e o registro inicial foi "não detectou".

**`go test` sem `-count=1` responde do cache.** Várias destas guardas leem YAML *fora* do
próprio pacote. O cache do Go não observa esses arquivos, então mutar o manifesto e
reexecutar devolve o PASS anterior. Todo resultado desta auditoria foi obtido com
`-count=1`.

**Uma mutação que não compila não é uma detecção.** A primeira tentativa contra o caminho
`join` do Keycloak deixou um `Fprintf` com argumento sobrando; o pacote falhou a build. A
suíte fica vermelha sem que a guarda tenha opinado — o mesmo erro de forma que a regra
"nunca mutar para uma deleção" descreve.

E uma armadilha adjacente, que não chegou a produzir um erro registrado só porque foi
conferida: **um `-run` que não casa com teste nenhum sai 0.** Um filtro com nome errado
produz sucesso silencioso.

---

## Estado dos gates de CI

Levantado durante o inventário, porque um gate que não roda é uma guarda que não guarda.

| Gate | Self-test? |
|---|---|
| `doc-links`, `frontend`, `license-headers`, `toolchain-pins` | **sim** |
| `toolkit`, `backend-scenario-a`, `backend-scenario-b`, `contracts-scenario-a`, `contracts-scenario-b`, `gitleaks`, `proto-lint`, `authz-parity`, `api-artifacts` | não |

Quatro de treze provam a própria via de falha. Não é necessariamente defeito — um gate
trivial pode não justificar — mas é o mapa de onde a auditoria não pode confiar no verde.

**Uma suspeita que se provou falsa, registrada de propósito:** o job do `toolkit` chama-se
*"vet, build and test"* e uma leitura apressada não achou o passo de teste. Ele existe —
`go test -count=1 ./...`. O grep tinha cortado nos primeiros resultados. Vale como
lembrete de que este documento inteiro é feito do tipo de leitura que produz esse erro.

---

## O que falta

1. ~~**Passo 2 do card:** quebrar o que cada guarda protege e confirmar que ela falha.~~
   **Feito** — as sete estão na tabela acima.
2. ~~**Corrigir a exposição de segredo em argv em A**, portando junto o teste de B.~~
   **Feito** — ver a seção acima.
3. ~~**Medir a consequência real da lacuna de recusas codificadas em A** antes de portar.~~
   **Feito** — o sintoma de B não se reproduz. O que A precisa é mostrar
   `response.data.error` em vez de `error.message` na store do extrato; codificar as
   recusas é uma segunda etapa, de ganho menor. Ver a seção acima.
4. ~~**Decidir o sentido inverso:** as guardas de literal de imagem vão para B?~~
   **Feito** — foram, com o pin centralizado numa constante. Ver a seção acima.
5. **Abrir card próprio para a asserção de estado final do Keycloak em A** — o script de
   provisionamento de A não afirma nada, então um realm incompleto reporta sucesso. Não é
   port de guarda; é defeito de outra natureza.
6. **Senha de operador em argv nos dois cenários** (`set-password --new-password`) —
   lacuna comum, fora do escopo de paridade, mas encontrada por ela. Nomeada em
   [`docs/scenario-drift.md`](scenario-drift.md) §14 para não ser redescoberta como
   assimetria.
