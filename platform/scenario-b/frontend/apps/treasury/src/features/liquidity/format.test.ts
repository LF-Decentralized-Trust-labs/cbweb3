import { describe, expect, it } from "vitest";
import { classifyPoolSides, poolPairCurrencies, poolSideInfo, sideRoleLabel } from "./format";

describe("poolPairCurrencies", () => {
  it("parses a wrapped pair id", () => {
    expect(poolPairCurrencies("W-BRL-ARS")).toEqual({ a: "BRL", b: "ARS" });
  });

  it("parses an unwrapped pair id", () => {
    expect(poolPairCurrencies("BRL-ARS")).toEqual({ a: "BRL", b: "ARS" });
  });

  it("is case-insensitive for the wrapped prefix", () => {
    expect(poolPairCurrencies("w-EUR-USD")).toEqual({ a: "EUR", b: "USD" });
  });

  it("falls back to A/B placeholders when unparseable", () => {
    expect(poolPairCurrencies("")).toEqual({ a: "A", b: "B" });
    expect(poolPairCurrencies(null)).toEqual({ a: "A", b: "B" });
    expect(poolPairCurrencies("W-BRL")).toEqual({ a: "A", b: "B" });
  });
});

describe("classifyPoolSides", () => {
  it("marks the matching side national and the other foreign (CB-A view)", () => {
    expect(classifyPoolSides("W-BRL-ARS", "BRL")).toEqual({
      a: { currency: "BRL", role: "national" },
      b: { currency: "ARS", role: "foreign" },
    });
  });

  it("is view-dependent: the same pool flips for CB-B", () => {
    expect(classifyPoolSides("W-BRL-ARS", "ARS")).toEqual({
      a: { currency: "BRL", role: "foreign" },
      b: { currency: "ARS", role: "national" },
    });
  });

  it("matches the national currency case-insensitively", () => {
    expect(classifyPoolSides("W-brl-ars", "BRL").a.role).toBe("national");
  });

  it("leaves roles null when the national currency is unknown", () => {
    expect(classifyPoolSides("W-BRL-ARS", "")).toEqual({
      a: { currency: "BRL", role: null },
      b: { currency: "ARS", role: null },
    });
  });

  it("treats both sides as foreign for a pool the CB is not part of", () => {
    const sides = classifyPoolSides("W-EUR-USD", "BRL");
    expect(sides.a.role).toBe("foreign");
    expect(sides.b.role).toBe("foreign");
  });
});

describe("poolSideInfo", () => {
  it("resolves a side letter to its classification", () => {
    expect(poolSideInfo("A", "W-BRL-ARS", "BRL")).toEqual({ currency: "BRL", role: "national" });
    expect(poolSideInfo("b", "W-BRL-ARS", "BRL")).toEqual({ currency: "ARS", role: "foreign" });
  });

  it("returns null for an unknown side", () => {
    expect(poolSideInfo("X", "W-BRL-ARS", "BRL")).toBeNull();
    expect(poolSideInfo(null, "W-BRL-ARS", "BRL")).toBeNull();
  });
});

describe("sideRoleLabel", () => {
  const national = { currency: "BRL", role: "national" as const };
  const foreign = { currency: "ARS", role: "foreign" as const };
  const unknown = { currency: "BRL", role: null };

  it("renders full role labels", () => {
    expect(sideRoleLabel(national)).toBe("National Currency (BRL)");
    expect(sideRoleLabel(foreign)).toBe("Foreign Currency (ARS)");
  });

  it("renders short role labels", () => {
    expect(sideRoleLabel(national, { short: true })).toBe("National (BRL)");
    expect(sideRoleLabel(foreign, { short: true })).toBe("Foreign (ARS)");
  });

  it("uses the fallback label when the role is unknown", () => {
    expect(sideRoleLabel(unknown, { fallbackLabel: "Token A" })).toBe("Token A (BRL)");
  });

  it("shows just the currency when role is unknown and no fallback is given", () => {
    expect(sideRoleLabel(unknown)).toBe("BRL");
  });

  it("returns empty string for null info", () => {
    expect(sideRoleLabel(null)).toBe("");
  });
});
