// SPDX-License-Identifier: Apache-2.0

import type { Participant } from "../../types";
import { apiFetch } from "./apiClient";

interface ParticipantsResponse {
  participants: Array<{
    user_id: string;
    institution_name: string;
    wallet_address: string;
    status: string;
    country_code: string;
    bank_code: string;
    role: string;
  }>;
}

function toParticipant(p: ParticipantsResponse["participants"][number]): Participant {
  return {
    id: p.user_id,
    institutionName: p.institution_name,
    evmAddress: p.wallet_address,
    publicKey: "",
    credentialStatus: (p.status as Participant["credentialStatus"]) ?? "PENDING",
    jurisdictionCode: p.country_code,
    institutionType: "COMMERCIAL_BANK",
    onboardedAt: "",
    lastActivityAt: "",
  };
}

export const governanceApi = {
  listParticipants: (): Promise<Participant[]> =>
    apiFetch<ParticipantsResponse>("/api/v1/compliance/participants/summary").then((r) =>
      r.participants.map(toParticipant),
    ),
};
