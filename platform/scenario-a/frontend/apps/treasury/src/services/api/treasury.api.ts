// SPDX-License-Identifier: Apache-2.0

import type { BurnPayload, MintPayload, SupplySnapshot, TreasuryOperation } from "../../types";
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
    // The field names here must match what the gateway actually serves. They did
    // not: AuditRecord is marshalled as log_id / details / timestamp, while this
    // mapper read id / metadata / created_at, so every row would have come out
    // with no id, amount "0" and an invalid date. Only `action` ever lined up.
    // The mismatch was invisible because nothing wrote TREASURY entries at all
    // (R2-M-8), so the table was always empty.
    const response = await httpClient.get<{
      logs: Array<{ log_id: string; action: string; details?: string; timestamp: string }>;
    }>("/governance/audit/logs", { params: { category: "TREASURY" } });
    return (response.data.logs ?? []).map((log) => ({
      id: log.log_id,
      kind: log.action.toUpperCase().includes("BURN") ? "BURN" : "MINT",
      amount: extractAmountFromMetadata(log.details) ?? "0",
      status: "CONFIRMED",
      createdAt: log.timestamp,
      reference: log.details,
    }));
  },
  mint: async (payload: MintPayload): Promise<TreasuryOperation> => {
    const response = await httpClient.post<TxResponse>("/token/mint", {
      to: payload.targetInstitutionId,
      amount: payload.amount,
      // The funding request and reserve proof the operator states as backing.
      // These used to be collected on screen and then dropped here, so the
      // server held no record of what an issuance was backed by (R2-M-8).
      request_id: payload.requestId,
      reserve_proof_ref: payload.reserveProofRef,
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
      // The justification for destroying money. Required by the server, and
      // recorded in the audit trail — it used to stop at the browser (R2-M-8).
      reason: payload.reason,
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
