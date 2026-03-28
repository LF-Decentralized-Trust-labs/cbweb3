import type { AuditFilter, GovernanceAuditEntry } from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const auditApi = {
  list: async (filter?: AuditFilter): Promise<GovernanceAuditEntry[]> => {
    if (useMocks) {
      return mockDb.listAuditLogs(filter);
    }
    const response = await httpClient.get<GovernanceAuditEntry[]>("/compliance/audit/logs", {
      params: filter,
    });
    return response.data;
  },
};
