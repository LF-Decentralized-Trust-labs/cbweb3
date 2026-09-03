// SPDX-License-Identifier: Apache-2.0

import type {
  AccountEntry,
  FreezePayload,
  FreezeResult,
  GovernanceParameters,
  UpdateParametersPayload,
} from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

// Shape returned by the Compliance participant registry (Go json tags).
type RawParticipant = {
  user_id: string;
  institution_name: string;
  bank_code: string;
  country_code: string;
  role: string;
  wallet_address: string;
  status: string;
};

// Map a registry participant to the account row the UI renders. frozen_at /
// reason are not surfaced by the registry endpoint, so they stay null (the
// action stays available; history lives in the Audit log).
const toAccountEntry = (p: RawParticipant): AccountEntry => ({
  id: p.user_id,
  participantId: p.bank_code || p.user_id,
  participantName: p.institution_name || p.bank_code || p.user_id,
  frozen: (p.status ?? "").toUpperCase() === "FROZEN",
  frozenAt: null,
  frozenReason: null,
  role: p.role,
});

// The V1 circuit-breaker calls were removed: they branched on VITE_USE_MOCKS, so the chrome
// indicator they fed could show synthetic state while the Circuit Breaker page showed live
// on-chain state. The indicator now derives its condition from the authoritative per-pair V2
// status (see stores/circuit-breaker.store.ts), which has no mock path.
export const governanceApi = {
  // Accounts are always served from the real Compliance participant registry
  // (GET /api/v1/governance/accounts), never mocked: this is the source of truth
  // the Central Bank freezes/unfreezes against.
  listAccounts: async (): Promise<AccountEntry[]> => {
    const response = await httpClient.get<{ accounts?: RawParticipant[] }>("/governance/accounts");
    return (response.data.accounts ?? []).map(toAccountEntry);
  },
  freezeAccount: async (payload: FreezePayload): Promise<FreezeResult> => {
    // The backend keys the participant by `subject`; our AccountEntry.id carries it.
    await httpClient.post("/governance/accounts/freeze", { subject: payload.accountId, reason: payload.reason });
    return { success: true, accountId: payload.accountId, frozenAt: new Date().toISOString() };
  },
  unfreezeAccount: async (payload: FreezePayload): Promise<FreezeResult> => {
    await httpClient.post("/governance/accounts/unfreeze", { subject: payload.accountId, reason: payload.reason });
    return { success: true, accountId: payload.accountId, frozenAt: new Date().toISOString() };
  },
  getParameters: async (): Promise<GovernanceParameters> => {
    if (useMocks) {
      return mockDb.getParameters();
    }
    const response = await httpClient.get<GovernanceParameters>("/governance/parameters");
    return response.data;
  },
  updateParameters: async (payload: UpdateParametersPayload): Promise<GovernanceParameters> => {
    if (useMocks) {
      return mockDb.updateParameters(payload);
    }
    const response = await httpClient.put<GovernanceParameters>("/governance/parameters", payload);
    return response.data;
  },
};
