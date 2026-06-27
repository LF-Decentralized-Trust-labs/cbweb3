# Data Model: MD-3 — Campos Spoke-Keyed no FXAgreement (Scenario A)

**Data**: 2026-06-27

Não há novo modelo de dados nesta feature — o MD-2 já definiu o schema de banco. Este documento registra o modelo canônico atual em todas as camadas.

---

## Entidades

### FXAgreement (domínio)

Representa um acordo de câmbio bilateral entre dois spokes do Scenario A. Após MD-1/MD-2/MD-3, os legs são identificados por spoke id, não por posição.

| Campo | Tipo | Descrição |
|---|---|---|
| `trade_id` | string | Identificador único do trade (UUID) |
| `originator` | string | Entidade originadora do acordo |
| `counterparty_b` | string | Contraparte do acordo |
| `settlement_agent` | string | Agente de liquidação |
| `custodian` | string | Custodiante |
| `beneficiary` | string | Beneficiário |
| `origin_amount` | string | Valor no spoke de origem |
| `counter_amount` | string | Valor no spoke de destino |
| `origin_currency` | string | Moeda de origem |
| `counter_currency` | string | Moeda de destino |
| `rate` | string | Taxa de câmbio |
| **`source_spoke_id`** | string | ID do spoke de origem (ex: `"spoke-brl"`) — **novo** |
| **`dest_spoke_id`** | string | ID do spoke de destino (ex: `"spoke-usd"`) — **novo** |
| **`source_receiver`** | string | Identidade Paladin no spoke de origem — **novo** |
| **`dest_receiver`** | string | Identidade Paladin no spoke de destino — **novo** |
| `expiry_date` | uint64 | Timestamp de expiração (Unix seconds) |
| `state` | enum | `proposed` → `accepted` → `settled` / `expired` |
| `group_id` | string | Contexto bilateral Pente |
| `contract_address` | string | Endereço do contrato FXAgreement on-chain |

**Campos removidos (reservados no proto)**: `spoke_a_receiver` (field 17), `spoke_b_receiver` (field 18)

---

### Mapeamento entre camadas

```
Proto message          →  Go domain struct       →  DB column (GORM)
─────────────────────────────────────────────────────────────────────
source_spoke_id (21)   →  SourceSpokeId          →  source_spoke_id
dest_spoke_id (22)     →  DestSpokeId            →  dest_spoke_id
source_receiver (23)   →  SourceReceiver         →  source_receiver
dest_receiver (24)     →  DestReceiver           →  dest_receiver
```

```
Proto message          →  TypeScript interface    →  REST JSON key
─────────────────────────────────────────────────────────────────────
source_spoke_id (21)   →  source_spoke_id         →  source_spoke_id
dest_spoke_id (22)     →  dest_spoke_id           →  dest_spoke_id
source_receiver (23)   →  source_receiver         →  source_receiver
dest_receiver (24)     →  dest_receiver           →  dest_receiver
```

```
REST JSON              →  Relay (TypeScript)
────────────────────────────────────────────
source_spoke_id        →  sourceSpokeId  (camelCase interno)
dest_spoke_id          →  destSpokeId
source_receiver        →  sourceReceiver
dest_receiver          →  destReceiver
```

---

## Validações

- `source_spoke_id` e `dest_spoke_id` DEVEM ser non-empty ao propor um FX Agreement
- `source_receiver` e `dest_receiver` DEVEM ser non-empty ao propor um FX Agreement
- `source_spoke_id` ≠ `dest_spoke_id` (um trade não pode ser intra-spoke)
- Nenhum dos quatro campos aceita PII ou valores monetários — são identificadores de roteamento

---

## Estado de transição

O ciclo de vida do `FXAgreement.state` não muda nesta feature:

```
proposed → accepted → settled
        ↘         ↘
         expired    expired
```

Os novos campos de spoke são imutáveis após a criação do acordo.
