# Contract: Go Package API — `engine/manifest`

**Package path**: `github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest`
**Consumers**: provisioning engine (TK-5), any future CLI that applies a manifest

---

## Exported types

### `Manifest`

Top-level parsed representation of a `ParticipantDeployment` YAML file.

```go
type Manifest struct {
    APIVersion string   // "cbweb3/v1"
    Kind       string   // "ParticipantDeployment"
    Metadata   Metadata
    Spec       Spec
}
```

### `Metadata`

```go
type Metadata struct {
    Name string
}
```

### `Spec`

```go
type Spec struct {
    Scenario      string  // "a"
    Environment   string  // optional: "local" | "staging" | "prod"
    Role          string  // "central-bank" | "commercial-bank"
    Mode          string  // "found" | "join"
    Spoke         Spoke
    Node          Node
    Image         string  // "build" or registry ref
    KeyProvider   string  // "kms://..."
    CertSource    string  // "self-signed" | "ca://..."
    Relay         *Relay  // optional
    JoinBundleRef string  // optional; contextually required when Mode == "join"
}
```

### `Spoke`

```go
type Spoke struct {
    ID       string
    ChainID  int
    Currency string
}
```

### `Node`

```go
type Node struct {
    AdvertisedHost string  // required; never inferred
    RPC            *Port   // optional
    WS             *Port   // optional
    P2P            *Port   // optional
}
```

### `Port`

```go
type Port struct {
    Port int
}
```

### `Relay`

```go
type Relay struct {
    Endpoint string
}
```

---

## Exported functions

### `Load`

```go
func Load(path string) (*Manifest, error)
```

Reads the YAML file at `path`, parses it into a `*Manifest`, and returns it.

Returns a non-nil error if:
- The file does not exist or cannot be read.
- The file is not valid YAML.
- YAML unmarshal into the struct fails.

`Load` does **not** validate field values or required-field presence — use `Validate` for that.

### `Validate`

```go
func Validate(m *Manifest) error
```

Validates all required fields and enum constraints against the rules in the spec (FR-001 through FR-011).

Collects **all** violations before returning; the returned error joins them via `errors.Join`.

Returns `nil` if the manifest is valid.

Returns a non-nil error whose message contains one line per violation. Each line identifies:
- The field path (e.g., `spec.node.advertisedHost`)
- The nature of the violation (missing, empty, or invalid value)
- For enum violations: the invalid value and the list of accepted values

Example error output for two violations:

```
spec.node.advertisedHost: required field is missing; this field must be set explicitly and is never inferred from co-location
spec.role: invalid value "central"; accepted values are: central-bank, commercial-bank
```

---

## Usage example

```go
m, err := manifest.Load("./manifests/central-bank-brazil.yaml")
if err != nil {
    log.Fatalf("load: %v", err)
}
if err := manifest.Validate(m); err != nil {
    log.Fatalf("invalid manifest:\n%v", err)
}
// m is safe to use
```
