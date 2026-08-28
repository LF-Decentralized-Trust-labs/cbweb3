// SPDX-License-Identifier: Apache-2.0

// Store-level tests for on-chain transaction-hash capture in the Scenario B circuit-breaker
// store (043-breaker-txhash-mock-docs / FR-006 / FR-009).
//
// These run in Vitest's default Node environment: no DOM, no component-testing library, no
// new dependencies (FR-026). They cover the layer where the reference could be silently
// dropped — capturing it from each response and retaining it across a status refresh.
// Rendering is verified manually per the feature quickstart.

import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCircuitBreakerV2Store } from "./circuit-breaker-v2.store";
import type {
  CircuitBreakerV2Status,
  ProposeResumeResponse,
} from "../../types/circuit-breaker-v2.types";

vi.mock("../../services/api/circuit-breaker-v2.api", () => ({
  circuitBreakerV2Api: {
    getStatus: vi.fn(),
    pause: vi.fn(),
    proposeResume: vi.fn(),
    signResume: vi.fn(),
  },
}));

const { circuitBreakerV2Api } = await import("../../services/api/circuit-breaker-v2.api");
const api = vi.mocked(circuitBreakerV2Api);

const PAIR = "W-BRL-W-ARS";

function statusResponse(overrides: Partial<CircuitBreakerV2Status> = {}): CircuitBreakerV2Status {
  return {
    pair: PAIR,
    state: "HALTED",
    pause_initiator: "central-bank-a",
    pause_reason: "INCIDENT",
    resume_request_id: null,
    ...overrides,
  };
}

function resetStore() {
  useCircuitBreakerV2Store.setState({
    cbStatus: null,
    resumeRequestId: null,
    isStale: false,
    status: "idle",
    error: null,
  });
}

describe("circuit-breaker v2 store — on-chain reference capture", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetStore();
  });

  it("captures tx_hash from a pause response", async () => {
    api.pause.mockResolvedValue(statusResponse({ tx_hash: "0xpause111" }));

    await useCircuitBreakerV2Store
      .getState()
      .pause({ pair: PAIR, bank_id: "central-bank-a", reason_code: "INCIDENT" });

    expect(useCircuitBreakerV2Store.getState().cbStatus?.tx_hash).toBe("0xpause111");
  });

  it("captures tx_hash from a sign-resume response", async () => {
    api.signResume.mockResolvedValue(statusResponse({ state: "LIVE", tx_hash: "0xsign444" }));

    await useCircuitBreakerV2Store
      .getState()
      .signResume({ pair: PAIR, request_id: "0xproposal222", bank_id: "central-bank-b" });

    expect(useCircuitBreakerV2Store.getState().cbStatus?.tx_hash).toBe("0xsign444");
  });

  it("captures tx_hash from a status refresh, so it survives a reload", async () => {
    api.getStatus.mockResolvedValue(statusResponse({ tx_hash: "0xstatus555" }));

    await useCircuitBreakerV2Store.getState().fetchStatus(PAIR);

    expect(useCircuitBreakerV2Store.getState().cbStatus?.tx_hash).toBe("0xstatus555");
  });

  // The critical case: proposeResume's reducer rebuilds cbStatus field by field instead of
  // replacing it wholesale, so the hash is lost unless it is carried explicitly.
  it("carries tx_hash through proposeResume when an existing status is present", async () => {
    useCircuitBreakerV2Store.setState({ cbStatus: statusResponse({ tx_hash: "0xpause111" }) });
    const response: ProposeResumeResponse = {
      request_id: "0xproposal222",
      state: "RESUME_PENDING",
      tx_hash: "0xtx333",
    };
    api.proposeResume.mockResolvedValue(response);

    await useCircuitBreakerV2Store
      .getState()
      .proposeResume({ pair: PAIR, bank_id: "central-bank-a" });

    const cbStatus = useCircuitBreakerV2Store.getState().cbStatus;
    expect(cbStatus?.tx_hash).toBe("0xtx333");
    expect(cbStatus?.resume_request_id).toBe("0xproposal222");
    // The proposal id and the transaction hash are distinct identifiers (rule D-3).
    expect(cbStatus?.tx_hash).not.toBe(cbStatus?.resume_request_id);
  });

  it("carries tx_hash through proposeResume when there is no prior status", async () => {
    api.proposeResume.mockResolvedValue({
      request_id: "0xproposal222",
      state: "RESUME_PENDING",
      tx_hash: "0xtx333",
    });

    await useCircuitBreakerV2Store
      .getState()
      .proposeResume({ pair: PAIR, bank_id: "central-bank-a" });

    expect(useCircuitBreakerV2Store.getState().cbStatus?.tx_hash).toBe("0xtx333");
  });

  it("leaves tx_hash undefined when the response omits it, rather than inventing a value", async () => {
    api.pause.mockResolvedValue(statusResponse());

    await useCircuitBreakerV2Store
      .getState()
      .pause({ pair: PAIR, bank_id: "central-bank-a", reason_code: "INCIDENT" });

    const cbStatus = useCircuitBreakerV2Store.getState().cbStatus;
    expect(cbStatus?.state).toBe("HALTED"); // the action still applied
    expect(cbStatus?.tx_hash).toBeUndefined();
  });

  it("does not gate the action on the reference: a failed action reports the error", async () => {
    api.pause.mockRejectedValue(new Error("on-chain pause failed"));

    await useCircuitBreakerV2Store
      .getState()
      .pause({ pair: PAIR, bank_id: "central-bank-a", reason_code: "INCIDENT" });

    expect(useCircuitBreakerV2Store.getState().status).toBe("error");
    expect(useCircuitBreakerV2Store.getState().error).toBe("on-chain pause failed");
  });
});
