import type { BurnPayload, BurnToMintValidation, MintPayload, SupplySnapshot, TreasuryOperation } from "../../types";
import { httpClient } from "./http-client";

type BalanceResponse = { balance: string };
type TxResponse = { tx_hash: string };

export const treasuryApi = {
  getSupply: async (): Promise<SupplySnapshot> => {
    const response = await httpClient.get<BalanceResponse>("/token/balance");
    return {
      circulatingSupply: response.data.balance,
      updatedAt: new Date().toISOString(),
    };
  },
  getOperations: async (): Promise<TreasuryOperation[]> => {
    const response = await httpClient.get<{ logs: Array<{ id: string; action: string; metadata?: string; created_at: string }> }>(
      "/governance/audit/logs",
      { params: { category: "TREASURY" } },
    );
    return (response.data.logs ?? []).map((log) => ({
      id: log.id,
      kind: log.action.toUpperCase().includes("BURN") ? "BURN" : "MINT",
      amount: extractAmountFromMetadata(log.metadata) ?? "0",
      status: "CONFIRMED",
      createdAt: log.created_at,
      reference: log.metadata,
    }));
  },
  validateBurnToMint: async (_requestId: string, _amount: string): Promise<BurnToMintValidation> => {
    return { isValid: true };
  },
  mint: async (payload: MintPayload): Promise<TreasuryOperation> => {
    const response = await httpClient.post<TxResponse>("/token/mint", {
      to: payload.targetInstitutionId,
      amount: payload.amount,
    });
    return {
      id: response.data.tx_hash,
      kind: "MINT",
      amount: payload.amount,
      status: "CONFIRMED",
      createdAt: new Date().toISOString(),
      reference: payload.requestId,
    };
  },
  burn: async (payload: BurnPayload): Promise<TreasuryOperation> => {
    const response = await httpClient.post<TxResponse>("/token/burn", {
      from: payload.sourceAccount,
      amount: payload.amount,
    });
    return {
      id: response.data.tx_hash,
      kind: "BURN",
      amount: payload.amount,
      status: "CONFIRMED",
      createdAt: new Date().toISOString(),
      reference: payload.reason,
    };
  },
};

function extractAmountFromMetadata(metadata?: string): string | null {
  if (!metadata) return null;
  const match = metadata.match(/amount["']?\s*[:=]\s*["']?(\d+)/i);
  return match ? match[1] : null;
}
