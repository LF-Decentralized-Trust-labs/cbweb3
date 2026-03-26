# Local PKI — CA Credentials

This directory holds the **Certificate Authority (CA)** credentials for each
entity across **two Besu spokes**. Each entity has its **own independent CA**
used to sign participant CSRs and bootstrap the governance participant on
startup.

**Architecture**

- **Spoke-A (chain 1338):** central-bank-a, bank-a, bank-c  
- **Spoke-B (chain 1339):** central-bank-b, bank-b, bank-d  

> **Security notice:** `*.key`, `*.crt`, and `*.pem` files in this directory are
> listed in `.gitignore` and must **never** be committed to the repository.

---

## File naming convention

| Entity          | CA certificate              | CA private key             |
|-----------------|-----------------------------|----------------------------|
| Central-Bank-A  | `central-bank-a-ca.crt`     | `central-bank-a-ca.key`    |
| Central-Bank-B  | `central-bank-b-ca.crt`     | `central-bank-b-ca.key`    |
| Bank-A          | `bank-a-ca.crt`             | `bank-a-ca.key`            |
| Bank-B          | `bank-b-ca.crt`             | `bank-b-ca.key`            |
| Bank-C          | `bank-c-ca.crt`             | `bank-c-ca.key`            |
| Bank-D          | `bank-d-ca.crt`             | `bank-d-ca.key`            |

---

## Generating local CA credentials (development only)

Use the Makefile targets from the **repository root** — they skip generation if
the key already exists (idempotent). To force regeneration add `FORCE=1`.

```bash
# All entity CAs + commercial bank certificates at once
make pki.gen-all

# Individual entity CAs
make pki.gen-central-bank-a
make pki.gen-central-bank-b
make pki.gen-bank-a
make pki.gen-bank-b
make pki.gen-bank-c
make pki.gen-bank-d

# Commercial bank participant certificates (bank-a through bank-d)
make pki.gen-commercial-banks
```

Alternatively, generate manually with `openssl`:

### Central-Bank-A CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/central-bank-a-ca.key

openssl req -new -x509 \
  -key backend/config/pki/central-bank-a-ca.key \
  -out backend/config/pki/central-bank-a-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-CentralBankA-CA/O=CentralBankA/C=BR"
```

### Bank-C CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/bank-c-ca.key

openssl req -new -x509 \
  -key backend/config/pki/bank-c-ca.key \
  -out backend/config/pki/bank-c-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-BankC-CA/O=BankC/C=BR"
```

### Bank-D CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/bank-d-ca.key

openssl req -new -x509 \
  -key backend/config/pki/bank-d-ca.key \
  -out backend/config/pki/bank-d-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-BankD-CA/O=BankD/C=BR"
```

After running generation, the directory should contain the six entity CA pairs
plus this file:

```
backend/config/pki/
├── central-bank-a-ca.crt   ← Central-Bank-A CA certificate
├── central-bank-a-ca.key   ← Central-Bank-A CA private key  (SECRET)
├── central-bank-b-ca.crt   ← Central-Bank-B CA certificate
├── central-bank-b-ca.key   ← Central-Bank-B CA private key  (SECRET)
├── bank-a-ca.crt           ← Bank-A CA certificate
├── bank-a-ca.key           ← Bank-A CA private key            (SECRET)
├── bank-b-ca.crt           ← Bank-B CA certificate
├── bank-b-ca.key           ← Bank-B CA private key            (SECRET)
├── bank-c-ca.crt           ← Bank-C CA certificate
├── bank-c-ca.key           ← Bank-C CA private key            (SECRET)
├── bank-d-ca.crt           ← Bank-D CA certificate
├── bank-d-ca.key           ← Bank-D CA private key            (SECRET)
└── README.md
```

---

## How the compliance service uses these files

The `compliance-orchestrator` reads the paths from two environment variables,
which are pre-configured per entity in the respective `.env.infra.*.example`
files:

| Entity         | `CA_CERT_FILE`                                                | `CA_KEY_FILE`                                                |
|----------------|---------------------------------------------------------------|--------------------------------------------------------------|
| Central-Bank-A | `/workspace/backend/config/pki/central-bank-a-ca.crt`       | `/workspace/backend/config/pki/central-bank-a-ca.key`        |
| Central-Bank-B | `/workspace/backend/config/pki/central-bank-b-ca.crt`       | `/workspace/backend/config/pki/central-bank-b-ca.key`        |
| Bank-A         | `/workspace/backend/config/pki/bank-a-ca.crt`               | `/workspace/backend/config/pki/bank-a-ca.key`                |
| Bank-B         | `/workspace/backend/config/pki/bank-b-ca.crt`               | `/workspace/backend/config/pki/bank-b-ca.key`                |
| Bank-C         | `/workspace/backend/config/pki/bank-c-ca.crt`               | `/workspace/backend/config/pki/bank-c-ca.key`                |
| Bank-D         | `/workspace/backend/config/pki/bank-d-ca.crt`               | `/workspace/backend/config/pki/bank-d-ca.key`                |

The Docker Compose files mount the `backend/config/pki` directory as
`/workspace/backend/config/pki` inside the container, so the paths above
resolve correctly without any extra configuration.

If `CA_CERT_FILE` is not set, the service starts in **dev mode** with certificate
issuance and governance bootstrap disabled (a warning is printed to the log).

---

## Production / staging environments

For non-local environments, **do not** place keys in this directory. Instead:

- Use **Docker Secrets** (`/run/secrets/ca.crt`, `/run/secrets/ca.key`) and
  override `CA_CERT_FILE` / `CA_KEY_FILE` accordingly.
- Or inject the paths via your secret management solution (Vault, AWS Secrets
  Manager, GCP Secret Manager, etc.).
