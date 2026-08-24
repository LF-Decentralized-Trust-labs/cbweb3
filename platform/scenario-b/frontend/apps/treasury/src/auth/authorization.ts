// SPDX-License-Identifier: Apache-2.0

import type { TreasuryUser } from "../types";

// These are the roles the API admits on the routes this portal calls, not a guess at who
// "treasury" means. The liquidity routes the portal is built around — deposit-side, finalize,
// positions — are guarded by RequireLiquidityProviderRole, which accepts central_bank or mlp.
//
// ROLE_TREASURY alone would be WRONG here, and stricter than the gateway: Scenario B issues
// `central_bank` to both its governance and treasury operators, so a governance session the
// API happily serves would be turned away at the door. A portal guard that refuses someone
// the API accepts is not a safer guard, it is a broken one. ROLE_TREASURY stays in the list
// because a deployment may issue it without central_bank.
export const TREASURY_ALLOWED_ROLES = [
  "central_bank",
  "mlp",
  "ROLE_TREASURY",
] as const;

export const TREASURY_UNAUTHORIZED_MESSAGE =
  "Your account is not authorized to access the Treasury Portal.";

export function hasTreasuryAccess(
  user: TreasuryUser | null | undefined,
): boolean {
  if (!user) {
    return false;
  }

  return user.roles.some((role) =>
    TREASURY_ALLOWED_ROLES.includes(role as (typeof TREASURY_ALLOWED_ROLES)[number]),
  );
}
