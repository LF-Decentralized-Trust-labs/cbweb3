# Quickstart: TK-8 e TK-9 — Commercial Bank Join

## Pré-requisitos

1. Um spoke local rodando via TK-4/5 (`mode: found`)
2. Um join bundle emitido pelo TK-6: `bundles/spoke-brl.bundle.yaml`
3. `cbweb3` CLI compilada: `cd scenario-a/toolkit && go build ./cmd/cbweb3/`

## Fluxo completo (local)

### 1. Provisionar o spoke (banco central)

```bash
# Já deve estar rodando via TK-4/5
cbweb3 apply -f provisioning/examples/central-bank-brl.yaml
# Emite: bundles/spoke-brl.bundle.yaml
```

### 2. Criar o manifesto do banco comercial

```yaml
# commercial-bank-alpha.yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata: { name: commercial-bank-alpha }
spec:
  scenario: a
  environment: local
  role: commercial-bank
  mode: join
  spoke: { id: spoke-brl, chainId: 1337, currency: BRL }
  joinBundleRef: ./bundles/spoke-brl.bundle.yaml
  node:
    advertisedHost: cbweb3-spoke-brl-besu.commercial-bank-alpha
    rpc:  { port: 8746 }
    ws:   { port: 8756 }
    p2p:  { port: 31403 }
    dataDir: /var/cbweb3/commercial-bank-alpha
  image: hyperledger/besu:25.8.0
  keyProvider: kms://local-emulator
  certSource: self-signed
```

### 3. Pré-visualizar o join (dry-run)

```bash
cbweb3 apply -f commercial-bank-alpha.yaml -dry-run
# Saída: lista dos 9 passos com configurações resolvidas, sem efeitos
```

### 4. Executar o join

```bash
cbweb3 apply -f commercial-bank-alpha.yaml
```

**Saída esperada**:
```
[spoke-brl][commercial-bank-alpha] step write-genesis: running...
[spoke-brl][commercial-bank-alpha] step write-genesis: done
[spoke-brl][commercial-bank-alpha] step start-besu-join: running...
[spoke-brl][commercial-bank-alpha] step start-besu-join: done
[spoke-brl][commercial-bank-alpha] step wait-sync: running...
[spoke-brl][commercial-bank-alpha] step wait-sync: done
[spoke-brl][commercial-bank-alpha] step vote-qbft: running...
[spoke-brl][commercial-bank-alpha] step vote-qbft: done (1/1 votes collected; joiner activated at block 48)
[spoke-brl][commercial-bank-alpha] step gen-csr: done
[spoke-brl][commercial-bank-alpha] step request-cert: done
[spoke-brl][commercial-bank-alpha] step receive-cert: done
[spoke-brl][commercial-bank-alpha] step proof-of-possession: done
[spoke-brl][commercial-bank-alpha] step start-backend: done
```

### 5. Verificar o join

```bash
# Validator set inclui o banco comercial?
curl -s http://localhost:8645 \
  -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"qbft_getValidatorsByBlockNumber","params":["latest"],"id":1}'

# IdentityRegistry inclui o banco comercial?
# (via script de verificação do TK-5)
cd scenario-a && go test ./deploy/local/paladin/scripts/... -run TestQueryIdentityRegistry -v

# Re-run idempotente (todos os passos devem ser pulados):
cbweb3 apply -f commercial-bank-alpha.yaml
# Esperado: todos os passos com "skipped (already done)"
```

## Testes unitários

```bash
cd scenario-a/toolkit
go test ./engine/orchestrator/... -v -run TestRunJoin
go test ./engine/bundle/... -v -run TestBundleExtension
go test ./engine/pki/... -v
```

## Teste do template TK-8

```bash
bash scenario-a/provisioning/tests/test-commercial-bank-template.sh
```
