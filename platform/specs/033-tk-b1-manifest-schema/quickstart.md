# Quickstart — TK-B1 (validar um manifesto)

Escopo desta fase: **validar** manifestos do Cenário B (sem provisionar nada).

## Pré-requisitos

- Go 1.26+
- Módulo `scenario-b/toolkit` compilado (`cmd/cbweb3b`)

## Validar um manifesto

```bash
cd scenario-b/toolkit
go run ./cmd/cbweb3b validate -f specs/033-tk-b1-manifest-schema/contracts/examples/found-hub.yaml
# → valid: true (exit 0)
```

## Validar um conjunto (checa colisões)

```bash
go run ./cmd/cbweb3b validate \
  -f .../examples/found-spoke.yaml \
  -f .../examples/join.yaml \
  -o json
# → report JSON com errors[]/warnings[] por manifesto; exit 1 se houver colisão de chainId/porta
```

## O que esperar

- **Válido** → `valid: true`, exit `0` (pode haver `warnings[]`, ex.: `join` com `validator: true`).
- **Inválido** → `valid: false`, exit `1`, `errors[]` nomeando cada campo/recurso (todas as
  violações de uma vez, não só a primeira).
- **Erro de uso/parse** → exit `2`.

## Critérios de aceite exercitados (ver spec §Success Criteria)

- SC-001: os três exemplos (`found-hub`/`found-spoke`/`join`) validam sem erro.
- SC-002/SC-005: casos-borda (modo inválido, campo faltante/estranho, `environment` não-local,
  segredo embutido, `certSource`/`keyProvider` inválidos) são rejeitados com mensagem nomeada.
- SC-003: colisão de `chainId`/porta entre 2+ manifestos falha nomeando os conflitantes.
- SC-004: JSON-Schema e validação Go concordam nas regras estruturais.
