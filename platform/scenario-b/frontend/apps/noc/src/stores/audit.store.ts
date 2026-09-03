// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { auditApi } from "../services/api";
import type { AsyncStatus, AuditLogEntry } from "../types";

type AuditState = {
  logs: AuditLogEntry[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const useAuditStore = create<AuditState>((set) => ({
  logs: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const logs = await auditApi.list();
      set({ logs, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load audit logs" });
    }
  },
}));
