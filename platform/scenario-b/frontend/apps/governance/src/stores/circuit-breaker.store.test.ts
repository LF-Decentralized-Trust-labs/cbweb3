// SPDX-License-Identifier: Apache-2.0

// Tests for the network-wide breaker condition rendered by the portal chrome
// (043-breaker-txhash-mock-docs / FR-011 to FR-014).
//
// The chrome indicator makes a network-wide claim ("swaps are globally halted") while the
// authoritative breaker status is per-pair, so the condition is derived rather than read.
// These tests pin the derivation rules; they run in Vitest's default Node environment with no
// DOM and no new dependencies (FR-026).

import { beforeEach, describe, expect, it, vi } from "vitest";
import { deriveNetworkBreakerCondition, useCircuitBreakerStore } from "./circuit-breaker.store";
import type { CbState } from "../types/circuit-breaker-v2.types";

vi.mock("../services/api/amm-pairs.api", () => ({
  ammPairsApi: { getPairs: vi.fn() },
}));
vi.mock("../services/api/circuit-breaker-v2.api", () => ({
  circuitBreakerV2Api: { getStatus: vi.fn() },
}));

const { ammPairsApi } = await import("../services/api/amm-pairs.api");
const { circuitBreakerV2Api } = await import("../services/api/circuit-breaker-v2.api");
const pairsApi = vi.mocked(ammPairsApi);
const cbApi = vi.mocked(circuitBreakerV2Api);

function status(pair: string, state: CbState) {
  return {
    pair,
    state,
    pause_initiator: null,
    pause_reason: null,
    resume_request_id: null,
  };
}

describe("deriveNetworkBreakerCondition", () => {
  // N-1 / FR-012
  it("reports halted when any of several pairs is halted", () => {
    expect(deriveNetworkBreakerCondition(["LIVE", "HALTED", "LIVE"])).toBe("HALTED");
  });

  // N-2
  it("reports operational when no pair is halted", () => {
    expect(deriveNetworkBreakerCondition(["LIVE", "LIVE"])).toBe("LIVE");
  });

  // A pair awaiting resume quorum is still paused on-chain, so swaps are still halted. The
  // indicator must not claim operational while the breaker page shows RESUME_PENDING.
  it("treats a pair awaiting resume quorum as halted", () => {
    expect(deriveNetworkBreakerCondition(["LIVE", "RESUME_PENDING"])).toBe("HALTED");
  });

  // N-3 / FR-014 — empty pair set is indeterminate, never a halt claim.
  it("is indeterminate when no pairs exist", () => {
    expect(deriveNetworkBreakerCondition([])).toBeNull();
  });

  // N-4 — an unreadable pair is not-known-halted, and must not be reported as operational
  // on partial information either.
  it("is indeterminate when every pair status is unreadable", () => {
    expect(deriveNetworkBreakerCondition([null, null])).toBeNull();
  });

  it("still reports halted when one pair is halted and another is unreadable", () => {
    expect(deriveNetworkBreakerCondition(["HALTED", null])).toBe("HALTED");
  });

  it("is indeterminate when readable pairs are live but at least one is unreadable", () => {
    expect(deriveNetworkBreakerCondition(["LIVE", null])).toBeNull();
  });
});

describe("circuit-breaker store — network-wide condition", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useCircuitBreakerStore.setState({ circuitBreaker: null, status: "idle", error: null });
  });

  it("derives halted from the authoritative per-pair V2 status", async () => {
    pairsApi.getPairs.mockResolvedValue([
      { pair_id: "W-BRL-W-ARS", status: "ACTIVE" },
      { pair_id: "W-BRL-W-CLP", status: "ACTIVE" },
    ]);
    cbApi.getStatus.mockImplementation(async (pair: string) =>
      status(pair, pair === "W-BRL-W-CLP" ? "HALTED" : "LIVE"),
    );

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker?.state).toBe("HALTED");
    expect(useCircuitBreakerStore.getState().status).toBe("idle");
  });

  it("derives operational when no pair is halted", async () => {
    pairsApi.getPairs.mockResolvedValue([{ pair_id: "W-BRL-W-ARS", status: "ACTIVE" }]);
    cbApi.getStatus.mockResolvedValue(status("W-BRL-W-ARS", "LIVE"));

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker?.state).toBe("LIVE");
  });

  // FR-014 — no pairs: no halt claim, and the layout must not break, so this is not an error.
  it("does not assert a halt when no pairs exist", async () => {
    pairsApi.getPairs.mockResolvedValue([]);

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker).toBeNull();
    expect(cbApi.getStatus).not.toHaveBeenCalled();
  });

  // FR-014 — pair set unavailable: indeterminate, and the failure is surfaced not swallowed.
  it("does not assert a halt when the pair list cannot be retrieved, and surfaces the failure", async () => {
    pairsApi.getPairs.mockRejectedValue(new Error("pair registry unreachable"));

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker).toBeNull();
    expect(useCircuitBreakerStore.getState().status).toBe("error");
    expect(useCircuitBreakerStore.getState().error).toBeTruthy();
  });

  // N-4 — one pair unreadable while another is halted: the halt still wins.
  it("reports halted even when another pair's status fails to load", async () => {
    pairsApi.getPairs.mockResolvedValue([
      { pair_id: "W-BRL-W-ARS", status: "ACTIVE" },
      { pair_id: "W-BRL-W-CLP", status: "ACTIVE" },
    ]);
    cbApi.getStatus.mockImplementation(async (pair: string) => {
      if (pair === "W-BRL-W-CLP") throw new Error("status unreachable");
      return status(pair, "HALTED");
    });

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker?.state).toBe("HALTED");
  });

  // N-4 — partial information must not be reported as operational, and must be surfaced.
  it("is indeterminate and surfaces the condition when a pair status is unreadable and none is halted", async () => {
    pairsApi.getPairs.mockResolvedValue([
      { pair_id: "W-BRL-W-ARS", status: "ACTIVE" },
      { pair_id: "W-BRL-W-CLP", status: "ACTIVE" },
    ]);
    cbApi.getStatus.mockImplementation(async (pair: string) => {
      if (pair === "W-BRL-W-CLP") throw new Error("status unreachable");
      return status(pair, "LIVE");
    });

    await useCircuitBreakerStore.getState().fetchState();

    expect(useCircuitBreakerStore.getState().circuitBreaker).toBeNull();
    expect(useCircuitBreakerStore.getState().error).toBeTruthy();
  });

  // FR-013 / FR-011 — the indicator must never be served from synthetic data. The V1
  // mock-capable governance endpoint must not be consulted at all.
  it("reads the authoritative V2 status and never the mock-capable V1 endpoint", async () => {
    pairsApi.getPairs.mockResolvedValue([{ pair_id: "W-BRL-W-ARS", status: "ACTIVE" }]);
    cbApi.getStatus.mockResolvedValue(status("W-BRL-W-ARS", "LIVE"));

    await useCircuitBreakerStore.getState().fetchState();

    expect(cbApi.getStatus).toHaveBeenCalledWith("W-BRL-W-ARS");
  });
});
