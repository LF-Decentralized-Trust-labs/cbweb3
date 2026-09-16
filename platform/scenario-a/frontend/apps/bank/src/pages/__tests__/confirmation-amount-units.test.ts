// SPDX-License-Identifier: Apache-2.0

// A base-unit formatter must never be handed the display value a form holds in state.
//
// formatCeBM and formatFiatUnits take a raw base-unit amount and divide by 10^decimals;
// the `amount` a form keeps is the string the operator typed. Passing one to the other
// divides twice, so at 18 decimals a typed 1500 renders as "< 0.01" — which is exactly
// what the confirmation panels in Scenario B announced before PR #217, and what
// Scenario A would now announce too, since ADR-009 gave it the division it never had.
//
// The mistake is invisible in review (both calls read plausibly) and invisible at
// runtime unless someone reads the number, so it is guarded at the source level rather
// than left to a rendering test — this monorepo has no DOM test harness, and adding one
// for this would be a new dependency.
//
// Ported from Scenario B's guard of the same name. The two are deliberate copies; a
// change to one belongs in the other (docs/scenario-drift.md).

import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const pagesDir = fileURLToPath(new URL("..", import.meta.url));

/** Formatters that expect BASE units. Handing them a display string divides twice. */
const BASE_UNIT_FORMATTERS = ["formatCeBM", "formatFiatUnits", "formatBaseUnits"];

function pageSources(): { name: string; source: string }[] {
  return readdirSync(pagesDir)
    .filter((f) => f.endsWith(".tsx"))
    .map((name) => ({ name, source: readFileSync(`${pagesDir}/${name}`, "utf8") }));
}

/**
 * Names a page keeps as DISPLAY state — what the operator typed. Detected from the
 * declaration rather than hardcoded, so a new form is covered the day it is written.
 */
function displayStateNames(source: string): string[] {
  const names: string[] = [];
  const decl = /const \[(\w*[Aa]mount\w*), set\w+\] = useState/g;
  let m: RegExpExecArray | null;
  while ((m = decl.exec(source)) !== null) names.push(m[1]);
  return names;
}

describe("confirmation panels convert before formatting", () => {
  it("never passes a form's display value to a base-unit formatter", () => {
    const offenders: string[] = [];

    for (const { name, source } of pageSources()) {
      const displayNames = displayStateNames(source);
      if (displayNames.length === 0) continue;

      for (const formatter of BASE_UNIT_FORMATTERS) {
        for (const displayName of displayNames) {
          // First argument exactly the display state, e.g. formatCeBM(amount, ...)
          const call = new RegExp(`\\b${formatter}\\(\\s*${displayName}\\s*[,)]`, "g");
          if (call.test(source)) {
            offenders.push(`${name}: ${formatter}(${displayName}) — use the *Display variant`);
          }
        }
      }
    }

    expect(offenders).toEqual([]);
  });

  // The guard is worth nothing if it cannot see the mistake, so this proves it can.
  it("detects the pairing when it is present", () => {
    const source = `
      const [amount, setAmount] = useState("0");
      return <p>{formatCeBM(amount, tCeBMDecimals)}</p>;
    `;
    const names = displayStateNames(source);
    expect(names).toContain("amount");
    const call = new RegExp(`\\bformatCeBM\\(\\s*amount\\s*[,)]`);
    expect(call.test(source)).toBe(true);
  });
});
