import type { AuditFilter, GovernanceAuditEntry } from "../../types";
import { httpClient } from "./http-client";

// Wire shape returned by GET /api/v1/governance/audit/logs.
// Mirrors the gateway AuditRecord JSON tags (compliance_grpc.go).
type AuditLogApi = {
  log_id: string;
  timestamp: string;
  actor: string;
  actor_address?: string;
  action: string;
  target_subject?: string;
  category: string;
  severity: string;
  outcome: string;
  details?: string;
};

type AuditLogsResponse = {
  logs: AuditLogApi[];
};

const mapEntry = (entry: AuditLogApi): GovernanceAuditEntry => ({
  id: entry.log_id,
  actor: entry.actor,
  action: entry.action,
  category: entry.category as GovernanceAuditEntry["category"],
  severity: entry.severity as GovernanceAuditEntry["severity"],
  outcome: entry.outcome as GovernanceAuditEntry["outcome"],
  metadata: entry.details ?? "",
  createdAt: entry.timestamp,
});

export const auditApi = {
  list: async (filter?: AuditFilter): Promise<GovernanceAuditEntry[]> => {
    const params: Record<string, string> = {};
    if (filter?.category && filter.category !== "ALL") params.category = filter.category;
    if (filter?.severity && filter.severity !== "ALL") params.severity = filter.severity;
    if (filter?.dateFrom) params.from_date = filter.dateFrom;
    if (filter?.dateTo) params.to_date = filter.dateTo;

    const response = await httpClient.get<AuditLogsResponse>("/governance/audit/logs", { params });
    return (response.data.logs ?? []).map(mapEntry);
  },
};
