# Interface Contract: Payment Orchestrator gRPC (Scenario A)

**Serviço**: `PaymentOrchestrator` — `scenario-a/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto`
**Consumidores**: `api-gateway` (via vendor), relay Cacti (`htlc-relay.ts`), testes de integração

---

## Método: ProposeFXAgreement

**Request** — `ProposeFXAgreementRequest`:

| Campo | Field # | Tipo | Obrigatório | Descrição |
|---|---|---|---|---|
| `trade_id` | 1 | string | não | UUID do trade; gerado pelo servidor se vazio |
| `counterparty_b` | 2 | string | sim | Contraparte |
| `originator` | 10 | string | sim | Originador |
| `settlement_agent` | 3 | string | sim | Agente de liquidação |
| `custodian` | 4 | string | sim | Custodiante |
| `beneficiary` | 5 | string | sim | Beneficiário |
| `origin_amount` | 6 | string | sim | Valor de origem |
| `counter_amount` | 7 | string | sim | Valor de destino |
| `origin_currency` | 8 | string | sim | Moeda de origem |
| `counter_currency` | 9 | string | sim | Moeda de destino |
| `rate` | 11 | string | sim | Taxa de câmbio |
| `expiry_date` | 12 | uint64 | sim | Timestamp Unix de expiração |
| `on_behalf` | 13 | bool | não | Flag de proposta em nome de terceiro |
| ~~`spoke_a_receiver`~~ | ~~14~~ | — | — | **Reservado. NÃO usar.** |
| ~~`spoke_b_receiver`~~ | ~~15~~ | — | — | **Reservado. NÃO usar.** |
| **`source_spoke_id`** | 16 | string | sim | ID do spoke de origem |
| **`dest_spoke_id`** | 17 | string | sim | ID do spoke de destino |
| **`source_receiver`** | 18 | string | sim | Identidade Paladin no spoke de origem |
| **`dest_receiver`** | 19 | string | sim | Identidade Paladin no spoke de destino |

**Response** — `ProposeFXAgreementResponse`:
Retorna `FXAgreement` com todos os campos preenchidos, incluindo `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`.

---

## Mensagem: FXAgreement

Usada em respostas de consulta e notificações de evento.

Campos spoke-keyed:

| Campo | Field # | Tipo | Descrição |
|---|---|---|---|
| **`source_spoke_id`** | 21 | string | ID do spoke de origem |
| **`dest_spoke_id`** | 22 | string | ID do spoke de destino |
| **`source_receiver`** | 23 | string | Identidade Paladin no spoke de origem |
| **`dest_receiver`** | 24 | string | Identidade Paladin no spoke de destino |
| ~~`spoke_a_receiver`~~ | ~~17~~ | — | **Reservado. Retorna vazio.** |
| ~~`spoke_b_receiver`~~ | ~~18~~ | — | **Reservado. Retorna vazio.** |

---

## Compatibilidade

- Os field numbers 17 e 18 estão **reservados** — não serão reutilizados. Clientes antigos que acessem esses fields via reflection receberão string vazia.
- Clientes que ainda enviam `spoke_a_receiver`/`spoke_b_receiver` NÃO terão esses valores persistidos — os fields são ignorados pelo protobuf (reserved).
- O vendor do `api-gateway` DEVE ser atualizado para que o código Go compile sem referências a campos inexistentes.

---

## REST (via api-gateway)

O `api-gateway` faz proxy das chamadas gRPC via REST. O payload JSON expõe os mesmos campos com snake_case:

```json
{
  "source_spoke_id": "spoke-brl",
  "dest_spoke_id":   "spoke-usd",
  "source_receiver": "funded_operator@spoke-brl-bank-a",
  "dest_receiver":   "funded_operator@spoke-usd-bank-b"
}
```

O relay consome esta interface via `pollFXAgreementsRest()`.
