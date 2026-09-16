// SPDX-License-Identifier: Apache-2.0

// A base-unit formatter must never be handed the display value a form holds in state.
//
// formatFiatUnits and formatCeBM take a raw base-unit amount and divide by 10^decimals;
// the `amount` a form keeps is the string the operator typed. Passing one to the other
// divides twice, so at 18 decimals a typed 1500 rendered as "0" — which is what the
// confirmation panels on /deposits, /escrows and /redeems announced while the submit path,
// converting properly with displayToBase, sent the correct value.
//
// The unit behaviour itself is covered by types/__tests__/payment.types.display.test.ts.
// This guards the call sites, so a fourth screen cannot reintroduce the pairing: the
// mistake is invisible in review (both calls read plausibly) and invisible at runtime
// unless someone reads the number.
//
// A source-level guard rather than a rendering test, for the reason
// amm-swap-confirmation.test.ts states: neither monorepo has a DOM test harness, and
// adding one for this would be a new dependency.

import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const pagesDir = fileURLToPath(new URL("..", import.meta.url));

/** Formatters documented as taking a RAW BASE-UNIT amount. */
const BASE_UNIT_FORMATTERS = ["formatFiatUnits", "formatCeBM", "formatTokenAmount"];

function pageSources(): { name: string; source: string }[] {
  return readdirSync(pagesDir)
    .filter((name) => name.endsWith(".tsx"))
    .map((name) => ({ name, source: readFileSync(`${pagesDir}/${name}`, "utf8") }));
}

/** The names a page holds a typed, display-unit amount under. */
function displayStateNames(source: string): string[] {
  const names: string[] = [];
  const re = /const \[(\w+), set\w+\] = useState[^(]*\(\s*"(?:0|)"\s*\)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    // Only the amount-ish ones: a page's unrelated string state is not our business.
    if (/amount/i.test(m[1])) {
      names.push(m[1]);
    }
  }
  return names;
}

describe("confirmation copy uses display-unit formatters", () => {
  it("no page formats its typed amount with a base-unit formatter", () => {
    const offences: string[] = [];

    for (const { name, source } of pageSources()) {
      for (const stateName of displayStateNames(source)) {
        for (const formatter of BASE_UNIT_FORMATTERS) {
          // `formatter(stateName` — the exact shape that divides a display value again.
          const call = new RegExp(`\\b${formatter}\\(\\s*${stateName}\\b`);
          if (call.test(source)) {
            offences.push(`${name}: ${formatter}(${stateName}, …) — use the *Display variant`);
          }
        }
      }
    }

    expect(offences, offences.join("\n")).toEqual([]);
  });

  it("the guard can actually see the shape it forbids", () => {
    // Without this, a broken regex would make the test above vacuously green — the way a
    // fake runner makes an unsatisfiable shell gate look satisfied.
    const fixture = `
      const [amount, setAmount] = useState("0");
      <CardDescription>{formatFiatUnits(amount, fDecimals, fCeBMSymbol)} will be submitted.</CardDescription>
    `;
    const names = displayStateNames(fixture);
    expect(names).toContain("amount");
    expect(new RegExp(`\\bformatFiatUnits\\(\\s*amount\\b`).test(fixture)).toBe(true);
  });

  it("still finds the display state it is meant to inspect", () => {
    // If a page stops matching displayStateNames — a rename, a different useState shape —
    // the first test passes for the wrong reason. Pin the three screens this bug hit.
    for (const page of ["DepositsPage.tsx", "EscrowsPage.tsx", "RedeemsPage.tsx"]) {
      const source = readFileSync(`${pagesDir}/${page}`, "utf8");
      expect(displayStateNames(source), `${page} declares no display amount state`).not.toEqual([]);
    }
  });
});
