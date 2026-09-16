// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  MIN_APPROVAL_REASON_LENGTH,
  approvalReasonIssue,
  approvalReasonLength,
  isApprovalReasonAcceptable,
} from "./approval-reason";

// Approving an institution's KYC is a governance act the audit trail keeps forever, and the
// justification is the part a human wrote. The rule was already enforced — the submit handler
// refused anything under 10 characters — but it lived only there: the field was a one-line Input in
// a table cell whose placeholder was the only statement of the requirement. An operator learned the
// rule by being refused. These tests pin it now that the button and the guard share it.
//
// These are the first tests in this app: the governance portal had a vitest script and no test
// files. Extracting the rule is what made anything here coverable, since there is no jsdom or
// testing-library to render the page with.

describe("approvalReasonIssue", () => {
  it("refuses an empty reason and says a reason is required", () => {
    const issue = approvalReasonIssue("");
    expect(issue).not.toBeNull();
    expect(issue).toMatch(/required/i);
    expect(issue).toContain(String(MIN_APPROVAL_REASON_LENGTH));
  });

  it("treats whitespace as empty", () => {
    // Otherwise a field of spaces would pass the length check and store a blank justification.
    expect(approvalReasonIssue("          ")).toMatch(/required/i);
    expect(approvalReasonLength("          ")).toBe(0);
  });

  it("says how far along a short reason is, rather than only that it is short", () => {
    expect(approvalReasonIssue("too short")).toContain("9 so far");
  });

  it("accepts a reason at the boundary", () => {
    expect(approvalReasonIssue("0123456789")).toBeNull();
    expect(isApprovalReasonAcceptable("0123456789")).toBe(true);
  });

  it("accepts a real justification", () => {
    expect(isApprovalReasonAcceptable("Documents verified against the central registry.")).toBe(true);
  });

  it("does not count leading or trailing whitespace towards the minimum", () => {
    // "  short  " trims to 5. Counting the padding would let a reason pass that the endpoint rejects.
    expect(isApprovalReasonAcceptable("  short  ")).toBe(false);
    expect(approvalReasonLength("  short  ")).toBe(5);
  });
});
