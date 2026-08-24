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
  it("admits a session carrying ROLE_TREASURY", () => {
    expect(hasTreasuryAccess(user(["ROLE_TREASURY"]))).toBe(true);
  });

  it("refuses supervisor, governance and bank sessions", () => {
    expect(hasTreasuryAccess(user(["ROLE_SUPERVISOR"]))).toBe(false);
    expect(hasTreasuryAccess(user(["ROLE_GOVERNANCE"]))).toBe(false);
    expect(hasTreasuryAccess(user(["ROLE_COMMERCIAL_BANK"]))).toBe(false);
  });

  it("refuses a user with no roles rather than falling through to a default", () => {
    expect(hasTreasuryAccess(user([]))).toBe(false);
  });

  it("refuses when there is no user at all", () => {
    expect(hasTreasuryAccess(null)).toBe(false);
  });
});
