# Contrato da CLI — validação (TK-B1)

O único ponto de invocação desta fase: parseia + valida + reporta, **sem executar steps**.

## Comando

```
cbweb3b validate -f <manifest.yaml> [-o json|yaml]
cbweb3b apply    -f <manifest.yaml> --dry-run [-o json|yaml]
```

- `validate` e `apply --dry-run` são equivalentes nesta fase (ambos só validam; `apply` sem
  `--dry-run` é implementado numa fase posterior).
- `-f`/`--file` (obrigatório): caminho do manifesto. Pode ser repetido para **validar um
  conjunto** (habilita as checagens de colisão entre manifestos).
- `-o`/`--output` (opcional, default `yaml`): formato do report (`json`|`yaml`).

## Saída (report)

Objeto com:
- `manifest`: `metadata.name` + `spec.mode` (por arquivo, quando conjunto).
- `valid`: bool.
- `errors[]`: `{ field, message }` — coletados (não para na primeira violação).
- `warnings[]`: `{ field, message }` — ex.: `join` com `validator: true`.

## Exit codes

- `0` — todos os manifestos válidos (pode haver warnings).
- `1` — ≥1 erro de validação (report ainda é emitido).
- `2` — erro de uso/parse (arquivo ausente, YAML malformado, flags inválidas).

## Exemplos

```
# válido → exit 0
cbweb3b validate -f examples/found-hub.yaml

# faltando hubBundleRef no found-spoke → exit 1, errors[] aponta o campo
cbweb3b validate -f bad-found-spoke.yaml

# conjunto com chainId colidente → exit 1, errors[] nomeia os dois manifestos
cbweb3b validate -f spoke-a.yaml -f spoke-b.yaml -o json
```
