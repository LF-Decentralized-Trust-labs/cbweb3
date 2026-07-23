# Secret Management — Signing Keys and Credentials

This document defines how private signing keys and other secrets are handled
across `cbweb3-platform` for local development, staging, and production. It
applies to both scenarios (`scenario-a/`, `scenario-b/`).

Finding reference: R2-10.4 (plaintext private keys in configuration).

## Principle

Private key material MUST NOT be embedded in tracked files. This includes
`.env.example` and `.env.*.example` templates, documentation, comments,
manifests, and container images. The only key material permitted in a local
working tree is a developer's own copied `.env` file (gitignored) populated with
well-known local-dev keys.

Environments escalate as follows:

| Environment | Key source | Written to disk / env? |
| --- | --- | --- |
| Local dev | Well-known, pre-funded Besu genesis keys (public knowledge) | Yes, in a gitignored `.env` copy only |
| Staging | Secret store / HSM via `KeyProvider` | No |
| Production | HSM (or cloud KMS with HSM-backed keys) via `KeyProvider` | No |

## Local development

The tracked template files ship with BLANK key values:

- `scenario-a/contracts/.env.example`, `scenario-b/contracts/.env.example`
  (`DEPLOYER_PRIVATE_KEY`, `ADMIN_PRIVATE_KEY`, `CENTRAL_BANK_PRIVATE_KEY`, ...)
- `scenario-b/backend/config/.env.infra.central-bank-a.example`
  (`CB_PRIVATE_KEY`, `SIGNER_PRIVATE_KEY`, `CENTRAL_BANK_A_PRIVATE_KEY`)
- `scenario-b/backend/config/.env.infra.central-bank-b.example`
  (`CB_PRIVATE_KEY`, `SIGNER_PRIVATE_KEY`)
- `scenario-b/backend/config/.env.infra.mlp.example` (`SIGNER_PRIVATE_KEY`)
- `scenario-b/deploy/local/.env.example` (private key removed from the
  `MLP_ADDRESS` comment)

To run the stack locally:

1. Copy the template: `cp <file>.example <file>`.
2. Populate the copied file with the well-known, pre-funded Besu genesis keys.
   These are public dev keys derived from the standard local genesis and carry no
   security value on a local network.
3. NEVER reuse these keys, addresses, or any local `.env` on a shared, staging,
   or production network.

The copied `.env` / `.env.infra.*` files are gitignored and must remain so.

## Staging and production

Signing keys MUST be sourced from a secret store or Hardware Security Module
(HSM) and MUST NOT be materialized as plaintext in files, environment variables,
container images, or logs. The private key never leaves the provider boundary;
callers obtain only public keys and signatures.

### KeyProvider abstraction

The provisioning toolkit already defines the key-management boundary in
`scenario-a/toolkit/engine/keyprovider`:

- `keyprovider.go` — the `KeyProvider` interface:
  - `GenerateKey(ctx, id) -> pubkey` — creates and stores a secp256k1 key pair;
    returns only the public key. Idempotent.
  - `Sign(ctx, id, digest) -> sig` — signs a 32-byte digest; returns a 65-byte
    Ethereum-compatible signature.
  - `GetPublicKey(ctx, id) -> pubkey` — returns the stored public key.
  - Package contract: "Private keys never appear in files, environment
    variables, or manifests."
- `factory.go` — `New(uri)` selects the backend from a `kms://` URI:
  - `kms://local-emulator` — in-memory `LocalKeyProvider` for tests and local
    dev. Keys are generated on demand and never persisted.
  - `kms://<any-other>` — the production provider stub (`prod.go`), currently
    returning `ErrNotImplemented`.

### Wiring keys in staging/production

1. Set the entity's key-provider URI to the target backend (for example a cloud
   KMS or HSM endpoint) instead of `kms://local-emulator`.
2. Implement `prodKeyProvider` (`scenario-a/toolkit/engine/keyprovider/prod.go`)
   against the chosen HSM / KMS SDK. It must satisfy the same `KeyProvider`
   contract: generate and hold keys inside the HSM, and expose only public keys
   and signatures. Until this is implemented, all methods return
   `ErrNotImplemented` — this is the intended guard, not a bug.
3. Services that currently read `*_PRIVATE_KEY` from the environment MUST instead
   request signatures from the `KeyProvider`. No deployment path may fall back to
   a plaintext key in a non-local environment.

### HSM / secret-store requirements

- Keys are generated inside the HSM/KMS and are non-exportable.
- Access is authorized per identity (central bank, MLP, deployer) with least
  privilege and full audit logging.
- Rotation and revocation are supported without redeploying application images.
- No secret is echoed to stdout/stderr or structured logs (see the observability
  rules in the constitution).

## Checklist for reviewers

- No `.env.example` / `.env.*.example` file contains a non-empty `*_PRIVATE_KEY`
  value or a 64-character hex key (including inside comments).
- New services read signing keys through `KeyProvider`, not from plaintext
  environment variables, in staging/production paths.
- Copied `.env` and `.env.infra.*` files remain gitignored.
