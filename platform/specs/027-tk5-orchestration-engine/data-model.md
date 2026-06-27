# Data Model: TK-5 — Motor de orquestração (`mode: found`)

**Feature**: `027-tk5-orchestration-engine`  
**Phase**: 1 — Design & Contracts  

---

## Entidades principais

### 1. `ProvisioningState` — estado persistido por spoke

**Arquivo**: `<SPOKE_DATA_DIR>/.provisioning-state.yaml`  
**Propósito**: fonte de verdade para idempotência e retomada de execução.

```yaml
# .provisioning-state.yaml
spokeID: spoke-brl
steps:
  - step: deploy-contracts
    status: done
    completedAt: "2026-06-27T14:32:01Z"
  - step: gen-tls
    status: done
    completedAt: "2026-06-27T14:32:05Z"
  - step: render-configs
    status: done
    completedAt: "2026-06-27T14:32:06Z"
  - step: register-nodes
    status: done
    completedAt: "2026-06-27T14:33:10Z"
  - step: start-paladin
    status: done
    completedAt: "2026-06-27T14:38:22Z"
  - step: create-zeto-token
    status: done
    completedAt: "2026-06-27T14:39:05Z"
  - step: create-pente-context
    status: done
    completedAt: "2026-06-27T14:40:11Z"
  - step: deploy-fxa-pente
    status: done
    completedAt: "2026-06-27T14:41:33Z"
  - step: onboard-registry
    status: done
    completedAt: "2026-06-27T14:41:55Z"
  - step: register-relay
    status: failed
    completedAt: ""
```

**Campos**:

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `spokeID` | `string` | Identificador do spoke (ex: `"spoke-brl"`) |
| `steps[].step` | `string` | Nome canônico do passo (constante definida no engine) |
| `steps[].status` | `string` | `"pending"` \| `"done"` \| `"failed"` — nunca `"running"` |
| `steps[].completedAt` | `string` | ISO-8601 UTC; vazio quando `status != "done"` |

**Invariantes**:
- `"running"` nunca é persistido: um passo que morreu mid-run fica como `"pending"` na próxima leitura.
- A ordem dos `steps[]` é canônica (mesma que a sequência de execução); o engine rejeita arquivos fora de ordem.
- O arquivo é criado na primeira chamada a `RunFound`; se não existir, todos os passos são tratados como `"pending"`.

**Go struct**:
```go
// ProvisioningState is the root of .provisioning-state.yaml.
type ProvisioningState struct {
    SpokeID string      `yaml:"spokeID"`
    Steps   []StepState `yaml:"steps"`
}

// StepState records the outcome of a single provisioning step.
type StepState struct {
    Step        string `yaml:"step"`
    Status      string `yaml:"status"`      // "pending" | "done" | "failed"
    CompletedAt string `yaml:"completedAt"` // ISO-8601 UTC; empty when not done
}
```

---

### 2. `Step` — interface de cada passo

Cada um dos 10 passos implementa esta interface interna:

```go
// Step is the unit of work in the orchestration engine.
// Each step is idempotent: Check returns true if the step is already complete.
type Step interface {
    // Name returns the canonical step name (e.g. "deploy-contracts").
    Name() string

    // Check returns true if this step has already been completed successfully.
    // Check must not have side effects and must not modify external state.
    Check(ctx context.Context) (bool, error)

    // Run executes the step. Called only when Check returns false.
    // A non-nil error causes the orchestrator to halt and mark the step as "failed".
    Run(ctx context.Context) error
}
```

---

### 3. `Deps` — dependências injetáveis

```go
// Deps holds all external dependencies injected into the orchestration engine.
// All fields are required unless marked optional.
type Deps struct {
    // KeyProvider manages secp256k1 keys. Never nil.
    KeyProvider keyprovider.KeyProvider

    // CertSource manages X.509 certificate issuance. Never nil.
    CertSource certsource.CertSource

    // RelayRegistrar handles spoke registration with the Cacti relay. Never nil
    // (use NoOpRelayRegistrar for testing or when relay is not yet available).
    RelayRegistrar RelayRegistrar

    // ScriptsDir is the absolute path to deploy/local/paladin/scripts/ —
    // where the Go test scripts (TestDeployEVMRegistry, etc.) live.
    ScriptsDir string

    // ComposeTemplatePath is the absolute path to the Paladin compose template
    // (provisioning/templates/central-bank/paladin-compose.yaml).
    ComposeTemplatePath string

    // PaladinConfigTemplateDir is the absolute path to provisioning/templates/
    // central-bank/paladin-config/ — templates for Paladin node config.yaml.
    PaladinConfigTemplateDir string

    // BesuRPCURL is the HTTP RPC URL of the spoke's Besu node (e.g. http://localhost:8645).
    BesuRPCURL string

    // Timeouts overrides the default per-step timeout values.
    // Zero values use the defaults defined in DefaultTimeouts.
    Timeouts Timeouts
}

// Timeouts configures per-step execution deadlines.
type Timeouts struct {
    // GoTestStep applies to steps 1, 4, 6, 7, 8 (subprocess go test). Default: 5m.
    GoTestStep time.Duration
    // PaladinHealthCheck applies to the polling loop in step 5. Default: 5m.
    PaladinHealthCheck time.Duration
    // PaladinHealthCheckInterval is the sleep between health check attempts. Default: 2s.
    PaladinHealthCheckInterval time.Duration
    // OnboardRegistry applies to step 9 (on-chain transaction). Default: 2m.
    OnboardRegistry time.Duration
    // RelayRegistration applies to step 10 (HTTP POST to relay). Default: 30s.
    RelayRegistration time.Duration
}
```

