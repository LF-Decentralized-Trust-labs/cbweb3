import type { OnRampRequestPayload, TransferRequest } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const tokenApi = {
  getBalance: () => mockDb.getBalance(),
  getTransactions: () => mockDb.getTransactions(),
  transfer: (payload: TransferRequest) => mockDb.transfer(payload),
  getOnRampRequests: () => mockDb.getOnRampRequests(),
  requestOnRamp: (payload: OnRampRequestPayload) => mockDb.submitOnRampRequest(payload),
};
