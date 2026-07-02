# API Contract: TK-6 — Bundle Schema & EmitBundle

## Função pública

```go
// EmitBundle lê os artefatos produzidos pelo engine TK-5 em dataDir e emite
// bundles/<spoke-id>.bundle.yaml no outputDir.
//
// Precondições:
//   - manifest.Spec.Mode == "found"
//   - manifest.Spec.Node.P2P.Port > 0
//   - <dataDir>/genesis/genesis.json existe
//   - <dataDir>/.deployed-addrs.env com todos os 7 endereços preenchidos
//   - <dataDir>/tls/central-bank.crt existe e contém bloco PEM CERTIFICATE válido
//   - in.EnodeProvider.NodeInfo retorna enode bem formado
//
// Pós-condições (em sucesso):
//   - <outputDir>/bundles/<spoke-id>.bundle.yaml existe e é YAML válido
//   - bundle contém apiVersion: cbweb3/v1 e kind: JoinBundle
//   - bundle não contém material de chave privada
//
// Erros sentinela: ErrInvalidMode, ErrInvalidInput, ErrGenesisNotFound,
// ErrCACertNotFound, ErrDeployedAddrsIncomplete, ErrEnodeUnavailable.
func EmitBundle(ctx context.Context, in BundleInput) (*JoinBundle, error)
```

---

## Interface EnodeProvider

```go
type EnodeProvider interface {
    // NodeInfo retorna a string bruta do enode do Besu, ex: "enode://abc@0.0.0.0:30303".
    // Implementações devem respeitar ctx (timeout/cancelamento).
    NodeInfo(ctx context.Context) (string, error)
}
```

### Implementação padrão: `BesuEnodeProvider`

```go
// NewBesuEnodeProvider cria um provider que faz JSON-RPC admin_nodeInfo para rpcURL.
// httpClient nil usa http.DefaultClient com timeout de 10 s.
func NewBesuEnodeProvider(rpcURL string, httpClient *http.Client) EnodeProvider
```

**Contrato de erros**:
- Timeout/connection refused → retorna `ErrEnodeUnavailable` (wrapped)
- HTTP 4xx/5xx → retorna `ErrEnodeUnavailable` (wrapped)
- `result.enode` ausente ou sem `@` → retorna `ErrEnodeUnavailable` com detalhe

---

## BundleInput

```go
type BundleInput struct {
    Manifest      *manifest.Manifest  // nunca nil; mode deve ser "found"
    DataDir       string              // caminho absoluto para SPOKE_DATA_DIR
    OutputDir     string              // raiz onde bundles/ é criado; "." é válido
    EnodeProvider EnodeProvider       // nunca nil; use NewBesuEnodeProvider em produção
}
```

---

## JoinBundle (struct retornada)

```go
type JoinBundle struct {
    APIVersion string         `yaml:"apiVersion"` // "cbweb3/v1"
    Kind       string         `yaml:"kind"`        // "JoinBundle"
    Metadata   BundleMetadata `yaml:"metadata"`
    Spec       BundleSpec     `yaml:"spec"`
}
```

Ver data-model.md para a definição completa de todos os sub-types.

---

## Erros sentinela

```go
var (
    // ErrInvalidMode é retornado quando manifest.Spec.Mode != "found".
    ErrInvalidMode = errors.New("bundle: EmitBundle requires mode: found")

    // ErrInvalidInput é retornado para campos obrigatórios ausentes no manifesto
    // (ex: spec.node.p2p.port == 0).
    ErrInvalidInput = errors.New("bundle: invalid manifest input")

    // ErrGenesisNotFound é retornado quando <dataDir>/genesis/genesis.json não existe.
    ErrGenesisNotFound = errors.New("bundle: genesis.json not found in dataDir")

    // ErrCACertNotFound é retornado quando <dataDir>/tls/central-bank.crt não existe,
    // não contém bloco PEM CERTIFICATE, ou contém material de chave privada.
    ErrCACertNotFound = errors.New("bundle: CA cert not found or invalid in dataDir/tls")

    // ErrDeployedAddrsIncomplete é retornado quando .deployed-addrs.env existe mas
    // um ou mais endereços obrigatórios estão ausentes ou vazios.
    // A mensagem de erro inclui o nome da chave ausente.
    ErrDeployedAddrsIncomplete = errors.New("bundle: deployed-addrs.env missing required key")

    // ErrEnodeUnavailable é retornado quando a chamada admin_nodeInfo falha ou
    // o enode retornado está mal formado.
    ErrEnodeUnavailable = errors.New("bundle: could not obtain enode from Besu")
)
```

