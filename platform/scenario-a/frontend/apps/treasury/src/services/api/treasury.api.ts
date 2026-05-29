import type { BurnPayload, BurnToMintValidation, MintPayload, SupplySnapshot, TreasuryOperation } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const treasuryApi = {
  getSupply: (): Promise<SupplySnapshot> => mockDb.getSupplySnapshot(),
  getOperations: (): Promise<TreasuryOperation[]> => mockDb.getOperations(),
  validateBurnToMint: (requestId: string, amount: string): Promise<BurnToMintValidation> => mockDb.validateBurnToMint(requestId, amount),
  mint: (payload: MintPayload): Promise<TreasuryOperation> => mockDb.mint(payload),
  burn: (payload: BurnPayload): Promise<TreasuryOperation> => mockDb.burn(payload),
};
