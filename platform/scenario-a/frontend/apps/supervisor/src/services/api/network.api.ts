import type { NetworkOverview } from "../../types";
import { apiFetch } from "./apiClient";

interface ParticipantsResponse {
  participants: Array<{ status: string }>;
}

export const networkApi = {
  getOverview: async (): Promise<NetworkOverview> => {
    try {
      const { participants } = await apiFetch<ParticipantsResponse>("/api/v1/compliance/participants");
      const activeInstitutions = participants.filter((p) => p.status === "ACTIVE").length;
      return {
        totalSupply: 0,
        activeInstitutions,
        activeHTLCs: 0,
        pendingSettlements: 0,
        lastUpdatedAt: new Date().toISOString(),
      };
    } catch {
      return {
        totalSupply: 0,
        activeInstitutions: 0,
        activeHTLCs: 0,
        pendingSettlements: 0,
        lastUpdatedAt: new Date().toISOString(),
      };
    }
  },
};
