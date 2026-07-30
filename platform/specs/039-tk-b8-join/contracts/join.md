# Contract — `join` mode (TK-B8)

## CLI

```
cbweb3b apply -f <join-manifest.yaml> [--dry-run] [-o json|yaml]
              [--data-dir <dir>] [--out-dir <dir>] [--repo-root <dir>]
              [--spoke-rpc <url>]     # RPC do nó DO BANCO — gate wait-sync
```

- `spec.mode: join` → dispatch `applyJoin`.
- Erro cedo (exit 1) se o spoke bundle (`spec.joinBundleRef`) for inválido/ausente — **antes** de
  qualquer efeito (FR-001/SC-001).
- `--dry-run`: todos os steps `planned`, nenhum efeito externo, nenhum RPC (FR-012).
- `node.validator: true`: **warning** no report de validação; o join prossegue como não-validador.
- Exit codes: `0` sucesso (done|skipped|planned); `1` config/step falho; `2` uso.

## `orchestrator.JoinSteps(cfg JoinConfig) []Step`

Retorna o step set canônico (ver data-model.md) em ordem de dependência. Contratos por step:

- **consume-spoke-bundle**: `bundle.LoadSpoke(cfg.SpokeBundlePath)`; erro → falha clara.
- **write-genesis**: escreve `SpokeBundle.Genesis` em `<GenesisDir>/genesis.json`.
  - Check: se existe e `sha256(conteúdo) == sha256(bundle.Genesis)` → skip; se difere → **erro**
    (`genesis mismatch: bank must run the spoke genesis`).
- **start-besu-join**: `docker compose -f entity-besu.compose.yaml --env-file <env> up -d` (não-
  validador; bootnode = enode do bundle) + `WaitRPC`.
- **wait-sync**: `WaitSync` — conclui só com `eth_syncing==false && block>0`; timeout → erro.
- **wire-addresses**: `addrs.AppendAddr` (upsert) dos endereços de spoke do bundle no `.env.bank`.
- **provision-keycloak-bank**: compose keycloak + `WaitKeycloak` + `ReadClientSecret` → write-back
  (`KEYCLOAK_CLIENT_SECRET`); Check idempotente por chave presente.
- **render-bank-env / start-bank-{infra,backend,frontend}**: compose dos serviços.
- **gen-csr**: MkdirAll `<dataDir>/pki` `0700` (host) + `pki.GenerateBankCSR(bankId, displayName,
  <dataDir>/pki)`; Check: pula se `{bank}.key` **e** `{bank}.csr` já existem.
  - Garantias: key `0600`, CSR `OU=ROLE_COMMERCIAL_BANK`/CN=`bankId`; **zero** `*-ca.*`; a chave
    privada nunca é transmitida nem serializada em estado/bundle.

## `orchestrator.EthSyncing` (seam) + `waitSync`

```
type EthSyncing func(ctx context.Context, rpcURL string) (syncing bool, block uint64, err error)
```

- `ethSyncing` (default): POST `eth_syncing` + `eth_blockNumber` ao `rpcURL`.
- `waitSync(ctx, rpcURL, timeout, poll, EthSyncing)`: polling até `!syncing && block>0`; erro claro no
  timeout. Injetável para teste (fake retornando sequência syncing→synced).

## Invariantes (asseguráveis por teste)

1. Bundle inválido → `applyJoin` erra antes de efeito (SC-001).
2. `write-genesis` re-run com mesmo genesis → skip; genesis divergente → erro (SC-002).
3. `wait-sync` só passa sincronizado; nó não é validador QBFT (SC-003).
4. `wire-addresses`/`provision-keycloak-bank` idempotentes (SC-004).
5. `gen-csr`: key `0600`, OU correta, idempotente, **zero** CA material (SC-005).
6. `node.validator: true` → warning, join como não-validador (SC-006).
7. `--dry-run` não aciona Docker/RPC (FR-012).
8. E2E (`e2e` tag): banco sincroniza contra spoke fundado + CSR gerado; ausente → skip (SC-007).
