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
| Shared multi-host (LNET) | Same public dev keys — see the caveat below | Container env only; not persisted by the toolkit |
| Staging | Secret store / HSM via `KeyProvider` | No |
| Production | HSM (or cloud KMS with HSM-backed keys) via `KeyProvider` | No |

### Caveat: today only the local tier is selectable

The staging and production rows above describe the target state, not a tier an
operator can select today. `spec.environment` accepts exactly one value: the
manifest validator rejects `staging` and `prod` outright
(`scenario-b/toolkit/engine/manifest/validate.go`). That gate is deliberate — it
prevents a production deploy from silently proceeding on dev key material — but it
also means every deploy that exists today runs in the local tier.

That includes the multi-host LNET deploy (`deploy-lnet/deploy.sh`), whose rendered
manifests all declare `environment: local`. LNET therefore operates on public dev
keys: the founding identity is a constant in the toolkit
(`scenario-b/toolkit/engine/orchestrator/localdev.go`), and the per-entity hub,
relayer and bank identities are deterministically derived from it. Those keys
protect nothing and must be treated as public. **A shared LNET network is not a
staging environment and must never hold value or real participant data.** Lifting
that restriction requires implementing the production `KeyProvider` (below) and
admitting a second `spec.environment` value.

## Local development

The tracked template files ship with BLANK key values:

- `scenario-a/contracts/.env.example`, `scenario-b/contracts/.env.example`
  (`DEPLOYER_PRIVATE_KEY`, `ADMIN_PRIVATE_KEY`, `CENTRAL_BANK_PRIVATE_KEY`, ...)
- `scenario-a/backend/config/.env.infra.central-bank-a.example`,
  `scenario-a/backend/config/.env.infra.central-bank-b.example` (`CB_PRIVATE_KEY`)
- `scenario-b/backend/config/.env.infra.central-bank-a.example`
  (`CB_PRIVATE_KEY`, `SIGNER_PRIVATE_KEY`, `CENTRAL_BANK_A_PRIVATE_KEY`)
- `scenario-b/backend/config/.env.infra.central-bank-b.example`
  (`CB_PRIVATE_KEY`, `SIGNER_PRIVATE_KEY`)
- `scenario-b/backend/config/.env.infra.mlp.example` (`SIGNER_PRIVATE_KEY`)
- `scenario-b/deploy/local/.env.example` (private key removed from the
  `MLP_ADDRESS` comment)

One template carries a deliberately non-functional placeholder rather than a blank:
`scenario-b/provisioning/templates/vars/entity-backend.env.example` sets
`CB_PRIVATE_KEY=0xREPLACE_ME`. That file is the variable contract of
`entity-backend.compose.yaml` and the env the `composetemplate` test suite
interpolates it against; no deploy reads it. A placeholder documents the contract
where an empty string would not, and it is not key material.

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

Each provisioning toolkit defines the key-management boundary in its own
`engine/keyprovider` package. Per the scenario-isolation rule in the constitution
these are **two independent implementations**, not shared code, and they differ in
ways that matter here:

- `scenario-a/toolkit/engine/keyprovider`
- `scenario-b/toolkit/engine/keyprovider`

Both expose the same interface (`keyprovider.go`):

- `GenerateKey(ctx, id) -> pubkey` — creates and stores a secp256k1 key pair;
  returns only the public key. Idempotent.
- `Sign(ctx, id, digest) -> sig` — signs a 32-byte digest; returns a 65-byte
  Ethereum-compatible signature.
- `GetPublicKey(ctx, id) -> pubkey` — returns the stored public key.
- Package contract: "Private keys never appear in files, environment variables, or
  manifests."

