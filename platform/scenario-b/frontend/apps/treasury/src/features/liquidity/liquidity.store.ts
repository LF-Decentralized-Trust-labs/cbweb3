import axios from "axios";
import { toast } from "@cbweb3/ui";
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
  // on_chain_commit_id of the counterpart commit we last toasted, to avoid re-notifying.
  lastNotifiedCounterpartId: string | null;
  fetchPoolStatus: (pair: string) => Promise<void>;
  removeLiquidity: (payload: RemoveLiquidityRequest) => Promise<void>;
  lockMint: (amount: string) => Promise<void>;
  approveAmm: (payload: ApproveAmmRequest) => Promise<void>;
  submitCommit: (payload: CommitRequest) => Promise<void>;
  refreshCommitStatus: (commitId: string) => Promise<void>;
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
  lastNotifiedCounterpartId: null,
  fetchPoolStatus: async (pair) => {
    set({ status: "loading", error: null });
    try {
      const poolStatus = await liquidityApi.getPoolStatus(pair);
      set((state) => {
        // Fire a one-time toast when a counterpart commit first appears (pool not yet ACTIVE),
        // deduping by on_chain_commit_id so repeated polls don't re-notify.
        const counterpart = poolStatus.counterpart_commit ?? null;
        let lastNotifiedCounterpartId = state.lastNotifiedCounterpartId;
        if (
          counterpart &&
          poolStatus.pool_status !== "ACTIVE" &&
          counterpart.on_chain_commit_id !== state.lastNotifiedCounterpartId
        ) {
          toast.info("Counterpart liquidity is waiting", {
            description: `A counterpart committed ${counterpart.amount} to side ${counterpart.side}. Match it to activate the pool.`,
          });
          lastNotifiedCounterpartId = counterpart.on_chain_commit_id;
        } else if (!counterpart) {
          lastNotifiedCounterpartId = null;
        }
        return {
          poolStatus,
          status: "idle",
          lastNotifiedCounterpartId,
          sovereignPhase:
            poolStatus.pool_status === "ACTIVE" && state.sovereignPhase === SOVEREIGN_FLOW_PHASE.COMMIT_EXECUTED
              ? SOVEREIGN_FLOW_PHASE.POOL_ACTIVE
              : state.sovereignPhase,
        };
      });
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
      const bankId = useAuthStore.getState().user?.institutionId ?? null;
      set({
        bridgeLockMintResult: response,
        currentBankId: bankId,
        sovereignPhase: SOVEREIGN_FLOW_PHASE.BRIDGE_ACTIVE_WAIT,
        operationalHint: "Waiting for bridge position to become ACTIVE.",
      });

      const ready = await liquidityApi.waitForBridgeActive();
      set({
        status: "idle",
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
      // Fire-and-forget: registering the commit persists it server-side (DB + on-chain
      // LiquidityCommitRegistry) and the Cacti watcher drives it to execution once the
      // counterpart matches. We do NOT block the UI waiting for settlement — the monitor
      // step polls refreshCommitStatus()/pool status, and the user can safely leave the
      // page and re-attach later via "Monitor Pending Commit".
      const result = await liquidityApi.commitLiquidity(payload);
      set({
        activeCommit: result,
        commitStatus: "idle",
        sovereignPhase:
          result.status === "EXECUTED"
            ? SOVEREIGN_FLOW_PHASE.COMMIT_EXECUTED
            : SOVEREIGN_FLOW_PHASE.COMMIT_PENDING,
        commitLatencyWarning: false,
        operationalHint:
          result.status === "EXECUTED"
            ? null
            : "Commit registered. Coordination continues server-side — you can safely leave this page.",
      });
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
  refreshCommitStatus: async (commitId) => {
    // Non-blocking poll of a single commit's server-side state. Safe to call on an
    // interval from the monitor step; never throws into the UI.
    try {
      const commit = await liquidityApi.getCommit(commitId);
      if (!commit) {
        return;
      }
      if (commit.status === "EXECUTED") {
        const latestPoolStatus = await liquidityApi.getPoolStatus(commit.pool_pair);
        set({
          activeCommit: commit,
          poolStatus: latestPoolStatus,
          sovereignPhase:
            latestPoolStatus.pool_status === "ACTIVE"
              ? SOVEREIGN_FLOW_PHASE.POOL_ACTIVE
              : SOVEREIGN_FLOW_PHASE.COMMIT_EXECUTED,
          operationalHint: null,
        });
        return;
      }
      if (commit.status === "CANCELLED" || commit.status === "EXPIRED") {
        set({
          activeCommit: commit,
          sovereignPhase: SOVEREIGN_FLOW_PHASE.CANCELLED,
          operationalHint:
            commit.status === "EXPIRED"
              ? "Commit expired before a counterpart matched. Cancel and retry when ready."
              : "Commit was cancelled.",
        });
        return;
      }
      // PENDING or MATCHED — still coordinating.
      set({ activeCommit: commit });
    } catch {
      // Transient read error — leave state as-is; the next poll retries.
    }
  },
  cancelActiveCommit: async (commitId) => {
    set({ commitStatus: "loading", commitError: null });
    try {
      const bankId = get().currentBankId ?? useAuthStore.getState().user?.institutionId ?? null;
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
