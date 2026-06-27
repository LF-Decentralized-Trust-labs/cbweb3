# Data Model: TK-6 — Emissor do Join Bundle

## Bundle YAML (saída pública)

O join bundle é o artefato público emitido pelo banco central após fundar um spoke. É consumido integralmente por TK-9 (`mode: join`). O schema é versionado via `apiVersion`.

```yaml
apiVersion: cbweb3/v1
kind: JoinBundle
metadata:
  name: spoke-brl                          # = manifest.spec.spoke.id
  generatedAt: "2026-06-27T14:30:00Z"     # ISO-8601 UTC, timestamp de emissão
spec:
  spokeId: spoke-brl
  chainId: 1337
  currency: BRL
  bootnode:
    enode: "enode://deadbeef1234...@cbweb3-spoke-brl-besu.central-bank-brazil:31303"
    advertisedHost: "cbweb3-spoke-brl-besu.central-bank-brazil"
    p2pPort: 31303
  genesis:
    hash: "sha256:a3b2c1..."               # SHA-256 hex do genesis.json, prefixo "sha256:"
    content: "eyJjaGFpbklkIjox..."         # genesis.json base64-encoded (RFC 4648, sem newlines)
  contracts:
    registryAddress:     "0xABC..."         # REGISTRY_CONTRACT_ADDRESS
    zetoFactoryAddress:  "0xDEF..."         # ZETO_FACTORY_ADDRESS
    penteFactoryAddress: "0x123..."         # PENTE_FACTORY_ADDRESS
    zetoTokenAddress:    "0x456..."         # ZETO_TOKEN_ADDRESS
    penteContextGroupId: "group-abc123"     # PENTE_CONTEXT_GROUP_ID
    penteContextAddress: "0x789..."         # PENTE_CONTEXT_ADDRESS
    fxAgreementAddress:  "0xFAF..."         # FX_AGREEMENT_DEPLOYED_AT
  relay:
    endpoint: "http://cbweb3-cacti:4000"   # manifest.spec.relay.endpoint (omitido se nil)
  trust:
    caCertPEM: |
      -----BEGIN CERTIFICATE-----
      MIIBxTCCAW...
      -----END CERTIFICATE-----
```

**Campos ausentes**: `spec.relay` é omitido se `manifest.Spec.Relay == nil`. Todos os outros campos são obrigatórios — presença é verificada antes de escrever o arquivo.

---

## Structs Go (package `bundle`)

```go
// JoinBundle é a representação Go do bundle YAML.
type JoinBundle struct {
    APIVersion string         `yaml:"apiVersion"`
    Kind       string         `yaml:"kind"`
    Metadata   BundleMetadata `yaml:"metadata"`
    Spec       BundleSpec     `yaml:"spec"`
}

type BundleMetadata struct {
    Name        string `yaml:"name"`
    GeneratedAt string `yaml:"generatedAt"` // ISO-8601 UTC
}

type BundleSpec struct {
    SpokeID   string           `yaml:"spokeId"`
    ChainID   int              `yaml:"chainId"`
    Currency  string           `yaml:"currency"`
    Bootnode  BootnodeSpec     `yaml:"bootnode"`
    Genesis   GenesisSpec      `yaml:"genesis"`
    Contracts ContractsSpec    `yaml:"contracts"`
    Relay     *RelaySpec       `yaml:"relay,omitempty"`
    Trust     TrustSpec        `yaml:"trust"`
}

type BootnodeSpec struct {
    Enode          string `yaml:"enode"`
    AdvertisedHost string `yaml:"advertisedHost"`
    P2PPort        int    `yaml:"p2pPort"`
}

type GenesisSpec struct {
    Hash    string `yaml:"hash"`    // "sha256:<hex>"
    Content string `yaml:"content"` // base64-encoded genesis.json (sem newlines)
}

type ContractsSpec struct {
    RegistryAddress     string `yaml:"registryAddress"`
    ZetoFactoryAddress  string `yaml:"zetoFactoryAddress"`
    PenteFactoryAddress string `yaml:"penteFactoryAddress"`
    ZetoTokenAddress    string `yaml:"zetoTokenAddress"`
    PenteContextGroupID string `yaml:"penteContextGroupId"`
    PenteContextAddress string `yaml:"penteContextAddress"`
    FXAgreementAddress  string `yaml:"fxAgreementAddress"`
}

type RelaySpec struct {
    Endpoint string `yaml:"endpoint"`
}

type TrustSpec struct {
    CACertPEM string `yaml:"caCertPEM"` // PEM completo de tls/central-bank.crt
}
```

