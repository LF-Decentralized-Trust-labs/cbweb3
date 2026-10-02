# ADR-011: Login administrativo no hub do Scenario B

**Status**: **Aceito** (2026-09-10)
**Data**: 2026-09-10
**Decisores**: Antonio Souza (liderança técnica)
**Card relacionado**: `[Scenario B · Hub] Nobody can log into the hub — its operators are provisioned in the realm but the gateway has no auth service`

---

## Pedido de decisão

| | |
|---|---|
| **O que se pede** | Decidir se o hub — o operador **neutro** da rede, não um participante — deve ter login administrativo. |
| **Por que é decisão e não implementação** | O `hub-backend.compose.yaml` afirmava por escrito que o hub *"needs no auth/login"*. Ligar o serviço **reverte um desenho declarado**, e isso não se faz num commit sem registo. |
| **Decidido** | **Sim, o login é para existir.** O serviço de auth passa a integrar o stack do hub. |
| **Alternativa rejeitada** | Remover os dois operadores do manifesto do hub e fazer o portal dizer que não há login. Menos código, mas ver §Por que não. |

---

## O que existia

Três coisas que não podiam estar todas certas ao mesmo tempo:

| Onde | O que dizia |
|---|---|
| `hub-backend.compose.yaml` | o hub **não precisa** de auth/login |
| `samples/hub/*.yaml` | declara **dois operadores**: `admin@hub.governance.gov`, `admin@hub.noc.gov` |
| `step_found_hub.go` | **provisiona-os** no realm, e cria um cliente `hub-backend` com segredo |

Resultado observado num stack recém-implantado: todos os portais respondem `200` no login, **menos o hub**, que responde `503 AUTH_SERVICE_UNAVAILABLE`. Testado com o operador do próprio manifesto do hub, não com credencial emprestada.

O erro estava **correto** — o gateway distingue um serviço de auth inalcançável de uma senha errada, e não havia a quem perguntar. É parte da razão de ter sobrevivido: de fora, o realm parece certo, o portal carrega, as credenciais são as documentadas, e a falha nomeia um serviço que o operador não tem motivo para saber que falta.

## Por que sim, e não a alternativa

A decisão de "sem login" era defensável quando foi tomada: a autenticação de um swap acontece nos gateways do banco central e do banco comercial, não aqui.

Mas **o resto da árvore já a tinha abandonado**. O realm do hub tem cliente de backend com segredo, e dois operadores são criados em cada `found-hub`. Manter o desenho antigo exigiria desfazer essas três coisas; o estado intermédio — criar contas que não servem para nada e servir um ecrã de login que responde 503 — é o pior dos três mundos.

O hub serve um portal de governança. Um operador do hub precisa de o alcançar.

## O que ficou

- Serviço `auth` no `hub-backend.compose.yaml`, com a forma do `entity-backend.compose.yaml`, **menos** o que o hub não usa: sem semente de KMS (o hub não faz onboarding de ninguém) e sem chave de serviço separada (corre um único assinante).
- `AUTH_GRPC_ADDR` no gateway do hub, apontando para o container de auth na rede da entidade.
- `AUTH_IMAGE` no env do hub, e a imagem construída pelo `build-hub-backend-image`, cujo `Check` passa a exigir **as duas** imagens.
- O comentário do template reescrito. Deixá-lo a dizer *"needs no auth/login"* ao lado de um serviço de auth seria pior que o estado anterior.

O hub já tinha tudo de que o auth depende — postgres, redis, Keycloak e um serviço de compliance próprio — então isto é um port, não desenho novo.

### Uma limpeza que a decisão obrigou

O passo `start-hub-compliance` construía a imagem do auth, com o comentário *"the hub does not run auth, but every spoke/bank backend does"*. As duas metades deixaram de ser verdade: o hub corre uma agora, e o `start-spoke-backend` constrói as suas três para o caso de daemon separado. A construção passou para junto do gateway com que é enviada.

## Consequências

- O hub ganha um serviço a mais no stack. Custo de recursos comparável ao de qualquer spoke.
- Os operadores do hub passam a conseguir entrar no portal de governança.
- **O hub passa a ter login administrativo.** É a consequência que justifica este ADR existir: o operador neutro da rede deixa de ser apenas neutro em infraestrutura e passa a ter uma porta de entrada autenticada. Quem for endurecer o ambiente para produção deve tratá-la com o mesmo cuidado que a de um banco central.

## Verificação

Seis guardas em `toolkit/engine/orchestrator/hub_auth_test.go`, cinco mutações, todas detectadas: remover o serviço do template, remover o `AUTH_GRPC_ADDR`, apontá-lo para `localhost`, deixar de construir a imagem, e voltar a olhar só a imagem do gateway no `Check`.

Duas dessas guardas nasceram fracas — uma procurava a substring `AUTH_GRPC_ADDR` (que sobrevive a `AUTH_GRPC_ADDR_REMOVIDO`) e outra procurava um caminho de Dockerfile no código-fonte (que aparecia duas vezes). Ambas foram reescritas para afirmar estrutura e comportamento: a primeira faz parse do YAML e confere para onde o endereço aponta; a segunda executa o passo e verifica que as duas imagens são construídas.
