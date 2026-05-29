# supervisor portal

> [scenario-a](../../../README.md) › [frontend](../../README.md) › supervisor

The **supervisor portal** is a read-only regulatory oversight interface. Supervisors and regulators can monitor transactions across the spoke with selective privacy — they see what they are authorized to see, with ZK proofs available for verification without full decryption.

---

## Architecture Placement

```
Browser (Supervisor / Regulator)
         │
         ▼
  [supervisor portal]
         │
         ▼
  [api-gateway]   REST (supervisor entity)
         │
         └──► Paladin (ZK audit — selective privacy)
```

---

## Users

Central bank supervisors, financial regulators, and auditors.

---

## Key Features

- **Transaction monitoring** — View payment and settlement activity across the spoke with selective disclosure based on the supervisor's authorization level.
- **ZK proof verification** — Access Paladin-generated zero-knowledge proofs to verify transaction correctness without decrypting confidential amounts.
- **Compliance reports** — Generate regulatory reports on transaction volumes, HTLC states, and participant activity.
- **KYC/AML status** — Review participant compliance flags and identity verification status.
- **Audit data export** — Export transaction datasets for external audit workflows.

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
| Privacy | ZK proof integration via Paladin for selective disclosure |

### Dev

```bash
cd frontend
npm run dev:supervisor
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup and npm scripts
- [compliance service](../../../backend/services/compliance/README.md) — KYC/AML data source
- [governance portal](../governance/README.md) — central bank counterpart (write access)