---

## BundleInput (entrada da função)

```go
// BundleInput agrega todos os parâmetros necessários para EmitBundle.
// Separar em struct evita assinatura longa e facilita extensão futura.
type BundleInput struct {
    Manifest      *manifest.Manifest // manifesto parseado (TK-1)
    DataDir       string             // SPOKE_DATA_DIR (artefatos do TK-5)
    OutputDir     string             // diretório raiz onde bundles/<spoke-id>.bundle.yaml é escrito
    EnodeProvider EnodeProvider      // abstração do Besu admin_nodeInfo (injetável)
}
```

---

## EnodeProvider (interface de abstração do Besu)

```go
// EnodeProvider abstrai a chamada ao Besu para obter o enode-id do bootnode.
// Permite testes sem Besu real.
type EnodeProvider interface {
    // NodeInfo retorna a string bruta do enode, ex: "enode://abc@0.0.0.0:30303".
    // Retorna ErrEnodeUnavailable se o Besu não responder ou o resultado for inválido.
    NodeInfo(ctx context.Context) (string, error)
}

// BesuEnodeProvider é a implementação padrão via JSON-RPC admin_nodeInfo.
type BesuEnodeProvider struct {
    RPCURL     string
    HTTPClient *http.Client // opcional; nil usa http.DefaultClient com 10s timeout
}
```

---

## Erros sentinela (package `bundle`)

```go
var (
    ErrInvalidMode           = errors.New("bundle: EmitBundle requires mode: found")
    ErrInvalidInput          = errors.New("bundle: invalid manifest input")
    ErrGenesisNotFound       = errors.New("bundle: genesis.json not found in dataDir")
    ErrCACertNotFound        = errors.New("bundle: CA cert not found or invalid in dataDir/tls")
    ErrDeployedAddrsIncomplete = errors.New("bundle: deployed-addrs.env missing required key")
    ErrEnodeUnavailable      = errors.New("bundle: could not obtain enode from Besu")
)
```

Erros com detalhe usam `fmt.Errorf("...: %w", ErrXxx)` para preservar wrapping via `errors.Is`.

---

## Caminhos de artefatos no `SPOKE_DATA_DIR`

| Artefato | Caminho relativo ao `dataDir` | Produzido por |
|---|---|---|
| Genesis | `genesis/genesis.json` | Template TK-4 (serviço `genesis-init`) |
| Deployed addresses | `.deployed-addrs.env` | Passos 1, 6, 7, 8 do engine TK-5 |
| CA cert (TLS) | `tls/central-bank.crt` | Passo 2 do engine TK-5 (`gen-tls`) |
| Enode (via RPC) | N/A — obtido de Besu em runtime | Nó Besu (TK-4) |

---

## Caminho de saída do bundle

```
<outputDir>/
  bundles/
    <spoke-id>.bundle.yaml    # ex: bundles/spoke-brl.bundle.yaml
```

`outputDir` é fornecido por BundleInput; por padrão TK-7 usa o diretório de trabalho do operador. O emissor cria `bundles/` com `os.MkdirAll` se não existir.

---

## Invariantes de segurança

1. **Sem chaves privadas**: `trust.caCertPEM` contém apenas o cert público (bloco `CERTIFICATE`). O emissor recusa qualquer PEM com `PRIVATE KEY` antes de escrever o arquivo.
2. **Sem material derivado de `KeyProvider`**: o emissor não acessa `KeyProvider` (TK-2) — todos os seus inputs são artefatos públicos persistidos no `dataDir`.
3. **Sem endereços de container**: `spec.bootnode.enode` usa `advertisedHost` do manifesto, nunca o IP retornado pelo Besu.
4. **Commitável**: o bundle pode ser commitado em repositório e revisado em PR sem risco de vazamento de segredos.
