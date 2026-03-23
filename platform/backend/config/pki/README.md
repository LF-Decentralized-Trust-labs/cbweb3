# Local PKI — CA Credentials

This directory holds the **Certificate Authority (CA)** credentials for each
environment. Each Central Bank node (Hub, Spoke A / BCB-A, Spoke B / BCB-B)
has its **own independent CA** used to sign participant CSRs and bootstrap the
governance participant on startup.

> **Security notice:** `*.key`, `*.crt`, and `*.pem` files in this directory are
> listed in `.gitignore` and must **never** be committed to the repository.

---

## File naming convention

| Environment | CA certificate         | CA private key         |
|-------------|------------------------|------------------------|
| Hub         | `hub-ca.crt`           | `hub-ca.key`           |
| Spoke A (BCB-A) | `spoke-a-ca.crt`   | `spoke-a-ca.key`       |
| Spoke B (BCB-B) | `spoke-b-ca.crt`   | `spoke-b-ca.key`       |

---

## Generating local CA credentials (development only)

Run the commands below from the **repository root**. They create self-signed CAs
using ECDSA P-256 — the same curve used by the platform's X.509 certificates.

### Hub CA

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/hub-ca.key

openssl req -new -x509 \
  -key backend/config/pki/hub-ca.key \
  -out backend/config/pki/hub-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-Hub-CA/O=CBWeb3-Hub/C=BR"
```

### Spoke A — BCB-A

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/spoke-a-ca.key

openssl req -new -x509 \
  -key backend/config/pki/spoke-a-ca.key \
  -out backend/config/pki/spoke-a-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-SpokeA-CA/O=BCB-A/C=BR"
```

### Spoke B — BCB-B

```bash
openssl ecparam -genkey -name prime256v1 -noout \
  -out backend/config/pki/spoke-b-ca.key

openssl req -new -x509 \
  -key backend/config/pki/spoke-b-ca.key \
  -out backend/config/pki/spoke-b-ca.crt \
  -days 3650 \
  -subj "/CN=CBWeb3-SpokeB-CA/O=BCB-B/C=BR"
```

After running these commands the directory should contain:

```
backend/config/pki/
├── hub-ca.crt          ← Hub CA certificate
├── hub-ca.key          ← Hub CA private key     (SECRET)
├── spoke-a-ca.crt      ← BCB-A CA certificate
├── spoke-a-ca.key      ← BCB-A CA private key   (SECRET)
├── spoke-b-ca.crt      ← BCB-B CA certificate
├── spoke-b-ca.key      ← BCB-B CA private key   (SECRET)
└── README.md
```

---

## How the compliance service uses these files

The `compliance-orchestrator` reads the paths from two environment variables,
which are pre-configured per environment in the respective `.env.infra.*.example`
files:

| Environment | `CA_CERT_FILE`                                  | `CA_KEY_FILE`                                  |
|-------------|-------------------------------------------------|------------------------------------------------|
| Hub         | `/workspace/backend/config/pki/hub-ca.crt`      | `/workspace/backend/config/pki/hub-ca.key`     |
| Spoke A     | `/workspace/backend/config/pki/spoke-a-ca.crt`  | `/workspace/backend/config/pki/spoke-a-ca.key` |
| Spoke B     | `/workspace/backend/config/pki/spoke-b-ca.crt`  | `/workspace/backend/config/pki/spoke-b-ca.key` |

The Docker Compose files mount the repository root as `/workspace` inside the
container, so the paths above resolve correctly without any extra configuration.

If `CA_CERT_FILE` is not set, the service starts in **dev mode** with certificate
issuance and governance bootstrap disabled (a warning is printed to the log).

---

## Production / staging environments

For non-local environments, **do not** place keys in this directory. Instead:

- Use **Docker Secrets** (`/run/secrets/ca.crt`, `/run/secrets/ca.key`) and
  override `CA_CERT_FILE` / `CA_KEY_FILE` accordingly.
- Or inject the paths via your secret management solution (Vault, AWS Secrets
  Manager, GCP Secret Manager, etc.).
