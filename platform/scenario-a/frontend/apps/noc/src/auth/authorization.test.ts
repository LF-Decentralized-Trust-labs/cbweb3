// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { NOC_ALLOWED_ROLES, hasNOCAccess } from "./authorization";
import type { SysAdminUser } from "../types";

// Same class as the supervisor portal defect reported from Peru, but the NOC pair is the case
// where the backend really would have served the data: its portal-read group carried
// RequireAuth alone, so any authenticated token reached the dashboard. Fixed on both sides —
// the backend now requires one of these roles, and the portal refuses the same sessions.
//
// Locally this guard is the only separation there is: NOC backends run with
// NOC_SKIP_AUTH=true, whose no-op Keycloak client hands back all three roles for any token.
function user(roles: string[]): SysAdminUser {
  return {
    id: "u1",
    name: "operator",
    roles,
    role: "SYS_ADMIN",
    institutionId: "cb",
  } as SysAdminUser;
}

describe("hasNOCAccess", () => {
  it("admits every NOC role the deployment issues", () => {
    for (const role of NOC_ALLOWED_ROLES) {
      expect(hasNOCAccess(user([role]))).toBe(true);
    }
  });

  it("refuses roles from the other portals", () => {
    expect(hasNOCAccess(user(["ROLE_TREASURY"]))).toBe(false);
    expect(hasNOCAccess(user(["ROLE_SUPERVISOR"]))).toBe(false);
    expect(hasNOCAccess(user(["ROLE_GOVERNANCE"]))).toBe(false);
  });

  it("refuses a user with no roles rather than falling through to a default", () => {
    expect(hasNOCAccess(user([]))).toBe(false);
  });

  it("refuses when there is no user at all", () => {
    expect(hasNOCAccess(null)).toBe(false);
  });
});