---

### 4. `SpokeInfo` — payload de registro no relay

```go
// SpokeInfo is the payload sent to the Cacti relay when registering a new spoke.
type SpokeInfo struct {
    SpokeID       string `json:"spoke_id"`
    BesuRPCURL    string `json:"besu_rpc_url"`
    BesuWSURL     string `json:"besu_ws_url"`
    HTLCAddress   string `json:"htlc_address"`
    GRPCEndpoint  string `json:"grpc_endpoint"`
}
```

---

### 5. `RelayRegistrar` — interface do relay

```go
// RelayRegistrar abstracts spoke registration with the Cacti relay.
// The relay REST endpoint does not yet exist (RL-1); this interface allows
// TK-5 to be implemented and tested independently.
type RelayRegistrar interface {
    // Register registers the spoke with the relay.
    // Returns ErrRelayUnavailable if the relay cannot be reached.
    // Returns ErrNotImplemented if the endpoint does not yet exist.
    Register(ctx context.Context, spoke SpokeInfo) error

    // IsRegistered returns true if the spoke is already registered.
    // Returns false (not error) if the relay is unreachable.
    IsRegistered(ctx context.Context, spokeID string) (bool, error)
}
```

---

### 6. `DeployedAddrs` — endereços de contratos deployados

Lido de `<SPOKE_DATA_DIR>/.deployed-addrs.env` após o passo 1.

```go
// DeployedAddrs holds the contract addresses written by the deploy Go test scripts.
// Read from <SPOKE_DATA_DIR>/.deployed-addrs.env (key=value format, one per line).
type DeployedAddrs struct {
    RegistryContractAddress string // REGISTRY_CONTRACT_ADDRESS
    ZetoFactoryAddress      string // ZETO_FACTORY_ADDRESS
    PenteFactoryAddress     string // PENTE_FACTORY_ADDRESS
    ZetoTokenAddress        string // ZETO_TOKEN_ADDRESS (written after step 6)
    PenteContextGroupID     string // PENTE_CONTEXT_GROUP_ID (written after step 7)
    PenteContextAddress     string // PENTE_CONTEXT_ADDRESS (written after step 7)
    FXAgreementDeployedAt   string // FX_AGREEMENT_DEPLOYED_AT (written after step 8)
}
```

---

## Estado externo verificado por cada passo

| Passo | Fonte de verdade externa | Campo/endpoint |
|-------|--------------------------|----------------|
| 1 `deploy-contracts` | `.deployed-addrs.env` | `REGISTRY_CONTRACT_ADDRESS` não vazio |
| 2 `gen-tls` | Filesystem | `<SPOKE_DATA_DIR>/tls/central-bank.crt` existe |
| 3 `render-configs` | Filesystem | `<SPOKE_DATA_DIR>/paladin/central-bank/config.yaml` existe |
| 4 `register-nodes` | `.provisioning-state.yaml` | único passo sem fonte externa |
| 5 `start-paladin` | Docker + HTTP | container `running` E `ptx_getTransaction` → `PD020704` |
| 6 `create-zeto-token` | `.deployed-addrs.env` | `ZETO_TOKEN_ADDRESS` não vazio |
| 7 `create-pente-context` | `.deployed-addrs.env` | `PENTE_CONTEXT_GROUP_ID` não vazio |
| 8 `deploy-fxa-pente` | `.deployed-addrs.env` | `FX_AGREEMENT_DEPLOYED_AT` não vazio |
| 9 `onboard-registry` | IdentityRegistry on-chain | `isParticipant(evmAddress)` → `true` |
| 10 `register-relay` | Relay HTTP | `GET /api/v1/spokes/<id>` → HTTP 200 |

---

## Transições de estado

```
                     check() = false
  pending ──────────────────────────────► running (in-memory only)
                                                │
                              run() ok ◄────────┤
                                   │            │
                                   ▼            │ run() error
                                  done          ▼
                                              failed

  done ──────── check() = true ──────────► (skipped, no state change)
```

O estado `running` **nunca é escrito em disco**. Se o processo morrer durante `run()`, o arquivo de estado permanece com o valor anterior (`pending` ou `failed`), garantindo que a retomada re-execute o passo.
