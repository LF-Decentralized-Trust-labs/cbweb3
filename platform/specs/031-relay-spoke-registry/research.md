# Research: RL-1/RL-2/RL-3 — Relay: Registro Dinâmico de Spokes

**Branch**: `031-relay-spoke-registry` | **Date**: 2026-06-27

---

## 1. Framework de testes para o relay Cacti

**Decisão**: Adicionar **Vitest** como test framework.

**Rationale**: O `package.json` não tem nenhum framework de testes, nenhum script `test`, e nenhum devDependency de test runner. Vitest é a escolha correta porque:
- Compatível nativamente com TypeScript sem configuração extra (o projeto usa `ts-node` e `typescript ^5.4`)
- API compatível com Jest (sem curva de aprendizado)
- Suporta mocks de módulos ESM/CJS nativos
- Sem conflito com `strict: true` do tsconfig
- Não requer Jest `--experimental-vm-modules` para TypeScript

**Alternativas consideradas**:
- Jest: requer `ts-jest` ou `@swc/jest` para TypeScript; setup mais pesado
- Mocha + Chai: verbose; sem type narrowing nativo nos assertions
- Node test runner (stdlib): sem mocks nativos de módulos; limitado para spy/stub de gRPC client

**Impacto no package.json**:
```json
"devDependencies": {
  "vitest": "^2.0.0"
},
"scripts": {
  "test": "vitest run",
  "test:watch": "vitest"
}
```

---

## 2. Parsing de YAML — js-yaml vs yaml

**Decisão**: Adicionar **`js-yaml`** (não `yaml`).

**Rationale**: `js-yaml` NÃO está no `package.json` atual. Precisa ser adicionado como dependência de produção. A escolha de `js-yaml` (em vez do pacote `yaml`) é justificada por:
- Já é dependência indireta de pacotes Cacti (via `@hyperledger/cactus-*`), reduzindo tamanho do bundle
- API de parsing segura por padrão (`js-yaml.load` com schema `SAFE_LOAD` evita execução de código JS embutido no YAML)
- Tipos para TypeScript via `@types/js-yaml`

**Alternativa considerada**: pacote `yaml` (alternativa mais recente). Rejeitado por não ser indiretamente presente no lockfile; adicionaria um novo grafo de dependências.

**Impacto no package.json**:
```json
"dependencies": {
  "js-yaml": "^4.1.0"
},
"devDependencies": {
  "@types/js-yaml": "^4.0.9"
}
```

---

## 3. Mapeamento semântico do shim de vars legadas

**Decisão**: No shim, `spoke-a.grpcEndpoint = SPOKE_A_PAYMENT_GRPC` e `spoke-b.grpcEndpoint = SPOKE_B_PAYMENT_GRPC`.

**Rationale**: Esta é a mudança semântica mais crítica do ponto de vista do shim. Na configuração atual:

```ts
// config.ts ATUAL (bilateral hardcoded):
spokeA.counterpartGrpc = SPOKE_B_PAYMENT_GRPC  // spoke-a "sabe" o gRPC do spoke-b
spokeB.counterpartGrpc = SPOKE_A_PAYMENT_GRPC  // spoke-b "sabe" o gRPC do spoke-a
```

No novo modelo, cada spoke armazena o **próprio** endpoint gRPC e o relay roteia para o destino:

```ts
// NOVO modelo (registro + roteamento por dest_spoke_id):
spokes["spoke-a"].grpcEndpoint = SPOKE_A_PAYMENT_GRPC  // o endpoint do spoke-a
spokes["spoke-b"].grpcEndpoint = SPOKE_B_PAYMENT_GRPC  // o endpoint do spoke-b
// Ao liquidar: relay lê dest_spoke_id="spoke-b" → lookup → usa SPOKE_B_PAYMENT_GRPC
```

O resultado final para o par (spoke-a → spoke-b) é idêntico — o gRPC chamado é `SPOKE_B_PAYMENT_GRPC` em ambos os modelos. A diferença é que agora o lookup é por `dest_spoke_id` em vez de por "contraparte estática".

**O shim de vars legadas**, portanto, deve mapear:
- `SPOKE_A_PAYMENT_GRPC` → `grpcEndpoint` do entry spoke-a
- `SPOKE_B_PAYMENT_GRPC` → `grpcEndpoint` do entry spoke-b

**Verificação**: docker-compose.yaml confirma que `SPOKE_A_PAYMENT_GRPC=host.docker.internal:19094` é o gRPC do payment-orchestrator que serve spoke-a (Bank-A, porta 19094). `SPOKE_B_PAYMENT_GRPC=host.docker.internal:59094` serve spoke-b (Bank-D custodian, porta 59094). Correto.

---

## 4. gRPC client: criação por spoke no startup vs. por-trade

**Decisão**: Criar um cliente gRPC por spoke no startup e mantê-lo em `Map<spokeId, PaymentOrchestratorClient>` dentro de `HtlcRelay`.

**Rationale**: O padrão atual cria `grpcClient` por spoke no loop de polling (linha 347 de `htlc-relay.ts` — `createGrpcClient(spoke.counterpartGrpc, ...)`). Esse cliente é criado UMA VEZ por `startSpokeLoop` e reutilizado durante todo o ciclo de vida do loop. No novo modelo, o cliente gRPC de cada spoke é criado no constructor ou no `start()` de `HtlcRelay`, keyed por `spoke.id`, e reutilizado por todas as trades que apontam para aquele spoke.

**Alternativa considerada**: criar cliente por-trade (lookup + `createGrpcClient` em cada `LogHTLCClaimed`). Rejeitado: overhead de conexão por trade; TCP handshake + TLS handshake desnecessários para cada liquidação.

---

## 5. Formato do arquivo YAML de spokes

**Decisão**: YAML com chave raiz `spokes:` contendo array de entries.

```yaml
spokes:
  - id: spoke-a
    besuRpc: "http://host.docker.internal:8645"
    besuWs: "ws://host.docker.internal:8655"
    htlcAddress: "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e"
    internalApiUrl: "http://host.docker.internal:18080"
    grpcEndpoint: "host.docker.internal:19094"
  - id: spoke-b
    besuRpc: "http://host.docker.internal:8745"
    besuWs: "ws://host.docker.internal:8755"
    htlcAddress: "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e"
    internalApiUrl: "http://host.docker.internal:58080"
    grpcEndpoint: "host.docker.internal:59094"
```

**Rationale**: Estrutura plana, sem nesting desnecessário. Fácil de gerar por script (ex: `contracts.sync-addresses`). A chave raiz `spokes:` é explícita e extensível (poderia conter metadados top-level no futuro sem quebrar o parser).

**Alternativa considerada**: array JSON via env var `CACTI_SPOKES_JSON`. Rejeitado: JSON em env var é frágil (escapamento de aspas em shells/compose).

---

## 6. Compatibilidade com o Dockerfile existente

**Decisão**: Sem alteração no Dockerfile.

**Rationale**: O Dockerfile copia o projeto e roda `npm install + tsc`. A adição de `js-yaml` e `vitest` é transparente — `npm install` os instala normalmente. Nenhum `COPY` adicional é necessário para o YAML de spokes (ele é montado via volume ou env var no docker-compose, não no build).

---

## Resolução de NEEDS CLARIFICATION

Nenhum marcador `[NEEDS CLARIFICATION]` foi identificado no spec — todos os pontos foram resolvidos acima.
