# Quickstart — TK-B7 (apply found-spoke)

Pré-requisito: um **hub fundado** (hub bundle emitido pelo `found-hub`, TK-B6).

## Dry-run (sem efeitos)

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b apply -f <found-spoke-manifest>.yaml --dry-run -o yaml
# report lista os steps do found-spoke (planned) na ordem, sem subir nada
```

## Fundar o spoke de verdade (E2E — requer hub fundado + Docker/Foundry/Besu)

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b apply -f <found-spoke-manifest>.yaml \
  --hub-rpc <hub-rpc> --spoke-rpc <spoke-rpc> --gateway-url <spoke-gateway> --relay relay://<host>:4000
# consome o hub bundle, registra o CB no hub, gera genesis do spoke, sobe o nó (CB validador),
# deploya os contratos do spoke, faz wire dos endereços do hub, provisiona Keycloak,
# sobe infra/backend/frontend, registra o spoke no relay, sobe o noc-agent (soft),
# emite bundles/spoke-<id>.bundle.yaml (genesis + enode + endereços)
```

Re-rodar converge (idempotente). `add-noc-agent` que falha aparece como `soft-failed` e **não**
derruba o `found-spoke`.

## Testes

```bash
cd scenario-b/toolkit
go test ./engine/orchestrator/... ./engine/bundle/... ./engine/apply/...
# found-spoke com FakeRunner: ordem/deps, register-cb vs hub, deploy via CBWeb3Spoke.s.sol,
# wire/keycloak idempotentes, register-relay via RelayRegistrar fake, soft add-noc-agent,
# spoke bundle round-trip (genesis+enode, sem segredos)

# E2E (real; pulado com aviso se hub/forge/docker/besu ausentes):
go test -tags e2e ./tests/e2e/...
```

Cobre SC-001…SC-009.

## Inspecionar o spoke bundle

```bash
cat <out-dir>/bundles/spoke-<id>.bundle.yaml   # version, spokeId, chainId, enode, genesis, contracts — sem segredos
```

## Escopo

- Apenas `found-spoke`. Par soberano / liquidez / seed-oracle são **TK-B9**.
- `found-hub` (TK-B6) continua funcionando; `join` → "not supported yet" (TK-B8).
- Não altera Makefiles nem `deploy/local`.
