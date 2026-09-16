// SPDX-License-Identifier: Apache-2.0

import type { UserProfile } from "../types";

export const GOVERNANCE_REQUIRED_ROLE = "ROLE_GOVERNANCE";
/**
 * Admission (spec 042) owns the mutating commercial-bank onboarding actions:
 * approving KYC and registering the participant record. It does NOT authorize any
 * central-bank key operation — certificate issuance and on-chain signing stay with
 * governance — so it must not unlock the rest of the portal.
 */
export const ADMISSION_REQUIRED_ROLE = "ROLE_ADMISSION";
export const GOVERNANCE_UNAUTHORIZED_MESSAGE = "Your account is not authorized to access the Governance Portal.";

export function hasGovernanceAccess(profile: UserProfile | null | undefined): boolean {
  if (!profile) {
    return false;
  }

  return profile.roles.includes(GOVERNANCE_REQUIRED_ROLE);
}

/** Gates the mutating onboarding controls (approve KYC, register participant). */
export function hasAdmissionAccess(profile: UserProfile | null | undefined): boolean {
  if (!profile) {
    return false;
  }

  return profile.roles.includes(ADMISSION_REQUIRED_ROLE);
}

/**
 * May this profile open the portal at all? Either profile may — an Admission-only
 * operator has to be able to sign in, or it cannot do its job (FR-016).
 *
 * This is deliberately NOT the same predicate as page access: widening the single
 * portal gate without per-route requirements would expose every governance page to an
 * Admission-only operator. Per-page requirements live in routes/index.tsx and are
 * enforced by ProtectedRoute.
 */
export function hasPortalAccess(profile: UserProfile | null | undefined): boolean {
  return hasGovernanceAccess(profile) || hasAdmissionAccess(profile);
}

/** Which roles may open a given route. */
export type RouteRequirement = readonly string[];

/**
 * Default requirement. A page added without an explicit requirement stays closed to
 * the Admission profile rather than silently opening to it (FR-008 — the frontend
 * counterpart of the router hazard where relaxing a group widens everything inside).
 */
export const GOVERNANCE_ONLY: RouteRequirement = [GOVERNANCE_REQUIRED_ROLE];

/** The shared onboarding surface: reachable by either profile (FR-003a). */
export const GOVERNANCE_OR_ADMISSION: RouteRequirement = [
  GOVERNANCE_REQUIRED_ROLE,
  ADMISSION_REQUIRED_ROLE,
];

/** Does this profile satisfy a route's requirement? */
export function isAllowedOnRoute(
  profile: UserProfile | null | undefined,
  required: RouteRequirement = GOVERNANCE_ONLY,
): boolean {
  if (!profile) {
    return false;
  }
  return required.some((role) => profile.roles.includes(role));
}
