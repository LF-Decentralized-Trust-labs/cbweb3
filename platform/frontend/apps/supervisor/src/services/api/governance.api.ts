import { mockDb } from "../mocks/mock-db";
import type { CredentialRevocationRequest, InstitutionalOnboardingRequest } from "../../types";

export const governanceApi = {
  listParticipants: () => mockDb.listParticipants(),
  onboardInstitution: (payload: InstitutionalOnboardingRequest) => mockDb.onboardInstitution(payload),
  revokeCredential: (payload: CredentialRevocationRequest) => mockDb.revokeCredential(payload),
  checkSanctions: (address: string) => mockDb.checkSanctions(address),
};
