// SPDX-License-Identifier: Apache-2.0

import type {
  AddLiquidityRequest,
  AddLiquidityResponse,
  ConfirmPairRequest,
  ConfirmPairResponse,
  HubConfig,
  HubCurrenciesResponse,
  HubPairsResponse,
  MintAndApproveRequest,
  MintAndApproveResponse,
  ProposePairRequest,
  ProposePairResponse,
  RegisterCurrencyRequest,
  RegisterCurrencyResponse,
} from "../../types/hub-liquidity.types";
import { httpClientV2 } from "./http-client";

// Hub AMM pool provisioning API — the full pool-creation pipeline: register
// currency, propose pair, confirm pair, mint & approve, then seed liquidity,
// plus the currency/pair listing helpers. Base client already targets /api/v2.
export const hubLiquidityApi = {
  // Canonical hub addresses (AMM + registries); values may be empty if unset.
  getHubConfig: async (): Promise<HubConfig> => {
    const response = await httpClientV2.get<HubConfig>("/amm/hub-config");
    return response.data;
  },
  // Register a national currency (token) on the Hub currency registry.
  registerCurrency: async (payload: RegisterCurrencyRequest): Promise<RegisterCurrencyResponse> => {
    const response = await httpClientV2.post<RegisterCurrencyResponse>("/hub/currencies", payload);
    return response.data;
  },
  // List registered currencies (feeds the token address selects).
  listCurrencies: async (): Promise<HubCurrenciesResponse> => {
    const response = await httpClientV2.get<HubCurrenciesResponse>("/hub/currencies");
    return response.data;
  },
  // Propose a new AMM pair (step 2 — proposer CB).
  proposePair: async (payload: ProposePairRequest): Promise<ProposePairResponse> => {
    const response = await httpClientV2.post<ProposePairResponse>("/amm/pairs/propose", payload);
    return response.data;
  },
  // Confirm a proposed AMM pair (step 3 — confirmer CB).
  confirmPair: async (payload: ConfirmPairRequest): Promise<ConfirmPairResponse> => {
    const response = await httpClientV2.post<ConfirmPairResponse>("/amm/pairs/confirm", payload);
    return response.data;
  },
  // Mint & approve tokens for the AMM (per pool + side).
  mintAndApprove: async (payload: MintAndApproveRequest): Promise<MintAndApproveResponse> => {
    const response = await httpClientV2.post<MintAndApproveResponse>("/amm/token/mint-and-approve", payload);
    return response.data;
  },
  // Seed / add initial liquidity to the pool.
  addLiquidity: async (payload: AddLiquidityRequest): Promise<AddLiquidityResponse> => {
    const response = await httpClientV2.post<AddLiquidityResponse>("/amm/liquidity/add", payload);
    return response.data;
  },
  // List existing pairs so the operator can pick the pool_pair.
  listPairs: async (): Promise<HubPairsResponse> => {
    const response = await httpClientV2.get<HubPairsResponse>("/amm/pairs");
    return response.data;
  },
};
