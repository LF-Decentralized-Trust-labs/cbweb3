import type {
  ApproveKycPayload,
  ApproveKycResponse,
  CredentialIssuanceResult,
  IssueCredentialPayload,
  KycStatusEntry,
  PendingKycApiResponse,
  Participant,
} from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const registryApi = {
  list: async (): Promise<Participant[]> => {
    if (useMocks) {
      return mockDb.listParticipants();
    }
    const response = await httpClient.get<Participant[]>(
      "/compliance/registry",
    );
    return response.data;
  },
  issueCredential: async (
    payload: IssueCredentialPayload,
  ): Promise<CredentialIssuanceResult> => {
    if (useMocks) {
      return mockDb.issueCredential(payload);
    }
    const response = await httpClient.post<CredentialIssuanceResult>(
      "/compliance/kyc/issue-credential",
      payload,
    );
    return response.data;
  },
  listPendingKyc: async (): Promise<KycStatusEntry[]> => {
    const response = await httpClient.get<PendingKycApiResponse>(
      "/compliance/participants",
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
    if (useMocks) {
      return mockDb.getKycStatus(subject);
    }

    const response = await httpClient.get<KycStatusEntry>(
      `/compliance/kyc/status/${subject}`,
    );
    return response.data;
  },
  approveKyc: async (
    payload: ApproveKycPayload,
  ): Promise<ApproveKycResponse> => {
    const response = await httpClient.post<ApproveKycResponse>(
      "/compliance/approve-kyc",
      payload,
    );
    return response.data;
  },
};
