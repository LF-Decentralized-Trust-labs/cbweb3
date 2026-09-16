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
 * The other ways the central bank says "I cannot tell that this is you".
 *
 * Matching only RELAY_SIGNATURE_INVALID left siblings escaping into the session branch, where a 401
 * means an expired token: the interceptor refreshed, retried, failed again and logged the operator
 * out — the ejection this notice was added to replace. RELAY_SIGNATURE_REQUIRED is the reachable one
 * (an unsigned request against a central bank that enforces signatures, e.g. ours failed to load its
 * signing key and fell back to the shared secret), and the caller-identity codes are the same class:
 * the request authenticated, but not as anyone the central bank can act for.
 */
export const RELAY_SIGNATURE_REQUIRED = "RELAY_SIGNATURE_REQUIRED";
export const RELAY_CALLER_IDENTITY_REQUIRED = "RELAY_CALLER_IDENTITY_REQUIRED";
export const RELAY_CALLER_BANK_MISMATCH = "RELAY_CALLER_BANK_MISMATCH";
export const REQUESTER_NOT_A_PARTICIPANT = "REQUESTER_NOT_A_PARTICIPANT";

/**
 * The central bank now accepts each relay signature once, so a request it has already seen is
 * refused with this code. Our gateway signs every call afresh and should never produce it — which is
 * exactly why it belongs here: an unexpected 401 that nothing classifies is the one that reaches the
 * session branch and ejects the operator.
 */
export const RELAY_SIGNATURE_REPLAYED = "RELAY_SIGNATURE_REPLAYED";

/**
 * The relay credential itself is missing, wrong, or absent from the server.
 *
 * A third condition, and the reason it is not folded into the set above. Our gateway authenticates
 * itself to the central bank's with a relay signature or, during the migration, the shared
 * INTERNAL_RELAY_AUTH_SECRET. When that credential is absent or divergent the request never
 * authenticates at all: the central bank is not refusing an identity it dislikes, it never learned
 * one. Nothing about this institution's registration is at fault, so the onboarding wizard — the
 * remedy a trust rejection points at — would be a dead end, and the operator cannot fix it from the
 * portal in any case. It is a deployment fault, and it says so.
 *
 * RELAY_AUTH_NOT_CONFIGURED arrives as a 503 rather than a 401. It is classified here anyway: the
 * cause and the remedy are identical, and an unclassified 503 surfaces as a raw transport error.
 */
export const RELAY_AUTH_REQUIRED = "RELAY_AUTH_REQUIRED";
export const RELAY_AUTH_INVALID = "RELAY_AUTH_INVALID";
export const RELAY_AUTH_NOT_CONFIGURED = "RELAY_AUTH_NOT_CONFIGURED";

const trustRejectionCodes: ReadonlySet<string> = new Set([
  RELAY_SIGNATURE_INVALID,
  RELAY_SIGNATURE_REQUIRED,
  RELAY_SIGNATURE_REPLAYED,
  RELAY_CALLER_IDENTITY_REQUIRED,
  RELAY_CALLER_BANK_MISMATCH,
  REQUESTER_NOT_A_PARTICIPANT,
]);

/**
 * isTrustRejection reports whether error is the central bank refusing our identity.
 *
 * The match is on the code rather than the status: the status is an implementation detail of the
 * gateway and moving it from 401 to 403 should not silently turn this notice off.
 */
export function isTrustRejection(error: unknown): boolean {
  const code = codeOf(error);
  return code !== null && trustRejectionCodes.has(code);
}

const relayConfigurationFaultCodes: ReadonlySet<string> = new Set([
  RELAY_AUTH_REQUIRED,
  RELAY_AUTH_INVALID,
  RELAY_AUTH_NOT_CONFIGURED,
]);

/**
 * isRelayConfigurationFault reports whether the relay credential, not our identity, is the problem.
 *
 * Kept disjoint from isTrustRejection on purpose: the two sets must never overlap, because the
 * notices they raise send the operator to different places. The match is on the code rather than the
 * status for the same reason as there, and because one of these three is not a 401 at all.
 */
export function isRelayConfigurationFault(error: unknown): boolean {
  const code = codeOf(error);
  return code !== null && relayConfigurationFaultCodes.has(code);
}

/** codeOf returns the error code the gateway named, or null when the response carries none. */
function codeOf(error: unknown): string | null {
  if (!axios.isAxiosError(error)) {
    return null;
  }
  const body = error.response?.data;
  if (typeof body !== "object" || body === null) {
    return null;
  }
  const code = (body as { code?: unknown }).code;
  return typeof code === "string" ? code : null;
}

export type TrustBlockKind =
  | "onboarding-required"
  | "onboarding-in-progress"
  | "credential-inactive"
  | "not-recognized"
  | "relay-misconfigured"
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

/**
 * relayConfigurationBlock is the third presentation: the channel is misconfigured, not this
 * institution.
 *
 * It takes no onboarding status because none is relevant. classifyTrustBlock asks that question to
 * name the cause of a trust rejection, and asking it here would produce a confident wrong answer —
 * an ACTIVE bank told it is "not recognized", a bank mid-onboarding told to go and finish it, when
 * in both cases what failed is a credential shared between two gateways. The cause is already known
 * from the code, so the notice states it and names who can act: not the operator reading it.
 */
export function relayConfigurationBlock(): TrustBlock {
  return {
    kind: "relay-misconfigured",
    title: "This portal cannot authenticate to the central bank",
    description:
      "The credential this institution's gateway uses to identify itself to the central bank was rejected or is not configured. This is a deployment setting rather than anything about this institution's registration, and it cannot be resolved from the portal. Contact the platform operator.",
    showOnboardingLink: false,
  };
}
