// SPDX-License-Identifier: Apache-2.0

import type { SysAdminUser } from "../types";

// The NOC backend guards its portal-read routes on this family of roles, and its admin routes
// on ROLE_NOC_ADMIN. The portal must refuse the same session the backend refuses, or an
// operator reaches a shell that renders and then fails every request.
//
// Worth stating plainly: local stacks run the NOC backend with NOC_SKIP_AUTH=true, whose
// no-op Keycloak client returns all three roles for any token. This guard is therefore the
// only thing that separates the portals locally, and the backend check is what holds in a
// deployment with auth on.
export const NOC_ALLOWED_ROLES = [
  "ROLE_NOC_ADMIN",
  "ROLE_NOC_OPERATOR",
  "ROLE_NOC_VIEWER",
] as const;

export const NOC_UNAUTHORIZED_MESSAGE =
  "Your account is not authorized to access the NOC Portal.";

export function hasNOCAccess(user: SysAdminUser | null | undefined): boolean {
  if (!user) {
    return false;
  }

  return user.roles.some((role) =>
    NOC_ALLOWED_ROLES.includes(role as (typeof NOC_ALLOWED_ROLES)[number]),
  );
}
