export type AuditCategory = "REGISTRY" | "CIRCUIT_BREAKER" | "FREEZE" | "PARAMETER" | "AUTH";
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