---

## Schema YAML do bundle (versão `cbweb3/v1`)

```yaml
apiVersion: cbweb3/v1          # string; constante
kind: JoinBundle               # string; constante
metadata:
  name: <string>               # = manifest.spec.spoke.id
  generatedAt: <string>        # ISO-8601 UTC, ex: "2026-06-27T14:30:00Z"
spec:
  spokeId: <string>            # = manifest.spec.spoke.id
  chainId: <int>               # = manifest.spec.spoke.chainId
  currency: <string>           # = manifest.spec.spoke.currency
  bootnode:
    enode: <string>            # enode://<id>@<advertisedHost>:<p2pPort>
    advertisedHost: <string>   # = manifest.spec.node.advertisedHost
    p2pPort: <int>             # = manifest.spec.node.p2p.port
  genesis:
    hash: <string>             # "sha256:<hex-lowercase>" do genesis.json
    content: <string>          # genesis.json base64-encoded (RFC 4648, sem newlines)
  contracts:
    registryAddress: <string>  # REGISTRY_CONTRACT_ADDRESS
    zetoFactoryAddress: <string>
    penteFactoryAddress: <string>
    zetoTokenAddress: <string>
    penteContextGroupId: <string>
    penteContextAddress: <string>
    fxAgreementAddress: <string>
  relay:                       # omitido se manifest.spec.relay == nil
    endpoint: <string>
  trust:
    caCertPEM: <string>        # PEM completo de tls/central-bank.crt (bloco CERTIFICATE)
```

### Restrições de validação

| Campo | Restrição |
|---|---|
| `apiVersion` | deve ser `"cbweb3/v1"` |
| `kind` | deve ser `"JoinBundle"` |
| `metadata.name` | não vazio |
| `spec.bootnode.enode` | match regex `^enode://[0-9a-f]+@.+:[0-9]+$` |
| `spec.genesis.hash` | match regex `^sha256:[0-9a-f]{64}$` |
| `spec.genesis.content` | base64 decodificável, resultado parseable como JSON |
| `spec.contracts.*` | todos os 7 campos não vazios |
| `spec.trust.caCertPEM` | contém `-----BEGIN CERTIFICATE-----` |
| `spec.trust.caCertPEM` | não contém `PRIVATE KEY` |

---

## Invariante de versionamento

Mudanças em campos existentes ou remoção de campos são **breaking changes** para TK-9. Adições de campos opcionais (com `omitempty`) são backward-compatible. Mudanças de schema requerem bump de `apiVersion` (`cbweb3/v2`) e migration path documentado.

---

## Exemplo de uso (Go)

```go
import (
    "context"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func applyFound(ctx context.Context, m *manifest.Manifest, dataDir, outputDir string) error {
    // 1. Provisionar spoke (TK-5)
    if err := orchestrator.RunFound(ctx, m, deps); err != nil {
        return err
    }

    // 2. Emitir join bundle (TK-6)
    ep := bundle.NewBesuEnodeProvider(deps.BesuRPCURL, nil)
    jb, err := bundle.EmitBundle(ctx, bundle.BundleInput{
        Manifest:      m,
        DataDir:       dataDir,
        OutputDir:     outputDir,
        EnodeProvider: ep,
    })
    if err != nil {
        return fmt.Errorf("emit join bundle: %w", err)
    }
    fmt.Printf("bundle written: %s/bundles/%s.bundle.yaml\n", outputDir, jb.Spec.SpokeID)
    return nil
}
```
