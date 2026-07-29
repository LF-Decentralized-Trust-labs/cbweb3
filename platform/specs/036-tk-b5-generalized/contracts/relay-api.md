# Contracts — TK-B5 (endpoint do relay + API Go do RelayRegistrar)

Fase 1. Contrato do endpoint de registro em runtime e da interface Go plugável.

## HTTP — `POST /api/v1/spokes` (relay)

Registra (upsert idempotente) um spoke em runtime; cria conector/watcher + rota sem restart.

**Request** (JSON):
```json
{
  "spokeId": "spoke-br",
  "besuRpc": "http://host.docker.internal:8855",
  "besuWs":  "ws://host.docker.internal:8856",
  "gatewayUrl": "http://host.docker.internal:8090"
}
```

**Respostas**:
| Status | Quando | Corpo |
|---|---|---|
| `200 OK` | registrado/atualizado (idempotente) | `{ "status": "registered", "spokeId": "..." }` |
| `400 Bad Request` | payload inválido (campo faltando/URL malformada) | `{ "error": "..." }` (store inalterado) |
| `401/403` | segredo compartilhado ausente/errado (auth atual, fallback) | `{ "error": "..." }` |

**Invariantes**: idempotente por `spokeId` (re-registro não duplica conector/rota); em `400`, o
RelayStore **não** é alterado; auth atual = `X-Relay-Auth` (segredo compartilhado; auth-por-CB fora
de escopo).

## Roteamento — `POST /api/v1/cross-currency/bridge-out` (existente, generalizado)

Payload (campos relevantes): `correlation_id`, `swap_tx_hash`, `amount_out`, `beneficiary_bank_id`,
`spoke_out`, **`amm_address`** (endereço do AMM do par).

- Resolve o destino por **lookup de `spoke_out` → `gatewayUrl`** no registry (não CB fixo).
- `spoke_out` desconhecido → `400`/erro claro (spoke não registrado).
- **Antes de encaminhar**: lê `isPaused()` **on-chain** no `amm_address` do payload; `paused`,
  `amm_address` ausente/inválido, ou leitura indisponível → recusa (`409`/erro "circuit breaker"),
  **falha segura** (não confia em flag do payload).

## Go — package `engine/relayregistrar`

```go
// Spoke é o registro mínimo de um spoke no relay.
type Spoke struct {
    ID         string
    BesuRPC    string
    BesuWS     string
    GatewayURL string
}

// RelayRegistrar registra spokes no relay (fronteira plugável).
type RelayRegistrar interface {
    // Register faz upsert idempotente do spoke.
    Register(ctx context.Context, s Spoke) error
}

// LocalRegistry (impl. local) expõe List para asserção em teste.
type LocalRegistry interface {
    RelayRegistrar
    List() []Spoke
}

// New seleciona a implementação pela URI:
//   local              -> in-memory (idempotente)
//   relay://<host>[...] -> stub de produção (POST /api/v1/spokes via net/http)
//   outro              -> ErrUnsupportedURI
func New(uri string) (RelayRegistrar, error)

var (
    ErrUnsupportedURI = errors.New("relayregistrar: unsupported factory URI")
    ErrInvalidSpoke   = errors.New("relayregistrar: invalid spoke (id/rpc/ws/gateway required)")
    ErrNotImplemented = errors.New("relayregistrar: production registrar requires a relay endpoint")
)
```

**Invariantes de contrato**:
- `Register` valida `Spoke` (todos os campos obrigatórios) → `ErrInvalidSpoke` se faltar.
- Local: `Register` idempotente por `ID`; `List()` reflete o conjunto.
- Prod stub: `Register` monta o JSON acima e faz `POST /api/v1/spokes`; sem endpoint configurado →
  `ErrNotImplemented`.
- Nenhum segredo é persistido pela interface.
