import type { BurnPayload, BurnToMintValidation, MintPayload, SupplySnapshot, TreasuryOperation } from "../../types";
import { httpClient } from "./http-client";

type OperationsResponse = {
  operations: TreasuryOperation[];
};

export const treasuryApi = {
  getSupply: async (): Promise<SupplySnapshot> => {
    const response = await httpClient.get<SupplySnapshot>("/treasury/supply");
    return response.data;
  },
  getOperations: async (): Promise<TreasuryOperation[]> => {
    const response = await httpClient.get<OperationsResponse>("/treasury/operations");
    return response.data.operations ?? [];
  },
  validateBurnToMint: async (requestId: string, amount: string): Promise<BurnToMintValidation> => {
    const response = await httpClient.get<BurnToMintValidation>("/treasury/validate-burn-to-mint", {
      params: { requestId, amount },
    });
    return response.data;
  },
  mint: async (payload: MintPayload): Promise<TreasuryOperation> => {
    const response = await httpClient.post<TreasuryOperation>("/treasury/mint", payload);
    return response.data;
  },
  burn: async (payload: BurnPayload): Promise<TreasuryOperation> => {
    const response = await httpClient.post<TreasuryOperation>("/treasury/burn", payload);
    return response.data;
  },
};
