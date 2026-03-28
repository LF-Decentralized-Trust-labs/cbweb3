export type AuditLogEntry = {
  id: string;
  category: "AUTH" | "FUNDING" | "TREASURY" | "KYC" | "RECONCILIATION";
  message: string;
  severity: "INFO" | "WARNING" | "CRITICAL";
  createdAt: string;
};

export type AuditFilter = {
  category?: AuditLogEntry["category"];
  severity?: AuditLogEntry["severity"];
};
