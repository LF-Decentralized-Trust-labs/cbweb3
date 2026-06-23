// SPDX-License-Identifier: Apache-2.0

import type { HTLCSummary, StabilityAlert } from "../../types";
import { apiFetch } from "./apiClient";

interface HTLCLock {
  contract_id: string;
  hash_lock: string;
  zeto_lock_ref: string;
  time_lock: number;
  state: string;
}

interface HTLCSearchResponse {
  locks: HTLCLock[];
  total: number;
}

function toHTLCSummary(lock: HTLCLock): HTLCSummary {
  return {
    id: lock.contract_id,
    state: (lock.state as HTLCSummary["state"]) ?? "HTLC_STATE_LOCKED",
    hashLock: lock.hash_lock ?? "",
    zetoLockRef: lock.zeto_lock_ref ?? "",
    expiresAt: lock.time_lock ? new Date(lock.time_lock * 1000).toISOString() : "",
  };
}

export const stabilityApi = {
  getHTLCs: async (): Promise<HTLCSummary[]> => {
    const resp = await apiFetch<HTLCSearchResponse>("/api/v1/htlc/search");
    return resp.locks.map(toHTLCSummary);
  },
  getAlerts: (): Promise<StabilityAlert[]> => Promise.resolve([]),
};
