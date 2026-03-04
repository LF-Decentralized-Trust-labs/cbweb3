import type { MintRequest, TransferRequest } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const tokenApi = {
  getBalance: () => mockDb.getBalance(),
  getTransactions: () => mockDb.getTransactions(),
  mint: (payload: MintRequest) => mockDb.mint(payload.amount, payload.fiatProofRef),
  transfer: (payload: TransferRequest) => mockDb.transfer(payload),
};
