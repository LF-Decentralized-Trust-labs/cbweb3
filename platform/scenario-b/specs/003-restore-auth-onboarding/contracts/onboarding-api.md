# API Contracts: Auth & Onboarding Endpoints

**Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06

> Contratos das rotas restauradas no API Gateway. Todos os endpoints abaixo existem nos handlers (`onboarding.go`, `onboarding_proxy.go`, `auth.go`) mas estão atualmente retornando HTTP 404 por falta de registro no router.

---

## Banco Comercial (via OnboardingProxyHandler)

> **Pré-requisito**: `CENTRAL_BANK_API_URL` configurado no env do banco comercial.  
> **Autenticação**: Cookie `access_token` (JWT session) — via `RequireCookieAuth`.

### POST /api/v1/onboarding/initiate

Fase 1 — submete pedido de credencial ao Central Bank. O proxy injeta `csr_pem` e `blockchain_pub_key_hex` automaticamente (smart mode).

**Request body** (frontend precisa enviar apenas):
```json
{
  "institution_name": "Bank A S.A.",
  "bank_code": "a",
  "country": "BR",
  "role": "ROLE_COMMERCIAL_BANK",
  "email": "ops@bank-a.com.br",
  "username": "bank-a"
}
```

**Response 201**:
```json
{
  "request_id": "<uuid-keycloak-user-id>",
  "user_id": "<uuid>",
  "wallet_address": "0x...",
  "status": "CREDENTIAL_REQUESTED"
}
```

**Response 409** (banco já registrado):
```json
{ "error": "credential request: user already exists" }
```

**Response 502** (Central Bank indisponível):
```json
{ "error": "proxy: central bank unreachable: ..." }
```

---

### GET /api/v1/onboarding/status/:requestId

Fase 2.5 — polling de status pelo `request_id`. Encaminha para o Central Bank.

**Response 200**:
```json
{
  "request_id": "<uuid>",
  "user_id": "<uuid>",
  "status": "KYC_APPROVED",
  "wallet_address": "0x...",
  "pop_nonce": "a3f8..."
}
```

> `pop_nonce` só aparece quando `status = KYC_APPROVED` e o nonce não expirou.

---

### GET /api/v1/onboarding/my-status

Recupera status sem `request_id`. Resolve `bank_code` a partir do claim `BankID` do JWT.

**Response 200 (com registro)**:
```json
{
  "request_id": "<uuid>",
  "user_id": "<uuid>",
  "status": "CREDENTIAL_REQUESTED"
}
```

**Response 200 (sem registro)**:
```json
{ "status": "NONE" }
```

---

### POST /api/v1/onboarding/complete

Fase 3 — conclusão do onboarding com prova de posse. O proxy busca o `pop_nonce`, assina via KMS e encaminha ao Central Bank.

**Request body** (frontend precisa enviar apenas):
```json
{
  "request_id": "<uuid>",
  "user_id": "<uuid>"
}
```

**Response 200**:
```json
{
  "user_id": "<uuid>",
  "wallet_address": "0x...",
  "cert_pem": "-----BEGIN CERTIFICATE-----\n...",
  "tx_hash": "0x...",
  "client_secret": "<one-time-secret>",
  "status": "COMPLETED",
  "access_token": "<jwt>"
}
```

> O proxy encadeia o PKI login automaticamente e inclui `access_token` na resposta.

---

### POST /api/v1/auth/pki-login

Re-login PKI após onboarding. Lê o certificado salvo em `PKI_DIR/{bankCode}-participant.crt`.

**Request body**:
```json
{
  "user_id": "<uuid>",
  "client_secret": "<secret>"
}
```

**Response 200**:
```json
{ "accessToken": "<jwt>" }
```

---

## Central Bank (via OnboardingHandler)

> **Sem autenticação** — endpoints públicos que recebem pedidos dos bancos comerciais.

### POST /api/v1/onboarding/credential-request

Fase 1 (recepção). Valida CSR, cria usuário Keycloak, persiste participante com status `CREDENTIAL_REQUESTED`.

**Request body**:
```json
{
  "csr_pem": "-----BEGIN CERTIFICATE REQUEST-----\n...",
  "blockchain_pub_key_hex": "04...",
  "institution_name": "Bank A S.A.",
  "bank_code": "a",
  "country": "BR",
  "role": "ROLE_COMMERCIAL_BANK",
  "email": "ops@bank-a.com.br",
  "username": "bank-a"
}
```

**Response 201**:
```json
{
  "request_id": "<uuid>",
  "user_id": "<uuid>",
  "wallet_address": "0x...",
  "status": "CREDENTIAL_REQUESTED"
}
```

---

### GET /api/v1/onboarding/status/:requestId

Fase 2.5 (polling direto no CB). Retorna status atual incluindo `pop_nonce` quando aprovado.

---

### GET /api/v1/onboarding/my-status?bank_code=:bankCode

Lookup por `bank_code`. Utilizado pelo proxy do banco comercial.

**Response 200**: igual ao endpoint de status por `request_id`, sem `pop_nonce`.

---

### POST /api/v1/onboarding/complete

Fase 3 (recepção). Valida PoP, emite certificado, registra on-chain, gera `client_secret`.

---

## Auth — Login PKI de 2 Fatores (Bancos Comerciais)

### POST /api/v1/auth/login

**Para bancos comerciais** (`ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY`): retorna nonce de 30 minutos.

**Request body**:
```json
{
  "clientId": "bank-a-client",
  "clientSecret": "<secret>"
}
```

**Response 200 (fluxo PKI — banco comercial)**:
```json
{ "nonce": "a3f8e2d1..." }
```

**Response 200 (fluxo direto — governança)**:
```json
{
  "accessToken": "<jwt>",
  "tokenType": "Bearer",
  "expiresIn": 300
}
```

---

### POST /api/v1/auth/wallet/bind

Passo 2 do login PKI — submete assinatura do nonce + certificado X.509.

**Request body**:
```json
{
  "userId": "<uuid>",
  "signatureHex": "3045...",
  "certPem": "-----BEGIN CERTIFICATE-----\n..."
}
```

**Response 200**: seta cookies `access_token` + `refresh_token` (HttpOnly).

---

## Compliance — Aprovação/Rejeição KYC

> **Autenticação**: Cookie `access_token` + `ROLE_GOVERNANCE`.

### POST /api/v1/compliance/approve-kyc

Fase 2 — Central Bank aprova KYC do banco.

**Request body**:
```json
{ "user_id": "<uuid>" }
```

**Response 200**:
```json
{ "status": "KYC_APPROVED", "pop_nonce": "a3f8..." }
```

---

### POST /api/v1/compliance/reject-kyc *(a verificar se existe no handler atual)*

Fase 2 — Central Bank rejeita KYC com motivo.

**Request body**:
```json
{
  "user_id": "<uuid>",
  "rejection_reason": "Documentação incompleta"
}
```

**Response 200**:
```json
{ "status": "KYC_REJECTED" }
```
