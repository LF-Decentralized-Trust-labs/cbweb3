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
