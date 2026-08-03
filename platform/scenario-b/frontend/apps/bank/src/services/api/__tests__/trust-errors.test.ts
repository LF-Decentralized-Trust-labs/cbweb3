// SPDX-License-Identifier: Apache-2.0

import { AxiosError, AxiosHeaders } from "axios";
import { describe, expect, it } from "vitest";
import { RELAY_SIGNATURE_INVALID, classifyTrustBlock, isTrustRejection } from "../trust-errors";

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
