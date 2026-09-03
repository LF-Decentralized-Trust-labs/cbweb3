// SPDX-License-Identifier: Apache-2.0

import type { UserProfile } from "../types";

export const TREASURY_REQUIRED_ROLE = "ROLE_TREASURY";
export const TREASURY_UNAUTHORIZED_MESSAGE = "Your account is not authorized to access the Treasury Portal.";

export function hasTreasuryAccess(profile: UserProfile | null | undefined): boolean {
  if (!profile) {
    return false;
  }

  return profile.roles.includes(TREASURY_REQUIRED_ROLE);
}
