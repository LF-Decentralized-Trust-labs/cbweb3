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

## Keycloak realm posture (R1-10.7)

The realms the toolkit provisions used to ship three permissive defaults, identical on a
laptop and on a routable host: `redirectUris: ["*"]`, `webOrigins: ["*"]` and
`sslRequired: "none"`. Two rules now govern them — and a third governs the `deploy/local`
realms, provisioned by a different path, which carried a permissive token lifespan of
their own.

**Browser origins come from one source.** A realm's `redirectUris` and `webOrigins` are
derived from the same origin list the entity's api-gateway receives as
`CORS_ALLOW_ORIGINS` — `cbCORSOriginsFor` / `bankCORSOriginsFor` in scenario-a,
`nocPortalOrigins` for the public `noc-portal` client in scenario-b.

In **scenario-a** that is an identity, and it is pinned. Both sides are built from the
same call inside `buildSteps`, and `TestCBRealmOriginsMatchTheGatewayCORS` /
`TestBankRealmOriginsMatchTheGatewayCORS` walk the steps the engine actually assembles,
recompute the gateway's list from the env step's own inputs, and fail if the two ever
disagree. Keycloak and the gateway therefore cannot drift from each other, nor from the
ports the stack publishes.

In **scenario-b** the two lists are related but not identical, and the claim should not be
stated more strongly than that. The spoke is consistent: `corsOriginsCB` includes the NOC
portal's RPC+12000 origin that `nocPortalOrigins` grants the client. The hub is not —
`nocPortalOrigins` gives the `noc-portal` client RPC+12000, while the hub gateway's
`CORS_ALLOW_ORIGINS` is `corsOriginSingle`, RPC+9000 only (`step_found_hub.go`). That
mismatch is pre-existing and inert today, because `found-hub` never composes the
`noc-stack` the client exists for, but it is a mismatch and not an invariant.

A wildcard here is not cosmetic. `redirectUris: ["*"]` makes the realm an open redirector
for the authorization code, and `webOrigins: ["*"]` lets any page read a token response.
The `noc-portal` client is the sharpest case: it is **public** and uses a direct password
grant, so no client secret stands between an attacker's page and a usable token.

`renderRealmJSON` **fails closed** — a plan that reaches it with no origins is an error,
not a document that falls back to a wildcard. That is deliberate: the wildcard existed
precisely because an absent value had a permissive default.

**`sslRequired` follows `spec.environment`.** Only an explicit `local` gets `none`, which
is the affordance a developer needs to reach Keycloak and the portals over plain HTTP.
Every other value — including an empty one — gets Keycloak's own default, `external`: TLS
demanded on any non-private address.

The consequence for a deployment is worth stating plainly. A stack that serves its portals
over **plain HTTP on a routable address** must declare `spec.environment: local`, or
Keycloak will refuse the password grant with `HTTPS required`. That is the intended
pressure: the alternative is to put a TLS terminator in front, which is what a non-local
deployment should be doing anyway.

The field is no longer optional anywhere, so an absent value cannot reach the renderer by
accident. There is **no JSON-Schema for the manifest in this repository** — the Go
validators are the whole rule. `scenario-b/toolkit/engine/manifest/validate.go` requires
`spec.environment` and accepts only `local` in this phase (staging/prod are rejected
outright); `scenario-a/toolkit/engine/manifest/validate.go` requires it and accepts
`local`, `staging` or `prod`. Requiring it is what turns a forgotten field into a
validation error naming it, instead of a login failing with `HTTPS required` far from the
cause.

**Access-token lifespan.** The realms the toolkit provisions never set
`accessTokenLifespan`, so they inherit Keycloak's own default of 300 seconds. The
`deploy/local` path is a **second enforcement point**, and it did set one: `init.sh` — the
Keycloak container's entrypoint, so every `make *.up` — defaulted to 21600 (six hours),
and `setup-noc-realm.sh` created the NOC realm at 86400 (twenty-four). Both now default to
300, overridable through `KC_ACCESS_TOKEN_LIFESPAN`.

The lifespan is the window in which a leaked, logged or shoulder-surfed bearer is
replayable, and it costs nothing to shorten here: every portal renews silently — a
proactive refresh timer plus a 401-retry interceptor (`services/api/token-refresh.ts`, and
`apps/noc/src/services/api/token.ts`, whose own comment already assumed a 5-minute token).
`TestDeployLocalAccessTokenLifespanIsShort` in each toolkit reads those scripts, rejects
any value above 900 seconds, and fails closed if it can no longer find a lifespan
declaration at all.

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
