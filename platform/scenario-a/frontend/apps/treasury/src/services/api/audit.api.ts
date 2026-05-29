import type { AuditLogEntry } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const auditApi = {
  list: (): Promise<AuditLogEntry[]> => mockDb.getAuditLogs(),
};
