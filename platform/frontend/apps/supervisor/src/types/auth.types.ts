export const SupervisorRole = {
  SUPERVISOR_ROLE: "SUPERVISOR_ROLE",
  CENTRAL_BANK_ADMIN: "CENTRAL_BANK_ADMIN",
  COMPLIANCE_OFFICER: "COMPLIANCE_OFFICER",
} as const;

export type SupervisorRole = (typeof SupervisorRole)[keyof typeof SupervisorRole];

export const Permission = {
  VIEW_NETWORK: "VIEW_NETWORK",
  MANAGE_PARTICIPANTS: "MANAGE_PARTICIPANTS",
  REVOKE_CREDENTIALS: "REVOKE_CREDENTIALS",
  DECRYPT_TRANSACTIONS: "DECRYPT_TRANSACTIONS",
  MANAGE_AMM: "MANAGE_AMM",
  CIRCUIT_BREAKER: "CIRCUIT_BREAKER",
  CHECK_SANCTIONS: "CHECK_SANCTIONS",
} as const;

export type Permission = (typeof Permission)[keyof typeof Permission];

export interface SupervisorUser {
  id: string;
  username: string;
  role: SupervisorRole;
  institutionId: string;
  institutionName: string;
  walletAddress: string;
  permissions: Permission[];
  createdAt: string;
}

export interface LoginResponse {
  user: SupervisorUser;
  sessionTimeoutSeconds: number;
}
