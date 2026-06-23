// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { auditApi } from "../services/api";
import type { AsyncStatus, AuditFilter, GovernanceAuditEntry } from "../types";

type AuditStore = {
  logs: GovernanceAuditEntry[];
  status: AsyncStatus;
  error: string | null;
  fetch: (filter?: AuditFilter) => Promise<void>;
};

export const useAuditStore = create<AuditStore>((set) => ({
  logs: [],
  status: "idle",
  error: null,
  fetch: async (filter) => {
    set({ status: "loading", error: null });
    try {
      const logs = await auditApi.list(filter);
      set({ logs, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load audit logs" });
    }
  },
}));
