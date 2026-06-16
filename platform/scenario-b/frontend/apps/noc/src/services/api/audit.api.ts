import type { AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

// Backend audit entry shape from GET /api/v1/audit
type BackendAuditEntry = {
  id: string;
  actor: string;
  action: string;
  target_id: string;
  target_type: string;
  detail: string;
  created_at: string;
};

export const auditApi = {
  list: async (): Promise<AuditLogEntry[]> => {
    const res = await httpClient.get<{ data: BackendAuditEntry[] }>("/audit");
    const entries: BackendAuditEntry[] = res.data.data ?? [];
    return entries.map((e) => ({
      id: e.id,
      // Map action string to a component category for display
      component: mapActionToComponent(e.action),
      severity: mapActionToSeverity(e.action),
      message: e.detail || `${e.action} on ${e.target_type} ${e.target_id} by ${e.actor}`,
      createdAt: e.created_at,
    }));
  },
};

function mapActionToComponent(action: string): AuditLogEntry["component"] {
  if (action.includes("ALERT")) return "SYSTEM";
  return "SYSTEM";
}

function mapActionToSeverity(action: string): AuditLogEntry["severity"] {
  if (action === "DISMISS_ALERT") return "WARNING";
  if (action === "ACKNOWLEDGE_ALERT") return "INFO";
  return "INFO";
}
