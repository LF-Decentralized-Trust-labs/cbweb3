# Data Model: RL-1/RL-2/RL-3 — Relay: Registro Dinâmico de Spokes

**Branch**: `031-relay-spoke-registry` | **Date**: 2026-06-27

---

## SpokeConfig (config.ts — exportado)

Substitui as propriedades `spokeA` / `spokeB` do `config` object.

| Campo | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `id` | `string` | ✅ | Identificador único do spoke (ex: `"spoke-a"`). Usado como chave de lookup no registro. |
| `besuRpc` | `string` | ✅ | URL HTTP JSON-RPC do nó Besu (ex: `"http://host:8645"`). |
| `besuWs` | `string` | ✅ | URL WebSocket JSON-RPC do nó Besu (ex: `"ws://host:8655"`). |
| `htlcAddress` | `string` | ✅ | Endereço 0x-prefixado do contrato HTLC deployado neste spoke. |
| `internalApiUrl` | `string` | ✅ | URL HTTP da API interna do api-gateway para polling de FX agreements (ex: `"http://host:18080"`). |
| `grpcEndpoint` | `string` | ✅ | Target gRPC do payment-orchestrator deste spoke (ex: `"host:29094"`). **Cada spoke armazena o próprio endpoint** — o relay faz lookup por `dest_spoke_id` para rotear a liquidação. |

**Removidos**: `counterpartGrpc`, `counterpartName` (eram premissas bilaterais hardcoded).

---

## SpokeDep (htlc-relay.ts — interface interna)

Espelho de `SpokeConfig` usado internamente pelo `HtlcRelay`. Campos idênticos ao `SpokeConfig` exportado.

| Campo | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `id` | `string` | ✅ | Identificador único do spoke. |
| `besuRpc` | `string` | ✅ | URL HTTP JSON-RPC. |
| `besuWs` | `string` | ✅ | URL WebSocket JSON-RPC. |
| `htlcAddress` | `string` | ✅ | Endereço 0x do contrato HTLC. |
| `internalApiUrl` | `string` | ✅ | URL da API interna para polling. |
| `grpcEndpoint` | `string` | ✅ | Target gRPC do payment-orchestrator deste spoke. |

**Removidos**: `counterpartGrpc`, `counterpartName`.

---

## Spoke Registry (runtime — em memória)

Estrutura de dados derivada de `config.spokes[]` no startup. Mantida em `HtlcRelay` como:

```
Map<string, SpokeDep>  — chave = spoke.id
Map<string, PaymentOrchestratorClient>  — chave = spoke.id
Map<string, PluginLedgerConnectorBesu>  — chave = spoke.id  (em index.ts)
```

- Populada no startup, imutável em runtime (sem hot-reload).
- Lookup O(1) por `dest_spoke_id` durante liquidação.

---

## YAML de configuração de spokes (arquivo externo)

Arquivo YAML lido de `CACTI_SPOKES_CONFIG`. Schema:

```yaml
spokes:
  - id: string           # obrigatório; único por entry
    besuRpc: string      # obrigatório; URL HTTP
    besuWs: string       # obrigatório; URL WebSocket
    htlcAddress: string  # obrigatório; 0x-prefixado
    internalApiUrl: string  # obrigatório; URL HTTP
    grpcEndpoint: string # obrigatório; host:port
```

**Validação no startup**:
1. `spokes` é array não-vazio → `Fatal: at least one spoke must be configured`
2. Cada campo obrigatório presente → `Fatal: spoke[N].<field> is required`
3. Nenhum `id` duplicado → `Fatal: duplicate spoke id "<id>"`
4. Arquivo legível → `Fatal: cannot read spokes config: <path>: no such file`

---

## Shim de vars legadas — mapeamento

Quando `CACTI_SPOKES_CONFIG` está ausente e `SPOKE_A_BESU_RPC` + `SPOKE_B_BESU_RPC` estão definidos, o shim sintetiza dois entries:

| Env var legado | Campo no novo entry | Spoke |
|---|---|---|
| `SPOKE_A_BESU_RPC` | `besuRpc` | spoke-a |
| `SPOKE_A_BESU_WS` | `besuWs` | spoke-a (default: `ws://localhost:8655`) |
| `SPOKE_A_HTLC_ADDRESS` | `htlcAddress` | spoke-a |
| `SPOKE_A_INTERNAL_API` | `internalApiUrl` | spoke-a |
| `SPOKE_A_PAYMENT_GRPC` | `grpcEndpoint` | spoke-a (**próprio** endpoint — mudança semântica) |
| `SPOKE_B_BESU_RPC` | `besuRpc` | spoke-b |
| `SPOKE_B_BESU_WS` | `besuWs` | spoke-b (default: `ws://localhost:8755`) |
| `SPOKE_B_HTLC_ADDRESS` | `htlcAddress` | spoke-b |
| `SPOKE_B_INTERNAL_API` | `internalApiUrl` | spoke-b |
| `SPOKE_B_PAYMENT_GRPC` | `grpcEndpoint` | spoke-b (**próprio** endpoint — mudança semântica) |

> **Mudança semântica importante**: na config antiga, `spokeA.counterpartGrpc = SPOKE_B_PAYMENT_GRPC` (spoke-a apontava para o gRPC do spoke-b). No novo modelo, `spoke-a.grpcEndpoint = SPOKE_A_PAYMENT_GRPC` (spoke-a armazena o próprio gRPC). O resultado de roteamento é idêntico para o par bilateral, mas a semântica muda de "endpoint do outro" para "endpoint do próprio". O shim reflete esse mapeamento correto.

---

## Transições de estado relevantes

Não há novos estados introduzidos. O fluxo de liquidação HTLC existente é preservado:

```
LogHTLCClaimed (spoke-src)
  → lookup dest_spoke_id no registry
  → resolveCounterpartContractId(hashLock) → contractId no spoke-dest
  → SettleHTLC(contractId, secret) via grpcClient do spoke-dest
```

A única mudança é que o `grpcClient` agora é obtido por `spokeRegistry.get(destSpokeId).grpcEndpoint` em vez de `spoke.counterpartGrpc`.
