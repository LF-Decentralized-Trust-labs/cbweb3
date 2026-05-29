# Deploy Directory

This directory centralizes deployment assets for the CBWeb3 platform.

Following the architecture defined in the project root documentation, `deploy/` is the operational layer that turns code into runnable environments with reproducible configuration and clear environment boundaries.

## Purpose

The `deploy/` folder is responsible for:

- infrastructure provisioning definitions,
- runtime manifests and environment configuration,
- local developer runtime orchestration,
- repeatable promotion paths across environments (dev, staging, testnet, production).

In short, it is where platform services, ledgers, and supporting infrastructure are wired for execution.

## Folder scope

Current and planned subfolders:

- [local](local): local runtime for development and integration testing (Besu, Keycloak, PostgreSQL).

## Relationship with the platform architecture

From the main repository structure:

- application code lives in `backend/`, `frontend/`, `contracts/`, and `interop/`,
- API contracts live in `apis/openapi/`,
- operational execution and environment realization live in `deploy/`.

This separation keeps development modular and deployment reproducible.

## Environment strategy

`deploy/` should maintain environment-specific values without changing application source code.

Recommended approach:

- keep reusable templates and defaults under each deploy target,
- override values per environment via environment variables/values files,
- avoid hardcoded credentials and ports,
- keep secrets out of Git and use secret managers when available.

## Local deployment reference

For the complete local stack documentation (components, communication model, and configuration), see:

- [local/README.md](local/README.md)

## Operational principles

This directory should follow the same governance principles highlighted in the main project README:

- reproducibility,
- clear ownership,
- secure configuration handling,
- environment consistency,
- auditable changes through version control.

## Security note

Do not commit real credentials, tokens, or private keys in this directory.
