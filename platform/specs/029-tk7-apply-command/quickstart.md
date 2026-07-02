# Quickstart: TK-7 — Comando `apply`

**Phase 1 output** | **Branch**: `029-tk7-apply-command`

---

## Pré-requisitos

- Go 1.26+ instalado (`go version`)
- TK-1 a TK-6 implementados (manifesto, keyProvider, certSource, compose central-bank, engine, bundle)
- Para execução live: stack local do Scenario A rodando (Besu, Paladin)
- Para dry-run: nenhum serviço externo necessário

---

## Build

```bash
# A partir da raiz do módulo
cd scenario-a/toolkit

# Build do binário
go build -o cbweb3 ./cmd/cbweb3/

# Verificar
./cbweb3 --help
# Output: Usage: cbweb3 <subcommand> [flags]
#   Subcommands: apply
```

### Instalação local (opcional)

```bash
go install ./cmd/cbweb3/
# Binário em $GOPATH/bin/cbweb3 ou $HOME/go/bin/cbweb3
```

---

## Manifesto de exemplo — central bank, perfil local

Salve como `manifests/central-bank-brl.yaml`:

```yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata:
  name: central-bank-brazil
spec:
  scenario: a
  environment: local
  role: central-bank
  mode: found
  spoke:
    id: spoke-brl
    chainId: 1337
    currency: BRL
  node:
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil
    rpc:
      port: 8645
    ws:
      port: 8655
    p2p:
      port: 31303
    dataDir: /opt/cbweb3/data/spoke-brl
  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed://local
  relay:
    endpoint: http://cbweb3-cacti:4000
```

---

## Uso

### 1. Inspecionar o plano (dry-run — sem side-effects)

```bash
./cbweb3 apply --dry-run -f manifests/central-bank-brl.yaml
```

**Saída esperada** (spoke novo, todos os steps pendentes):
```yaml
spoke: spoke-brl
mode: found
dryRun: true
status: dry-run
steps:
  - name: deploy-contracts
    status: pending
  - name: gen-tls
    status: pending
  - name: render-configs
    status: pending
  - name: register-nodes
    status: pending
  - name: start-paladin
    status: pending
  - name: create-zeto-token
    status: pending
  - name: create-pente-context
    status: pending
  - name: deploy-fxa-pente
    status: pending
  - name: onboard-registry
    status: pending
  - name: register-relay
    status: pending
```

**Dry-run funciona sem Besu rodando** — não acessa serviços externos.

### 2. Provisionar o spoke (execução live)

```bash
./cbweb3 apply -f manifests/central-bank-brl.yaml
```

**Pré-condição**: `<dataDir>/genesis/genesis.json` deve existir (gerado pelo TK-4 compose central-bank).

**Saída esperada** (spoke provisionado com sucesso):
```yaml
spoke: spoke-brl
mode: found
dryRun: false
status: success
steps:
  - name: deploy-contracts
    status: completed
    completedAt: "2026-06-27T10:00:01Z"
  - name: gen-tls
    status: completed
    completedAt: "2026-06-27T10:00:03Z"
  - name: render-configs
    status: completed
    completedAt: "2026-06-27T10:01:12Z"
  - name: register-nodes
    status: completed
    completedAt: "2026-06-27T10:01:15Z"
  - name: start-paladin
    status: completed
    completedAt: "2026-06-27T10:02:44Z"
  - name: create-zeto-token
    status: completed
    completedAt: "2026-06-27T10:03:01Z"
  - name: create-pente-context
    status: completed
    completedAt: "2026-06-27T10:03:18Z"
  - name: deploy-fxa-pente
    status: completed
    completedAt: "2026-06-27T10:03:45Z"
  - name: onboard-registry
    status: completed
    completedAt: "2026-06-27T10:04:02Z"
  - name: register-relay
    status: completed
    completedAt: "2026-06-27T10:04:05Z"
bundle:
  path: bundles/spoke-brl.bundle.yaml
```

Bundle emitido em `<outputDir>/bundles/spoke-brl.bundle.yaml`.

### 3. Re-executar (idempotente)

```bash
./cbweb3 apply -f manifests/central-bank-brl.yaml
# Todos os steps mostram status: skipped; bundle re-emitido; exit 0
```

### 4. Saída JSON para automação

```bash
# Verificar status
./cbweb3 apply -f manifests/central-bank-brl.yaml --output json | jq .status

# Extrair path do bundle
./cbweb3 apply -f manifests/central-bank-brl.yaml --output json | jq -r '.bundle.path'

# Dry-run em JSON
./cbweb3 apply --dry-run -f manifests/central-bank-brl.yaml --output json | jq '.steps[] | select(.status == "pending") | .name'
```

### 5. Overrides de path via variáveis de ambiente

```bash
# Para ambiente de CI com paths personalizados:
CBWEB3_SCRIPTS_DIR=/opt/cbweb3/scripts \
CBWEB3_PALADIN_CB_URL=http://paladin.internal:31648 \
./cbweb3 apply -f manifests/central-bank-brl.yaml
```

---

## Verificação do bundle após apply

```bash
# Verificar que o bundle existe e tem os campos obrigatórios
yq .spec.bootnode.enode <outputDir>/bundles/spoke-brl.bundle.yaml
yq .spec.contracts.identityRegistry <outputDir>/bundles/spoke-brl.bundle.yaml
yq .spec.trust.caCertPEM <outputDir>/bundles/spoke-brl.bundle.yaml | head -1
# Deve começar com: -----BEGIN CERTIFICATE-----

# Verificar que o bundle NÃO contém chaves privadas
grep "PRIVATE KEY" <outputDir>/bundles/spoke-brl.bundle.yaml
# Deve retornar vazio (exit 1)
```

---

## Erros comuns

### `manifest file not found: /path/to/manifest.yaml`

O arquivo do manifesto não existe no path especificado. Verifique o path com `-f`.

### `validation error: spec.spoke.id is required`

O manifesto está com campo obrigatório ausente. Abra o YAML e adicione o campo indicado na mensagem de erro.

### `mode: join not yet supported — will be implemented in TK-9`

O manifesto tem `mode: join`. Apenas `mode: found` está implementado em TK-7.

### `environment prod is not yet supported in TK-7 — only local is available`

O manifesto tem `environment: prod`. Apenas `environment: local` está implementado em TK-7.

### `keyProvider URI error: ...`

URI inválida em `spec.keyProvider`. Exemplo válido: `kms://local-emulator`.

### `orchestrator: genesis.json not found — run the TK-4 Besu compose first`

O arquivo `<dataDir>/genesis/genesis.json` não existe. Suba o compose TK-4 para o spoke antes de executar `apply`.

### `orchestrator: another process is provisioning this spoke`

Outra instância de `cbweb3 apply` está rodando para o mesmo spoke. Aguarde a conclusão ou verifique o arquivo de lock em `<dataDir>/.provisioning.lock`.

---

## Testes

```bash
# Testes unitários do pacote apply (sem CLI)
cd scenario-a/toolkit
go test -race ./engine/apply/...

# Testes de integração da CLI (com os/exec)
go test -race ./cmd/cbweb3/...

# Testes de integração completos (requerem Docker Compose TK-4 rodando)
go test -race -tags integration ./cmd/cbweb3/...

# Build de verificação (zero novas deps)
go build ./cmd/cbweb3/...
go mod tidy && git diff go.mod go.sum  # deve ser vazio
```
