# Interface Contract: Variáveis de Ambiente do Relay Cacti

**Branch**: `031-relay-spoke-registry` | **Date**: 2026-06-27

---

## Variáveis Novas

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `CACTI_SPOKES_CONFIG` | Não (se shim legado disponível) | — | Caminho absoluto para o arquivo YAML de registro de spokes. Quando presente, tem precedência sobre todas as vars `SPOKE_*`. |

---

## Variáveis Legadas (shim de compatibilidade — deprecadas)

As variáveis abaixo continuam funcionando quando `CACTI_SPOKES_CONFIG` está ausente. Um aviso de deprecação é emitido no startup.

| Variável | Obrigatória (sem YAML) | Padrão |
|---|---|---|
| `SPOKE_A_BESU_RPC` | ✅ | — |
| `SPOKE_A_BESU_WS` | Não | `ws://localhost:8655` |
| `SPOKE_A_HTLC_ADDRESS` | ✅ | — |
| `SPOKE_A_INTERNAL_API` | ✅ | — |
| `SPOKE_A_PAYMENT_GRPC` | ✅ | — |
| `SPOKE_B_BESU_RPC` | ✅ | — |
| `SPOKE_B_BESU_WS` | Não | `ws://localhost:8755` |
| `SPOKE_B_HTLC_ADDRESS` | ✅ | — |
| `SPOKE_B_INTERNAL_API` | ✅ | — |
| `SPOKE_B_PAYMENT_GRPC` | ✅ | — |

**Comportamento de precedência**: Se `CACTI_SPOKES_CONFIG` e `SPOKE_A_BESU_RPC` estão ambos definidos, `CACTI_SPOKES_CONFIG` tem precedência e as vars legadas são ignoradas.

---

## Variáveis Inalteradas

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `CACTI_API_PORT` | Não | `4000` | Porta da REST API do relay. |
| `POLL_INTERVAL_MS` | Não | `3000` | Intervalo de polling por spoke (ms). |
| `INTERNAL_RELAY_AUTH_SECRET` | ✅ | — | Segredo compartilhado para header `X-Relay-Auth`. |
| `SOCKET_IO_ALLOWED_ORIGINS` | Não | `""` | Origens permitidas para Socket.IO. |
| `RELAY_STORE_PATH` | Não | `/tmp/cacti-relay-store.json` | Caminho do arquivo de estado persistido. |
| `PROTO_PATH` | Não | `/app/apis/proto/...` | Caminho do arquivo `.proto` do payment-orchestrator. |

---

## Esquema YAML (`CACTI_SPOKES_CONFIG`)

```yaml
# Exemplo: /etc/cacti/spokes.yaml
spokes:
  - id: spoke-a                                          # string, obrigatório, único
    besuRpc: "http://host.docker.internal:8645"          # string, obrigatório
    besuWs: "ws://host.docker.internal:8655"             # string, obrigatório
    htlcAddress: "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e"  # string, obrigatório
    internalApiUrl: "http://host.docker.internal:18080"  # string, obrigatório
    grpcEndpoint: "host.docker.internal:19094"           # string, obrigatório
  - id: spoke-b
    besuRpc: "http://host.docker.internal:8745"
    besuWs: "ws://host.docker.internal:8755"
    htlcAddress: "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e"
    internalApiUrl: "http://host.docker.internal:58080"
    grpcEndpoint: "host.docker.internal:59094"
```

---

## REST API — endpoint de saúde (inalterado)

`GET /api/v1/health` — sem mudança de schema. O relay pode opcionalmente incluir a lista de `spokeIds` no body para facilitar diagnóstico (não obrigatório nesta feature).
