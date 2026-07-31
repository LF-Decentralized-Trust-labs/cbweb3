# Quickstart — TK-B6 (apply found-hub)

## Dry-run (sem efeitos)

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b apply -f <found-hub-manifest>.yaml --dry-run -o yaml
# report lista os steps (status: planned) na ordem, sem subir nada
```

## Fundar o hub de verdade (E2E — requer Docker + Foundry + Besu)

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b apply -f <found-hub-manifest>.yaml -o yaml
# sobe o nó do hub, builda + deploya CBWeb3Hub.s.sol, provisiona Keycloak
# (write-back de secrets), sobe infra/backend/frontend + relay + NOC,
# emite bundles/hub.bundle.yaml
```

Re-rodar converge (idempotente): steps satisfeitos aparecem como `skipped`.

## Testes

```bash
cd scenario-b/toolkit
go test ./engine/orchestrator/... ./engine/bundle/... ./engine/apply/... ./engine/exec/...
# motor (idempotência, ordem, dry-run, lock, retomada), bundle (round-trip, sem segredos),
# apply (dispatch, report) — com FakeRunner, sem Docker

# Suíte E2E (real; pulada com aviso se forge/docker/besu ausentes):
go test -tags e2e ./tests/e2e/...
```

Cobre: idempotência/ordem/lock/retomada (SC-001/002/003), dry-run sem efeitos (SC-004), ordem de
contratos (SC-005), write-back idempotente (SC-006), bundle round-trip sem segredos (SC-007),
manifesto inválido rejeitado (SC-008), fundação E2E + bundle vs chain viva (SC-009).

## Verificar o hub bundle

```bash
cat <out-dir>/bundles/hub.bundle.yaml   # version, chainId, hubRpc, contracts{...} — sem segredos
```

## Escopo

- Apenas `found-hub` nesta fase. Um manifesto `found-spoke`/`join` → erro "mode not supported yet"
  (TK-B7/B8).
- Não altera Makefiles nem `deploy/local` (o toolkit provisiona por conta própria).
