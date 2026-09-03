// SPDX-License-Identifier: Apache-2.0

import type {
  ApproveKycPayload,
  ApproveKycResponse,
  KycStatusEntry,
  PendingKycApiResponse,
  Participant,
} from "../../types";
import { httpClient } from "./http-client";

export const registryApi = {
  list: async (): Promise<Participant[]> => {
    const response = await httpClient.get<{ participants: Participant[] }>("/governance/registry");
    return response.data.participants ?? [];
  },
  listPendingKyc: async (): Promise<KycStatusEntry[]> => {
    const response = await httpClient.get<PendingKycApiResponse>(
      "/governance/registry",
      { params: { status: "CREDENTIAL_REQUESTED" } },
    );
    const participants = response.data.participants ?? [];
    return participants
      .filter((entry) => entry.status === "CREDENTIAL_REQUESTED")
      .map((entry) => ({
        subject: entry.user_id,
        status: entry.status,
        institution_name: entry.institution_name,
        bank_code: entry.bank_code,
        country: entry.country_code,
        wallet_address: entry.wallet_address,
      }));
  },
  getKycStatus: async (subject: string): Promise<KycStatusEntry> => {
    const response = await httpClient.get<KycStatusEntry>(`/compliance/kyc/status/${subject}`);
    return response.data;
  },
  approveKyc: async (payload: ApproveKycPayload): Promise<ApproveKycResponse> => {
    const response = await httpClient.post<ApproveKycResponse>("/governance/approve-kyc", payload);
    return response.data;
  },
};
