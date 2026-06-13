import type { HTLCSummary, StabilityAlert } from "../../types";
import { apiFetch } from "./apiClient";

interface HTLCLock {
  contract_id: string;
  sender: string;
  receiver: string;
  time_lock: number;
  state: string;
  counterparty_locked: boolean;
}

interface HTLCSearchResponse {
  locks: HTLCLock[];
  total: number;
}

function toHTLCSummary(lock: HTLCLock): HTLCSummary {
  return {
    id: lock.contract_id,
    state: (lock.state as HTLCSummary["state"]) ?? "HTLC_STATE_LOCKED",
    sender: lock.sender,
    receiver: lock.receiver,
    expiresAt: lock.time_lock ? new Date(lock.time_lock * 1000).toISOString() : "",
    counterpartyLocked: lock.counterparty_locked,
  };
}

export const stabilityApi = {
  getHTLCs: async (): Promise<HTLCSummary[]> => {
    const resp = await apiFetch<HTLCSearchResponse>("/api/v1/htlc/search");
    return resp.locks.map(toHTLCSummary);
  },
  getAlerts: (): Promise<StabilityAlert[]> => Promise.resolve([]),
};
