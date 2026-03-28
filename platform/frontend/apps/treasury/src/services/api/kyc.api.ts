import type { IssueCredentialPayload, IssuedCredential } from "../../types";
import { mockDb } from "../mocks/mock-db";

const issuer = "Treasury Operator";

export const kycApi = {
  issue: (payload: IssueCredentialPayload): Promise<IssuedCredential> => mockDb.issueCredential(payload, issuer),
  list: (): Promise<IssuedCredential[]> => mockDb.getIssuedCredentials(),
};
