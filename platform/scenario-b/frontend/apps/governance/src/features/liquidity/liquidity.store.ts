import axios from "axios";
import { create } from "zustand";
import { liquidityApi } from "../../services/api/liquidity.api";
import { useAuthStore } from "../../stores/auth.store";
import {
  LIQUIDITY_OPERATIONAL_ERROR,
  LP_STATUS,
  SOVEREIGN_FLOW_PHASE,
} from "../../types/liquidity.types";
import type {
  ApproveAmmRequest,
  BridgeLockMintResponse,
  CommitRequest,
  CommitResult,
  LiquidityPosition,
  PoolStatus,
  RemoveLiquidityRequest,
  SovereignFlowPhase,
} from "../../types/liquidity.types";

type LiquidityStore = {
  poolStatus: PoolStatus | null;
  lpPositions: LiquidityPosition[];
  bridgeLockMintResult: BridgeLockMintResponse | null;
  currentBankId: string | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  activeCommit: CommitResult | null;
  commitStatus: "idle" | "loading" | "error";
  commitError: string | null;
  sovereignPhase: SovereignFlowPhase;
  commitLatencyWarning: boolean;
  operationalHint: string | null;
  fetchPoolStatus: (pair: string) => Promise<void>;
  removeLiquidity: (payload: RemoveLiquidityRequest) => Promise<void>;
  lockMint: (amount: string) => Promise<void>;
  approveAmm: (payload: ApproveAmmRequest) => Promise<void>;
  submitCommit: (payload: CommitRequest) => Promise<void>;
  cancelActiveCommit: (commitId: string) => Promise<void>;
  fetchLpPositions: (poolPair: string, providerBankId: string) => Promise<void>;
  clearCommit: () => void;
  getOperationalSummary: () => {
    phase: SovereignFlowPhase;
    poolStatus: string;
    commitStatus: string;
    hint: string | null;
  };
};

function extractApiError(error: unknown, fallback: string): string {
  if (axios.isAxiosError(error)) {
    const apiError = error.response?.data as { error?: string; message?: string } | undefined;
    if (apiError?.error) {
      return apiError.error;
    }
    if (apiError?.message) {
      return apiError.message;
    }
  }
  return error instanceof Error ? error.message : fallback;
}