**That contract describes the interface, not today's local tier.** Both packages
also declare a `LocalKeyExporter` (`ExportPrivateKeyHex`) implemented *only* by the
local emulator, and the Scenario B toolkit uses it: the per-entity identities that
`found-spoke`, `found-hub` and `join` create (each CB's hub signer, its relayer, its
token administrator, and each bank's key) are derived *through* the provider and then
exported as hex into the container environment. The spoke deployer identity is not
even derived — it is a constant in `orchestrator/localdev.go`. So in the local tier
the provider is a deterministic key *derivation* service whose output reaches process
and container environments; it is not yet a signing boundary that keys never cross.
Reaching the latter is what step 3 below is about.

The toolkit does not write these keys to any file: they are passed to `docker
compose` in the process environment (`engine/exec`). They are still visible to
anyone who can run `docker inspect` on the host, which is another reason the local
tier must stay valueless.

`factory.go` — `New(uri)` selects the backend from a `kms://` URI:

- `kms://local-emulator` — in-memory local provider for tests and local dev; keys
  are never persisted to disk. Scenario B additionally accepts
  `kms://local-emulator?seed=<domain>`, which makes derivation deterministic per
  namespace so that a re-apply reproduces the same addresses (on-chain roles are
  bound to them). Scenario A offers the same via `NewLocalKeyProviderSeeded`.
- `kms://<any-other>` — the production provider stub (`prod.go`), currently
  returning `ErrNotImplemented` in both toolkits. Neither stub implements
  `LocalKeyExporter`, by design: production never exports private key material.

### Wiring keys in staging/production

1. Set the entity's key-provider URI to the target backend (for example a cloud
   KMS or HSM endpoint) instead of `kms://local-emulator`, and admit the
   corresponding `spec.environment` value in the manifest validator.
2. Implement `prodKeyProvider` (`prod.go`) in **each** toolkit against the chosen
   HSM / KMS SDK. It must satisfy the same `KeyProvider` contract: generate and hold
   keys inside the HSM, and expose only public keys and signatures — never
   `LocalKeyExporter`. Until this is implemented, all methods return
   `ErrNotImplemented` — this is the intended guard, not a bug.
3. Remove the export path for non-local tiers. Both the toolkit (which currently
   exports derived keys into container env) and the services that read
   `*_PRIVATE_KEY` from the environment MUST instead request signatures from the
   `KeyProvider`. No deployment path may fall back to a plaintext key in a non-local
   environment.

### HSM / secret-store requirements

- Keys are generated inside the HSM/KMS and are non-exportable.
- Access is authorized per identity (central bank, MLP, deployer) with least
  privilege and full audit logging.
- Rotation and revocation are supported without redeploying application images.
- No secret is echoed to stdout/stderr or structured logs (see the observability
  rules in the constitution).

## Checklist for reviewers

Run the two automated gates first — they cover the live deploy path and are cheaper
and more reliable than reading by eye:

```bash
# 1. No literal key material in the compose templates the toolkit renders.
#    (checkNoSecrets in engine/composetemplate rejects hardcoded *_PRIVATE_KEY /
#     *_PASSWORD / *_SECRET values and embedded PEM blocks.)
cd scenario-b/toolkit && go test -count=1 ./engine/composetemplate/...

# 2. Repository-wide secret scan — the same two invocations CI runs
#    (.github/workflows/gitleaks.yml, gitleaks 8.28.0 pinned). Working tree, then
#    full history; --redact keeps any finding's value out of the log.
gitleaks detect --no-git --source . --config .gitleaks.toml --redact --exit-code 1
gitleaks git . --config .gitleaks.toml --redact --exit-code 1
```

Then confirm by inspection:

- No `.env.example` / `.env.*.example` file contains a non-empty `*_PRIVATE_KEY`
  value or a 64-character hex key (including inside comments).
- New services read signing keys through `KeyProvider`, not from plaintext
  environment variables, in staging/production paths.
- Copied `.env` and `.env.infra.*` files remain gitignored.
- Any new `.gitleaks.toml` exemption is a value in `regexes`, not a path glob. A
  glob exempts the file from **every** default rule (AWS, GCP, Azure, PEM); a value
  exempts one string. See the provenance notes in that file.

Related: `docs/deliverables/D11-secret-scan-report.md` (R2-11.3) records the scan
results and classifies each exemption as LIVE or PROVISIONAL against the three
deploy scripts. Key material in a PROVISIONAL (make-era) asset is resolved by
deleting the asset, not by annotating it.
