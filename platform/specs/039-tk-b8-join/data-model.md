# Phase 1 — Data Model: TK-B8 join

Sem persistência nova além do estado por step do motor (TK-B6). "Entidades" = as structs de config, o
step set e o gate injetável.

## JoinConfig (orchestrator)

Parametriza o modo `join`. Efeitos externos e reads via seams injetáveis (testável sem Besu/Keycloak).

| Campo | Tipo | Origem | Uso |
|---|---|---|---|
| `Runner` | `exec.CommandRunner` | apply (Real/Dry/Fake) | compose, docker |
| `TemplatesDir` | `string` | repoRoot/provisioning/templates | templates de compose (TK-B4) |
| `OutDir` | `string` | flag/manifest | (reservado; join não emite bundle) |
| `BankID` | `string` | `spec.bankId` | CN do CSR, `BANK_ID`, nomes de container |
| `Institution` | `string` | `spec.displayName` | O do CSR |
| `SpokeID` | `string` | `spec.spoke.id` | nomeação, template |
| `SpokeChainID` | `uint64` | `spec.spoke.chainId` | sanity vs bundle |
| `BankRPC` | `string` | flag `--spoke-rpc` | gate `wait-sync` |
| `SpokeBundlePath` | `string` | `spec.joinBundleRef` (resolvido) | consume/write-genesis/wire |
| `GenesisDir` | `string` | derivado do dataDir | onde o genesis do bundle é escrito |
| `DataDir` | `string` | flag/manifest | estado, lock, `pki/` |
| `BankEnvFile` | `string` | `<dataDir>/.env.bank` | wire-addresses, keycloak |
| `KeycloakEnv` | `[]string` | `[<dataDir>/.env.bank]` | write-back de secrets |
| **Seams** | | | |
| `WaitRPC` | `func(ctx) error` | default = `waitRPC(BankRPC)` | RPC no ar antes do sync |
| `WaitSync` | `func(ctx) error` | default = `waitSync(BankRPC)` via `EthSyncing` | gate de sincronização |
| `WaitKeycloak` | `func(ctx) error` | default = no-op | Keycloak pronto |
| `ReadClientSecret` | `func(ctx) (string, error)` | default = `docker exec ... cat` | write-back |
| `EthSyncing` | `EthSyncing` | default = `ethSyncing` (JSON-RPC) | injetável p/ testes |

`WithDefaults()` preenche os seams (padrão do `SpokeConfig`/`HubConfig`).

## Step set do `join` (fluxo canônico, roadmap §6)

Ordem por dependência (topoSort do motor). Todos idempotentes (Check → skip).

| Step | Deps | Check (idempotência) | Run |
|---|---|---|---|
| `consume-spoke-bundle` | — | — | `bundle.LoadSpoke(path)` (falha clara em bundle inválido) |
| `write-genesis` | consume-spoke-bundle | genesis presente **e** `sha256` == bundle → skip; hash divergente → **erro** | escreve `SpokeBundle.Genesis` em `<GenesisDir>/genesis.json` (atômico) |
| `start-besu-join` | write-genesis | — | compose `entity-besu` (não-validador, peer do enode do bundle) + `WaitRPC` |
| `wait-sync` | start-besu-join | — | `WaitSync` (bloqueia até `eth_syncing==false` e block>0) |
| `wire-addresses` | consume-spoke-bundle | — (AppendAddr é upsert) | escreve endereços de spoke do bundle no `.env.bank` |
| `provision-keycloak-bank` | wait-sync | `KEYCLOAK_CLIENT_SECRET` presente no env → skip | compose `entity-keycloak` + `WaitKeycloak` + write-back |
| `render-bank-env` | wait-sync, wire-addresses | — | consolida o `.env.bank` |
| `start-bank-infra` | render-bank-env | — | compose `entity-infra` |
| `start-bank-backend` | start-bank-infra, render-bank-env | — | compose `entity-backend` |
| `start-bank-frontend` | start-bank-backend | — | compose `entity-frontend` |
| `gen-csr` | — (cauda diferida) | `{bank}.key` **e** `{bank}.csr` existem → skip | MkdirAll `<dataDir>/pki` `0700` (host) + `pki.GenerateBankCSR` |

Notas:
- **Sem** `register-relay-bank` nem `add-noc-agent` (clarificação: fluxo canônico).
- `gen-csr` não tem dep de rede (é PKI local); pode rodar cedo, mas fica na **cauda** por semântica de
  onboarding. Sem dependências → o topoSort o coloca conforme ordem de inserção.

## EthSyncing (gate seam)

```
type EthSyncing func(ctx context.Context, rpcURL string) (syncing bool, block uint64, err error)
```

- Default `ethSyncing`: POST `eth_syncing` (bool `false` ou objeto de progresso) + `eth_blockNumber`.
- `waitSync` conclui quando `syncing == false && block > 0`; timeout → erro claro.

## CSR do banco (artefato local)

- `<dataDir>/pki/{bankId}.key` (`0600`, EC P-256, **nunca transmitida**).
- `<dataDir>/pki/{bankId}.csr` (`0644`, `CN=bankId`, `O=displayName`, `OU=ROLE_COMMERCIAL_BANK`,
  `C=BR`).
- **Zero** `*-ca.key`/`*-ca.crt` — o toolkit nunca gera CA (FR-009/SC-005).

## Validações (reuso `manifest/validate.go`, já presente)

- `join` requer `spoke`, `joinBundleRef`, `bankId` (map `requiredByMode`).
- `node.validator: true` → **warning** (não erro). Já implementado (FR-011/SC-006).
- Scan de segredos rejeita `PRIVATE KEY` em manifesto/bundle.
