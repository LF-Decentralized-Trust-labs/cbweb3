# Data Model: DB Migration — Spoke-Keyed Leg Fields

**Phase 1 output** | Branch: `021-db-migration-spoke-keyed`

---

## Tabela: `fx_agreements`

### Estado ANTES da migração (schema legado)

| Coluna              | Tipo    | Descrição                                              |
|---------------------|---------|--------------------------------------------------------|
| trade_id            | TEXT PK | Identificador único do trade                           |
| originator          | TEXT    | Entidade que propõe o trade                            |
| counterparty_b      | TEXT    | Contraparte do trade                                   |
| settlement_agent    | TEXT    | Agente de liquidação                                   |
| custodian           | TEXT    | Custodiante                                            |
| beneficiary         | TEXT    | Beneficiário                                           |
| origin_amount       | TEXT    | Valor na moeda de origem                               |
| counter_amount      | TEXT    | Valor na moeda de destino                              |
| origin_currency     | TEXT    | Moeda de origem                                        |
| counter_currency    | TEXT    | Moeda de destino                                       |
| rate                | TEXT    | Taxa de câmbio                                         |
| **spoke_a_receiver**| TEXT    | ⚠️ LEGADO — receptor no spoke posicional A             |
| **spoke_b_receiver**| TEXT    | ⚠️ LEGADO — receptor no spoke posicional B             |
| expiry_date         | INTEGER | Timestamp de expiração do trade                        |
| state               | TEXT    | Estado do trade (PROPOSED, ACCEPTED, SETTLED, …)       |
| on_chain_tx_hash    | TEXT    | Hash da transação on-chain                             |
| group_id            | TEXT    | ID do grupo Paladin/Pente                              |
| contract_address    | TEXT    | Endereço do contrato FXAgreement                       |
| created_at          | DATETIME| Timestamp de criação                                   |
| updated_at          | DATETIME| Timestamp de última atualização                        |

---

### Estado DEPOIS da migração (schema alvo)

| Coluna               | Tipo    | Descrição                                                        |
|----------------------|---------|------------------------------------------------------------------|
| trade_id             | TEXT PK | Identificador único do trade                                     |
| originator           | TEXT    | Entidade que propõe o trade                                      |
| counterparty_b       | TEXT    | Contraparte do trade                                             |
| settlement_agent     | TEXT    | Agente de liquidação                                             |
| custodian            | TEXT    | Custodiante                                                      |
| beneficiary          | TEXT    | Beneficiário                                                     |
| origin_amount        | TEXT    | Valor na moeda de origem                                         |
| counter_amount       | TEXT    | Valor na moeda de destino                                        |
| origin_currency      | TEXT    | Moeda de origem                                                  |
| counter_currency     | TEXT    | Moeda de destino                                                 |
| rate                 | TEXT    | Taxa de câmbio                                                   |
| **source_spoke_id**  | TEXT    | ✅ NOVO — ID do spoke de origem (ex.: `"spoke-a"`, `"spoke-brl"`) |
| **dest_spoke_id**    | TEXT    | ✅ NOVO — ID do spoke de destino (ex.: `"spoke-b"`, `"spoke-usd"`) |
| **source_receiver**  | TEXT    | ✅ NOVO — identidade Paladin no spoke de origem                   |
| **dest_receiver**    | TEXT    | ✅ NOVO — identidade Paladin no spoke de destino                  |
| expiry_date          | INTEGER | Timestamp de expiração do trade                                  |
| state                | TEXT    | Estado do trade (PROPOSED, ACCEPTED, SETTLED, …)                 |
| on_chain_tx_hash     | TEXT    | Hash da transação on-chain                                       |
| group_id             | TEXT    | ID do grupo Paladin/Pente                                        |
| contract_address     | TEXT    | Endereço do contrato FXAgreement                                 |
| created_at           | DATETIME| Timestamp de criação                                             |
| updated_at           | DATETIME| Timestamp de última atualização                                  |

---

## Regra de Backfill (registros legados → schema alvo)

```
spoke_a_receiver  →  source_receiver   (COALESCE para '' se NULL)
spoke_b_receiver  →  dest_receiver     (COALESCE para '' se NULL)
source_spoke_id   ←  'spoke-a'         (valor fixo; o modelo legado só suportava spoke A)
dest_spoke_id     ←  'spoke-b'         (valor fixo; o modelo legado só suportava spoke B)
```

A regra só é aplicada a registros onde `source_spoke_id IS NULL OR source_spoke_id = ''` (não-migrados).

---

## Estados de migração possíveis

| hasA | hasB | Descrição                                  | Ação                                           |
|------|------|--------------------------------------------|------------------------------------------------|
| true | true | Schema legado completo                     | Backfill + DROP A + DROP B (transação única)   |
| false| true | Estado parcial (A já removido, B remanescente) | Pular backfill + DROP B (dados já migrados) |
| true | false| Estado impossível com o código original    | Pular backfill + DROP A (defensive coding)     |
| false| false| Schema alvo; já migrado ou novo            | No-op                                          |

---

## Tabelas não afetadas

- **`fx_agreement_events`**: registra apenas transições de estado (from_state, to_state, actor). Não contém campos de receiver; não requer migração.
- **`relay_delivery_records`**: rastreia entregas do relay. Usa `source_spoke` / `target_spoke` (campos já spoke-keyed por design); não é afetada.
