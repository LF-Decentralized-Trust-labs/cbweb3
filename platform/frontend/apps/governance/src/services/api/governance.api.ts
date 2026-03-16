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
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const governanceApi = {
  getCircuitBreaker: async (): Promise<CircuitBreakerState> => {
    if (useMocks) {
      return mockDb.getCircuitBreaker();
    }
    const response = await httpClient.get<CircuitBreakerState>("/api/v1/amm/governance/circuit-breaker");
    return response.data;
  },
  setCircuitBreaker: async (payload: CircuitBreakerPayload): Promise<CircuitBreakerResult> => {
    if (useMocks) {
      return mockDb.setCircuitBreaker(payload);
    }
    const response = await httpClient.post<CircuitBreakerResult>("/api/v1/amm/governance/circuit-breaker", payload);
    return response.data;
  },
  listAccounts: async (): Promise<AccountEntry[]> => {
    if (useMocks) {
      return mockDb.listAccounts();
    }
    const response = await httpClient.get<AccountEntry[]>("/api/v1/compliance/accounts");
    return response.data;
  },
  freezeAccount: async (payload: FreezePayload): Promise<FreezeResult> => {
    if (useMocks) {
      return mockDb.freezeAccount(payload);
    }
    const response = await httpClient.post<FreezeResult>("/api/v1/compliance/accounts/freeze", payload);
    return response.data;
  },
  getParameters: async (): Promise<GovernanceParameters> => {
    if (useMocks) {
      return mockDb.getParameters();
    }
    const response = await httpClient.get<GovernanceParameters>("/api/v1/governance/parameters");
    return response.data;
  },
  updateParameters: async (payload: UpdateParametersPayload): Promise<GovernanceParameters> => {
    if (useMocks) {
      return mockDb.updateParameters(payload);
    }
    const response = await httpClient.put<GovernanceParameters>("/api/v1/governance/parameters", payload);
    return response.data;
  },
};
