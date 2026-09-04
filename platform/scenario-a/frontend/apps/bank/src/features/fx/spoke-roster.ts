// SPDX-License-Identifier: Apache-2.0

// Spoke ids for the FX agreement form, derived from the Paladin identity roster.
//
// The roster is a flat list of `funded_operator@<node>` strings and carries no
// structure, so the spoke id has to be recovered from the node name. That cannot
// be done by splitting on "-", and the reason is worth stating because the
// obvious fix in either direction breaks the other environment:
//
//   | bankId      | node                   | first two segments | all but last     |
//   | ----------- | ---------------------- | ------------------ | ---------------- |
//   | `cb1`       | spoke-costa-rica-cb1   | spoke-costa    ✗   | spoke-costa-rica ✓ |
//   | `bank-itau` | spoke-brl-bank-itau    | spoke-brl      ✓   | spoke-brl-bank   ✗ |
//
// Both conventions are live: LNET manifests use `cb1`…`cb6`, the samples use
// `bank-itau`. A single-word country hid this for a long time — `spoke-chile-cb3`
// and `spoke-peru-cb5` come out right under the first rule purely by accident,
// so the form worked everywhere except Costa Rica, where it offered the
// unroutable `spoke-costa`.
//
// What IS reliable: a spoke's central bank node is exactly `<spokeId>-cb`
// (`cbNodeName` in the toolkit — `spokeID + "-cb"`, with no bank id appended).
// Stripping that one suffix recovers the spoke id verbatim under both
// conventions, and it is complete: every spoke has exactly one central bank, so
// no spoke can be missed by looking only at those entries.

const CB_NODE_SUFFIX = "-cb";

/** The node part of a Paladin identity — `funded_operator@<node>` yields `<node>`. */
export function nodeFromIdentity(identity: string): string {
  const trimmed = identity.trim();
  const at = trimmed.indexOf("@");
  return at >= 0 ? trimmed.slice(at + 1) : trimmed;
}

/**
 * The spoke id a central-bank identity belongs to, or null for anything else.
 *
 * Deliberately narrow: it answers only for `<spokeId>-cb` nodes and refuses to
 * guess for a commercial-bank node, because guessing is what produced
 * `spoke-costa`. A bank identity carries no separator that tells the spoke id
 * apart from the bank id.
 */
export function spokeIdFromCentralBankIdentity(identity: string): string | null {
  const node = nodeFromIdentity(identity);
  if (!node.endsWith(CB_NODE_SUFFIX)) return null;
  const spokeId = node.slice(0, -CB_NODE_SUFFIX.length);
  return spokeId === "" ? null : spokeId;
}

/**
 * Spoke ids offered by the FX form, sorted and de-duplicated.
 *
 * Derived from the central-bank entries alone. An identity that is not a central
 * bank contributes nothing rather than a truncated guess: offering a spoke id
 * that does not exist is worse than offering none, because the backend accepts
 * it and writes it into the Pente group's immutable private storage, where the
 * relay can no longer route it.
 */
export function spokeIdsFromRoster(identities: readonly string[]): string[] {
  const seen = new Set<string>();
  for (const identity of identities) {
    const spokeId = spokeIdFromCentralBankIdentity(identity);
    if (spokeId) seen.add(spokeId);
  }
  return Array.from(seen).sort();
}
