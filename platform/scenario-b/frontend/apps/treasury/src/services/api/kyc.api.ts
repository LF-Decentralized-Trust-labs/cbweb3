import type { IssueCredentialPayload, IssuedCredential } from "../../types";
import { httpClient } from "./http-client";

export const kycApi = {
  issue: async (payload: IssueCredentialPayload): Promise<IssuedCredential> => {
    const response = await httpClient.post<IssuedCredential>("/kyc/credentials/issue", payload);
    return response.data;
  },
  list: async (): Promise<IssuedCredential[]> => {
    const response = await httpClient.get<{ credentials: IssuedCredential[] }>("/kyc/credentials");
    return response.data.credentials ?? [];
  },
};
