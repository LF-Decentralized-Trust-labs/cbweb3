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
});
