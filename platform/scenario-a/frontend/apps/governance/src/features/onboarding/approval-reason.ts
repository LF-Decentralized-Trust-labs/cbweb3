// SPDX-License-Identifier: Apache-2.0

/**
 * The rule for the justification a governance operator writes when approving an institution's KYC.
 *
 * It lives here, and not inline in the page, so the SAME rule decides two things that must never
 * disagree: whether the Approve button is offered, and whether a submitted approval is accepted.
 * When those were separate the requirement was invisible — it existed only inside the submit
 * handler, so an operator discovered it by being refused, and the only hint was placeholder text
 * inside a one-line box in a table cell.
 *
 * Keeping it pure also keeps it testable: this app has no jsdom or testing-library, so a rule
 * embedded in a component could not be covered at all.
 *
 * Scenario B carries its own copy of this rule. That duplication is deliberate — the constitution
 * forbids sharing code across scenarios except through an explicitly versioned shared library — so
 * the two are expected to be edited independently, not kept byte-identical.
 */

/** MIN_APPROVAL_REASON_LENGTH is the shortest justification the approval endpoint accepts. */
export const MIN_APPROVAL_REASON_LENGTH = 10;

/**
 * approvalReasonIssue returns what is wrong with a justification, or null when it is acceptable.
 *
 * The message is the operator-facing copy, so the same words appear under the field and in the
 * refusal — a requirement described two different ways reads as two different requirements.
 */
export function approvalReasonIssue(reason: string): string | null {
  const trimmed = reason.trim();
  if (trimmed.length === 0) {
    return `An approval reason is required — at least ${MIN_APPROVAL_REASON_LENGTH} characters.`;
  }
  if (trimmed.length < MIN_APPROVAL_REASON_LENGTH) {
    // Whitespace does not count towards the minimum, so say what is actually missing rather than
    // letting the operator conclude the counter is broken.
    return `The approval reason needs at least ${MIN_APPROVAL_REASON_LENGTH} characters (${trimmed.length} so far).`;
  }
  return null;
}

/** isApprovalReasonAcceptable is the same rule as a predicate, for enabling the action. */
export function isApprovalReasonAcceptable(reason: string): boolean {
  return approvalReasonIssue(reason) === null;
}

/**
 * approvalReasonLength is what the counter under the field shows: trimmed, because that is what the
 * rule measures. A field holding only spaces reads 0, which is the honest number.
 */
export function approvalReasonLength(reason: string): number {
  return reason.trim().length;
}
