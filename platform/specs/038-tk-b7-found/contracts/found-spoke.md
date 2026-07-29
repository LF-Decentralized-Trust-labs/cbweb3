# Contracts — TK-B7 (CLI found-spoke + SpokeBundle + extensões do motor)

Fase 1. Contratos da CLI, do spoke bundle e das extensões (soft step, enode).

## CLI — `cbweb3b apply` (modo found-spoke)

```
cbweb3b apply -f <found-spoke-manifest> [--dry-run] [-o json|yaml]
              [--data-dir <dir>] [--out-dir <dir>] [--repo-root <dir>]
              [--hub-rpc <url>] [--spoke-rpc <url>] [--gateway-url <url>] [--relay <uri>]
```

- Modo lido do manifesto (`spec.mode: found-spoke`). Requer `spec.hubBundleRef` (caminho do hub
  bundle). Emite report sempre; `--dry-run` planeja sem efeitos; report parcial em sinal.
- Modo `found-hub` continua funcionando (TK-B6); `join` → "not supported yet" (TK-B8).
- `--relay` seleciona a impl. do `RelayRegistrar` (`local` | `relay://host:port`).

## Go — extensão do motor (`engine/orchestrator`)

```go
type Step struct {
    Name  string
    Deps  []string
    Soft  bool // NEW: failure is non-fatal (soft-failed; does not stop the run)
    Check func(ctx context.Context) (bool, error)
    Run   func(ctx context.Context) error
}

const StatusSoftFailed Status = "soft-failed" // NEW
```

## Go — enode (`engine/orchestrator`)

```go
// EnodeReader returns the node's enode URL (admin_nodeInfo). Injectable for tests.
type EnodeReader func(ctx context.Context, rpcURL string) (string, error)
func adminNodeInfoEnode(ctx context.Context, rpcURL string) (string, error) // real
```

## Go — spoke bundle (`engine/bundle`)

```go
type SpokeBundle struct {
    Version   string
    SpokeID   string
    ChainID   uint64
    Enode     string
    SpokeRPC  string
    SpokeWS   string
    Genesis   string            // genesis.json contents (public network config)
    Contracts map[string]string // identityRegistry, tCeBM, spokeBridge, fCeBM
}

func EmitSpoke(b SpokeBundle, outDir string) (path string, err error) // bundles/spoke-<id>.bundle.yaml
func LoadSpoke(path string) (SpokeBundle, error)
func ValidateSpoke(b SpokeBundle) error // required fields; rejects private-key material
```

## Go — found-spoke steps (`engine/orchestrator`)

```go
type SpokeConfig struct {
    Runner        exec.CommandRunner
    ContractsDir, TemplatesDir, OutDir string
    SpokeID       string
    SpokeChainID  uint64
    SpokeRPC, SpokeWS string
    CBAddress     string
    GenesisDir    string
    HubBundlePath string
    SpokeEnvFile  string
    KeycloakEnv   []string
    GatewayURL    string
    // seams
    WaitRPC, WaitKeycloak func(ctx context.Context) error
    ReadClientSecret      func(ctx context.Context) (string, error)
    EnodeReader           EnodeReader
    Registrar             relayregistrar.RelayRegistrar
}

func FoundSpokeSteps(c SpokeConfig) []Step
```

## Invariantes de contrato

- `consume-hub-bundle` falha antes de qualquer efeito se o hub bundle for inválido/ausente.
- `register-cb`/`deploy-spoke-contracts` idempotentes (Check on-chain / broadcast presente).
- `register-relay-spoke` idempotente (upsert por spoke id — TK-B5).
- `add-noc-agent` é `Soft`: falha → `soft-failed`, não interrompe.
- `ValidateSpoke` rejeita qualquer material com `PRIVATE KEY` (sem segredos).
- `--dry-run` nunca aciona a impl. real do runner/registrar.
