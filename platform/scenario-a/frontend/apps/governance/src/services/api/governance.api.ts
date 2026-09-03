// SPDX-License-Identifier: Apache-2.0

import type {
  AccountEntry,
  CircuitBreakerPayload,
  CircuitBreakerResult,
  CircuitBreakerState,
  FreezePayload,
  FreezeResult,
  GovernanceParameters,
  UpdateParametersPayload,
} from "../../types";
import { httpClient } from "./http-client";

type CircuitBreakerStatusResponse = {
  is_paused: boolean;
  paused_at?: string;
  paused_by?: string;
  reason?: string;
};

type ToggleCircuitBreakerResponse = {
  is_paused: boolean;
};

type AccountsListResponse = {
  accounts: Array<{
    user_id: string;
    institution_name?: string;
    status: string;
    frozen_at?: string;
    frozen_reason?: string;
  }>;
};

const mapCircuitBreaker = (response: CircuitBreakerStatusResponse | ToggleCircuitBreakerResponse): CircuitBreakerState => ({
  state: response.is_paused ? "HALTED" : "LIVE",
  updatedAt: ("paused_at" in response && response.paused_at) || new Date().toISOString(),
  updatedBy: ("paused_by" in response && response.paused_by) || "governance",
});

const mapAccount = (entry: AccountsListResponse["accounts"][number]): AccountEntry => ({
  id: entry.user_id,
  participantId: entry.user_id,
  participantName: entry.institution_name ?? entry.user_id,
  frozen: entry.status === "FROZEN",
  frozenAt: entry.frozen_at ?? null,
  frozenReason: entry.frozen_reason ?? null,
});

type SystemParametersResponse = {
  transaction_minimum: string;
  transaction_maximum: string;
  slippage_tolerance: number;
  settlement_window: number;
};

const mapParameters = (raw: SystemParametersResponse): GovernanceParameters => ({
  txLimitMin: parseFloat(raw.transaction_minimum),
  txLimitMax: parseFloat(raw.transaction_maximum),
  slippageTolerance: raw.slippage_tolerance,
  settlementWindowSeconds: raw.settlement_window,
});

export const governanceApi = {
  getCircuitBreaker: async (): Promise<CircuitBreakerState> => {
    const response = await httpClient.get<CircuitBreakerStatusResponse>("/governance/circuit-breaker/status");
    return mapCircuitBreaker(response.data);
  },
  setCircuitBreaker: async (payload: CircuitBreakerPayload): Promise<CircuitBreakerResult> => {
    const response = await httpClient.post<ToggleCircuitBreakerResponse>("/governance/circuit-breaker/toggle", payload);
    const state = mapCircuitBreaker(response.data);
    return { success: true, state: state.state, updatedAt: state.updatedAt, updatedBy: state.updatedBy };
  },
  listAccounts: async (): Promise<AccountEntry[]> => {
    const response = await httpClient.get<AccountsListResponse>("/governance/accounts");
    return (response.data.accounts ?? []).map(mapAccount);
  },
  freezeAccount: async (payload: FreezePayload): Promise<FreezeResult> => {
    const response = await httpClient.post<{ subject: string; status: string }>("/governance/accounts/freeze", {
      subject: payload.accountId,
      reason: payload.reason,
    });
    return {
      success: response.data.status === "FROZEN",
      accountId: response.data.subject,
      frozenAt: new Date().toISOString(),
    };
  },
  unfreezeAccount: async (payload: FreezePayload): Promise<FreezeResult> => {
    const response = await httpClient.post<{ subject: string; status: string }>("/governance/accounts/unfreeze", {
      subject: payload.accountId,
      reason: payload.reason,
    });
    return {
      success: response.data.status === "ACTIVE",
      accountId: response.data.subject,
      frozenAt: new Date().toISOString(),
    };
  },
  getParameters: async (): Promise<GovernanceParameters> => {
    const response = await httpClient.get<SystemParametersResponse>("/governance/parameters");
    return mapParameters(response.data);
  },
  updateParameters: async (payload: UpdateParametersPayload): Promise<GovernanceParameters> => {
    const response = await httpClient.put<GovernanceParameters>("/governance/parameters", payload);
    return response.data;
  },
};
