import { describe, expect, it } from "vitest";
import { calcMaxAmountIn, weiToDisplay } from "../cross-currency-swap.api";

describe("cross-currency-swap utils", () => {
  it("calcMaxAmountIn applies slippage basis points", () => {
    expect(calcMaxAmountIn("1000000000000000000", 1100)).toBe("1100000000000000000");
  });

  it("calcMaxAmountIn uses default slippage when omitted", () => {
    expect(calcMaxAmountIn("1000000000000000000")).toBe("1100000000000000000");
  });

  it("weiToDisplay formats wei with decimals", () => {
    expect(weiToDisplay("1000000000000000000", 6)).toBe("1.000000");
  });

  it("weiToDisplay returns em dash for invalid value", () => {
    expect(weiToDisplay("")).toBe("—");
    expect(weiToDisplay("abc")).toBe("—");
  });
});
