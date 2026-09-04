// SPDX-License-Identifier: Apache-2.0

// Bank membership of a Paladin identity.
//
// A node name is `<spokeId>-<bankId>` and BOTH halves may contain hyphens, so no
// split recovers the two parts:
//
//   spoke-costa-rica-cb1   spokeId=spoke-costa-rica  bankId=cb1
//   spoke-brl-bank-itau    spokeId=spoke-brl         bankId=bank-itau
//
// Both shapes are live: LNET manifests use `cb1`…`cb6`, the samples use
// `bank-itau`. The page previously skipped the first two hyphens and took the
// rest, which reads `spoke-costa-rica-cb1` as bank `rica-cb1`. Since the operator's
// own bank id is `cb1`, the "am I the originator?" comparison failed and the
// originator was shown the counterparty's Accept and Reject buttons on its own
// agreement. The backend guard for the same rule broke identically, so the
// originator could actually self-approve.
//
// Testing is exact where extracting is not: the caller already knows the bank id
// it is asking about, so the identity only has to end at that boundary.
//
// Kept in sync with identity.BelongsToBank in the payment-orchestrator and
// identityBelongsToBank in the api-gateway's payment handler. All three carry the
// same case table in their tests; change them together.

/**
 * Whether a Paladin identity belongs to `bankId`.
 *
 * The leading `-` is what prevents substring spoofing: `bank` must not match a
 * node ending in `-bank-abc`, and `cb1` must not match one ending in `-cb11`.
 * A party stored as a bare bank id (`cb1`) is accepted by exact equality.
 */
export function identityBelongsToBank(paladinIdentity: string, bankId: string): boolean {
  const bank = bankId.trim();
  if (bank === "") return false;

  const identity = paladinIdentity.trim();
  const at = identity.indexOf("@");
  const node = at >= 0 ? identity.slice(at + 1) : identity;
  if (node === "") return false;

  return node === bank || node.endsWith(`-${bank}`);
}
