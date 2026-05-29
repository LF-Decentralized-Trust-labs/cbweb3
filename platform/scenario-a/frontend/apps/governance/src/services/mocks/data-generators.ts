import type {
  AccountEntry,
  GovernanceAuditEntry,
  GovernanceParameters,
  Participant,
} from "../../types";

const now = new Date();

const minutesAgo = (minutes: number) => new Date(now.getTime() - minutes * 60_000).toISOString();

export const generateParticipants = (): Participant[] => [
  {
    id: "pt-001",
    name: "Banco Aurora S.A.",
    cnpj: "11.222.333/0001-44",
    status: "ACTIVE",
    credentialId: "cred-aurora-001",
    credentialExpiry: minutesAgo(-60 * 24 * 365),
    createdAt: minutesAgo(60 * 24 * 90),
  },
  {
    id: "pt-002",
    name: "Banco Delta S.A.",
    cnpj: "22.333.444/0001-55",
    status: "ACTIVE",
    credentialId: "cred-delta-009",
    credentialExpiry: minutesAgo(-60 * 24 * 365),
    createdAt: minutesAgo(60 * 24 * 70),
  },
  {
    id: "pt-003",
    name: "Banco Horizonte S.A.",
    cnpj: "33.444.555/0001-66",
    status: "PENDING",
    credentialId: null,
    credentialExpiry: null,
    createdAt: minutesAgo(60 * 24 * 3),
  },
  {
    id: "pt-004",
    name: "Banco Terra S.A.",
    cnpj: "44.555.666/0001-77",
    status: "REVOKED",
    credentialId: "cred-terra-002",
    credentialExpiry: minutesAgo(60 * 24 * 30),
    createdAt: minutesAgo(60 * 24 * 300),
  },
  {
    id: "pt-005",
    name: "Banco Prisma S.A.",
    cnpj: "55.666.777/0001-88",
    status: "FROZEN",
    credentialId: "cred-prisma-007",
    credentialExpiry: minutesAgo(-60 * 24 * 180),
    createdAt: minutesAgo(60 * 24 * 120),
  },
];

export const generateAccounts = (): AccountEntry[] => [
  {
    id: "acc-001",
    participantId: "pt-001",
    participantName: "Banco Aurora S.A.",
    frozen: false,
    frozenAt: null,
    frozenReason: null,
  },
  {
    id: "acc-002",
    participantId: "pt-002",
    participantName: "Banco Delta S.A.",
    frozen: false,
    frozenAt: null,
    frozenReason: null,
  },
  {
    id: "acc-003",
    participantId: "pt-005",
    participantName: "Banco Prisma S.A.",
    frozen: true,
    frozenAt: minutesAgo(120),
    frozenReason: "Suspicious transaction concentration detected",
  },
];

export const generateParameters = (): GovernanceParameters => ({
  txLimitMin: 1_000,
  txLimitMax: 50_000_000,
  slippageTolerance: 0.005,
  settlementWindowSeconds: 3_600,
});

export const generateAudit = (): GovernanceAuditEntry[] => [
  {
    id: "audit-001",
    actor: "governance.admin",
    action: "Circuit breaker resumed",
    category: "CIRCUIT_BREAKER",
    severity: "WARNING",
    outcome: "SUCCESS",
    metadata: "{\"pause\":false}",
    createdAt: minutesAgo(85),
  },
  {
    id: "audit-002",
    actor: "governance.admin",
    action: "Account frozen",
    category: "FREEZE",
    severity: "CRITICAL",
    outcome: "SUCCESS",
    metadata: "{\"accountId\":\"acc-003\"}",
    createdAt: minutesAgo(120),
  },
  {
    id: "audit-003",
    actor: "governance.admin",
    action: "Credential issued",
    category: "REGISTRY",
    severity: "INFO",
    outcome: "SUCCESS",
    metadata: "{\"participantId\":\"pt-002\"}",
    createdAt: minutesAgo(180),
  },
];
