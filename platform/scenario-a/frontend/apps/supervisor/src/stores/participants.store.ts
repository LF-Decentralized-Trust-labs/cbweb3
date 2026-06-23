// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { governanceApi } from "../services/api";
import type { Participant } from "../types";

type ParticipantsState = {
  participants: Participant[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
};

export const useParticipantsStore = create<ParticipantsState>((set) => ({
  participants: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const participants = await governanceApi.listParticipants();
      set({ participants, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch participants" });
    }
  },
}));
