import type { CredentialIssuanceResult, IssueCredentialPayload, Participant } from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const registryApi = {
  list: async (): Promise<Participant[]> => {
    if (useMocks) {
      return mockDb.listParticipants();
    }
    const response = await httpClient.get<Participant[]>("/compliance/registry");
    return response.data;
  },
  issueCredential: async (payload: IssueCredentialPayload): Promise<CredentialIssuanceResult> => {
    if (useMocks) {
      return mockDb.issueCredential(payload);
    }
    const response = await httpClient.post<CredentialIssuanceResult>("/compliance/kyc/issue-credential", payload);
    return response.data;
  },
};
