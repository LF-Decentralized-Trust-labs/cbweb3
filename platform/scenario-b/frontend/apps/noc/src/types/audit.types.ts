export type AuditLogEntry = {
  id: string;
  component: "BESU" | "PALADIN" | "CACTI" | "SYSTEM";
  severity: "INFO" | "WARNING" | "CRITICAL";
  message: string;
  createdAt: string;
};

export type AuditFilter = {
  component?: AuditLogEntry["component"];
  severity?: AuditLogEntry["severity"];
};
