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

  // --- the relay credential is a different explanation, and it must not be reclassified ---
  //
  // classifyTrustBlock reads this institution's onboarding status, which is the right question when
  // the central bank refuses a known identity and the wrong one here: the request never
  // authenticated, so the cause is a credential on the wire, not a registration. An ACTIVE bank
  // would be told its institution is "not recognized"; a bank mid-onboarding would be told to go and
  // finish it. Both are dead ends, and the second is a dead end the operator would actually walk.

  it("explains a configuration fault without asking about onboarding", async () => {
    await useTrustStore.getState().reportConfigurationFault("/payments/deposits");

    expect(useTrustStore.getState().block?.kind).toBe("relay-misconfigured");
    expect(getMyStatus).not.toHaveBeenCalled();
  });

  it("retires the notice when a previously refused path answers again", async () => {
    const trust = useTrustStore.getState();

    await trust.reportConfigurationFault("/payments/deposits");
    trust.noteSuccess("/payments/deposits");

    expect(useTrustStore.getState().block).toBeNull();
  });

  it("recheck keeps the configuration explanation when the read is refused again", async () => {
    // The regression this guards: recheck's failure path re-reads the onboarding status, which would
    // silently swap the configuration notice for an onboarding one and send the operator to a wizard
    // that cannot fix a shared secret.
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    const probe = vi.fn().mockRejectedValue(new Error("still refused"));

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "get", probe);
    await useTrustStore.getState().recheck();

    expect(probe).toHaveBeenCalledOnce();
    expect(useTrustStore.getState().block?.kind).toBe("relay-misconfigured");
    expect(getMyStatus).not.toHaveBeenCalled();
    expect(useTrustStore.getState().checking).toBe(false);
  });

  it("recheck retires the configuration notice when the channel answers", async () => {
    const probe = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "get", probe);
    await useTrustStore.getState().recheck();

    expect(useTrustStore.getState().block).toBeNull();
  });

  it("recheck never replays a write refused for a configuration fault", async () => {
    getMyStatus.mockResolvedValue(myStatus("ACTIVE"));
    const replayPayment = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "post", replayPayment);
    await useTrustStore.getState().recheck();

    expect(replayPayment).not.toHaveBeenCalled();
    // Nothing safe to repeat, and the reason is already known from the code — so the notice stands
    // as it is, without a status call that could only replace it with a wrong explanation.
    expect(getMyStatus).not.toHaveBeenCalled();
    expect(useTrustStore.getState().block?.kind).toBe("relay-misconfigured");
  });

  // Review finding: the notice was pinned by a module-level flag that only clear() reset, so a
  // classification arriving from an in-flight status call could contradict it — the flag said
  // "configuration fault" while the screen said "finish your onboarding", and the next recheck
  // flipped it back. The state is now derived from the notice itself, so the two cannot disagree.
  it("does not let an in-flight onboarding read overwrite a configuration notice", async () => {
    let resolveStatus: (value: OnboardingMyStatusResponse) => void = () => {};
    getMyStatus.mockReturnValue(
      new Promise<OnboardingMyStatusResponse>((resolve) => {
        resolveStatus = resolve;
      }),
    );

    // A trust rejection starts reading the status; the relay fault lands before it answers.
    const pending = useTrustStore.getState().reportRejection("/payments/escrows");
    await useTrustStore.getState().reportConfigurationFault("/payments/deposits");
    expect(useTrustStore.getState().block?.kind).toBe("relay-misconfigured");

    resolveStatus(myStatus("NONE"));
    await pending;

    expect(useTrustStore.getState().block?.kind).toBe("relay-misconfigured");
  });

  // Review finding: with only a write refused there is nothing safe to replay, so "Check again" made
  // no request and changed nothing on screen — a button that cannot work. The store says so instead,
  // and the notice stops offering it.
  it("cannot recheck a configuration fault seen only on a write", async () => {
    const replayPayment = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "post", replayPayment);

    expect(useTrustStore.getState().canRecheck).toBe(false);
  });

  it("can recheck a configuration fault once a read was refused too", async () => {
    const probe = vi.fn().mockResolvedValue({ status: 200 });

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "post", vi.fn());
    await useTrustStore.getState().reportConfigurationFault("/payments/escrows", "get", probe);

    expect(useTrustStore.getState().canRecheck).toBe(true);
  });

  // The button stays for a trust rejection with nothing to replay: there, re-reading the reason is
  // something — onboarding may have advanced, a credential may have been frozen. Only the
  // configuration fault has a reason that cannot change.
  it("can still recheck a trust rejection that has no read to replay", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));

    await useTrustStore.getState().reportRejection("/payments/deposits", "post", vi.fn());

    expect(useTrustStore.getState().canRecheck).toBe(true);
  });

  it("clear restores the ability to recheck", async () => {
    await useTrustStore.getState().reportConfigurationFault("/payments/deposits", "post", vi.fn());
    expect(useTrustStore.getState().canRecheck).toBe(false);

    useTrustStore.getState().clear();

    expect(useTrustStore.getState().canRecheck).toBe(true);
  });

  it("clear resets the configuration fault, so a later trust rejection classifies normally", async () => {
    getMyStatus.mockResolvedValue(myStatus("NONE"));

    await useTrustStore.getState().reportConfigurationFault("/payments/deposits");
    useTrustStore.getState().clear();
    await useTrustStore.getState().reportRejection("/payments/deposits");

    expect(useTrustStore.getState().block?.kind).toBe("onboarding-required");
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
