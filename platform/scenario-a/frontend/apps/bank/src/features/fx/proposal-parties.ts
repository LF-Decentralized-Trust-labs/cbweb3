// SPDX-License-Identifier: Apache-2.0

// Party derivations for the FX (PvP) proposal form.
//
// The form used to ask for eight party identities and two spoke ids, each picked from the full
// network roster. Most of them are fixed by who is proposing and to whom:
//
//   source spoke       the proposer's own spoke
//   destination spoke  the counterparty's spoke
//   settlement agent   defaults to the source spoke's central bank
//   custodian          defaults to the counterparty (it must be a bank on the destination
//                      spoke: the destination resolves its privacy group from it)
//   beneficiary        defaults to the destination receiver
//
// These are defaults, not rules: the request sent to the gateway is unchanged, and the last
// three stay editable. The values match those the end-to-end sample proposes with
// (samples/sample-tryout.sh).
//
// Everything here works from the roster alone. A node name is `<spokeId>-<bankId>` and both
// halves may contain hyphens, so it cannot be split (see spoke-roster.ts). The spoke ids,
// however, are known exactly from the central-bank entries, so a node's spoke is the one spoke
// id it starts with. When more than one does, no answer is given rather than a guess.

import { identityBelongsToBank } from "./identity";
import { nodeFromIdentity, spokeIdFromCentralBankIdentity } from "./spoke-roster";

/** The spoke an identity belongs to, or null when no known spoke, or more than one, matches. */
export function spokeOfIdentity(identity: string, spokes: readonly string[]): string | null {
  const node = nodeFromIdentity(identity);
  const matches = spokes.filter((spoke) => node.startsWith(`${spoke}-`));
  return matches.length === 1 ? matches[0] : null;
}

/**
 * The logged-in bank's own identity in the roster, or null when it cannot be told apart.
 * Membership is tested, not extracted, with the same rule the agreement page uses.
 */
export function ownIdentity(identities: readonly string[], bankId: string): string | null {
  if (bankId.trim() === "") return null;
  const matches = identities.filter((identity) => identityBelongsToBank(identity, bankId));
  return matches.length === 1 ? matches[0] : null;
}

/** The central bank identity of a spoke (`<spokeId>-cb`), or null. */
export function centralBankOfSpoke(spoke: string | null, identities: readonly string[]): string | null {
  if (!spoke) return null;
  return identities.find((identity) => spokeIdFromCentralBankIdentity(identity) === spoke) ?? null;
}

/**
 * Commercial banks that can be the counterparty: those on a known spoke other than the
 * proposer's. The backend refuses a trade whose source and destination spoke are the same.
 */
export function counterpartyOptions(
  identities: readonly string[],
  spokes: readonly string[],
  ownSpoke: string | null,
): string[] {
  return identities
    .filter((identity) => spokeIdFromCentralBankIdentity(identity) === null)
    .filter((identity) => {
      const spoke = spokeOfIdentity(identity, spokes);
      return spoke !== null && spoke !== ownSpoke;
    })
    .sort();
}

/** Every identity on a spoke, central bank included, optionally without one identity. */
export function spokeMembers(
  identities: readonly string[],
  spokes: readonly string[],
  spoke: string | null,
  exclude?: string | null,
): string[] {
  if (!spoke) return [];
  return identities
    .filter((identity) => identity !== exclude && spokeOfIdentity(identity, spokes) === spoke)
    .sort();
}

// ─── The whole party set ─────────────────────────────────────────────────────
//
// What the operator chose, and what is derived from it, kept apart. A choice only counts while it
// is still valid for the current spokes and counterparty: a receiver picked on one spoke, a
// counterparty on the proposer's own spoke, or an override made for another counterparty is
// ignored rather than carried into the request. The form cannot then submit a stale party
// whatever order the fields were changed in, and no reset can be forgotten.

/** An override that holds only for the context it was chosen in. */
export type ScopedChoice = { value: string; scope: string };

export type PartyChoices = {
  counterparty: string;
  sourceReceiver: string;
  destReceiver: string | null;
  custodian: string | null;
  /** Scoped to the source spoke it was chosen for. */
  settlementAgent: ScopedChoice | null;
  /** Scoped to the counterparty it was chosen for. */
  beneficiary: ScopedChoice | null;
};

