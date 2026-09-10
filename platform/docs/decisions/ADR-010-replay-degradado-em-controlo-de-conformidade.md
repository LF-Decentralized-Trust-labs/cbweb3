# ADR-010: Proteção de repetição degradada num controlo de conformidade

**Status**: **Aceito** (2026-09-10) — Opção B
**Data**: 2026-09-10
**Decisores**: Antonio Souza (liderança técnica)
**Card relacionado**: `[Relay replay] Fail-open replay protection on a compliance control (transfer-limits/restore)`
**Origem**: residual do achado NEW-2 da revisão de segurança de `feat/scenario-b-sovereign-hub-delegation`

---

## Pedido de decisão

| | |
|---|---|
| **O que se pede** | Escolher o comportamento de `POST /internal/v2/transfer-limits/restore` quando o armazenamento partilhado de repetição está inacessível. |
| **Quem assina** | Liderança técnica. A constituição exige aprovação do líder para enfraquecer — ou, aqui, para **manter** enfraquecido — um controlo de conformidade. |
| **Decidido** | **Opção B, aprovada em 2026-09-10.** Recusar nessa rota específica enquanto a proteção estiver degradada; todas as outras continuam a servir. |
| **Se não fosse aprovado** | Teria de ficar registado com sign-off que a proteção contra repetição neste controlo é **melhor-esforço**, e que uma indisponibilidade do Redis abre uma janela em que um banco pode transacionar acima do limite fixado pelo seu banco central. |

---

## Contexto

A assinatura de relay tem proteção contra repetição em `internal/relayauth/replay.go`. Ela é
bem construída: indexa o cache pelo valor `r` do ECDSA, o que derrota maleabilidade de
assinatura, e o armazenamento partilhado em Redis (`SET NX`) fecha os buracos de reinício do
gateway e de múltiplas réplicas — **enquanto o Redis responder**.

O residual: `Admit` falhava **aberto** em qualquer erro do armazenamento partilhado, degradando
para a memória do processo. Para `transfer-limits/restore` — um controlo de conformidade **não
idempotente**, porque credita de volta a margem diária de um banco — isso tornava a proteção
melhor-esforço em vez de garantida.

Quem provoque alguns minutos de indisponibilidade do Redis, ou apanhe um reinício do gateway
antes de a camada Redis estar ligada, pode reenviar um `restore` capturado dentro da janela de
±5 minutos da assinatura e deixar um banco transacionar acima do limite. O único sinal era uma
linha de log.

## Por que não falhar fechado globalmente

Derrubaria a ponte, o swap delegado do hub e o retorno de resíduo — raio de explosão muito
maior que o problema. Esta opção foi rejeitada na revisão e continua rejeitada.

## Opções consideradas

| | Opção | Avaliação |
|---|---|---|
| A | Aceitar formalmente o risco | Rejeitada. O custo de fechar **uma** rota é menor que o risco que se aceitaria. |
| **B** | **Falhar fechado só em `transfer-limits/restore`** | **Escolhida.** |
| C | Chave de idempotência enviada pelo cliente | Adiada. Mais correta a prazo; maior, e o projeto está perto da entrega final. |

### Por que B e não C

A C elimina a dependência do cache em vez de a contornar, e é o destino certo. Mas exige
mudar o contrato da rota, os dois lados que a chamam e a persistência da chave. A B usa o
que já existe e reduz a janela a zero **hoje**, sem alterar contrato. A C continua desejável
e deve ser aberta como card quando houver folga.

### Uma armadilha registada de propósito

Existe `X-Correlation-Id` no gateway e é fácil concluir que a C já está feita. **Não está**: o
middleware é explícito — *"Any value provided by the client is ignored/overridden"* — e gera um
id novo por requisição. Um reenvio recebe um id novo, portanto nada é deduplicado. Serve para
rastrear, não para identificar repetição.

## O que foi implementado

`Admit` deixou de devolver um booleano. Ele colapsava duas situações que o chamador precisa de
distinguir: o armazenamento partilhado confirmou que a assinatura é nova, e o armazenamento não
respondeu. Passou a devolver `Admission`, com três valores — e o **zero é `AdmissionRefused`**,
para que uma atribuição esquecida negue em vez de admitir.

O middleware recusa com **503** e código `RELAY_REPLAY_STORE_UNAVAILABLE` quando o estado é
degradado **e** a rota está no conjunto de falha-fechada. 503 e não 401: as credenciais de quem
chama estão corretas; o que falhou foi a nossa capacidade de verificar o pedido em segurança, e
503 diz "tente de novo" onde 401 diz "chave errada".

**Um deployment sem armazenamento partilhado não é degradado.** É o deployment de processo único
em que esta guarda nasceu, e a sua resposta é tão autoritativa quanto esse deployment consegue
ser. Tratá-lo como degradado tornaria a rota permanentemente indisponível onde o Redis nunca foi
ligado — transformaria endurecimento em indisponibilidade.

`check-and-deduct` **não** entra no conjunto: ela consome margem, logo uma repetição é
auto-limitante no sentido seguro.

## Consequências

- Durante uma indisponibilidade do Redis, `restore` responde 503 e o chamador repete. A margem
  diária não é creditada de volta nesse intervalo — atraso, não perda.
- As rotas de liquidação continuam a servir degradadas, exatamente como antes.
- Acrescentar uma rota ao conjunto passa a ser uma decisão com custo de disponibilidade. A
  guarda `TestOtherInternalRoutesStillServeWhileTheReplayStoreIsDown` existe para que alargar o
  conjunto por descuido falhe: sem ela, esta correção seria indistinguível, numa suíte verde, do
  falhar-fechado global que foi rejeitado.

## Verificação

Cinco mutações, todas detectadas: repor a admissão com o armazenamento fora; tornar a recusa
global; apontar a rota errada; classificar "sem armazenamento partilhado" como degradado; e
tornar o zero de `Admission` uma admissão.

`go build`, `go vet`, `go test`, `go test -race`, e os alvos que o CI usa (`make vet`,
`make test-coverage`, `make security`) — todos exit 0 no api-gateway do Scenario B.
