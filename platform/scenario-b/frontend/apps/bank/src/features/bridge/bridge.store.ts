import { create } from "zustand";
import { bridgeApi } from "../../services/api/bridge.api";
import type {
  BridgedAssetPosition,
  BurnUnlockRequest,
  LockMintRequest,
} from "../../types/bridge.types";

type BridgeStore = {
  positions: BridgedAssetPosition[];
  status: "idle" | "loading" | "error";
  error: string | null;
  loadPositions: () => Promise<void>;
  submitLockMint: (payload: LockMintRequest) => Promise<void>;
  submitBurnUnlock: (payload: BurnUnlockRequest) => Promise<void>;
};

export const useBridgeStore = create<BridgeStore>((set) => ({
  positions: [],
  status: "idle",
  error: null,
  loadPositions: async () => {
    set({ status: "loading", error: null });
    try {
      const positions = await bridgeApi.listPositions();
      set({ positions, status: "idle" });
    } catch (error) {
      set({
        status: "error",
        error: error instanceof Error ? error.message : "Unable to load bridge positions",
      });
    }
  },
  submitLockMint: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const created = await bridgeApi.lockMint(payload);
      set((state) => ({
        positions: [created, ...state.positions],
        status: "idle",
      }));
    } catch (error) {
      set({
        status: "error",
        error: error instanceof Error ? error.message : "Unable to create bridge position",
      });
    }
  },
  submitBurnUnlock: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const update = await bridgeApi.burnUnlock(payload);
      const updatedAt = new Date().toISOString();
      set((state) => ({
        positions: state.positions.map((position) =>
          position.position_id === update.position_id
            ? { ...position, bridge_state: update.bridge_state, updated_at: updatedAt }
            : position,
        ),
        status: "idle",
      }));
    } catch (error) {
      set({
        status: "error",
        error: error instanceof Error ? error.message : "Unable to burn and unlock",
      });
    }
  },
}));
