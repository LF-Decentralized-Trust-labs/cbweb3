// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OnboardingMyStatusResponse } from "../../types";
import { useTrustStore } from "../trust.store";

const getMyStatus = vi.fn<() => Promise<OnboardingMyStatusResponse>>();

vi.mock("../../services/api", () => ({
  onboardingApi: {
    getMyStatus: () => getMyStatus(),
  },
}));

const myStatus = (status: OnboardingMyStatusResponse["status"]): OnboardingMyStatusResponse => ({
  request_id: "req-1",
  user_id: "user-1",
  status,
});

describe("useTrustStore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useTrustStore.getState().clear();
  });

  it("classifies the rejection from the onboarding status", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));

    await useTrustStore.getState().reportRejection("/payments/deposits");

    expect(useTrustStore.getState().block?.kind).toBe("onboarding-required");
  });

  // The regression this rule exists for. The payments page loads five endpoints at once: three go to
  // the central bank and are rejected, while token/balance and token/fiat-balance read the bank's own
  // chain state and return 200. Treating any success as proof of trust let those two erase the notice
  // the three rejections had just raised — the banner never appeared, and the empty table went back to
  // claiming there were no records.
  it("keeps the notice when an unrelated endpoint succeeds in the same page load", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    const trust = useTrustStore.getState();

    await trust.reportRejection("/payments/deposits");
    trust.noteSuccess("/token/balance");
    trust.noteSuccess("/token/fiat-balance");

    expect(useTrustStore.getState().block).not.toBeNull();
  });

  it("retires the notice when a previously rejected path succeeds", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    const trust = useTrustStore.getState();

    await trust.reportRejection("/payments/deposits");
    expect(useTrustStore.getState().block).not.toBeNull();

    trust.noteSuccess("/payments/deposits");

    expect(useTrustStore.getState().block).toBeNull();
  });

  it("ignores a success on a path that was never rejected", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    const trust = useTrustStore.getState();

    await trust.reportRejection("/payments/deposits");
    trust.noteSuccess("/onboarding/my-status");

    expect(useTrustStore.getState().block).not.toBeNull();
  });

  it("asks the central bank once when a page fires several calls in parallel", async () => {
    // The payments page loads deposits, escrows, redeems and two balances at once. Five rejections
    // must not become five status calls.
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    const report = useTrustStore.getState().reportRejection;

    await Promise.all([report("/payments/deposits"), report("/payments/escrows"), report("/payments/redeems"), report("/token/balance"), report("/token/fiat-balance")]);

    expect(getMyStatus).toHaveBeenCalledOnce();
  });

  it("does not re-ask while a notice is already on screen", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    await useTrustStore.getState().reportRejection("/payments/deposits");
    expect(getMyStatus).toHaveBeenCalledOnce();

    await useTrustStore.getState().reportRejection("/payments/deposits");

    expect(getMyStatus).toHaveBeenCalledOnce();
  });

  it("still explains itself when the status call also fails", async () => {
    getMyStatus.mockRejectedValue(new Error("gateway unreachable"));

    await useTrustStore.getState().reportRejection("/payments/deposits");

    expect(useTrustStore.getState().block?.kind).toBe("unknown");
  });

  it("clear removes the notice and allows a fresh look afterwards", async () => {
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    await useTrustStore.getState().reportRejection("/payments/deposits");
    expect(useTrustStore.getState().block).not.toBeNull();

    useTrustStore.getState().clear();
    expect(useTrustStore.getState().block).toBeNull();

    await useTrustStore.getState().reportRejection("/payments/deposits");
    expect(getMyStatus).toHaveBeenCalledTimes(2);
  });

  it("recheck re-reads the status even with a notice on screen", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    await useTrustStore.getState().reportRejection("/payments/deposits");

    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    await useTrustStore.getState().recheck();

    expect(getMyStatus).toHaveBeenCalledTimes(2);
    expect(useTrustStore.getState().block?.kind).toBe("not-recognized");
  });

  // --- recheck has to exercise the channel, not just re-read the reason ---
  //
  // classifyTrustBlock returns a block for EVERY status, ACTIVE included, so re-reading the status
  // could only ever swap one notice for another: the Recheck button could not retire the notice it
  // sits on, no matter what the operator had fixed. And clearing on ACTIVE would be wrong rather
  // than merely coarse — ACTIVE is exactly the case where the rejection is a configuration problem
  // and the notice is still true. What proves the channel works is the same thing noteSuccess
  // already trusts: a path the central bank had refused answering again.

  it("recheck retries a refused read and retires the notice when it answers", async () => {
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    const probe = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportRejection("/payments/deposits", "get", probe);
    expect(useTrustStore.getState().block).not.toBeNull();

    await useTrustStore.getState().recheck();

    expect(probe).toHaveBeenCalledOnce();
    expect(useTrustStore.getState().block).toBeNull();
    // The channel answered, so there is nothing left to explain and no reason to ask the status.
    expect(getMyStatus).toHaveBeenCalledOnce();
  });

  it("recheck keeps the notice and refreshes the reason when the read is refused again", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));
    const probe = vi.fn().mockRejectedValue(new Error("still refused"));

    await useTrustStore.getState().reportRejection("/payments/deposits", "get", probe);

    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    await useTrustStore.getState().recheck();

    expect(probe).toHaveBeenCalledOnce();
    // Still blocked, but the operator gets the current cause rather than the stale one.
    expect(useTrustStore.getState().block?.kind).toBe("not-recognized");
    expect(useTrustStore.getState().checking).toBe(false);
  });

  // The one that matters. rejectedPaths holds whatever the central bank refused, and that includes
  // POSTs: initiating a payment, a bridge-out. Replaying one of those because an operator pressed a
  // button labelled "Recheck" would move value. Only a read may be repeated.
  it("recheck never replays a refused write", async () => {
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    const replayPayment = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportRejection("/payments/deposits", "post", replayPayment);
    await useTrustStore.getState().recheck();

    expect(replayPayment).not.toHaveBeenCalled();
    // With nothing safe to repeat, recheck falls back to re-reading the reason.
    expect(getMyStatus).toHaveBeenCalledTimes(2);
    expect(useTrustStore.getState().block).not.toBeNull();
  });

  it("recheck prefers a refused read even when a write was refused first", async () => {
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    const replayPayment = vi.fn().mockResolvedValue({ status: 200 });
    const probeRead = vi.fn().mockResolvedValue({ status: 200 });

    // A page load rejects the write first and the reads right after it.
    await useTrustStore.getState().reportRejection("/payments/deposits", "post", replayPayment);
    await useTrustStore.getState().reportRejection("/payments/escrows", "get", probeRead);

    await useTrustStore.getState().recheck();

    expect(replayPayment).not.toHaveBeenCalled();
    expect(probeRead).toHaveBeenCalledOnce();
    expect(useTrustStore.getState().block).toBeNull();
  });
});
