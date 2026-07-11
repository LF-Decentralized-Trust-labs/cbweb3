# Contract — sovereign-pair tail (TK-B9)

Cauda soberana **soft** do `found-spoke`, ativada quando `spec.pair` existe. Sem novo modo/CLI: roda
dentro do `apply` do `found-spoke`.

## Gatilho

- `spec.pair` presente → `FoundSpokeSteps` anexa `open-sovereign-pair` → `commit-liquidity` →
  `seed-oracle` (todos `Soft`), com `Deps` em `add-noc-agent`; `emit-spoke-bundle` depende deles.
- `spec.pair` ausente → nenhum dos três é anexado (SC-001).
- `--dry-run`: os três aparecem como `planned`; nenhum efeito on-chain (FR-011).

## `orchestrator.sovereignPairSteps(cfg SpokeConfig) []Step`

### open-sovereign-pair (Soft)
- **Check**: `pairStatus(pairId)` — `ACTIVE`→skip; `PROPOSED` & sou proponente→skip; caso contrário run.
- **Run (proponente = CB-A)**:
  1. scaffolding (idempotente por moeda): `forge create` dos W-tokens que faltarem (dedup por símbolo),
     da AMM `(tokenA, tokenB, hubIdentityRegistry)` e do `LiquidityCommitRegistry(hubIdentityRegistry)`
     (reutiliza se `HubLCR` já setado) — assinado com `HubAdminKey`.
  2. `cast send <LCR-owner/registry> setCentralBankOf(tokenA, cbA)` / `setCentralBankOf(tokenB, cbB)`.
  3. `cast send <tokenA/tokenB> grantRole(CENTRAL_BANK_ROLE, relayer)` se `RelayerAddr` != "".
  4. `cast send <pairRegistry> proposePair(pairId, tokenA, tokenB, amm)` — assinado com `CBHubKey`.
- **Run (confirmador = CB-B)**: só se `getPair == PROPOSED` → `cast send <pairRegistry>
  confirmPair(pairId)` com `CBHubKey`. Se o par ainda não existe → **pending** (erro soft, não fatal).
- **Invariantes**: nenhum run usa a chave da contraparte; `pairId` = `W-<A>-<B>`; W-token dedup por
  moeda (SC-003).

### commit-liquidity (Soft)
- **Check**: `getPendingCommit(poolPair, side-do-CB)` já existe → skip.
- **Run**: `cast send <LCR> registerCommit(poolPair, side, amount, wToken)` **só** do lado da moeda do
  CB corrente (SC-004). O relay casa `CommitMatched` ao ver ambos os lados.

### seed-oracle (Soft)
- **Check**: `Environment != "local"` → skip; (opcional) `getRate` já setado → skip.
- **Run**: `cast send <manualOracle> setRate(tokenA, tokenB, rate)` (local-only, FR-009).

## `orchestrator` helpers (pairstate.go)
- `pairID(symbolA, symbolB) string` → `"W-"+A+"-"+B`.
- `pairStatus(ctx, runner, rpc, pairRegistry, pairId) (status string, exists bool, err error)` via
  `cast call ... getPair(string)`; injetável (FakeRunner).

## Invariantes asseguráveis por teste
1. Sem `spec.pair` → cauda não anexada (SC-001).
2. Proponente: scaffolding (dedup por moeda) + `proposePair`; **não** confirma (sem chave de CB-B);
   par fica `PROPOSED`/pending (SC-002, SC-007).
3. Confirmador: `confirmPair` só se `PROPOSED`; `ACTIVE`→skip (idempotente, SC-002).
4. W-token reutilizado entre corredores (SC-003).
5. commit-liquidity registra só o lado do CB; re-run não duplica (SC-004).
6. seed-oracle setRate em `local`; skip fora de `local` (SC-005).
7. Falha de qualquer step é `soft-failed`; found-spoke e emit-spoke-bundle prosseguem (SC-006).
8. `--dry-run` não aciona `cast`/`forge` (FR-011).
9. E2E (`e2e` tag): CB-A propõe, CB-B confirma (→ACTIVE), ambos commitam (relay casa), oráculo semeado;
   ausente → skip (SC-008).