export const useLiquidityStore = create<LiquidityStore>((set, get) => ({
  poolStatus: null,
  lpPositions: [],
  bridgeLockMintResult: null,
  currentBankId: null,
  status: "idle",
  error: null,
  activeCommit: null,
  commitStatus: "idle",
  commitError: null,
  sovereignPhase: SOVEREIGN_FLOW_PHASE.LOCK_MINT,
  commitLatencyWarning: false,
  operationalHint: null,
  fetchPoolStatus: async (pair) => {
    set({ status: "loading", error: null });
    try {
      const poolStatus = await liquidityApi.getPoolStatus(pair);
      set((state) => ({
        poolStatus,
        status: "idle",
        sovereignPhase:
          poolStatus.pool_status === "ACTIVE" && state.sovereignPhase === SOVEREIGN_FLOW_PHASE.COMMIT_EXECUTED
            ? SOVEREIGN_FLOW_PHASE.POOL_ACTIVE
            : state.sovereignPhase,
      }));
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to fetch pool status") });
    }
  },
  removeLiquidity: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await liquidityApi.removeLiquidity(payload);
      set((state) => ({
        lpPositions: state.lpPositions.map((item) =>
          item.lp_id === payload.lp_id ? { ...item, status: LP_STATUS.WITHDRAWN } : item,
        ),
        status: "idle",
      }));
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to remove liquidity") });
    }
  },
  lockMint: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await liquidityApi.lockMint({ amount });
      const bankId = useAuthStore.getState().profile?.bankId ?? null;
      set({
        bridgeLockMintResult: response,
        currentBankId: bankId,
        status: "idle",
        sovereignPhase: SOVEREIGN_FLOW_PHASE.BRIDGE_ACTIVE_WAIT,
        operationalHint: "Waiting for bridge position to become ACTIVE.",
      });

      const ready = await liquidityApi.waitForBridgeActive();
      set({
        sovereignPhase: ready ? SOVEREIGN_FLOW_PHASE.COMMIT_PENDING : SOVEREIGN_FLOW_PHASE.TIMEOUT,
        operationalHint: ready ? null : "Bridge position did not become ACTIVE within 120s.",
      });
    } catch (error) {
      set({
        status: "error",
        error: extractApiError(error, "Unable to complete bridge lock-mint"),
        sovereignPhase: SOVEREIGN_FLOW_PHASE.FAILED,
      });
    }
  },
  approveAmm: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await liquidityApi.approveAmm(payload);
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to approve AMM") });
    }
  },
  submitCommit: async (payload) => {
    set({
      commitStatus: "loading",
      commitError: null,
      sovereignPhase: SOVEREIGN_FLOW_PHASE.COMMIT_PENDING,
      commitLatencyWarning: false,
      operationalHint: null,
    });
    try {
      const result = await liquidityApi.commitLiquidity(payload);
      set({ activeCommit: result, commitStatus: "idle" });

      if (result.status === "PENDING") {
        const settledCommit = await liquidityApi.waitForCommitSettled(payload.pool_pair, result.commit_id, () => {
          set({
            commitLatencyWarning: true,
            operationalHint: "Commit still pending for over 30s. Waiting for watcher execution.",
          });
        });

        if (!settledCommit) {
          set({
            sovereignPhase: SOVEREIGN_FLOW_PHASE.TIMEOUT,
            commitError: "Commit execution timeout after 180s. Track commit status and retry if needed.",
          });
          return;
        }

        if (settledCommit.status === "CANCELLED") {
          set({
            activeCommit: settledCommit,
            sovereignPhase: SOVEREIGN_FLOW_PHASE.CANCELLED,
            commitLatencyWarning: false,
            operationalHint: "Commit was cancelled before execution.",
          });
          return;
        }

        const latestPoolStatus = await liquidityApi.getPoolStatus(payload.pool_pair);
        set({
          activeCommit: settledCommit,
          poolStatus: latestPoolStatus,
          sovereignPhase:
            latestPoolStatus.pool_status === "ACTIVE"
              ? SOVEREIGN_FLOW_PHASE.POOL_ACTIVE
              : SOVEREIGN_FLOW_PHASE.COMMIT_EXECUTED,
          commitLatencyWarning: false,
        });
      }
    } catch (error) {
      const apiError = extractApiError(error, "Unable to submit commit");

      if (apiError === LIQUIDITY_OPERATIONAL_ERROR.BRIDGE_POSITION_NOT_ACTIVE) {
        set({
          commitStatus: "error",
          commitError: apiError,
          sovereignPhase: SOVEREIGN_FLOW_PHASE.BRIDGE_ACTIVE_WAIT,
          operationalHint: "Waiting for Relayer confirmation. Polling bridge positions every 5s (timeout 120s).",
        });

        const ready = await liquidityApi.waitForBridgeActive();
        set({
          sovereignPhase: ready ? SOVEREIGN_FLOW_PHASE.COMMIT_PENDING : SOVEREIGN_FLOW_PHASE.TIMEOUT,
          operationalHint: ready
            ? "Bridge position is ACTIVE. You can resubmit the commit."
            : "Bridge position did not become ACTIVE within 120s.",
        });
        return;
      }

      set({
        commitStatus: "error",
        commitError: apiError,
        sovereignPhase: SOVEREIGN_FLOW_PHASE.FAILED,
      });
    }
  },
  cancelActiveCommit: async (commitId) => {
    set({ commitStatus: "loading", commitError: null });
    try {
      const bankId = get().currentBankId ?? useAuthStore.getState().profile?.bankId ?? null;
      if (!bankId) {
        set({
          commitStatus: "error",
          commitError: "Missing provider bank id to cancel commit.",
        });
        return;
      }
      await liquidityApi.cancelCommit(commitId, bankId);
      set({
        activeCommit: null,
        commitStatus: "idle",
        sovereignPhase: SOVEREIGN_FLOW_PHASE.CANCELLED,
        commitLatencyWarning: false,
      });
    } catch (error) {
      set({
        commitStatus: "error",
        commitError: extractApiError(error, "Unable to cancel commit"),
      });
    }
  },
  fetchLpPositions: async (poolPair, providerBankId) => {
    set({ status: "loading", error: null });
    try {
      const response = await liquidityApi.listPositions(poolPair, providerBankId);
      set({ lpPositions: response.positions, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to fetch LP positions") });
    }
  },
  clearCommit: () => {
    set({
      activeCommit: null,
      bridgeLockMintResult: null,
      commitStatus: "idle",
      commitError: null,
      sovereignPhase: SOVEREIGN_FLOW_PHASE.LOCK_MINT,
      commitLatencyWarning: false,
      operationalHint: null,
    });
  },
  getOperationalSummary: () => {
    const state = get();
    return {
      phase: state.sovereignPhase,
      poolStatus: state.poolStatus?.pool_status ?? "UNKNOWN",
      commitStatus: state.activeCommit?.status ?? "NONE",
      hint: state.operationalHint,
    };
  },
}));
