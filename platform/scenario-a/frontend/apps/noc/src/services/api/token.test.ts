// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { REFRESH_SKEW_SECONDS, decodeJwtExp, decodeJwtPayload, isDueForRefresh } from "./token";

/** Builds an unsigned JWT-shaped string carrying the given payload. */
function jwt(payload: Record<string, unknown>): string {
  const body = btoa(JSON.stringify(payload)).replace(/\+/g, "-").replace(/\//g, "_");
  return `header.${body}.signature`;
}

describe("decodeJwtPayload", () => {
  it("reads the claims of a well-formed token", () => {
    const payload = decodeJwtPayload(jwt({ sub: "abc", preferred_username: "admin@brasil.noc.gov" }));
    expect(payload["preferred_username"]).toBe("admin@brasil.noc.gov");
  });

  it("returns an empty object for garbage instead of throwing", () => {
    expect(decodeJwtPayload("not-a-token")).toEqual({});
    expect(decodeJwtPayload("")).toEqual({});
    expect(decodeJwtPayload("header..signature")).toEqual({});
  });
});

describe("decodeJwtExp", () => {
  it("extracts a numeric exp", () => {
    expect(decodeJwtExp(jwt({ exp: 1_800_000_000 }))).toBe(1_800_000_000);
  });

  it("returns null when exp is absent or not numeric", () => {
    expect(decodeJwtExp(jwt({ sub: "abc" }))).toBeNull();
    expect(decodeJwtExp(jwt({ exp: "later" }))).toBeNull();
    expect(decodeJwtExp("not-a-token")).toBeNull();
  });
});

describe("isDueForRefresh", () => {
  const now = 1_800_000_000_000; // ms

  it("is false while the token is comfortably valid", () => {
    const exp = now / 1000 + REFRESH_SKEW_SECONDS + 30;
    expect(isDueForRefresh(exp, now)).toBe(false);
  });

  it("is true inside the skew window, before actual expiry", () => {
    const exp = now / 1000 + REFRESH_SKEW_SECONDS - 1;
    expect(isDueForRefresh(exp, now)).toBe(true);
    // ...but with zero skew the same token still counts as usable
    expect(isDueForRefresh(exp, now, 0)).toBe(false);
  });

  it("is true once expired", () => {
    expect(isDueForRefresh(now / 1000 - 1, now, 0)).toBe(true);
  });

  it("treats an unreadable exp as due, so a malformed token is renewed not used", () => {
    expect(isDueForRefresh(null, now)).toBe(true);
  });
});
