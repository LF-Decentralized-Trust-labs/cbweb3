import { create } from "zustand";
import { auditApi } from "../services/api";
import type { AuditLogEntry, AuditLogsResponse, DecryptTransactionRequest, DecryptTransactionResponse } from "../types";

type AuditState = {
  logs: AuditLogEntry[];
  page: number;
  limit: number;
  lastDecrypted: DecryptTransactionResponse | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  refreshLogs: (params?: { severity?: string; category?: string }) => Promise<void>;
  decryptTransaction: (payload: DecryptTransactionRequest) => Promise<void>;
};

export const useAuditStore = create<AuditState>((set) => ({
  logs: [],
  page: 1,
  limit: 20,
  lastDecrypted: null,
  status: "idle",
  error: null,
  refreshLogs: async (params) => {
    set({ status: "loading", error: null });
    try {
      const resp: AuditLogsResponse = await auditApi.getAuditLogs({ limit: 20, page: 1, ...params });
      set({ logs: resp.logs, page: resp.page, limit: resp.limit, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch audit logs" });
    }
  },
  decryptTransaction: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const result = await auditApi.decryptTransaction(payload);
      const resp: AuditLogsResponse = await auditApi.getAuditLogs({ limit: 20, page: 1 });
      set({ lastDecrypted: result, logs: resp.logs, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to decrypt transaction" });
    }
  },
}));
