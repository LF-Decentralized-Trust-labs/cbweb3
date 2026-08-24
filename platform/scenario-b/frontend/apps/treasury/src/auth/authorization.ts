// SPDX-License-Identifier: Apache-2.0

import type { TreasuryUser } from "../types";

// The gateway guards the treasury routes on this realm role. The portal must refuse the same
// session the API refuses, or an operator reaches a shell that renders and then fails every
// request — the symptom reported against the supervisor portal, from the same missing check.
export const TREASURY_REQUIRED_ROLE = "ROLE_TREASURY";
export const TREASURY_UNAUTHORIZED_MESSAGE =
  "Your account is not authorized to access the Treasury Portal.";

export function hasTreasuryAccess(
  user: TreasuryUser | null | undefined,
): boolean {
  if (!user) {
    return false;
  }

  return user.roles.includes(TREASURY_REQUIRED_ROLE);
}
