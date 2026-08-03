// SPDX-License-Identifier: Apache-2.0

import axios from "axios";
import type { OnboardingRequestStatus } from "../../types";

/**
 * The code the central bank returns when it cannot verify who is calling.
 *
 * Our gateway signs its internal calls to the central bank with this institution's own key, and the
 * central bank verifies that signature against the participants it has registered. Anything that
 * leaves us out of that set — onboarding never done, credential revoked, wrong key — lands here.
 */
export const RELAY_SIGNATURE_INVALID = "RELAY_SIGNATURE_INVALID";

/**
 * isTrustRejection reports whether error is the central bank refusing our identity.
 *
 * The match is on the code rather than the status: the status is an implementation detail of the
 * gateway and moving it from 401 to 403 should not silently turn this notice off.
 */
export function isTrustRejection(error: unknown): boolean {
  if (!axios.isAxiosError(error)) {
    return false;
  }
  const body = error.response?.data;
  if (typeof body !== "object" || body === null) {
    return false;
  }
  return (body as { code?: unknown }).code === RELAY_SIGNATURE_INVALID;
}

export type TrustBlockKind =
  | "onboarding-required"
  | "onboarding-in-progress"
  | "credential-inactive"
  | "not-recognized"
  | "unknown";

export type TrustBlock = {
  kind: TrustBlockKind;
  title: string;
  description: string;
  /** Whether pointing the operator at the onboarding wizard is a truthful next step. */
  showOnboardingLink: boolean;
};

const inProgressStatuses: ReadonlySet<OnboardingRequestStatus> = new Set([
  "PENDING",
  "CREDENTIAL_REQUESTED",
  "APPROVED",
  "KYC_APPROVED",
]);

const inactiveStatuses: ReadonlySet<OnboardingRequestStatus> = new Set(["FROZEN", "REVOKED", "REJECTED"]);

/**
 * classifyTrustBlock turns the rejection into something the operator can act on.
 *
 * The rejection itself only says "we could not verify you"; it never says why. Missing onboarding is
 * the common cause but not the only one, so the reason comes from this institution's own onboarding
 * status. Claiming "complete onboarding" to an institution that already did would send the operator
 * down a dead end — hence a distinct message once the status is ACTIVE.
 */
export function classifyTrustBlock(status: OnboardingRequestStatus | null): TrustBlock {
  if (status === "NONE") {
    return {
      kind: "onboarding-required",
      title: "Onboarding required",
      description:
        "The central bank does not recognize this institution yet. Complete onboarding to enable issuance, escrow and redemption.",
      showOnboardingLink: true,
    };
  }

  if (status !== null && inProgressStatuses.has(status)) {
    return {
      kind: "onboarding-in-progress",
      title: "Onboarding under review",
      description: `Onboarding is registered as ${status} and the central bank has not activated this institution's credential yet. These operations stay unavailable until it does.`,
      showOnboardingLink: true,
    };
  }

  if (status !== null && inactiveStatuses.has(status)) {
    return {
      kind: "credential-inactive",
      title: "Credential not active",
      description: `The central bank reports this institution's credential as ${status}. Contact the central bank operator to restore it.`,
      showOnboardingLink: false,
    };
  }

  if (status === "ACTIVE") {
    return {
      kind: "not-recognized",
      title: "Institution not recognized by the central bank",
      description:
        "This institution is registered and active, so the rejection is a configuration problem rather than a missing registration: the central bank could not verify our signing identity. Contact the central bank operator.",
      showOnboardingLink: false,
    };
  }

  return {
    kind: "unknown",
    title: "The central bank did not recognize this institution",
    description:
      "The request was rejected and the onboarding status could not be read, so the cause is undetermined. If onboarding was never completed, start it; otherwise contact the central bank operator.",
    showOnboardingLink: true,
  };
}
