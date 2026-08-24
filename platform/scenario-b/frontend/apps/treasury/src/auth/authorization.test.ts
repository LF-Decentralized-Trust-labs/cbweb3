// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { hasTreasuryAccess } from "./authorization";
import type { TreasuryUser } from "../types";

// Same class as the supervisor portal defect reported from Peru: the portal admitted any
// authenticated session, and `role` was the fixed string "TREASURY" rather than anything the
// token said, so every session mapped through looked like a treasury user. Authorization
// reads the realm roles instead.
function user(roles: string[]): TreasuryUser {
  return {
    id: "u1",
    name: "operator",
    roles,
    institutionId: "cb",
    role: "TREASURY",
    walletAddress: "0x0",
    authorizedIssuer: false,
  } as TreasuryUser;
}

describe("hasTreasuryAccess", () => {
  // The portal's routes are guarded by RequireLiquidityProviderRole (central_bank or mlp),
  // so those are what must be admitted. Scenario B gives `central_bank` to its governance
  // operator too, and the API serves them — refusing them here would make the portal
  // stricter than the gateway, which is a defect, not caution.
  it("admits the roles the API admits on the routes this portal calls", () => {
    expect(hasTreasuryAccess(user(["central_bank"]))).toBe(true);
    expect(hasTreasuryAccess(user(["mlp"]))).toBe(true);
  });

  it("admits a governance operator, who holds central_bank and is served by the API", () => {
    expect(hasTreasuryAccess(user(["central_bank", "ROLE_GOVERNANCE"]))).toBe(true);
  });

  it("admits ROLE_TREASURY, for a deployment that issues it without central_bank", () => {
    expect(hasTreasuryAccess(user(["ROLE_TREASURY"]))).toBe(true);
  });

  it("refuses sessions the API refuses on those routes", () => {
    expect(hasTreasuryAccess(user(["ROLE_SUPERVISOR"]))).toBe(false);
    expect(hasTreasuryAccess(user(["ROLE_NOC_ADMIN"]))).toBe(false);
    expect(hasTreasuryAccess(user(["commercial_bank", "ROLE_COMMERCIAL_BANK"]))).toBe(false);
  });

  it("refuses a user with no roles rather than falling through to a default", () => {
    expect(hasTreasuryAccess(user([]))).toBe(false);
  });

  it("refuses when there is no user at all", () => {
    expect(hasTreasuryAccess(null)).toBe(false);
  });
});
