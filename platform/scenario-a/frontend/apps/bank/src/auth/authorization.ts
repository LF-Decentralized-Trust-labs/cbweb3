// SPDX-License-Identifier: Apache-2.0

import type { UserProfile } from "../types";

export const BANK_ALLOWED_ROLES = [
  "ROLE_COMMERCIAL_BANK_OPERATOR",
  "ROLE_COMMERCIAL_BANK",
  "ROLE_GOVERNANCE",
] as const;
export const BANK_UNAUTHORIZED_MESSAGE =
  "Your account is not authorized to access the Bank Portal.";

export function hasBankAccess(
  profile: UserProfile | null | undefined,
): boolean {
  if (!profile) {
    return false;
  }

  return profile.roles.some((role) =>
    BANK_ALLOWED_ROLES.includes(role as (typeof BANK_ALLOWED_ROLES)[number]),
  );
}
