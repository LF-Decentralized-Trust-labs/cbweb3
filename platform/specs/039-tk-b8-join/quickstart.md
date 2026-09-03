# Quickstart — TK-B8 join (full node não-validador)

Pré-requisito: um **spoke fundado** (TK-B7) que emitiu `bundles/spoke-<id>.bundle.yaml`.

## 1. Dry-run (sem efeitos)

```bash
cd scenario-b/toolkit
go build -o /tmp/cbweb3b ./cmd/cbweb3b

/tmp/cbweb3b apply \
  -f engine/manifest/testdata/join.yaml \
  --dry-run \
  --data-dir /tmp/bank-a
```

Esperado: report com todos os steps `planned` (consume-spoke-bundle → write-genesis → start-besu-join
→ wait-sync → wire-addresses → provision-keycloak-bank → render-bank-env → infra/backend/frontend →
gen-csr); **nenhum** efeito externo; bundle inválido/ausente → exit 1 com erro claro.

## 2. Execução real (contra um spoke fundado)

```bash
/tmp/cbweb3b apply \
  -f ./bank-a.join.yaml \
  --data-dir ./.data/spoke-a/bank-a \
  --spoke-rpc http://host.docker.internal:8646 \
  --repo-root .
```

Fluxo: escreve o genesis do bundle (guard não-destrutivo), sobe o nó do banco (não-validador),
**aguarda a sincronização**, conecta os endereços de spoke, provisiona o Keycloak do banco, sobe
infra/backend/frontend e gera o CSR local.

## 3. Verificações

```bash
# genesis idêntico ao do bundle (write-genesis não regenera)
sha256sum ./.data/spoke-a/bank-a/genesis/genesis.json

# nó sincronizado e NÃO-validador (CB é o validador único)
curl -s -X POST http://localhost:8646 -H 'content-type: application/json' \
  --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}'      # → false
curl -s -X POST http://localhost:8646 -H 'content-type: application/json' \
  --data '{"jsonrpc":"2.0","method":"qbft_getValidatorsByChainHeight","params":["latest"],"id":1}' # só o CB

# CSR gerado localmente; chave 0600; ZERO material de CA
ls -l ./.data/spoke-a/bank-a/pki/          # bank-a.key (0600), bank-a.csr
grep -c "BEGIN CERTIFICATE REQUEST" ./.data/spoke-a/bank-a/pki/bank-a.csr   # 1
find ./.data/spoke-a/bank-a/pki -name '*-ca.*' | wc -l                      # 0
```

## 4. Idempotência

Rodar `apply join` de novo converge: `consume`/`write-genesis`(hash igual)/`wait-sync`/
`provision-keycloak-bank`/`gen-csr` são pulados (Check → skip). Genesis divergente do bundle → erro
claro.

## 5. Cauda de runtime (NÃO é o toolkit)

Após o `gen-csr`, o onboarding é **runtime**: o gateway submete o CSR ao CB (`credential-request`), o
**compliance do CB assina** (gated por KYC no portal `governance`), há proof-of-possession, e a
**governança do CB registra** o endereço EVM no `IdentityRegistry`. O toolkit **não** faz nenhuma
dessas etapas.

## 6. E2E (opcional)

```bash
CBWEB3B_E2E_JOIN_MANIFEST=./bank-a.join.yaml \
CBWEB3B_E2E_REPO_ROOT=$(git rev-parse --show-toplevel) \
CBWEB3B_E2E_BANK_RPC=http://host.docker.internal:8646 \
  go test -tags e2e ./tests/e2e/ -run TestJoin -v
```

Ambiente ausente (Docker/Besu/spoke) ⇒ **skip com aviso** (nunca falso verde).
