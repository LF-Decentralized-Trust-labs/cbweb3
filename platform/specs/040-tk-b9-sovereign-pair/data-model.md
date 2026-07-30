# Phase 1 — Data Model: TK-B9 par soberano

Sem persistência nova além do estado por step do motor (TK-B6). "Entidades" = a extensão de config, o
step set soberano e a leitura on-chain de idempotência.

## SpokeConfig — extensão (Pair)

Campos acrescentados ao `SpokeConfig` (TK-B7), preenchidos por `applyFoundSpoke` quando `spec.pair`
existe. Efeitos externos via o executor injetável.

| Campo | Tipo | Origem | Uso |
|---|---|---|---|
| `Pair` | `*PairConfig` | `spec.pair` | dispara a cauda soberana; nil → não anexa |
| `Environment` | `string` | `spec.environment` | `seed-oracle` local-only; gate de `pending` |
| `HubPairRegistry` | `string` | hub bundle | `proposePair`/`confirmPair`/`getPair` |
| `HubManualOracle` | `string` | hub bundle | `seed-oracle` (`setRate`) |
| `HubIdentityRegistry` | `string` | hub bundle | construtor de AMM/LCR; `setCentralBankOf` |
| `RelayerAddr` | `string` | flag/relay | grant `CENTRAL_BANK_ROLE` ao relayer |
| `HubAdminKey` | `string` | local (KeyProvider) | scaffolding (deploy/grants) — ato de admin do hub |
| `CBHubKey` | `string` | local (KeyProvider) | ato soberano do CB corrente (propose/confirm/commit) |

### PairConfig

| Campo | Tipo | Origem | Uso |
|---|---|---|---|
| `ProposerCB` | `string` | `spec.pair.proposerCB` | papel: proponente (scaffolding + propose) |
| `ConfirmerCB` | `string` | `spec.pair.confirmerCB` | papel: confirmador (confirm) |
| `SymbolA` | `string` | `spec.pair.symbolA` | W-token A, pairId, oracle |
| `SymbolB` | `string` | `spec.pair.symbolB` | W-token B, pairId, oracle |
| `Rate` | `string` | `spec.pair` (opcional) | `seed-oracle` (local) |
| `CurrentCB` | `string` | `spec.bankId`/CB do spoke | discrimina o papel neste run |

`pairId` determinístico: `"W-" + SymbolA + "-" + SymbolB`.

## Step set soberano (anexado ao found-spoke quando Pair != nil)

Todos `Soft: true`. Ordem: após `add-noc-agent`; `emit-spoke-bundle` depende deles.

| Step | Deps | Check (idempotência on-chain) | Run (ato do CB corrente) |
|---|---|---|---|
| `open-sovereign-pair` | add-noc-agent | `getPair(pairId).status`: `ACTIVE`→skip; `PROPOSED`+sou proponente→skip; senão run | proponente: scaffolding (forge create W-tokens dedup/AMM/LCR + `cast setCentralBankOf`/grant) + `cast proposePair`; confirmador: `cast confirmPair` (só se `PROPOSED`); pré-condição ausente → `pending` (erro soft) |
| `commit-liquidity` | open-sovereign-pair | `getPendingCommit(poolPair, side)` do lado do CB → skip se já existe | `cast registerCommit(poolPair, side, amount, wToken)` **só** do lado da moeda do CB corrente |
| `seed-oracle` | open-sovereign-pair | não-`local` → skip; (opcional) `getRate` já setado → skip | `cast setRate(tokenA, tokenB, rate)` no `ManualOracle` (local-only) |

Notas:
- `open-sovereign-pair` **não** confirma no run do proponente (não detém a chave de CB-B) → fica
  `PROPOSED`/`pending` até o run de CB-B. Soberania estrita (Q2).
- Soft: qualquer falha vira `soft-failed`; não bloqueia o found-spoke nem o bundle.

## Leitura on-chain (pairstate.go)

- `pairStatus(runner, rpc, pairRegistry, pairId) (status string, exists bool, err error)` — via
  `cast call <pairRegistry> "getPair(string)" <pairId>`; parse do enum (`PROPOSED`/`ACTIVE`); revert/
  vazio → `exists=false`.
- `pairID(symbolA, symbolB) string` — `"W-"+A+"-"+B` (determinístico).
- Injetável pelo executor (`CommandRunner`) → testável com FakeRunner (Outputs simulam o stdout do
  `cast call`).

## Validações (reuso manifest)

- `spec.pair` já existe no manifesto (`proposerCB`/`confirmerCB`/`symbolA`/`symbolB`), opcional em
  found-spoke; sem par → cauda não anexa (FR-001/SC-001).
- Scan de segredos: chaves (`HubAdminKey`/`CBHubKey`) **nunca** em manifesto/estado/bundle — vêm do
  KeyProvider/flags em runtime; nunca serializadas.
