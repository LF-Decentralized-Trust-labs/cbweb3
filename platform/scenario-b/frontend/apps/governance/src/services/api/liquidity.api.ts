import type {
  ApproveAmmRequest,
  BridgeLockMintRequest,
  BridgeLockMintResponse,
  CommitListItem,
  CommitRequest,
  CommitResult,
  CommitsListResponse,
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

function mapCommitListItemToCommitResult(item: CommitListItem): CommitResult {
  const side = item.Side === "B" ? "B" : "A";
  const status = item.Status === "CANCELLED" ? "CANCELLED" : "EXECUTED";
  return {
    commit_id: item.CommitID,
    on_chain_commit_id: item.OnChainCommitID,
    pool_pair: item.PoolPair,
    side,
    amount: item.Amount,
    status,
    expires_at: item.ExpiresAt,
    lp_ids: null,
  };
}

export const liquidityApi = {
  getPoolStatus: async (pair: string): Promise<PoolStatus> => {
    const response = await httpClientV2.get<PoolStatus>(`/amm/pool/${pair}/status`);
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
  waitForCommitSettled: async (
    poolPair: string,
    commitId: string,
    onWarn: () => void,
  ): Promise<CommitResult | null> => {
    const startedAt = Date.now();
    let warned = false;

    while (Date.now() - startedAt <= SOVEREIGN_FLOW_POLICY.commitTimeoutMs) {
      const response = await liquidityApi.listCommits(poolPair);
      const matchedCommit = response.commits.find((item) => item.CommitID === commitId) ?? null;
      if (matchedCommit && (matchedCommit.Status === "EXECUTED" || matchedCommit.Status === "CANCELLED")) {
        return mapCommitListItemToCommitResult(matchedCommit);
      }

      if (!warned && Date.now() - startedAt >= SOVEREIGN_FLOW_POLICY.commitWarnAfterMs) {
        warned = true;
        onWarn();
      }

      await sleep(SOVEREIGN_FLOW_POLICY.commitPollingIntervalMs);
    }

    return null;
  },
};
