// SPDX-License-Identifier: Apache-2.0

import type { AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

type AuditLogsResponse = {
  logs: Array<{
    id: string;
    actor_subject?: string;
    action: string;
    category: string;
    severity: string;
    metadata?: string;
    created_at: string;
  }>;
};

const KNOWN_CATEGORIES: Array<AuditLogEntry["category"]> = ["AUTH", "FUNDING", "TREASURY", "KYC", "RECONCILIATION"];

const KNOWN_SEVERITIES: Array<AuditLogEntry["severity"]> = ["INFO", "WARNING", "CRITICAL"];

const mapCategory = (raw: string): AuditLogEntry["category"] => {
  const upper = raw.toUpperCase();
  return (KNOWN_CATEGORIES.find((category) => upper.includes(category)) ?? "TREASURY") as AuditLogEntry["category"];
};

const mapSeverity = (raw: string): AuditLogEntry["severity"] => {
  const upper = raw.toUpperCase();
  return (KNOWN_SEVERITIES.find((severity) => severity === upper) ?? "INFO") as AuditLogEntry["severity"];
};

export const auditApi = {
  list: async (): Promise<AuditLogEntry[]> => {
    const response = await httpClient.get<AuditLogsResponse>("/governance/audit/logs");
    return (response.data.logs ?? []).map((log) => ({
      id: log.id,
      category: mapCategory(log.category),
      message: log.action + (log.metadata ? ` — ${log.metadata}` : ""),
      severity: mapSeverity(log.severity),
      createdAt: log.created_at,
    }));
  },
};
