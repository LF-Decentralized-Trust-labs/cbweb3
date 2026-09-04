// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { identityBelongsToBank } from "../identity";

// belongsToBankCases is the shared case table. THE SAME TABLE lives in the
// payment-orchestrator's internal/identity tests and in the api-gateway handler
// tests. Three implementations exist because those services are separate Go
// modules and this one is TypeScript. One table in three places is the anti-drift
// mechanism: change it in all three, or a fix in one silently regresses the
// others — which is how this defect spread in the first place.
const cases: ReadonlyArray<[name: string, identity: string, bankId: string, want: boolean]> = [
  // The reported defect. A two-word country puts a hyphen inside the spoke id,
  // which the old "skip two hyphens" rule read as bank `rica-cb1`.
  ["two-word country, full identity", "funded_operator@spoke-costa-rica-cb1", "cb1", true],
  ["two-word country, central bank", "funded_operator@spoke-costa-rica-cb", "cb", true],
  // Single-word countries worked before purely by accident; they must keep working.
  ["single-word country", "funded_operator@spoke-chile-cb3", "cb3", true],
  ["single-word country, peru", "funded_operator@spoke-peru-cb5", "cb5", true],
  // The other live convention: the bank id itself carries a hyphen.
  ["hyphenated bank id", "funded_operator@spoke-brl-bank-itau", "bank-itau", true],
  ["hyphenated bank id, bradesco", "funded_operator@spoke-brl-bank-bradesco", "bank-bradesco", true],
  // Parties are sometimes stored as a bare bank id rather than a full identity.
  ["bare bank id", "cb1", "cb1", true],
  ["bare hyphenated bank id", "bank-itau", "bank-itau", true],

  // Anti-spoofing. These are why the boundary is `-${bankId}` and not a plain
  // endsWith or an includes: a shorter id must not match a longer one.
  ["prefix must not match a longer bank id", "funded_operator@spoke-x-cb11", "cb1", false],
  ["prefix must not match a longer hyphenated id", "funded_operator@spoke-x-bank-abc", "bank", false],
  ["different bank on the same spoke", "funded_operator@spoke-costa-rica-cb1", "cb2", false],
  ["spoke name must not be mistaken for a bank", "funded_operator@spoke-costa-rica-cb1", "rica-cb1", true],

  // Degenerate input fails closed.
  ["empty bank id", "funded_operator@spoke-costa-rica-cb1", "", false],
  ["empty identity", "", "cb1", false],
  ["identity is only the @", "funded_operator@", "cb1", false],
  ["whitespace is trimmed", "  funded_operator@spoke-costa-rica-cb1  ", " cb1 ", true],
];

describe("identityBelongsToBank", () => {
  for (const [name, identity, bankId, want] of cases) {
    it(name, () => {
      expect(identityBelongsToBank(identity, bankId)).toBe(want);
    });
  }
});

// The page-level consequence, stated as the rule the detail page depends on:
// Accept and Reject are counterparty-only, gated on `!isOriginator`. When the
// membership test fails for the originator, the originator is handed the
// counterparty's buttons on its own agreement.
describe("originator detection (the reported symptom)", () => {
  const originator = "funded_operator@spoke-costa-rica-cb1";

  it("recognises the originator on a two-word-country spoke", () => {
    expect(identityBelongsToBank(originator, "cb1")).toBe(true);
  });

  it("does not mistake the counterparty for the originator", () => {
    expect(identityBelongsToBank(originator, "cb2")).toBe(false);
  });
});
