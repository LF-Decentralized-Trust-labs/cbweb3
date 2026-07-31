# Quickstart — TK-B5 (relay generalizado)

Como exercitar o relay generalizado e o `RelayRegistrar`. A auth-por-CB está fora de escopo; o
segredo compartilhado atual permanece.

## Relay (TypeScript)

Boot neutro (sem spokes):
```bash
cd scenario-b/interop/hub-and-spoke/cacti
npm run build && npm start     # sobe com 0 spokes, sem SPOKE_A/B_* fatais
```

Registrar um spoke em runtime (sem restart):
```bash
curl -X POST http://localhost:4000/api/v1/spokes \
  -H "Content-Type: application/json" \
  -H "X-Relay-Auth: $RELAY_AUTH_SECRET" \
  -d '{"spokeId":"spoke-br","besuRpc":"http://host.docker.internal:8855",
       "besuWs":"ws://host.docker.internal:8856","gatewayUrl":"http://host.docker.internal:8090"}'
# 200 { "status": "registered", "spokeId": "spoke-br" }  (idempotente)
```

Reiniciar o relay ⇒ `spoke-br` é recarregado do RelayStore (JSON) sem novo POST.

Rodar os testes (lógica pura, sem Besu):
```bash
npm test    # node:test — registry, store, roteamento por spoke_out, breaker, validação de payload
```

Cobre: boot neutro e N spokes (SC-001); registro runtime idempotente + recarga pós-reinício
(SC-002/003); roteamento por `spoke_out` + destino desconhecido (SC-004); recusa quando `isPaused`
ou consulta indisponível (SC-005); ausência de `spoke-a`/`spoke-b` e de `SPOKE_A/B_BESU_RPC`
(SC-006).

## RelayRegistrar (Go)

```go
// local (dev/testes) — registry in-memory
rr, _ := relayregistrar.New("local")
_ = rr.Register(ctx, relayregistrar.Spoke{
    ID: "spoke-br", BesuRPC: "http://...:8855", BesuWS: "ws://...:8856",
    GatewayURL: "http://...:8090",
})

// produção (stub) — encaminha para POST /api/v1/spokes do relay
rrProd, _ := relayregistrar.New("relay://relay-host:4000")
```

```bash
cd scenario-b/toolkit
go test ./engine/relayregistrar/...
```

Cobre: registro local idempotente + `List()`; factory `local` vs `relay://…` (SC-008); URI não
suportada → erro; `Spoke` inválido → `ErrInvalidSpoke`.

## Fora de escopo (documentado)

- **Auth por CB** (§14.D) — requer aprovação do project-lead; o segredo compartilhado (`X-Relay-Auth`)
  segue como fallback local/dev.
