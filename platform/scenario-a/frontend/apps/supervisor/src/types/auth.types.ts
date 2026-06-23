// SPDX-License-Identifier: Apache-2.0

export const SupervisorRole = {
  SUPERVISOR_ROLE: "SUPERVISOR_ROLE",
  CENTRAL_BANK_ADMIN: "CENTRAL_BANK_ADMIN",
  COMPLIANCE_OFFICER: "COMPLIANCE_OFFICER",
} as const;

export type SupervisorRole = (typeof SupervisorRole)[keyof typeof SupervisorRole];

export const Permission = {
  VIEW_NETWORK: "VIEW_NETWORK",
  VIEW_REGISTRY: "VIEW_REGISTRY",
  VIEW_AUDIT_LOGS: "VIEW_AUDIT_LOGS",
  DECRYPT_TRANSACTIONS: "DECRYPT_TRANSACTIONS",
  VIEW_STABILITY: "VIEW_STABILITY",
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
