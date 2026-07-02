# Data Model: ParticipantDeployment Manifest

**Feature**: 023-manifest-schema-validation
**Date**: 2026-06-27

## Top-level structure

```
ParticipantDeployment
├── apiVersion       string   required — "cbweb3/v1"
├── kind             string   required — "ParticipantDeployment"
├── metadata         Metadata required
└── spec             Spec     required
```

## Metadata

```
Metadata
└── name   string   required — human-readable name for the deployment (e.g., "central-bank-brazil")
```

## Spec

```
Spec
├── scenario     string       required — enum: "a"
├── environment  string       optional — enum: "local" | "staging" | "prod"
├── role         string       required — enum: "central-bank" | "commercial-bank"
├── mode         string       required — enum: "found" | "join"
├── spoke        Spoke        required
├── node         Node         required
├── image        string       required — "build" | <registry-image-ref>
├── keyProvider  string       required — URI, e.g. "kms://local-emulator"
├── certSource   string       required — "self-signed" | "ca://<endpoint>"
├── relay        Relay        optional
└── joinBundleRef string      optional — path to bundle YAML; contextually required when mode=join
                               (conditional enforcement is out of scope for TK-1; see FR-011)
```

> **Note on secrets**: the manifest structure deliberately contains no field for private keys,
> passphrases, or credentials. All key material is referenced via `keyProvider` URI only.

## Spoke

```
Spoke
├── id        string   required — spoke identifier, e.g. "spoke-brl"
├── chainId   integer  required — EVM chain ID, e.g. 1337
└── currency  string   required — ISO 4217 code, e.g. "BRL"
```

## Node

```
Node
├── advertisedHost  string   required — external address of the node; MUST be set explicitly,
│                             never inferred from co-location or container IP
├── rpc             Port     optional — defaults applied by environment preset
├── ws              Port     optional — defaults applied by environment preset
└── p2p             Port     optional — defaults applied by environment preset
```

## Port

```
Port
└── port   integer   required when parent present — TCP port number (1–65535)
```

## Relay

```
Relay
└── endpoint   string   required when relay present — HTTP(S) URL of the LNET-operated relay
```

## Validation rules

| Field | Rule |
|-------|------|
| `apiVersion` | Must equal `"cbweb3/v1"` |
| `kind` | Must equal `"ParticipantDeployment"` |
| `spec.scenario` | Must equal `"a"` |
| `spec.role` | Must be one of: `central-bank`, `commercial-bank` |
| `spec.mode` | Must be one of: `found`, `join` |
| `spec.node.advertisedHost` | Required; must be non-empty; no default ever applied |
| `spec.spoke.chainId` | Must be a positive integer |
| `spec.image` | No format constraint; `"build"` and any non-empty string accepted |
| `spec.keyProvider` | Must be non-empty; format `kms://<...>` expected but not regex-enforced |
| `spec.certSource` | Must be non-empty; `"self-signed"` or `"ca://<...>"` expected |
| `spec.environment` | When present, must be one of: `local`, `staging`, `prod` |
| Secrets | No field for private keys, passwords, or credentials exists in the schema |

## State transitions (mode semantics)

The `mode` field drives the engine flow; the manifest is the input, not a state machine itself:

- `mode: found` — this participant is founding a new spoke (its node becomes the bootnode; it deploys spoke contracts; it emits a join bundle). `joinBundleRef` MUST be absent.
- `mode: join` — this participant is joining an existing spoke (it consumes a join bundle at `joinBundleRef`). `joinBundleRef` contextually required (not enforced by TK-1).
