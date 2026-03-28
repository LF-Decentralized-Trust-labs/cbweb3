import { create } from "zustand";
import { auditApi } from "../services/api";
import type { AuditLogEntry, DecryptTransactionRequest, DecryptTransactionResponse } from "../types";

type AuditState = {
  logs: AuditLogEntry[];
  lastDecrypted: DecryptTransactionResponse | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  refreshLogs: () => Promise<void>;
  decryptTransaction: (payload: DecryptTransactionRequest) => Promise<void>;
};

export const useAuditStore = create<AuditState>((set) => ({
  logs: [],
  lastDecrypted: null,
  status: "idle",
  error: null,
  refreshLogs: async () => {
    set({ status: "loading", error: null });
    try {
      const logs = await auditApi.getAuditLogs();
      set({ logs, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch audit logs" });
    }
  },
  decryptTransaction: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const result = await auditApi.decryptTransaction(payload);
      const logs = await auditApi.getAuditLogs();
      set({ lastDecrypted: result, logs, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to decrypt transaction" });
    }
  },
}));
