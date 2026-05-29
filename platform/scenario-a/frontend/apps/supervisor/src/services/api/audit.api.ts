import { mockDb } from "../mocks/mock-db";
import type { DecryptTransactionRequest } from "../../types";

export const auditApi = {
  decryptTransaction: (payload: DecryptTransactionRequest) => mockDb.decryptTransaction(payload),
  getAuditLogs: () => mockDb.getAuditLogs(),
};
