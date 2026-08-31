# auth

> [scenario-a](../../../README.md) › [backend](../../README.md) › auth

The **auth** service (internally called `identity`) is the identity and authentication microservice. It handles user login, token lifecycle, wallet creation and binding, PKI-based verification, and participant onboarding — exposing all operations via gRPC.

---

## Architecture Placement

```
[api-gateway]
      │
      ▼
  [auth]   gRPC :9091
      │
      ├──► Keycloak          JWT issuance and validation
      ├──► KMS (local/AWS)   Key management for wallet signing
      ├──► Redis             Nonce store (replay protection)
      ├──► Besu              On-chain participant registration
      └──► [compliance]  gRPC   Participant role resolution
```

---

## Responsibilities

- **Login** — Authenticates users via Keycloak OIDC; issues access and refresh tokens.
- **Token validation** — Verifies Bearer tokens on behalf of the api-gateway.
- **Token revocation** — Invalidates active sessions on logout.
- **Wallet creation** — Generates Ethereum key pairs; supports local KMS and AWS KMS modes.
- **Wallet binding** — Associates a Keycloak user identity with an on-chain Ethereum address, verified via signature.
- **PKI login** — Two-factor authentication via X.509 certificate challenge/response.
- **Onboarding** — Registers new users in Keycloak and on-chain.

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | gRPC |
| Port | `9091` (per-entity offset in compose) |
| Language | Go |

### Provider Model

The service uses a provider/adapter pattern to decouple external identity integrations:

| Provider | `IDENTITY_PROVIDER` value | Description |
|----------|--------------------------|-------------|
| Local | `local` | In-process key management, no external KMS |
| dWallet API | `dwallet_api` | External distributed wallet API |

The JWT issuer is similarly configurable:

| JWT Provider | `INTERNAL_JWT_PROVIDER` value | Description |
|-------------|------------------------------|-------------|
| Local | `local` | In-process HMAC-signed JWTs |
| Keycloak | `keycloak` | JWTs issued and validated by Keycloak |

### Key gRPC RPCs

| RPC | Description |
|-----|-------------|
| `Login` | Authenticate user, return access + refresh tokens |
| `RefreshToken` | Exchange refresh token for a new access token |
| `RevokeToken` | Invalidate a session |
| `ValidateToken` | Verify a Bearer token and return claims |
| `CreateWallet` | Generate a new Ethereum key pair |
| `BindWallet` | Associate a user identity with a wallet address |
| `VerifyPKILogin` | PKI-based 2FA challenge/response |
| `OnboardUser` | Register a new user in Keycloak and on-chain |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `IDENTITY_PROVIDER` | `local` or `dwallet_api` |
| `IDENTITY_GRPC_PORT` | gRPC listen port (default: 9091) |
| `IDENTITY_HOST_URL` | External identity provider URL (for `dwallet_api`) |
| `INTERNAL_JWT_PROVIDER` | `local` or `keycloak` |
| `INTERNAL_KEYCLOAK_TOKEN_URL` | Keycloak token endpoint (for `keycloak` provider) |
| `INTERNAL_KEYCLOAK_CLIENT_ID` | Keycloak client ID |
| `INTERNAL_KEYCLOAK_CLIENT_SECRET` | Keycloak client secret |
| `REDIS_ADDR` | Redis address for nonce store |
| `BESU_RPC_URL` | Besu JSON-RPC endpoint for on-chain registration |

---

## Internal Structure

```
auth/
├── cmd/identity/main.go              Entrypoint
├── internal/
│   ├── grpc/                         gRPC server and handler
│   ├── identityprovider/             Provider interface and shared types
│   │   └── providers/                Concrete provider implementations
│   ├── tokenissuer/                  JWT issuer (local or Keycloak)
│   └── dataaccessclient/             gRPC client for participant metadata
└── Dockerfile
```

---

## Related

- [api-gateway](../api-gateway/README.md) — calls this service for login and token validation
- [compliance](../compliance/README.md) — called by this service for participant role resolution
- [deploy › keycloak](../../../deploy/README.md) — OIDC provider this service integrates with
