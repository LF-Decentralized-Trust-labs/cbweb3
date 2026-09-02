# Changelog — Authentication & Session Contract

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Contract versions track the **CBWeb3 API Gateway version** they describe, so this file
starts at 2.3.0.

## [2.3.0] — 2026-08-20

### Corrected before publication (2026-08-21)

- The example `client_secret` inherited verbatim from the platform document — a
  plausible-looking 32-byte base64 value — is replaced by the self-labelling placeholder
  `SYNTHETIC-CLIENT-SECRET-DO-NOT-USE`, matching every other credential in the kit
  (`mocks/auth/*`). It was never a leak, but an unlabelled secret-shaped string does not
  belong in the artefact a DPG submission points at. The same applies to
  `new_client_secret`.

### Added

- Initial publication. Derived from the delivered **CBWeb3 API Gateway v2.3.0** OpenAPI
  document, cross-checked against the gateway's Go router, middleware and handlers.
- 8 operations: `GET /healthz` and the seven `/api/v1/auth/*` operations
  (`login`, `walletBind`, `refreshToken`, `logout`, `getMe`, `pkiLogin`,
  `changeClientSecret`). `operationId` values match the platform's own.
- `CookieAuth` security scheme (`access_token`, HttpOnly, `SameSite=Strict`) — the real
  scheme. **No `bearerAuth` is declared**, because the `/api/v1` surface does not accept
  one, and **no CSRF scheme is declared**, because the platform implements none.
- The real `ErrorResponse` (`{error}`) — a bare human-readable message with no
  machine-readable code. Not RFC 7807, and not the invented coded error model that
  earlier Toolbox drafts assumed.
- Both login flows documented, including the PKI nonce shape that exists in the platform
  document in prose only.
- Five documented divergences between the platform's OpenAPI document and the delivered
  gateway (see `README.md`), each marked inline in the contract.

### Notes

- This contract did not exist before 2.3.0. It was extracted because the authentication
  block is byte-identical between the Scenario A and Scenario B gateways, so publishing it
  once prevents the two scenario contracts from drifting on session bootstrap.
- There are **no compatibility guarantees** with any earlier Toolbox artefact. The
  previous PvP contract (`openapi_pvp_v0.1.0.yaml`) assumed `Authorization: Bearer`, which
  the platform has never supported on this surface.
