// SPDX-License-Identifier: Apache-2.0

import axios from "axios";
import type {
  ApproveAmmRequest,
  BridgeLockMintRequest,
  BridgeLockMintResponse,
  CommitListItem,
  CommitRequest,
  CommitResult,
  CommitsListResponse,
  LpBalanceResponse,
  LpPositionsResponse,
  PoolStatus,
  RemoveLiquidityRequest,
} from "../../types/liquidity.types";
import { SOVEREIGN_FLOW_POLICY } from "../../types/liquidity.types";
import { httpClientV2 } from "./http-client";

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

type BridgePosition = {
  position_id: string;
  bridge_state: string;
};

type BridgePositionsResponse = {
  positions: BridgePosition[];
};

function mapCommitStatus(raw: string): CommitResult["status"] {
  switch (raw) {
    case "MATCHED":
      return "MATCHED";
    case "EXECUTED":
      return "EXECUTED";
    case "EXPIRED":
      return "EXPIRED";
    case "CANCELLED":
      return "CANCELLED";
    default:
      return "PENDING";
  }
}

function mapCommitListItemToCommitResult(item: CommitListItem): CommitResult {
  return {
    commit_id: item.CommitID,
    on_chain_commit_id: item.OnChainCommitID,
    pool_pair: item.PoolPair,
    side: item.Side === "B" ? "B" : "A",
    amount: item.Amount,
    status: mapCommitStatus(item.Status),
    expires_at: item.ExpiresAt,
    lp_ids: null,
  };
}

export const liquidityApi = {
  getPoolStatus: async (pair: string): Promise<PoolStatus> => {
    const response = await httpClientV2.get<PoolStatus>(`/amm/pool/${pair}/status`);
    return response.data;
  },
  // Live on-chain CBW3-LP position of this CB (013-amm-lp-shares).
  getLpBalance: async (): Promise<LpBalanceResponse> => {
    const response = await httpClientV2.get<LpBalanceResponse>("/amm/lp-balance");
    return response.data;
  },
  removeLiquidity: async (payload: RemoveLiquidityRequest): Promise<{ status: string }> => {
    const response = await httpClientV2.post<{ status: string }>("/amm/liquidity/remove", payload);
    return response.data;
  },
  lockMint: async (req: BridgeLockMintRequest): Promise<BridgeLockMintResponse> => {
    const response = await httpClientV2.post<BridgeLockMintResponse>("/bridge/lock-mint", {
      amount: req.amount,
    });
    return response.data;
  },
  approveAmm: async (payload: ApproveAmmRequest): Promise<{ status: string }> => {
    const response = await httpClientV2.post<{ status: string }>("/amm/token/approve-amm", payload);
    return response.data;
  },
  commitLiquidity: async (req: CommitRequest): Promise<CommitResult> => {
    const response = await httpClientV2.post<CommitResult>("/amm/liquidity/commit", {
      pool_pair: req.pool_pair,
      amount: req.amount,
    });
    return response.data;
  },
  listCommits: async (poolPair: string, status?: string): Promise<CommitsListResponse> => {
    const params: Record<string, string> = { pool_pair: poolPair };
    if (status) {
      params.status = status;
    }
    const response = await httpClientV2.get<CommitsListResponse>("/amm/liquidity/commits", { params });
    return response.data;
  },
  // Single-commit status — the commit lifecycle is driven server-side (DB + on-chain
  // registry + Cacti watcher), so the UI just polls this and reflects the current state
  // instead of blocking on a long-lived request. Returns null if the commit is unknown.
  getCommit: async (commitId: string): Promise<CommitResult | null> => {
    try {
      const response = await httpClientV2.get<CommitListItem>(`/amm/liquidity/commits/${commitId}`);
      return mapCommitListItemToCommitResult(response.data);
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 404) {
        return null;
      }
      throw error;
    }
  },
  cancelCommit: async (commitId: string, providerId: string): Promise<void> => {
    await httpClientV2.delete(`/amm/liquidity/commits/${commitId}`, {
      params: { provider_id: providerId },
    });
  },
  listPositions: async (poolPair: string, providerBankId: string): Promise<LpPositionsResponse> => {
    const response = await httpClientV2.get<LpPositionsResponse>("/amm/liquidity/positions", {
      params: {
        pool_pair: poolPair,
        provider_id: providerBankId,
      },
    });
    return response.data;
  },
  waitForBridgeActive: async (): Promise<boolean> => {
    const startedAt = Date.now();
    while (Date.now() - startedAt <= SOVEREIGN_FLOW_POLICY.bridgeTimeoutMs) {
      const response = await httpClientV2.get<BridgePositionsResponse | BridgePosition[]>(
        "/bridge/positions",
        { params: { state: "ACTIVE" } },
      );
      const positions = Array.isArray(response.data)
        ? response.data
        : response.data.positions ?? [];
      if (positions.length > 0) {
        return true;
      }
      await sleep(SOVEREIGN_FLOW_POLICY.bridgePollingIntervalMs);
    }
    return false;
  },
};
