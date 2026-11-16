// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  centralBankOfSpoke,
  counterpartyOptions,
  ownIdentity,
  spokeMembers,
  spokeOfIdentity,
} from "../proposal-parties";
import { spokeIdsFromRoster } from "../spoke-roster";

// The proposal form used to ask for eight party identities and two spoke ids, each from the
// full network roster. Most of those are fixed by who is proposing and to whom, so the form now
// derives them. These tests pin the derivations against both live bank-id conventions, because a
// node name cannot be split into spoke and bank (see spoke-roster.ts): LNET uses `cb1`…, the
// samples use `bank-itau`.

const LNET_ROSTER = [
  "funded_operator@spoke-chile-cb",
  "funded_operator@spoke-chile-cb3",
  "funded_operator@spoke-chile-cb4",
  "funded_operator@spoke-costa-rica-cb",
  "funded_operator@spoke-costa-rica-cb1",
  "funded_operator@spoke-costa-rica-cb2",
  "funded_operator@spoke-peru-cb",
  "funded_operator@spoke-peru-cb5",
];

const SAMPLES_ROSTER = [
  "funded_operator@spoke-brl-cb",
  "funded_operator@spoke-brl-bank-itau",
  "funded_operator@spoke-brl-bank-bradesco",
  "funded_operator@spoke-costa-rica-cb",
  "funded_operator@spoke-costa-rica-cb1",
  "funded_operator@spoke-costa-rica-cb2",
];

const lnetSpokes = spokeIdsFromRoster(LNET_ROSTER);
const sampleSpokes = spokeIdsFromRoster(SAMPLES_ROSTER);

describe("spokeOfIdentity", () => {
  it("finds a commercial bank's spoke under both conventions", () => {
    expect(spokeOfIdentity("funded_operator@spoke-costa-rica-cb1", lnetSpokes)).toBe("spoke-costa-rica");
    expect(spokeOfIdentity("funded_operator@spoke-brl-bank-itau", sampleSpokes)).toBe("spoke-brl");
  });

  it("finds a central bank's spoke", () => {
    expect(spokeOfIdentity("funded_operator@spoke-chile-cb", lnetSpokes)).toBe("spoke-chile");
  });

  it("returns null for a node on no known spoke", () => {
    expect(spokeOfIdentity("funded_operator@spoke-mx-bank-a", lnetSpokes)).toBeNull();
  });

  // `spoke-a-b-x` could be bank `b-x` on spoke-a or bank `x` on spoke-a-b. Guessing is what
  // produced the unroutable `spoke-costa`, so an ambiguous node gets no answer.
  it("refuses to guess when two spoke ids both prefix the node", () => {
    expect(spokeOfIdentity("funded_operator@spoke-a-b-x", ["spoke-a", "spoke-a-b"])).toBeNull();
  });

  it("does not match a spoke id that is only a substring prefix", () => {
    // spoke-peru must not claim a node on a hypothetical spoke-peruvia.
    expect(spokeOfIdentity("funded_operator@spoke-peruvia-cb1", ["spoke-peru"])).toBeNull();
  });
});

describe("ownIdentity", () => {
  it("resolves the logged-in bank's identity under both conventions", () => {
    expect(ownIdentity(LNET_ROSTER, "cb1")).toBe("funded_operator@spoke-costa-rica-cb1");
    expect(ownIdentity(SAMPLES_ROSTER, "bank-itau")).toBe("funded_operator@spoke-brl-bank-itau");
  });

  // `cb1` must not resolve to `...-cb11`, and an empty bank id resolves to nothing.
  it("does not match by substring, and needs a bank id", () => {
    expect(ownIdentity(["funded_operator@spoke-x-cb11"], "cb1")).toBeNull();
    expect(ownIdentity(SAMPLES_ROSTER, "")).toBeNull();
  });

  it("returns null when the bank id matches more than one identity", () => {
    expect(ownIdentity(["funded_operator@spoke-a-cb1", "funded_operator@spoke-b-cb1"], "cb1")).toBeNull();
  });
});

describe("centralBankOfSpoke", () => {
  it("returns the spoke's `<spokeId>-cb` identity", () => {
    expect(centralBankOfSpoke("spoke-brl", SAMPLES_ROSTER)).toBe("funded_operator@spoke-brl-cb");
    expect(centralBankOfSpoke("spoke-costa-rica", LNET_ROSTER)).toBe("funded_operator@spoke-costa-rica-cb");
  });

  it("returns null for an unknown or missing spoke", () => {
    expect(centralBankOfSpoke("spoke-mx", SAMPLES_ROSTER)).toBeNull();
    expect(centralBankOfSpoke(null, SAMPLES_ROSTER)).toBeNull();
  });
});

describe("counterpartyOptions", () => {
  // The backend refuses a trade whose source and destination spoke are the same, so a bank on
  // the proposer's own spoke is never a valid counterparty. Central banks are not offered: a
  // counterparty is the commercial bank that accepts the trade.
  it("offers only commercial banks on other spokes", () => {
    expect(counterpartyOptions(SAMPLES_ROSTER, sampleSpokes, "spoke-brl")).toEqual([
      "funded_operator@spoke-costa-rica-cb1",
      "funded_operator@spoke-costa-rica-cb2",
    ]);
  });

  it("offers every commercial bank with a known spoke when the own spoke is unknown", () => {
    expect(counterpartyOptions(SAMPLES_ROSTER, sampleSpokes, null)).toEqual([
      "funded_operator@spoke-brl-bank-bradesco",
      "funded_operator@spoke-brl-bank-itau",
      "funded_operator@spoke-costa-rica-cb1",
      "funded_operator@spoke-costa-rica-cb2",
    ]);
  });
});

describe("spokeMembers", () => {
  it("lists every identity on a spoke, central bank included", () => {
    expect(spokeMembers(SAMPLES_ROSTER, sampleSpokes, "spoke-costa-rica")).toEqual([
      "funded_operator@spoke-costa-rica-cb",
      "funded_operator@spoke-costa-rica-cb1",
      "funded_operator@spoke-costa-rica-cb2",
    ]);
  });

  // The source receiver is the party paid on the proposer's own spoke; paying yourself is not
  // a leg, so the proposer is excluded when asked.
  it("can exclude the proposer", () => {
    expect(
      spokeMembers(SAMPLES_ROSTER, sampleSpokes, "spoke-brl", "funded_operator@spoke-brl-bank-itau"),
    ).toEqual(["funded_operator@spoke-brl-bank-bradesco", "funded_operator@spoke-brl-cb"]);
  });

  it("returns nothing for a missing spoke", () => {
    expect(spokeMembers(SAMPLES_ROSTER, sampleSpokes, null)).toEqual([]);
  });
});
