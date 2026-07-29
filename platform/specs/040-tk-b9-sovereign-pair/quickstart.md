# Quickstart — TK-B9 par soberano + liquidez cooperativa + seed-oracle

Pré-requisito: um **hub fundado** (TK-B6) e **dois spokes** (TK-B7) — CB-A e CB-B — cujos manifestos
`found-spoke` carreguem o **mesmo** `spec.pair`. O relay (TK-B5/found-hub) precisa estar no ar para
casar os commits.

## 1. Manifesto (trecho `spec.pair` em ambos os found-spoke)

```yaml
spec:
  # ... found-spoke normal ...
  pair:
    proposerCB: central-bank-a     # CB de tokenA (símbolo A)
    confirmerCB: central-bank-b     # CB de tokenB (símbolo B)
    symbolA: BRL
    symbolB: ARS
    # rate: "..."                    # opcional, seed-oracle (local)
```

Sem o bloco `pair`, a cauda soberana **não** roda.

## 2. Dry-run (sem efeitos)

```bash
cd scenario-b/toolkit
go build -o /tmp/cbweb3b ./cmd/cbweb3b

/tmp/cbweb3b apply -f ./central-bank-a.found-spoke.yaml --dry-run --data-dir /tmp/cba
```

Esperado: além dos steps do found-spoke, `open-sovereign-pair`, `commit-liquidity`, `seed-oracle`
aparecem como `planned`; nenhum `cast`/`forge` é executado.

## 3. Execução real — soberania estrita (dois runs)

```bash
# CB-A funda o spoke E propõe o par (scaffolding + proposePair). NÃO confirma (sem chave de CB-B).
/tmp/cbweb3b apply -f ./central-bank-a.found-spoke.yaml --data-dir ./.data/spoke-a/cb-a --repo-root .
# → par fica PROPOSED (open-sovereign-pair do CB-A: soft, "pending" quanto à confirmação)

# CB-B funda o seu spoke E confirma o par (confirmPair).
/tmp/cbweb3b apply -f ./central-bank-b.found-spoke.yaml --data-dir ./.data/spoke-b/cb-b --repo-root .
# → par vira ACTIVE
```

Cada CB contribui liquidez **só da sua moeda** (`commit-liquidity`); o relay casa `CommitMatched` ao
observar os dois lados. `seed-oracle` semeia a taxa (apenas em `local`).

## 4. Verificações

```bash
PAIR="W-BRL-ARS"; PR=<pairRegistry-addr>; RPC=http://host.docker.internal:8845
# status do par: PROPOSED após CB-A; ACTIVE após CB-B
cast call $PR "getPair(string)" "$PAIR" --rpc-url $RPC
# oráculo com taxa (local)
cast call <manualOracle> "getRate(address,address)" <tokenA> <tokenB> --rpc-url $RPC
```

Report do toolkit: `open-sovereign-pair`/`commit-liquidity`/`seed-oracle` como `done`/`skipped`/
`soft-failed`; nunca bloqueiam o found-spoke (o spoke bundle é emitido de todo modo).

## 5. Idempotência e soberania

- Re-`apply` de CB-A com o par já `ACTIVE` → `open-sovereign-pair` **pula**.
- Re-`apply` de CB-A com o par `PROPOSED` → **pula** (já propôs; aguarda CB-B).
- CB-A **nunca** confirma (não detém a chave de CB-B); o par fica `PROPOSED`/pending até o run de CB-B.
- W-token de uma moeda já implantada em outro corredor é **reutilizado** (dedup por moeda).

## 6. E2E (opcional)

```bash
CBWEB3B_E2E_PAIR_CBA_MANIFEST=./central-bank-a.found-spoke.yaml \
CBWEB3B_E2E_PAIR_CBB_MANIFEST=./central-bank-b.found-spoke.yaml \
CBWEB3B_E2E_REPO_ROOT=$(git rev-parse --show-toplevel) \
CBWEB3B_E2E_HUB_RPC=http://host.docker.internal:8845 \
  go test -tags e2e ./tests/e2e/ -run TestSovereignPair -v
```

Ambiente ausente (Docker/Foundry/hub/relay) ⇒ **skip com aviso** (nunca falso verde).
