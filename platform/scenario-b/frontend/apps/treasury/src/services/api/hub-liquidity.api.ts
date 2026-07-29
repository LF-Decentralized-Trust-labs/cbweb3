// SPDX-License-Identifier: Apache-2.0

import type {
  ConfirmPairRequest,
  ConfirmPairResponse,
  DepositSideRequest,
  DepositSideResponse,
  EscrowStatus,
  FinalizeSeedResponse,
  HubConfig,
  HubCurrenciesResponse,
  HubPairsResponse,
  ProposePairRequest,
  ProposePairResponse,
  ReclaimSideResponse,
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
  // Sovereign seeding (escrow-and-finalize, no LCR): this CB deposits ONLY its own side.
  // The backend mints the CB's own W-token and escrows it against the pool's shared commit;
  // the side is auto-resolved on-chain (no picker).
  depositSide: async (payload: DepositSideRequest): Promise<DepositSideResponse> => {
    const response = await httpClientV2.post<DepositSideResponse>("/amm/liquidity/deposit-side", payload);
    return response.data;
  },
  // Finalize the pool once BOTH sides are escrowed (funds reserves atomically).
  finalizeSeed: async (poolPair: string): Promise<FinalizeSeedResponse> => {
    const response = await httpClientV2.post<FinalizeSeedResponse>("/amm/liquidity/finalize", { pool_pair: poolPair });
    return response.data;
  },
  // Reclaim this CB's own pending side (before finalize).
  reclaimSide: async (poolPair: string): Promise<ReclaimSideResponse> => {
    const response = await httpClientV2.post<ReclaimSideResponse>("/amm/liquidity/reclaim-side", { pool_pair: poolPair });
    return response.data;
  },
  // Read the escrow commit state for a pool (which sides deposited + finalized).
  getEscrow: async (poolPair: string): Promise<EscrowStatus> => {
    const response = await httpClientV2.get<EscrowStatus>("/amm/liquidity/escrow", { params: { pool_pair: poolPair } });
    return response.data;
  },
  // List existing pairs so the operator can pick the pool_pair.
  listPairs: async (): Promise<HubPairsResponse> => {
    const response = await httpClientV2.get<HubPairsResponse>("/amm/pairs");
    return response.data;
  },
};
