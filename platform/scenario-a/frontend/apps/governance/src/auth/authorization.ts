// SPDX-License-Identifier: Apache-2.0

import type { UserProfile } from "../types";

export const GOVERNANCE_REQUIRED_ROLE = "ROLE_GOVERNANCE";
export const GOVERNANCE_UNAUTHORIZED_MESSAGE = "Your account is not authorized to access the Governance Portal.";

export function hasGovernanceAccess(profile: UserProfile | null | undefined): boolean {
  if (!profile) {
    return false;
  }

  return profile.roles.includes(GOVERNANCE_REQUIRED_ROLE);
}
