import type { NetworkOverview } from "../../types";
import { apiFetch } from "./apiClient";

interface ParticipantsResponse {
  participants: Array<{ status: string }>;
}

interface HTLCSearchResponse {
  locks: Array<{ state?: string }>;
  total: number;
}

export const networkApi = {
  getOverview: async (): Promise<NetworkOverview> => {
    const [participantsResult, htlcResult] = await Promise.allSettled([
      apiFetch<ParticipantsResponse>("/api/v1/compliance/participants/summary"),
      apiFetch<HTLCSearchResponse>("/api/v1/htlc/search"),
    ]);

    const participants =
      participantsResult.status === "fulfilled"
        ? participantsResult.value.participants
        : [];
    const htlcs =
      htlcResult.status === "fulfilled" ? htlcResult.value.locks : [];

    return {
      activeInstitutions: participants.filter((p) => p.status === "ACTIVE").length,
      activeHTLCs: htlcs.filter((h) => h.state === "HTLC_STATE_LOCKED").length,
      pendingSettlements: htlcs.filter(
        (h) => h.state === "HTLC_STATE_LOCKED" || h.state === "HTLC_STATE_SETTLING",
      ).length,
      lastUpdatedAt: new Date().toISOString(),
    };
  },
};
