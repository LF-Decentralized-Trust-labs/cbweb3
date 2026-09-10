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
| Segredo em argv | `orchestrator/keycloak_secret_exposure_test.go` | — | **lacuna em A** |
| Deriva de literal de imagem | — | 3 testes (`TestNoImagePinLiteralsInThisPackage`, `TestDefaultImagePinsComeFromTheOrchestratorConstants`, `TestFrontendComposeEnv_PerEntityImageTag`) | **lacuna em B** — a deriva corre nos dois sentidos |
| Pré-requisitos do Keycloak | 18 testes | 6 testes | **parcial** |
| Portas efêmeras | 4 testes | os mesmos 4 | **convergida** |
| Limite DNS de 63 caracteres | 4 testes | 1 teste | **parcial** — falta em A o equivalente a `TestProxyUpstreamsFitDNSLabel` |
| Recusas codificadas | 3 arquivos | — | **lacuna em A**, com ressalva importante abaixo |

Todas as linhas acima estão no nível **fonte**. Nenhuma foi verificada por mutação ainda —
esse é o passo 2 do card e o trabalho real que resta.

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

> Próximo passo desta linha: dirigir o portal de A com um `INTERNAL_RELAY_AUTH_SECRET`
> divergente e registrar o que o operador vê. Só então decidir o que portar.

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

1. **Passo 2 do card, o trabalho real:** quebrar o que cada guarda protege e confirmar que
   ela falha. Nenhuma das sete passou disso ainda.
2. **Medir a consequência real da lacuna de recusas codificadas em A** antes de portar.
3. **Decidir o sentido inverso:** as três guardas de literal de imagem vão para B, ou a
   ausência é deliberada?
4. **Detalhar a linha do Keycloak** — 18 contra 6 é diferença grande demais para uma
   célula de tabela.
