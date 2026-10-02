// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { onboardingApi } from "../services/api";
import { classifyTrustBlock, relayConfigurationBlock, type TrustBlock } from "../services/api/trust-errors";

/**
 * TrustProbe re-issues one refused request through the same HTTP client that made it, so a success
 * runs the response interceptor and retires the notice by the store's own rule.
 *
 * Held only for reads. See reportRejection.
 */
export type TrustProbe = () => Promise<unknown>;

type TrustState = {
  /** The notice to show, or null when the central bank has not rejected us. */
  block: TrustBlock | null;
  checking: boolean;
  /**
   * Called from the HTTP layer when the central bank refuses our identity on path.
   *
   * method and probe are how recheck can do more than re-read the reason. The probe is kept ONLY
   * when the refused request was a read: rejectedPaths holds whatever the central bank refused, and
   * that includes POSTs — initiating a payment, a bridge-out. Replaying one of those because an
   * operator pressed a button labelled "Recheck" would move value.
   */
  reportRejection: (path: string, method?: string, probe?: TrustProbe) => Promise<void>;
  /**
   * Called from the HTTP layer when the relay credential, rather than our identity, was refused.
   *
   * Separate from reportRejection because the explanation is already known from the code: there is
   * no onboarding status to read, and reading one would replace a true notice with a wrong one.
   */
  reportConfigurationFault: (path: string, method?: string, probe?: TrustProbe) => Promise<void>;
  /** Called from the HTTP layer on every success; only a previously rejected path retires the notice. */
  noteSuccess: (path: string) => void;
  /**
   * Retries a refused read and, if it answers, retires the notice; otherwise re-reads the reason.
   *
   * Re-reading the status alone could never clear anything: classifyTrustBlock returns a block for
   * every status, ACTIVE included, so the button could only swap one notice for another. Clearing on
   * ACTIVE instead would be wrong rather than coarse — ACTIVE is exactly the case where the
   * rejection is a configuration problem and the notice is still true.
   */
  recheck: () => Promise<void>;
  /**
   * Whether recheck can do anything at all, so the notice does not offer a button that cannot work.
   *
   * False in exactly one case: a relay configuration fault with no refused read to replay. There the
   * reason cannot change — the code already named it, and the onboarding status has no bearing on a
   * credential shared between two gateways — and the only honest probe, replaying the refused
   * request, is a write the store must never repeat.
   */
  canRecheck: boolean;
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

/**
 * The refused READS, keyed by path, each with a way to ask again.
 *
 * Separate from rejectedPaths on purpose: that set decides what counts as proof of trust and holds
 * every refused path, writes included. This map decides what recheck is allowed to repeat, and a
 * write must never be in it.
 */
let readProbes = new Map<string, TrustProbe>();

/** basePath drops the query string so the same endpoint compares equal across calls. */
const basePath = (path: string) => path.split("?")[0] ?? path;

/** isRead reports whether a refused request is safe to repeat. */
const isRead = (method?: string) => (method ?? "").toLowerCase() === "get";

/**
 * useTrustStore holds a single explanation for "the central bank does not know us", shared by every
 * screen.
 *
 * It lives outside the pages because the rejection is not a property of any one page: the same cause
 * breaks payments, the bridge and the pools at once. Reporting it centrally means a screen added later
 * inherits the explanation instead of surfacing a raw signature error.
 */
export const useTrustStore = create<TrustState>((set, get) => {
  /**
   * Whether the notice on screen explains a relay configuration fault.
   *
   * Derived from the notice rather than tracked alongside it. A separate flag could disagree with
   * what the operator is reading — a status call already in flight resolves, replaces the block, and
   * leaves the flag saying otherwise — and every later decision would then be made on the wrong one.
   */
  const showingRelayFault = () => get().block?.kind === "relay-misconfigured";

  const syncRecheckability = () => {
    set({ canRecheck: readProbes.size > 0 || get().block?.kind !== "relay-misconfigured" });
  };

  /**
   * settle applies an onboarding classification, unless a configuration notice is already up.
   *
   * The classification answers "what is wrong with this institution's registration", which is not
   * the question when the gateway could not authenticate at all. Letting it land would replace a
   * true explanation with a confident wrong one.
   */
  const settle = (candidate: TrustBlock) => {
    set({ block: showingRelayFault() ? relayConfigurationBlock() : candidate, checking: false });
    syncRecheckability();
  };

  const load = async () => {
    set({ checking: true });
    try {
      const response = await onboardingApi.getMyStatus();
      settle(classifyTrustBlock(response.status));
    } catch {
      // The status call failing does not make the rejection go away; it only means we cannot name the
      // cause. Say that instead of guessing.
      settle(classifyTrustBlock(null));
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
    canRecheck: true,

    reportRejection: async (path, method, probe) => {
      const key = basePath(path);
      rejectedPaths.add(key);
      if (probe && isRead(method)) {
        readProbes.set(key, probe);
      }
      syncRecheckability();
      // A page load fires several requests at once and each one is rejected. Without this the operator
      // would trigger one status call per failed request.
      if (get().block !== null) {
        return;
      }
      await dedupedLoad();
    },

    reportConfigurationFault: async (path, method, probe) => {
      const key = basePath(path);
      rejectedPaths.add(key);
      if (probe && isRead(method)) {
        readProbes.set(key, probe);
      }
      // First notice wins, as with a trust rejection: a page load raises several at once and the
      // operator needs one explanation, not the last one to arrive.
      if (get().block === null) {
        set({ block: relayConfigurationBlock(), checking: false });
      }
      syncRecheckability();
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
      const next = readProbes.entries().next();
      if (next.done) {
        // Nothing safe to repeat — only writes were refused, or the notice came from somewhere with
        // no request to replay. Refreshing the reason is all this button can honestly do, and for a
        // configuration fault it cannot even do that: canRecheck is false there and the notice offers
        // no button, so this is the guard for a caller that asks anyway.
        if (!showingRelayFault()) {
          await dedupedLoad();
        }
        return;
      }
      const [path, probe] = next.value;

      set({ checking: true });
      try {
        await probe();
      } catch {
        // Still refused. The cause may have changed even so (onboarding advanced, a credential was
        // frozen), so the operator gets the current reason instead of the one from the first refusal
        // — unless the refusal was the relay credential, where the onboarding status has no bearing
        // on it. Re-reading it there would spend a request to learn nothing, so the notice stands and
        // only the spinner stops.
        if (showingRelayFault()) {
          set({ checking: false });
          return;
        }
        await load();
        return;
      }
      // The channel answered on a path the central bank had refused, which is exactly the proof
      // noteSuccess acts on. Calling it here rather than relying on the interceptor keeps the outcome
      // of this action in one place; the interceptor's own call is then a no-op.
      set({ checking: false });
      get().noteSuccess(path);
    },

    clear: () => {
      rejectedPaths = new Set();
      readProbes = new Map();
      set({ block: null, checking: false, canRecheck: true });
    },
  };
});
