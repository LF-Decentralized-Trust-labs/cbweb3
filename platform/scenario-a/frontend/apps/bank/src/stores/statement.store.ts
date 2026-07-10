// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { statementApi } from "../services/api/statement.api";
import type { Movement } from "../types";

type StatementState = {
  movements: Movement[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchAll: () => Promise<void>;
};

export const useStatementStore = create<StatementState>((set) => ({
  movements: [],
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const movements = await statementApi.list();
      set({ movements, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load statement" });
    }
  },
}));
