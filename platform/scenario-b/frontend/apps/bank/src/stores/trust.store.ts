// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { onboardingApi } from "../services/api";
import { classifyTrustBlock, type TrustBlock } from "../services/api/trust-errors";

type TrustState = {
  /** The notice to show, or null when the central bank has not rejected us. */
  block: TrustBlock | null;
  checking: boolean;
  /** Called from the HTTP layer when the central bank refuses our identity on path. */
  reportRejection: (path: string) => Promise<void>;
  /** Called from the HTTP layer on every success; only a previously rejected path retires the notice. */
  noteSuccess: (path: string) => void;
  /** Re-reads the onboarding status even with a notice on screen — the operator asked. */
  recheck: () => Promise<void>;
  clear: () => void;
};

let inFlight: Promise<void> | null = null;
/**
 * The paths the central bank has refused.
 *
 * Trust is not restored by "some request worked": the payments page loads five endpoints at once, and
 * two of them (token/balance, token/fiat-balance) read this bank's own chain state without going
 * through the central bank at all. They answer 200 while the other three are rejected, so treating any
 * success as proof of trust erased the notice in the same page load that raised it.
 *
 * Remembering which paths were refused makes the rule self-calibrating: when one of THOSE succeeds, the
 * channel is genuinely working. No list of "trust-backed endpoints" to write down or keep up to date.
 */
let rejectedPaths = new Set<string>();

/** basePath drops the query string so the same endpoint compares equal across calls. */
const basePath = (path: string) => path.split("?")[0] ?? path;

/**
 * useTrustStore holds a single explanation for "the central bank does not know us", shared by every
 * screen.
 *
 * It lives outside the pages because the rejection is not a property of any one page: the same cause
 * breaks payments, the bridge and the pools at once. Reporting it centrally means a screen added later
 * inherits the explanation instead of surfacing a raw signature error.
 */
export const useTrustStore = create<TrustState>((set, get) => {
  const load = async () => {
    set({ checking: true });
    try {
      const response = await onboardingApi.getMyStatus();
      set({ block: classifyTrustBlock(response.status), checking: false });
    } catch {
      // The status call failing does not make the rejection go away; it only means we cannot name the
      // cause. Say that instead of guessing.
      set({ block: classifyTrustBlock(null), checking: false });
    }
  };

  const dedupedLoad = () => {
    if (!inFlight) {
      inFlight = load().finally(() => {
        inFlight = null;
      });
    }
    return inFlight;
  };

  return {
    block: null,
    checking: false,

    reportRejection: async (path) => {
      rejectedPaths.add(basePath(path));
      // A page load fires several requests at once and each one is rejected. Without this the operator
      // would trigger one status call per failed request.
      if (get().block !== null) {
        return;
      }
      await dedupedLoad();
    },

    noteSuccess: (path) => {
      if (get().block === null) {
        return;
      }
      if (rejectedPaths.has(basePath(path))) {
        get().clear();
      }
    },

    recheck: async () => {
      await dedupedLoad();
    },

    clear: () => {
      rejectedPaths = new Set();
      set({ block: null, checking: false });
    },
  };
});
