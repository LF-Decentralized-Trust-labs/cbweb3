import type { AuditFilter, AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

type AuditLogApi = {
  id: string;
  category: AuditLogEntry["category"];
  message: string;
  severity: AuditLogEntry["severity"];
  created_at: string;
};

type AuditLogsResponse = {
  logs: AuditLogApi[];
};

const mapEntry = (entry: AuditLogApi): AuditLogEntry => ({
  id: entry.id,
  category: entry.category,
  message: entry.message,
  severity: entry.severity,
  createdAt: entry.created_at,
});

export const auditApi = {
  list: async (filter?: AuditFilter): Promise<AuditLogEntry[]> => {
    const params: Record<string, string> = {};
    if (filter?.category) params.category = filter.category;
    if (filter?.severity) params.severity = filter.severity;

    const response = await httpClient.get<AuditLogsResponse>("/treasury/audit/logs", { params });
    return (response.data.logs ?? []).map(mapEntry);
  },
};
