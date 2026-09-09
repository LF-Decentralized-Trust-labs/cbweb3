// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { amountRefusalMessage, parseAmount, parseBaseUnits } from "../amount";

// The rule pinned here is ISO 20022: dot decimal separator, no grouping. The
// project chose to make the form accept exactly what the wire accepts rather
// than translate between them, so a comma is refused at the keyboard.
describe("parseAmount", () => {
  describe("accepts the ISO 20022 form", () => {
    it.each([
      ["a whole amount", "1000", "1000", 1000],
      ["a decimal", "1000.10", "1000.10", 1000.1],
      ["a sub-unit amount", "0.5", "0.5", 0.5],
      ["surrounding whitespace", "  1000.10  ", "1000.10", 1000.1],
      ["trailing zeros, kept verbatim", "1000.100", "1000.100", 1000.1],
    ])("%s", (_label, input, canonical, value) => {
      const parsed = parseAmount(input);
      expect(parsed.ok).toBe(true);
      if (!parsed.ok) return;
      expect(parsed.canonical).toBe(canonical);
      expect(parsed.value).toBeCloseTo(value, 10);
    });

    // The canonical string, not the number, is what gets sent. A JS number
    // cannot hold this; the string can, and big.Rat reads it exactly.
    it("keeps precision a JS number would lose", () => {
      const parsed = parseAmount("1000.100000000000000001");
      expect(parsed.ok).toBe(true);
      if (!parsed.ok) return;
      expect(parsed.canonical).toBe("1000.100000000000000001");
    });
  });

  describe("refuses everything else", () => {
    it.each([
      ["an empty field", "", "empty"],
      ["whitespace alone", "   ", "empty"],
      // The comma is a valid decimal marker per CGPM Resolution 10, but not in
      // an ISO 20022 amount. Refusing it at the keyboard is how the operator
      // learns that here rather than at the gateway.
      ["a comma decimal", "1000,10", "separator"],
      ["pt-BR grouping", "1.000,10", "separator"],
      ["en-US grouping", "1,000.10", "separator"],
      ["dot grouping", "1.000.000", "separator"],
      ["a bare grouping comma", "1,000", "separator"],
      ["a space as a thousands separator", "1 000.10", "malformed"],
      ["a currency symbol", "R$ 1000", "malformed"],
      ["scientific notation", "1e3", "malformed"],
      ["a Go underscore separator", "1_000", "malformed"],
      ["a hex literal", "0x10", "malformed"],
      ["a rational fraction", "1/3", "malformed"],
      ["a leading plus", "+5", "malformed"],
      ["letters", "abc", "malformed"],
      ["zero", "0", "not-positive"],
      ["zero with decimals", "0.00", "not-positive"],
      ["a negative amount", "-5", "not-positive"],
      ["a negative decimal", "-5.5", "not-positive"],
    ])("%s", (_label, input, refusal) => {
      const parsed = parseAmount(input);
      expect(parsed.ok).toBe(false);
      if (parsed.ok) return;
      expect(parsed.refusal).toBe(refusal);
    });

    // These four reach the orchestrator's big.Rat as valid numbers — 1_000 is
    // 1000, 0x10 is 16, 1/3 is a third, 1e3 is a thousand. The form is
    // deliberately the stricter layer. If this starts failing because the regex
    // was loosened, a hex literal can become a payment.
    it("is stricter than the orchestrator it feeds", () => {
      for (const backendAccepts of ["1_000", "0x10", "1/3", "1e3"]) {
        expect(parseAmount(backendAccepts).ok).toBe(false);
      }
    });
  });
});

// The FX proposal legs carry base units, which must be whole.
describe("parseBaseUnits", () => {
  it.each(["1000", "5000", "1"])("accepts the whole amount %s", (input) => {
    const parsed = parseBaseUnits(input);
    expect(parsed.ok).toBe(true);
    if (parsed.ok) expect(parsed.canonical).toBe(input);
  });

  it.each(["1000.10", "1000,10", "0.5", "1.000,10", "1,000.10", "1 000"])(
    "refuses %s as not-whole",
    (input) => {
      const parsed = parseBaseUnits(input);
      expect(parsed.ok).toBe(false);
      if (!parsed.ok) expect(parsed.refusal).toBe("not-whole");
    },
  );

  it("never tells the operator to type a decimal into a whole-number field", () => {
    const msg = amountRefusalMessage("Send amount", "not-whole");
    expect(msg).not.toMatch(/1000\.10|1000,10/);
    expect(msg).toContain("digits only");
  });
});

describe("amountRefusalMessage", () => {
  it("states the ISO 20022 rule positively", () => {
    const msg = amountRefusalMessage("Amount", "separator");
    expect(msg).toContain("dot as the decimal separator");
    expect(msg).toContain("1000.10");
  });

  it("names the field and the fix", () => {
    expect(amountRefusalMessage("Amount", "empty")).toBe("Amount is required.");
    expect(amountRefusalMessage("Receive amount", "not-positive")).toBe(
      "Receive amount must be greater than zero.",
    );
  });
});
