# Data Model: TK-8 e TK-9 — Commercial Bank Join

**Feature**: `032-commercial-bank-join`
**Date**: 2026-06-27

---

## Entidades

### 1. `JoinBundle` (estendido)

Documento YAML emitido por TK-6. Esta feature adiciona dois campos ao `BundleSpec`.

**Arquivo**: `scenario-a/toolkit/engine/bundle/types.go`

**Mudanças**:

```go
// ValidatorSpec identifies a QBFT validator in the spoke.
// Used by the mode:join engine to cast validator votes.
type ValidatorSpec struct {
    Address string `yaml:"address"` // Ethereum address (0x-prefixed)
    RPCURL  string `yaml:"rpcUrl"`  // JSON-RPC HTTP endpoint
}

// BundleSpec — campos adicionados:
//   Validators []ValidatorSpec `yaml:"validators"`
//   CBEndpoint string          `yaml:"cbEndpoint,omitempty"`
```

**BundleSpec completo após extensão**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `spokeId` | string | ✅ | ID do spoke (ex: `spoke-brl`) |
| `chainId` | int | ✅ | Chain ID do Besu |
| `currency` | string | ✅ | Moeda (ex: `BRL`) |
| `bootnode.enode` | string | ✅ | Enode completo do bootnode |
| `bootnode.advertisedHost` | string | ✅ | Host anunciado do bootnode |
| `bootnode.p2pPort` | int | ✅ | Porta P2P do bootnode |
| `genesis.hash` | string | ✅ | SHA-256 do genesis.json (`sha256:<hex>`) |
| `genesis.content` | string | ✅ | genesis.json base64-encoded |
| `contracts.*` | string | ✅ | Endereços dos 7 contratos deployados |
| `relay.endpoint` | string | ✓ (omitempty) | Endpoint do relay LNET |
| `trust.caCertPEM` | string | ✅ | Cert CA do spoke (âncora de confiança) |
| `validators` | `[]ValidatorSpec` | ✅ **NOVO** | Validadores existentes (address + rpcUrl) |
| `cbEndpoint` | string | ✅ **NOVO** | URL credential-request do banco central |

**Invariantes**:
- `validators` não pode ser vazio em bundles usados por `mode: join`; falha com erro de schema
- `cbEndpoint` deve ser uma URL válida (scheme http/https)
- Nenhum campo pode conter chaves privadas

---

### 2. `CommercialBankManifest` (sem mudanças de schema Go)

O tipo `manifest.Spec` já contém `JoinBundleRef string`. Nenhuma nova struct Go é necessária.

**Campos relevantes para `mode: join`**:

| Campo spec | Obrigatório em join | Descrição |
|------------|---------------------|-----------|
| `scenario` | ✅ | Deve ser `"a"` |
| `role` | ✅ | Deve ser `"commercial-bank"` |
| `mode` | ✅ | Deve ser `"join"` |
| `spoke.id` | ✅ | ID do spoke a ingressar |
| `joinBundleRef` | ✅ | Path para o bundle YAML |
| `node.advertisedHost` | ✅ | Host anunciado do nó do banco |
| `node.rpc.port` | ✅ | Porta RPC do nó |
| `node.p2p.port` | ✅ | Porta P2P do nó |
| `node.dataDir` | ✅ | SPOKE_DATA_DIR do banco |
| `keyProvider` | ✅ | URI do KMS (ex: `kms://local-emulator`) |
| `certSource` | ✅ | `self-signed` ou `ca://...` |
| `image` | ✅ | Imagem Besu (`build` ou ref de registry) |

**Exemplo** (`testdata/commercial-bank-brl.yaml`):
```yaml
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

---

### 3. `ProvisioningState` (modo join — sem mudanças de struct)

A struct `ProvisioningState` (YAML em `.provisioning-state.yaml`) é reutilizada sem alteração. Os nomes de passo são diferentes dos do `mode: found`.

**Estado persistido de exemplo após join completo**:

```yaml
spokeID: spoke-brl
steps:
  - step: write-genesis
    status: done
    completedAt: "2026-06-27T10:01:00Z"
  - step: start-besu-join
    status: done
    completedAt: "2026-06-27T10:01:15Z"
  - step: wait-sync
    status: done
    completedAt: "2026-06-27T10:02:30Z"
  - step: vote-qbft
    status: done
    completedAt: "2026-06-27T10:02:45Z"
  - step: gen-csr
    status: done
    completedAt: "2026-06-27T10:02:46Z"
  - step: request-cert
    status: done
    completedAt: "2026-06-27T10:02:47Z"
  - step: receive-cert
    status: done
    completedAt: "2026-06-27T10:03:00Z"
  - step: proof-of-possession
    status: done
    completedAt: "2026-06-27T10:03:05Z"
  - step: start-backend
    status: done
    completedAt: "2026-06-27T10:03:20Z"
```

**Invariantes**:
- Cada step é escrito atomicamente (temp file + rename), igual ao modo found
- O arquivo de lock `.provisioning.lock` impede runs concorrentes no mesmo dataDir
- `spokeID` pode diferir de `manifest.Spec.Spoke.ID` apenas se o dataDir for compartilhado (não suportado; dataDir deve ser por banco)

---

### 4. `JoinDeps` (nova struct — extensão de `deps.go`)

Dependências injetadas no `RunJoin()`, separadas das `Deps` do `RunFound()`.

```go
type JoinDeps struct {
    // KeyProvider manages the commercial bank's blockchain secp256k1 key.
    KeyProvider keyprovider.KeyProvider

    // BankCode is the human-readable code for this commercial bank (used in CSR subject).
    BankCode string

    // Institution is the bank's legal name for the CSR subject O= field.
    Institution string

    // ComposeTemplatePath is the absolute path to provisioning/templates/commercial-bank/.
    ComposeTemplatePath string

    // BackendComposePath is the absolute path to the bank's backend docker-compose file.
    BackendComposePath string

    // Timeouts configures per-step execution deadlines.
    Timeouts JoinTimeouts
}

type JoinTimeouts struct {
    // WaitSync is the maximum time to wait for eth_blockNumber to reach target. Default: 3min.
    WaitSync time.Duration
    // WaitSyncInterval is the polling interval. Default: 5s.
    WaitSyncInterval time.Duration
    // VoteQBFT is the maximum time to wait for validator set activation. Default: 5min.
    VoteQBFT time.Duration
    // VoteQBFTInterval is the polling interval. Default: 3s.
    VoteQBFTInterval time.Duration
    // RequestCert is the timeout for the HTTP POST to cbEndpoint. Default: 30s.
    RequestCert time.Duration
    // ReceiveCert is the maximum polling time for the signed cert. Default: 5min.
    ReceiveCert time.Duration
    // ReceiveCertInterval is the polling interval. Default: 10s.
    ReceiveCertInterval time.Duration
    // ProofOfPossession is the timeout for the IdentityRegistry transaction. Default: 2min.
    ProofOfPossession time.Duration
}
```

## Transições de Estado dos Passos

```
write-genesis  →  start-besu-join  →  wait-sync  →  vote-qbft
                                                        ↓
                          start-backend  ←  proof-of-possession  ←  receive-cert  ←  request-cert  ←  gen-csr
```

- Cada seta representa uma dependência de sequência estrita
- Nenhum passo é executado se o anterior não completou com `status: done`
- Falha em qualquer passo (exceto `start-backend` que pode ser soft-failure como `register-relay` no found) interrompe o run
- Re-run: passos `done` são pulados; o run retoma do primeiro passo não-done
