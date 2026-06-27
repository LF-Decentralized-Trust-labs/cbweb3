# Contract: ParticipantDeployment YAML Manifest Format

**Version**: cbweb3/v1
**Schema file**: `scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml`
**Consumers**: operators authoring manifests; provisioning toolkit engine; CI validation jobs

---

## Minimal valid manifest (central-bank, mode: found, local profile)

```yaml
# yaml-language-server: $schema: ../../provisioning/schema/v1/participant-deployment.schema.yaml
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
    rpc: { port: 8645 }
    ws:  { port: 8655 }
    p2p: { port: 31303 }
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil
  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed
  relay:
    endpoint: http://cbweb3-cacti:4000
```

## Promoting to staging/prod

The same manifest with different values only — no structural changes:

```yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata:
  name: central-bank-brazil
spec:
  scenario: a
  environment: prod
  role: central-bank
  mode: found
  spoke:
    id: spoke-brl
    chainId: 4321
    currency: BRL
  node:
    rpc: { port: 8545 }
    ws:  { port: 8546 }
    p2p: { port: 30303 }
    advertisedHost: besu.central-bank-brazil.lnet.example.com
  image: registry.lnet.example.com/cbweb3/api-gateway:1.2.0
  keyProvider: kms://aws-kms.us-east-1.amazonaws.com/key/arn:...
  certSource: ca://pki.central-bank-brazil.lnet.example.com
  relay:
    endpoint: https://relay.lnet.example.com
```

## Commercial bank (mode: join)

```yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata:
  name: bank-alpha
spec:
  scenario: a
  environment: local
  role: commercial-bank
  mode: join
  spoke:
    id: spoke-brl
    chainId: 1337
    currency: BRL
  node:
    rpc: { port: 8646 }
    ws:  { port: 8656 }
    p2p: { port: 31304 }
    advertisedHost: cbweb3-spoke-brl-besu.bank-alpha
  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed
  relay:
    endpoint: http://cbweb3-cacti:4000
  joinBundleRef: ./bundles/spoke-brl.bundle.yaml
```

## Field reference

| Field | Required | Accepted values |
|-------|----------|-----------------|
| `apiVersion` | yes | `cbweb3/v1` |
| `kind` | yes | `ParticipantDeployment` |
| `metadata.name` | yes | any non-empty string |
| `spec.scenario` | yes | `a` |
| `spec.environment` | no | `local`, `staging`, `prod` |
| `spec.role` | yes | `central-bank`, `commercial-bank` |
| `spec.mode` | yes | `found`, `join` |
| `spec.spoke.id` | yes | any non-empty string |
| `spec.spoke.chainId` | yes | positive integer |
| `spec.spoke.currency` | yes | any non-empty string |
| `spec.node.advertisedHost` | yes | non-empty string — **never inferred** |
| `spec.node.rpc.port` | no | 1–65535 |
| `spec.node.ws.port` | no | 1–65535 |
| `spec.node.p2p.port` | no | 1–65535 |
| `spec.image` | yes | `build` or registry image reference |
| `spec.keyProvider` | yes | `kms://...` URI |
| `spec.certSource` | yes | `self-signed` or `ca://...` |
| `spec.relay.endpoint` | no | HTTP(S) URL |
| `spec.joinBundleRef` | contextual | path string; expected when `mode: join` |

## Invariants

- The manifest MUST NOT contain private keys, passwords, passphrases, or credential values.
- `spec.node.advertisedHost` MUST be set to the externally reachable address of the node. It is never computed or defaulted by the toolkit.
- `spec.mode: found` and `spec.joinBundleRef` are mutually exclusive by intent; `mode: join` without `joinBundleRef` will fail at a later engine stage.
