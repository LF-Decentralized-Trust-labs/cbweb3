// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  nodeFromIdentity,
  spokeIdFromCentralBankIdentity,
  spokeIdsFromRoster,
} from "../spoke-roster";

// The roster shapes that matter are the two live bank-id conventions. Any fix
// that splits the node name on "-" gets one of them right and the other wrong,
// so both are pinned here — a regression in either direction fails a test
// instead of shipping an unroutable spoke id to an operator.
const LNET_ROSTER = [
  "funded_operator@spoke-chile-cb",
  "funded_operator@spoke-chile-cb3",
  "funded_operator@spoke-chile-cb4",
  "funded_operator@spoke-costa-rica-cb",
  "funded_operator@spoke-costa-rica-cb1",
  "funded_operator@spoke-costa-rica-cb2",
  "funded_operator@spoke-peru-cb",
  "funded_operator@spoke-peru-cb5",
  "funded_operator@spoke-peru-cb6",
];

const SAMPLES_ROSTER = [
  "funded_operator@spoke-brl-cb",
  "funded_operator@spoke-brl-bank-itau",
  "funded_operator@spoke-brl-bank-bradesco",
  "funded_operator@spoke-cop-cb",
  "funded_operator@spoke-cop-bank-bancolombia",
  "funded_operator@spoke-cop-bank-davivienda",
];

describe("spokeIdsFromRoster", () => {
  // The reported defect: a two-word country came out truncated, and the form
  // offered `spoke-costa` — which the backend accepts and writes into the Pente
  // group's immutable storage, where the relay can no longer route it.
  it("keeps a multi-word country whole (the Costa Rica defect)", () => {
    expect(spokeIdsFromRoster(LNET_ROSTER)).toEqual([
      "spoke-chile",
      "spoke-costa-rica",
      "spoke-peru",
    ]);
  });

  // Chile and Peru came out right under the old rule purely because their names
  // are one word. They are asserted so a future change cannot "fix" Costa Rica
  // by breaking the cases that always worked.
  it("still resolves single-word countries", () => {
    const ids = spokeIdsFromRoster(LNET_ROSTER);
    expect(ids).toContain("spoke-chile");
    expect(ids).toContain("spoke-peru");
    expect(ids).not.toContain("spoke-costa");
  });

  // The other live convention. `bank-itau` carries its own hyphen, so "all
  // segments but the last" — the natural fix for the LNET case — would yield
  // `spoke-brl-bank` here.
  it("resolves the samples convention, where the bank id itself is hyphenated", () => {
    expect(spokeIdsFromRoster(SAMPLES_ROSTER)).toEqual(["spoke-brl", "spoke-cop"]);
  });

  it("de-duplicates and sorts", () => {
    const ids = spokeIdsFromRoster([
      "funded_operator@spoke-peru-cb",
      "funded_operator@spoke-chile-cb",
      "funded_operator@spoke-peru-cb",
    ]);
    expect(ids).toEqual(["spoke-chile", "spoke-peru"]);
  });

  it("returns nothing for an empty roster rather than inventing a spoke", () => {
    expect(spokeIdsFromRoster([])).toEqual([]);
  });

  // A roster carrying only commercial banks yields no spokes. That is the
  // deliberate choice: an empty dropdown is a visible problem, whereas a guessed
  // spoke id is an invisible one that only surfaces at the relay, after the
  // agreement is already on-chain.
  it("does not guess a spoke id from a commercial-bank identity alone", () => {
    expect(spokeIdsFromRoster(["funded_operator@spoke-costa-rica-cb1"])).toEqual([]);
  });
});

describe("spokeIdFromCentralBankIdentity", () => {
  it("strips the -cb suffix under both conventions", () => {
    expect(spokeIdFromCentralBankIdentity("funded_operator@spoke-costa-rica-cb")).toBe(
      "spoke-costa-rica",
    );
    expect(spokeIdFromCentralBankIdentity("funded_operator@spoke-brl-cb")).toBe("spoke-brl");
  });

  // `-cb3` is a bank, not the central bank. Matching a `-cb` PREFIX instead of
  // the suffix would return "spoke-costa-rica" here too and silently double-count.
  it("rejects a bank node whose id merely starts with cb", () => {
    expect(spokeIdFromCentralBankIdentity("funded_operator@spoke-costa-rica-cb1")).toBeNull();
    expect(spokeIdFromCentralBankIdentity("funded_operator@spoke-chile-cb3")).toBeNull();
  });

  it("rejects a node that is only the suffix", () => {
    expect(spokeIdFromCentralBankIdentity("funded_operator@-cb")).toBeNull();
  });

  it("handles an identity with no @ part", () => {
    expect(spokeIdFromCentralBankIdentity("spoke-peru-cb")).toBe("spoke-peru");
  });
});

describe("nodeFromIdentity", () => {
  it("takes everything after the first @", () => {
    expect(nodeFromIdentity("funded_operator@spoke-peru-cb")).toBe("spoke-peru-cb");
  });

  it("returns the input when there is no @", () => {
    expect(nodeFromIdentity("spoke-peru-cb")).toBe("spoke-peru-cb");
  });

  it("trims surrounding whitespace", () => {
    expect(nodeFromIdentity("  funded_operator@spoke-peru-cb  ")).toBe("spoke-peru-cb");
  });
});
