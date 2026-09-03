// SPDX-License-Identifier: Apache-2.0

import type { ZKPointerVerification } from "../../types";
import { apiFetch } from "./apiClient";

type ZKPointerResponse = {
  bank_id: string;
  pointer_id: string;
  commitment_hash: string;
  state: "VALID" | "INVALID" | "EXPIRED";
  expires_at: string | null;
};

export const zkPointerApi = {
  verify: (bankId: string, commitmentHash: string): Promise<ZKPointerVerification> => {
    const qs = new URLSearchParams({ bank_id: bankId, commitment_hash: commitmentHash });
    return apiFetch<ZKPointerResponse>(`/api/v1/compliance/zk-pointer/verify?${qs.toString()}`).then(
      (r): ZKPointerVerification => ({
        bankId: r.bank_id,
        pointerId: r.pointer_id,
        commitmentHash: r.commitment_hash,
        state: r.state,
        expiresAt: r.expires_at,
      }),
    );
  },
};
