# Local PKI — CA Credentials

This directory holds the **Certificate Authority (CA)** credentials for each
entity in **spoke-a**. All three entities — Central Bank, Bank-A and Bank-B —
share the same spoke-a Besu network (chain 1338) but each has its **own
independent CA** used to sign participant CSRs and bootstrap the governance
participant on startup.

> **Security notice:** `*.key`, `*.crt`, and `*.pem` files in this directory are
> listed in `.gitignore` and must **never** be committed to the repository.

---

## File naming convention

| Entity          | CA certificate           | CA private key           |
|-----------------|--------------------------|--------------------------|
| Central Bank    | `central-bank-ca.crt`    | `central-bank-ca.key`    |
| Bank-A          | `bank-a-ca.crt`          | `bank-a-ca.key`          |
| Bank-B          | `bank-b-ca.crt`          | `bank-b-ca.key`          |

---

## Generating local CA credentials (development only)

Use the Makefile targets from the **repository root** — they skip generation if
the key already exists (idempotent). To force regeneration add `FORCE=1`.

```bash
# All entity CAs + commercial bank certificates at once
make pki.gen-all

# Individual entity CAs
make pki.gen-central-bank
make pki.gen-bank-a
make pki.gen-bank-b

# Commercial bank participant certificates (bank-001 … bank-006)
make pki.gen-commercial-banks
```

Alternatively, generate manually with `openssl`:

### Central Bank CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/central-bank-ca.key

openssl req -new -x509 \
  -key backend/config/pki/central-bank-ca.key \
  -out backend/config/pki/central-bank-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-CentralBank-CA/O=CentralBank/C=BR"
```

### Bank-A CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/bank-a-ca.key

openssl req -new -x509 \
  -key backend/config/pki/bank-a-ca.key \
  -out backend/config/pki/bank-a-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-BankA-CA/O=BankA/C=BR"
```

### Bank-B CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/bank-b-ca.key

openssl req -new -x509 \
  -key backend/config/pki/bank-b-ca.key \
  -out backend/config/pki/bank-b-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-BankB-CA/O=BankB/C=BR"
```

After running these commands the directory should contain:

```
backend/config/pki/
├── central-bank-ca.crt   ← Central Bank CA certificate
├── central-bank-ca.key   ← Central Bank CA private key  (SECRET)
├── bank-a-ca.crt         ← Bank-A CA certificate
├── bank-a-ca.key         ← Bank-A CA private key        (SECRET)
├── bank-b-ca.crt         ← Bank-B CA certificate
├── bank-b-ca.key         ← Bank-B CA private key        (SECRET)
└── README.md
```

---

## How the compliance service uses these files

The `compliance-orchestrator` reads the paths from two environment variables,
which are pre-configured per entity in the respective `.env.infra.*.example`
files:

| Entity       | `CA_CERT_FILE`                                           | `CA_KEY_FILE`                                           |
|--------------|----------------------------------------------------------|---------------------------------------------------------|
| Central Bank | `/workspace/backend/config/pki/central-bank-ca.crt`     | `/workspace/backend/config/pki/central-bank-ca.key`    |
| Bank-A       | `/workspace/backend/config/pki/bank-a-ca.crt`           | `/workspace/backend/config/pki/bank-a-ca.key`          |
| Bank-B       | `/workspace/backend/config/pki/bank-b-ca.crt`           | `/workspace/backend/config/pki/bank-b-ca.key`          |

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
