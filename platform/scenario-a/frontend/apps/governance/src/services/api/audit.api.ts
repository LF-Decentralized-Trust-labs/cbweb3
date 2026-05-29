import type { AuditFilter, GovernanceAuditEntry } from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

type AuditLogsResponse = {
  logs: Array<{
    id: string;
    actor_subject: string;
    action: string;
    category: string;
    severity: string;
    outcome: string;
    metadata?: string;
    created_at: string;
  }>;
};

const mapEntry = (entry: AuditLogsResponse["logs"][number]): GovernanceAuditEntry => ({
  id: entry.id,
  actor: entry.actor_subject,
  action: entry.action,
  category: entry.category as GovernanceAuditEntry["category"],
  severity: entry.severity as GovernanceAuditEntry["severity"],
  outcome: entry.outcome as GovernanceAuditEntry["outcome"],
  metadata: entry.metadata ?? "",
  createdAt: entry.created_at,
});

export const auditApi = {
  list: async (filter?: AuditFilter): Promise<GovernanceAuditEntry[]> => {
    if (useMocks) {
      return mockDb.listAuditLogs(filter);
    }
    const params: Record<string, string> = {};
    if (filter?.category && filter.category !== "ALL") params.category = filter.category;
    if (filter?.severity && filter.severity !== "ALL") params.severity = filter.severity;
    if (filter?.dateFrom) params.from_date = filter.dateFrom;
    if (filter?.dateTo) params.to_date = filter.dateTo;

    const response = await httpClient.get<AuditLogsResponse>("/governance/audit/logs", { params });
    return (response.data.logs ?? []).map(mapEntry);
  },
};
