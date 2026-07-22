// SPDX-License-Identifier: Apache-2.0

import type { AuditFilter, AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

// Wire shape returned by GET /api/v1/audit/logs — the shared, role-broadened audit
// read (ROLE_GOVERNANCE/TREASURY/SUPERVISOR). Mirrors the gateway AuditRecord JSON tags
// (compliance_grpc.go). The /governance/audit/logs path is ROLE_GOVERNANCE-only (Fiber
// group middleware is prefix-scoped), so treasury consumes the standalone /audit/logs.
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

const mapEntry = (entry: AuditLogApi): AuditLogEntry => {
  const parts = [entry.action, entry.outcome ? `(${entry.outcome})` : "", entry.details ?? ""].filter(Boolean);
  return {
    id: entry.log_id,
    category: entry.category as AuditLogEntry["category"],
    message: parts.join(" ").trim() || entry.action,
    severity: entry.severity as AuditLogEntry["severity"],
    createdAt: entry.timestamp,
  };
};

export const auditApi = {
  list: async (filter?: AuditFilter): Promise<AuditLogEntry[]> => {
    const params: Record<string, string> = {};
    if (filter?.category) params.category = filter.category;
    if (filter?.severity) params.severity = filter.severity;

    const response = await httpClient.get<AuditLogsResponse>("/audit/logs", { params });
    return (response.data.logs ?? []).map(mapEntry);
  },
};
