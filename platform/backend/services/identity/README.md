# identity service

Identity gRPC microservice using a provider/adapter approach to decouple external integrations.

## Goal

- Expose a single internal contract via gRPC for login, wallet creation, token validation, wallet binding, onboarding, and signing.
- Select the provider through configuration (`local` or `dwallet_api`).
- Keep `local` as an in-memory package (without a separate HTTP mock).

## Structure

- `internal/identityprovider`: identity contract and shared types.
- `internal/identityprovider/providers`: identity providers factory and concrete implementations.
- `internal/kmsprovider`: KMS contract and shared types.
- `internal/kmsprovider/providers`: KMS factory and concrete implementations.
- `internal/dataaccessclient`: gRPC client for participants metadata in `data-access`.
- `internal/tokenissuer`: internal JWT issuer (`local` or `keycloak`).
- `internal/grpc`: gRPC contract and server.
- `cmd/identity`: gRPC microservice bootstrap.

## Suggested variables

- `IDENTITY_PROVIDER=local|dwallet_api`
- `IDENTITY_GRPC_PORT=9091`
- `IDENTITY_HOST_URL=https://<real-endpoint>`
- `IDENTITY_JWT_SECRET=<optional>`
- `IDENTITY_ACCESS_TOKEN_TTL_SEC=<optional, default 3600>`
- `IDENTITY_REQUEST_TIMEOUT_SEC=5`
- `DATA_ACCESS_GRPC_ADDR=localhost:9092`
- `DATA_ACCESS_REQUEST_TIMEOUT_SEC=5`
- `IDENTITY_KMS_PROVIDER=localkms|awskms|gcpkms` (`localkms` implemented now)
- `INTERNAL_JWT_PROVIDER=local|keycloak`
- `INTERNAL_JWT_SECRET=<optional>`
- `INTERNAL_JWT_TTL_SEC=<optional, default 3600>`
- `INTERNAL_KEYCLOAK_TOKEN_URL=<required if INTERNAL_JWT_PROVIDER=keycloak>`
- `INTERNAL_KEYCLOAK_CLIENT_ID=<required if INTERNAL_JWT_PROVIDER=keycloak>`
- `INTERNAL_KEYCLOAK_CLIENT_SECRET=<required if INTERNAL_JWT_PROVIDER=keycloak>`
- `INTERNAL_KEYCLOAK_TIMEOUT_SEC=<optional, default 5>`

## Example configuration

### Internal JWT via local issuer

```bash
IDENTITY_PROVIDER=local
IDENTITY_GRPC_PORT=9091
IDENTITY_HOST_URL=http://localhost:8081
IDENTITY_JWT_SECRET=local-identity-secret
IDENTITY_ACCESS_TOKEN_TTL_SEC=3600
IDENTITY_REQUEST_TIMEOUT_SEC=5
DATA_ACCESS_GRPC_ADDR=localhost:9092
DATA_ACCESS_REQUEST_TIMEOUT_SEC=5

IDENTITY_KMS_PROVIDER=localkms

INTERNAL_JWT_PROVIDER=local
INTERNAL_JWT_SECRET=identity-internal-secret
INTERNAL_JWT_ISSUER=identity-internal
INTERNAL_JWT_AUDIENCE=cbweb3-internal
INTERNAL_JWT_TTL_SEC=3600
```

### Internal JWT via Keycloak issuer

```bash
IDENTITY_PROVIDER=dwallet_api
IDENTITY_GRPC_PORT=9091
IDENTITY_HOST_URL=https://<dwallet-api-host>
IDENTITY_REQUEST_TIMEOUT_SEC=5
DATA_ACCESS_GRPC_ADDR=localhost:9092
DATA_ACCESS_REQUEST_TIMEOUT_SEC=5

IDENTITY_KMS_PROVIDER=localkms

INTERNAL_JWT_PROVIDER=keycloak
INTERNAL_KEYCLOAK_TOKEN_URL=http://localhost:8080/realms/cbweb3/protocol/openid-connect/token
INTERNAL_KEYCLOAK_CLIENT_ID=cbweb3-identity
INTERNAL_KEYCLOAK_CLIENT_SECRET=change-me
INTERNAL_KEYCLOAK_TIMEOUT_SEC=5
```

