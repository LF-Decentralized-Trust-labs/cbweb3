# Contrato: Endpoint credential-request do Banco Central

**Feature**: `032-commercial-bank-join`
**Consumidor**: `step_request_cert.go` + `pki.SubmitCSRToCB()`
**Produtor**: API Gateway do banco central (`onboarding_proxy.go`, smart mode)

## Contrato HTTP

### Request

```
POST {bundle.spec.cbEndpoint}
Content-Type: application/json

{
  "bank_code":        "commercial-bank-alpha",
  "institution":      "Alpha Bank S.A.",
  "csr_pem":          "-----BEGIN CERTIFICATE REQUEST-----\n...\n-----END CERTIFICATE REQUEST-----\n",
  "blockchain_pubkey": "0x04abcdef..."  // secp256k1 uncompressed pubkey hex
}
```

### Response (sucesso)

```
HTTP 200 OK
Content-Type: application/json

{
  "cert_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
  "status": "issued"
}
```

### Response (pendente — assíncrono)

```
HTTP 202 Accepted
Content-Type: application/json

{
  "request_id": "req-abc123",
  "status": "pending",
  "poll_url": "/api/v1/credential-requests/req-abc123"
}
```

### Response (erro)

```
HTTP 4xx/5xx
Content-Type: application/json

{
  "error": "CSR validation failed: OU must be ROLE_COMMERCIAL_BANK",
  "code": "INVALID_CSR"
}
```

## Comportamento de `step_request_cert.go`

1. Se a resposta é HTTP 200 → extrair `cert_pem`, passar para `step_receive_cert` via estado ou arquivo intermediário
2. Se a resposta é HTTP 202 → persistir `request_id` e `poll_url` no estado; `step_receive_cert` usa o `poll_url`
3. Se a resposta é HTTP 4xx → falhar com o erro do CB (sem retry)
4. Se a resposta é HTTP 5xx → falhar com erro de disponibilidade (sem retry no passo; operador re-run)

## Idempotência

O endpoint do CB é responsável por detectar reenvio do mesmo CSR (por `bank_code` e hash do CSR) e retornar o cert já emitido em vez de emitir duplicata. O motor confia nesta propriedade para não duplicar CSRs em re-runs.

## Segurança

- A chave privada TLS (`{bankCode}.key`) NUNCA é enviada — apenas o CSR (chave pública)
- A chave privada blockchain NUNCA é enviada — apenas a pubkey hex
- O payload deve ser transmitido sobre HTTPS em produção; em local pode usar HTTP (env `local`)
