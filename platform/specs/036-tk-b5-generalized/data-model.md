# Data Model — TK-B5 (relay generalizado)

Fase 1. Entidades, regras e estados. Relay (TypeScript) + `RelayRegistrar` (Go).

## Spoke (registro do relay)

- **Campos**: `spokeId` (string, único), `besuRpc` (URL http), `besuWs` (URL ws), `gatewayUrl`
  (URL http do gateway do spoke). Opcional: `label`.
- **Regras**: `spokeId` não-vazio e único; URLs não-vazias e bem-formadas; payload sem campo
  obrigatório → inválido (rejeição).

## SpokeRegistry (em memória)

- **Estado**: `Map<spokeId, Spoke>` + handles associados (conector/watcher/rota) por spoke.
- **Operações**: `hydrate(fromStore)` (boot), `upsert(spoke)` (idempotente por id), `get(spokeId)`,
  `list()`.
- **Regras**: `upsert` do mesmo id **não** duplica conector/rota (idempotência); tamanho ≥ 0 (boot
  neutro).

## RelayStore (persistido — JSON)

- **Estado**: conjunto durável de `Spoke` num arquivo JSON (caminho configurável no volume).
- **Operações**: `load()` (boot; ausência ⇒ vazio, sem erro), `save(spokes)` (escrita atômica:
  tmp + rename).
- **Regras**: fonte de verdade para recarga no boot (SC-003); o `cacti-relay-store.json` legado
  **não** é usado.

## GatewayRoute

- **Estado**: derivado do registry — `spoke_out → gatewayUrl`.
- **Regras**: roteamento cross-currency resolve por `spoke_out`; `spoke_out` ausente do registry →
  rejeição com erro claro (não roteia para CB fixo).

## CircuitBreakerCheck

- **Entrada**: `amm_address` (vindo do payload de bridge-out) + `provider` (ethers).
- **Saída**: `paused bool` **ou erro** → o chamador trata erro como **pausado** (falha segura).
- **Regras**: lê `isPaused()` **on-chain** no `amm_address` **antes** de encaminhar (não confia em
  flag do payload); `paused`, `amm_address` ausente/inválido, ou leitura indisponível ⇒ recusa
  encaminhar (motivo: circuit breaker / falha segura).

## Estados de um encaminhamento (swap/bridge-out)

```
recebido → valida payload (inclui amm_address) → resolve spoke_out → lê isPaused(amm_address)
  ├─ payload inválido        → rejeitado (400)
  ├─ spoke_out desconhecido  → rejeitado (erro claro)
  ├─ amm_address ausente/inv → recusado (falha segura)
  ├─ paused | leitura falhou → recusado (circuit breaker; falha segura)
  └─ ativo                   → encaminhado ao gateway do spoke_out
```

## RelayRegistrar (Go — toolkit)

- **Interface**: `Register(ctx, spoke) error` (idempotente), onde `spoke` = {id, besuRpc, besuWs,
  gatewayUrl}; opcional `List(ctx) ([]Spoke, error)` na impl. local para asserção de teste.
- **Impl. local**: `map[id]Spoke` protegido por lock; `Register` faz upsert idempotente.
- **Impl. prod (stub)**: `Register` faz `POST /api/v1/spokes` (JSON) via `net/http` ao relay.
- **Erros tipados**: `ErrUnsupportedURI`, `ErrInvalidSpoke`, `ErrNotImplemented` (se aplicável ao
  stub sem endpoint configurado).
- **Regras**: sem segredos persistidos; factory por URI (`local` | `relay://…`).

## Transições / persistência

Relay: registry em memória hidratado do RelayStore no boot; `upsert` persiste. Go local: só memória
(sem persistência). Sem migração de dados.
