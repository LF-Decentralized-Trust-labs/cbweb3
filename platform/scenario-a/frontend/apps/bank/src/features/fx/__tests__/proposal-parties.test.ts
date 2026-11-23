// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  centralBankOfSpoke,
  counterpartyOptions,
  deriveParties,
  NO_PARTY_CHOICES,
  ownIdentity,
  partyProblem,
  spokeMembers,
  spokeOfIdentity,
  type PartyChoices,
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

// ─── deriveParties ───────────────────────────────────────────────────────────
//
// A choice only counts while it is valid for the current spokes and counterparty. These pin the
// stale-state paths found in review: each one used to let a party from an earlier selection
// reach the request unnoticed.

const op = (node: string) => `funded_operator@${node}`;
const THREE_SPOKES = [
  op("spoke-brl-cb"), op("spoke-brl-bank-itau"), op("spoke-brl-bank-bradesco"),
  op("spoke-cop-cb"), op("spoke-cop-bank-bancolombia"), op("spoke-cop-bank-davivienda"),
  op("spoke-ars-cb"), op("spoke-ars-bank-nacion"),
];
const threeSpokes = spokeIdsFromRoster(THREE_SPOKES);
const ctx = (sourceSpoke: string, me: string | null = null) => ({
  identities: THREE_SPOKES, spokes: threeSpokes, me, sourceSpoke,
});
const pick = (patch: Partial<PartyChoices>): PartyChoices => ({ ...NO_PARTY_CHOICES, ...patch });

describe("deriveParties", () => {
  it("derives the full party set from a counterparty and a receiver", () => {
    const p = deriveParties(
      ctx("spoke-brl", op("spoke-brl-bank-itau")),
      pick({ counterparty: op("spoke-cop-bank-bancolombia"), sourceReceiver: op("spoke-brl-bank-bradesco") }),
    );
    expect(p).toMatchObject({
      sourceSpoke: "spoke-brl",
      destSpoke: "spoke-cop",
      counterparty: op("spoke-cop-bank-bancolombia"),
      sourceReceiver: op("spoke-brl-bank-bradesco"),
      destReceiver: op("spoke-cop-bank-bancolombia"),
      settlementAgent: op("spoke-brl-cb"),
      custodian: op("spoke-cop-bank-bancolombia"),
      beneficiary: op("spoke-cop-bank-bancolombia"),
    });
    expect(partyProblem(p)).toBeNull();
  });

  it("drops a source receiver chosen on a spoke the proposer is no longer on", () => {
    const choices = pick({ sourceReceiver: op("spoke-brl-bank-bradesco") });
    expect(deriveParties(ctx("spoke-brl"), choices).sourceReceiver).toBe(op("spoke-brl-bank-bradesco"));
    // On another spoke the earlier choice is gone, and nothing stale is left in its place.
    expect(deriveParties(ctx("spoke-ars"), choices).sourceReceiver).toBe("");
    expect(deriveParties(ctx("spoke-cop"), choices).sourceReceiver).toBe("");
  });

  it("drops a counterparty that ends up on the proposer's own spoke", () => {
    const choices = pick({ counterparty: op("spoke-cop-bank-bancolombia") });
    const p = deriveParties(ctx("spoke-cop"), choices);
    expect(p.counterparty).toBe("");
    expect(p.destSpoke).toBe("");
    expect(partyProblem(p)).toBe("Counterparty bank is required.");
  });

  it("refuses the same spoke on both sides", () => {
    const p = deriveParties(ctx("spoke-brl"), pick({ counterparty: op("spoke-cop-bank-bancolombia") }));
    expect(partyProblem({ ...p, sourceSpoke: "spoke-cop", sourceReceiver: "x" })).toBe(
      "The counterparty must be on a different spoke from yours.",
    );
  });

  it("keeps a settlement agent override only for the spoke it was chosen on", () => {
    const choices = pick({ settlementAgent: { value: op("spoke-ars-cb"), scope: "spoke-brl" } });
    expect(deriveParties(ctx("spoke-brl"), choices).settlementAgent).toBe(op("spoke-ars-cb"));
    expect(deriveParties(ctx("spoke-cop"), choices).settlementAgent).toBe(op("spoke-cop-cb"));
  });

  it("keeps a beneficiary override only for the counterparty it was chosen for", () => {
    const forBancolombia = { value: op("spoke-cop-bank-davivienda"), scope: op("spoke-cop-bank-bancolombia") };
    const same = deriveParties(ctx("spoke-brl"), pick({ counterparty: op("spoke-cop-bank-bancolombia"), beneficiary: forBancolombia }));
    expect(same.beneficiary).toBe(op("spoke-cop-bank-davivienda"));
    const other = deriveParties(ctx("spoke-brl"), pick({ counterparty: op("spoke-ars-bank-nacion"), beneficiary: forBancolombia }));
    expect(other.beneficiary).toBe(op("spoke-ars-bank-nacion"));
  });

  it("drops a destination receiver or custodian that is not on the counterparty's spoke", () => {
    const p = deriveParties(
      ctx("spoke-brl"),
      pick({
        counterparty: op("spoke-ars-bank-nacion"),
        destReceiver: op("spoke-cop-bank-davivienda"),
        custodian: op("spoke-cop-bank-davivienda"),
      }),
    );
    expect(p.destReceiver).toBe(op("spoke-ars-bank-nacion"));
    expect(p.custodian).toBe(op("spoke-ars-bank-nacion"));
  });

  it("does not route the proposer's own leg to the central bank without a choice", () => {
    const roster = [op("spoke-peru-cb"), op("spoke-peru-cb5"), op("spoke-brl-cb"), op("spoke-brl-bank-itau")];
    const spokes = spokeIdsFromRoster(roster);
    const base = { identities: roster, spokes, me: op("spoke-peru-cb5"), sourceSpoke: "spoke-peru" };

    const unchosen = deriveParties(base, NO_PARTY_CHOICES);
    expect(unchosen.sourceReceivers).toEqual([op("spoke-peru-cb")]);
    expect(unchosen.sourceReceiver).toBe("");
    expect(unchosen.sourceReceiverNote).toMatch(/central bank/);

    const chosen = deriveParties(base, pick({ sourceReceiver: op("spoke-peru-cb") }));
    expect(chosen.sourceReceiver).toBe(op("spoke-peru-cb"));
  });

  it("asks for nothing before the proposer's spoke is known", () => {
    const p = deriveParties(ctx(""), pick({ counterparty: op("spoke-cop-bank-bancolombia") }));
    expect(p.sourceReceiver).toBe("");
    expect(p.settlementAgent).toBe("");
    expect(partyProblem(p)).toBe("Your spoke could not be determined. Select it under Parties.");
  });
});
