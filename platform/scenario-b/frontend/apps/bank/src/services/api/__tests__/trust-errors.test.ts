// SPDX-License-Identifier: Apache-2.0

import { AxiosError, AxiosHeaders } from "axios";
import { describe, expect, it } from "vitest";
import {
  RELAY_AUTH_INVALID,
  RELAY_AUTH_NOT_CONFIGURED,
  RELAY_AUTH_REQUIRED,
  RELAY_CALLER_BANK_MISMATCH,
  RELAY_CALLER_IDENTITY_REQUIRED,
  RELAY_SIGNATURE_INVALID,
  RELAY_SIGNATURE_REPLAYED,
  RELAY_SIGNATURE_REQUIRED,
  REQUESTER_NOT_A_PARTICIPANT,
  classifyTrustBlock,
  isRelayConfigurationFault,
  isTrustRejection,
  relayConfigurationBlock,
} from "../trust-errors";

const axiosErrorWith = (status: number, data: unknown) => {
  const headers = new AxiosHeaders();
  const config = { headers };
  return new AxiosError("request failed", "ERR_BAD_REQUEST", config, undefined, {
    status,
    statusText: "",
    data,
    headers,
    config,
  });
};

describe("isTrustRejection", () => {
  it("recognizes the central bank rejecting our identity", () => {
    const error = axiosErrorWith(401, { code: RELAY_SIGNATURE_INVALID, error: "relay signature verification failed" });
    expect(isTrustRejection(error)).toBe(true);
  });

  it("matches on the code alone, so a future status change does not silently disable the notice", () => {
    expect(isTrustRejection(axiosErrorWith(403, { code: RELAY_SIGNATURE_INVALID }))).toBe(true);
  });

  // These are the siblings that used to escape into the session branch: the interceptor read the 401
  // as an expired token, refreshed, retried, failed and logged the operator out — the ejection this
  // classifier exists to prevent.
  it.each([
    RELAY_SIGNATURE_REQUIRED,
    RELAY_SIGNATURE_REPLAYED,
    RELAY_CALLER_IDENTITY_REQUIRED,
    RELAY_CALLER_BANK_MISMATCH,
    REQUESTER_NOT_A_PARTICIPANT,
  ])("recognizes %s as a trust rejection, not an expired session", (code) => {
    expect(isTrustRejection(axiosErrorWith(401, { code }))).toBe(true);
  });

  it("ignores an ordinary expired session", () => {
    expect(isTrustRejection(axiosErrorWith(401, { error: "invalid or expired token" }))).toBe(false);
  });

  it("ignores non-axios values", () => {
    expect(isTrustRejection(new Error("boom"))).toBe(false);
    expect(isTrustRejection(null)).toBe(false);
    expect(isTrustRejection(undefined)).toBe(false);
  });

  it("tolerates a response without a body", () => {
    expect(isTrustRejection(axiosErrorWith(401, undefined))).toBe(false);
    expect(isTrustRejection(axiosErrorWith(401, "plain text"))).toBe(false);
  });
});

describe("classifyTrustBlock", () => {
  it("asks for onboarding when none was ever started", () => {
    const block = classifyTrustBlock("NONE");
    expect(block.kind).toBe("onboarding-required");
    expect(block.showOnboardingLink).toBe(true);
  });

  it("reports a review in progress for every intermediate status", () => {
    for (const status of ["PENDING", "CREDENTIAL_REQUESTED", "APPROVED", "KYC_APPROVED"] as const) {
      const block = classifyTrustBlock(status);
      expect(block.kind, status).toBe("onboarding-in-progress");
      expect(block.showOnboardingLink, status).toBe(true);
    }
  });

  it("names the status when the credential is not usable", () => {
    for (const status of ["FROZEN", "REVOKED", "REJECTED"] as const) {
      const block = classifyTrustBlock(status);
      expect(block.kind, status).toBe("credential-inactive");
      expect(block.description, status).toContain(status);
      // Onboarding is already spent for these; sending the operator back to the wizard would not help.
      expect(block.showOnboardingLink, status).toBe(false);
    }
  });

  it("does not blame onboarding once it is complete", () => {
    const block = classifyTrustBlock("ACTIVE");
    expect(block.kind).toBe("not-recognized");
    expect(block.showOnboardingLink).toBe(false);
    expect(block.description.toLowerCase()).not.toContain("complete onboarding");
  });

  it("stays honest when the status could not be read", () => {
    const block = classifyTrustBlock(null);
    expect(block.kind).toBe("unknown");
    // Neither cause is asserted, so the link is offered as a possibility rather than a diagnosis.
    expect(block.showOnboardingLink).toBe(true);
  });
});

describe("isRelayConfigurationFault", () => {
  // The third class. The relay credential that authenticates this gateway to the central bank is
  // absent, divergent or unconfigured: the request never authenticated at all, so it is neither the
  // central bank refusing a known identity nor a session that expired.
  it.each([RELAY_AUTH_REQUIRED, RELAY_AUTH_INVALID, RELAY_AUTH_NOT_CONFIGURED])(
    "recognizes %s as a configuration fault",
    (code) => {
      expect(isRelayConfigurationFault(axiosErrorWith(401, { code }))).toBe(true);
    },
  );

  // RELAY_AUTH_NOT_CONFIGURED arrives as a 503, not a 401, and must still raise the notice rather
  // than surfacing as a raw error — the same reason the trust classifier matches on the code alone.
  it("matches on the code rather than the status", () => {
    expect(isRelayConfigurationFault(axiosErrorWith(503, { code: RELAY_AUTH_NOT_CONFIGURED }))).toBe(true);
  });

  // The load-bearing separation. Presenting a configuration fault as a trust rejection would tell an
  // operator whose institution is perfectly registered to go and finish onboarding.
  it.each([RELAY_AUTH_REQUIRED, RELAY_AUTH_INVALID, RELAY_AUTH_NOT_CONFIGURED])(
    "does not classify %s as a trust rejection",
    (code) => {
      expect(isTrustRejection(axiosErrorWith(401, { code }))).toBe(false);
    },
  );

  it.each([RELAY_SIGNATURE_INVALID, RELAY_CALLER_IDENTITY_REQUIRED, REQUESTER_NOT_A_PARTICIPANT])(
    "does not claim %s, which is a trust rejection",
    (code) => {
      expect(isRelayConfigurationFault(axiosErrorWith(401, { code }))).toBe(false);
    },
  );

  it("ignores an ordinary expired session", () => {
    expect(isRelayConfigurationFault(axiosErrorWith(401, { error: "invalid or expired token" }))).toBe(false);
  });

  it("ignores non-axios values and bodyless responses", () => {
    expect(isRelayConfigurationFault(new Error("boom"))).toBe(false);
    expect(isRelayConfigurationFault(null)).toBe(false);
    expect(isRelayConfigurationFault(axiosErrorWith(401, undefined))).toBe(false);
  });
});

describe("relayConfigurationBlock", () => {
  it("names a configuration fault and does not send the operator to onboarding", () => {
    const block = relayConfigurationBlock();

    expect(block.kind).toBe("relay-misconfigured");
    // Onboarding is not the remedy and may well be complete; offering it would be a dead end.
    expect(block.showOnboardingLink).toBe(false);
    expect(block.description.toLowerCase()).not.toContain("onboarding");
  });

  it("does not depend on the onboarding status", () => {
    // Unlike classifyTrustBlock, the cause is known from the code alone: the reason is on the wire,
    // not in this institution's registration, so there is nothing to look up and nothing to guess.
    expect(relayConfigurationBlock()).toEqual(relayConfigurationBlock());
  });
});
