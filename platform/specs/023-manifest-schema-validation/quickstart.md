# Quickstart: Manifest Schema and Validation (TK-1)

## What this delivers

After this task, the provisioning toolkit can load and validate a `ParticipantDeployment` YAML manifest. The JSON Schema file makes authoring manifests editor-assisted.

## Directory layout after implementation

```
scenario-a/provisioning/schema/v1/
└── participant-deployment.schema.yaml

scenario-a/toolkit/engine/manifest/
├── types.go
├── validate.go
└── validate_test.go
```

## Running the tests

```bash
cd scenario-a/toolkit
go test ./engine/manifest/...
```

Expected output: all table-driven cases pass (valid manifest, each missing required field, each invalid enum value).

## Using editor autocompletion

Add this header to any manifest YAML file:

```yaml
# yaml-language-server: $schema: ../../provisioning/schema/v1/participant-deployment.schema.yaml
```

Adjust the relative path to match the manifest's location. VS Code with the Red Hat YAML extension will then show inline errors and autocompletion for `role`, `mode`, `scenario`, and `environment` fields.

## Using the Go package

```go
import "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"

m, err := manifest.Load("./central-bank-brazil.yaml")
if err != nil {
    // file not found, not valid YAML, or unmarshal failure
    log.Fatalf("load: %v", err)
}
if err := manifest.Validate(m); err != nil {
    // one error line per violation
    log.Fatalf("invalid manifest:\n%v", err)
}
// m.Spec.Role, m.Spec.Mode, m.Spec.Node.AdvertisedHost, etc. are safe to use
```

## Verifying the JSON Schema is valid draft-07

```bash
# Using ajv-cli (npm)
npx ajv-cli validate \
  -s scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml \
  -d path/to/central-bank-brazil.yaml
```

Or open the schema in VS Code and confirm the YAML Language Server parses it without errors.
