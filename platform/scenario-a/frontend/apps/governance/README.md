# governance portal

> [scenario-a](../../../README.md) › [frontend](../../README.md) › governance

The **governance portal** is the web interface for Central Bank operators. It provides governance over the spoke's participants, policy controls, and CBDC lifecycle management — including minting, burning, and on-chain participant registration.

---

## Architecture Placement

```
Browser (Central Bank Operator)
         │
         ▼
  [governance portal]   :5177 / :5178
         │
         ▼
  [api-gateway]   REST :38080 / :60080
```

One instance is deployed per central bank (Central-Bank-A, Central-Bank-B), each pointing to its own api-gateway.

---

## Users

Central bank operators — governors, compliance officers, registry administrators.

---

## Key Features

- **Participant registry** — Register, view, and manage commercial banks and other participants on the spoke's `IdentityRegistry` contract.
- **CBDC issuance** — Mint and burn tCeBM tokens, enforcing the central bank's exclusive monetary authority.
- **Governance certificates** — Issue PKI certificates that attest to a participant's governance role.
- **Compliance overview** — Monitor Onboarding status, compliance flags, and transaction activity across all spoke participants.
- **Policy controls** — Configure CBDC transfer limits, circuit breakers, and other governance parameters.

---

## Key Details

| Property | Value |
|----------|-------|
| Framework | React 19 + TypeScript |
| Build tool | Vite 7 |
| Styling | Tailwind CSS v4 + `@cbweb3/ui` |
| State | Zustand + React Query |
| Routing | react-router-dom |
| API | Axios → api-gateway REST |

### Port Assignments

| Entity | URL |
|--------|-----|
| Central-Bank-A | http://localhost:5177 |
| Central-Bank-B | http://localhost:5178 |

### Dev

```bash
cd frontend
npm run dev:governance -- --port 5177
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup, npm scripts, Docker stacks
- [api-gateway](../../../backend/services/api-gateway/README.md) — backend this portal calls
- [compliance service](../../../backend/services/compliance/README.md) — handles participant registry and governance ops
- [bank portal](../bank/README.md) — commercial bank counterpart
