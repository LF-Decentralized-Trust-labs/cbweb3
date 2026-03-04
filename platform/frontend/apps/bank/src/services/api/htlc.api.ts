import type { CreateAgreementRequest } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const htlcApi = {
  createAgreement: (payload: CreateAgreementRequest) => mockDb.createAgreement(payload),
  getAgreements: () => mockDb.getAgreements(),
  lockFunds: (agreementId: string, hashLock: string) => mockDb.lockFunds(agreementId, hashLock),
  settle: (lockId: string, secret: string) => mockDb.settle(lockId, secret),
  getLocks: () => mockDb.getLocks(),
};
