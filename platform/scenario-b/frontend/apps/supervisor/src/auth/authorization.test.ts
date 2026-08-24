// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { hasSupervisorAccess } from "./authorization";
import type { SupervisorUser } from "../types";

// Reported from the Peru environment: the supervisor portal was reachable from an active
// treasury session. Confirmed live on a Scenario B stack — the gateway REJECTS the data
// (403 from /compliance/audit/logs, /compliance/participants/summary and /oversight/network
// for a ROLE_TREASURY session; 200 for ROLE_SUPERVISOR), so this was never an authorization
// hole. It was the portal admitting a session the API refuses, leaving the operator on a
// shell that renders and then fails every request.
//
// The portal made it worse than a missing check: roleFromClaims fell through to
// CENTRAL_BANK_ADMIN for any unrecognised role, and `permissions` is a fixed list, so a
// treasury token came back looking like a legitimate supervisor profile. Authorization
// therefore reads the realm roles the token actually carries, never the derived label.

function user(roles: string[]): SupervisorUser {
  return {
    id: "u1",
    username: "operator",
    roles,
    role: "CENTRAL_BANK_ADMIN",
    institutionId: "cb-brazil",
    institutionName: "Central Bank",
    walletAddress: "0x0",
    permissions: [],
    createdAt: "2026-01-01T00:00:00Z",
  } as SupervisorUser;
}

describe("hasSupervisorAccess", () => {
  it("admits a session carrying ROLE_SUPERVISOR", () => {
    expect(hasSupervisorAccess(user(["ROLE_SUPERVISOR"]))).toBe(true);
  });

  it("refuses a treasury session — the case reported from Peru", () => {
    expect(hasSupervisorAccess(user(["ROLE_TREASURY"]))).toBe(false);
  });

  it("refuses governance and NOC sessions, which the gateway also 403s here", () => {
    expect(hasSupervisorAccess(user(["ROLE_GOVERNANCE"]))).toBe(false);
    expect(hasSupervisorAccess(user(["ROLE_NOC_ADMIN"]))).toBe(false);
  });

  it("admits a session that holds the role among others", () => {
    expect(hasSupervisorAccess(user(["ROLE_TREASURY", "ROLE_SUPERVISOR"]))).toBe(true);
  });

  it("refuses a user with no roles rather than falling through to a default", () => {
    expect(hasSupervisorAccess(user([]))).toBe(false);
  });

  it("refuses when there is no user at all", () => {
    expect(hasSupervisorAccess(null)).toBe(false);
    expect(hasSupervisorAccess(undefined)).toBe(false);
  });
});