export const NO_PARTY_CHOICES: PartyChoices = {
  counterparty: "",
  sourceReceiver: "",
  destReceiver: null,
  custodian: null,
  settlementAgent: null,
  beneficiary: null,
};

export type PartyContext = {
  identities: readonly string[];
  spokes: readonly string[];
  /** The proposer's own identity, when known; it is never offered as its own receiver. */
  me: string | null;
  /** The proposer's spoke: derived from the roster, or chosen when it cannot be. */
  sourceSpoke: string;
};

export type Parties = {
  sourceSpoke: string;
  destSpoke: string;
  counterparty: string;
  sourceReceiver: string;
  destReceiver: string;
  settlementAgent: string;
  custodian: string;
  beneficiary: string;
  counterparties: string[];
  sourceReceivers: string[];
  destReceivers: string[];
  /** Why the source receiver was not filled in for the operator, when that is worth saying. */
  sourceReceiverNote: string | null;
};

const validIn = (value: string | null | undefined, options: readonly string[]): value is string =>
  !!value && options.includes(value);

/** Every party the request needs, from the context and the operator's choices. */
export function deriveParties(ctx: PartyContext, choices: PartyChoices): Parties {
  const { identities, spokes, me } = ctx;
  const sourceSpoke = ctx.sourceSpoke;
  const counterparties = counterpartyOptions(identities, spokes, sourceSpoke || null);
  const counterparty = validIn(choices.counterparty, counterparties) ? choices.counterparty : "";
  const destSpoke = counterparty ? spokeOfIdentity(counterparty, spokes) ?? "" : "";

  const sourceReceivers = spokeMembers(identities, spokes, sourceSpoke || null, me);
  const ownCentralBank = centralBankOfSpoke(sourceSpoke || null, identities);
  // A single candidate is filled in, unless it is the central bank: routing the operator's own
  // leg to the central bank should be a choice someone made, not a default nobody saw.
  const soleCandidate = sourceReceivers.length === 1 ? sourceReceivers[0] : "";
  let sourceReceiver = "";
  let sourceReceiverNote: string | null = null;
  if (validIn(choices.sourceReceiver, sourceReceivers)) {
    sourceReceiver = choices.sourceReceiver;
  } else if (soleCandidate && soleCandidate !== ownCentralBank) {
    sourceReceiver = soleCandidate;
  } else if (soleCandidate) {
    sourceReceiverNote = "The only other member of your spoke is its central bank. Select it explicitly if it should receive your payment.";
  }

  const destReceivers = spokeMembers(identities, spokes, destSpoke || null);
  const destReceiver = validIn(choices.destReceiver, destReceivers) ? choices.destReceiver : counterparty;
  const custodian = validIn(choices.custodian, destReceivers) ? choices.custodian : counterparty;
  const settlementAgent =
    choices.settlementAgent && sourceSpoke && choices.settlementAgent.scope === sourceSpoke
      ? choices.settlementAgent.value
      : ownCentralBank ?? "";
  const beneficiary =
    choices.beneficiary && counterparty && choices.beneficiary.scope === counterparty
      ? choices.beneficiary.value
      : destReceiver;

  return {
    sourceSpoke, destSpoke, counterparty, sourceReceiver, destReceiver, settlementAgent, custodian, beneficiary,
    counterparties, sourceReceivers, destReceivers, sourceReceiverNote,
  };
}

/** The first reason the parties cannot be proposed, or null. Mirrors the backend's refusals. */
export function partyProblem(p: Parties): string | null {
  if (!p.counterparty) return "Counterparty bank is required.";
  if (!p.sourceSpoke) return "Your spoke could not be determined. Select it under Parties.";
  if (!p.destSpoke) return "The counterparty's spoke could not be determined. Choose another counterparty.";
  if (p.sourceSpoke === p.destSpoke) return "The counterparty must be on a different spoke from yours.";
  if (!p.sourceReceiver) return "Receiver on your spoke is required.";
  if (!p.destReceiver) return "Receiver on the counterparty's spoke is required.";
  if (!p.settlementAgent || !p.custodian || !p.beneficiary) {
    return "Settlement agent, custodian and beneficiary are required. Review them under Advanced.";
  }
  return null;
}
