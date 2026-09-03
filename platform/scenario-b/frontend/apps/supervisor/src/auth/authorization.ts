// SPDX-License-Identifier: Apache-2.0

import type { SupervisorUser } from "../types";

// The gateway guards every supervisor route with RequireSupervisorRole, which is an exact
// match on this realm role — verified live: a ROLE_TREASURY session gets 403 from
// /compliance/audit/logs, /compliance/participants/summary and /oversight/network, while a
// ROLE_SUPERVISOR session gets 200. The portal must refuse the same session the API refuses,
// or an operator reaches a shell that renders and then fails every request.
export const SUPERVISOR_REQUIRED_ROLE = "ROLE_SUPERVISOR";
export const SUPERVISOR_UNAUTHORIZED_MESSAGE =
  "Your account is not authorized to access the Supervisor Portal.";

export function hasSupervisorAccess(
  user: SupervisorUser | null | undefined,
): boolean {
  if (!user) {
    return false;
  }

  return user.roles.includes(SUPERVISOR_REQUIRED_ROLE);
}
