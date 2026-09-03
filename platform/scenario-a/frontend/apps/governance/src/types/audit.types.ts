// SPDX-License-Identifier: Apache-2.0

// Categories emitted by the compliance service (domain/status.go).
export type AuditCategory = "CREDENTIAL" | "FREEZE" | "CIRCUIT_BREAKER" | "PARAMETER";
export type AuditSeverity = "INFO" | "WARNING" | "CRITICAL";

export type GovernanceAuditEntry = {
  id: string;
  actor: string;
  action: string;
  category: AuditCategory;
  severity: AuditSeverity;
  outcome: "SUCCESS" | "FAILED";
  metadata: string;
  createdAt: string;
};

export type AuditFilter = {
  category?: AuditCategory | "ALL";
  severity?: AuditSeverity | "ALL";
  dateFrom?: string;
  dateTo?: string;
};
