// SPDX-License-Identifier: Apache-2.0

// Amounts typed by an operator, under the ISO 20022 rule.
//
// The project settles through ISO 20022-shaped messages, and that standard is
// unambiguous about the amount field: the decimal separator is a **dot**, there
// is **no** thousands separator, and the fraction digits a currency may carry
// come from ISO 4217. JSON and XML Schema require the same thing of a decimal,
// so the wire format was never negotiable — the only open question was what the
// form should accept, and the project chose to make the two identical rather
// than translate between them.
//
// That is a deliberate narrowing. The 22nd CGPM (2003, Resolution 10) holds that
// "the symbol for the decimal marker shall be either the point on the line or
// the comma on the line", so "1000,10" is not wrong in general — it is simply
// not what this system's wire format accepts, and refusing it at the keyboard is
// how an operator finds that out immediately instead of at the gateway.
//
// Both standards agree on the part that caused the original defect. CGPM: digits
// "may be divided into groups of three by a space... neither dots nor commas are
// inserted in the spaces between groups". ISO 20022: no grouping at all. So
// "1.000,10" and "1,000.10" are refused by every rule in play, and they are
// exactly what `<input type="number">` used to mis-read.
//
//   accepted   1000    1000.10    0.5    1000.100000000000000001
//   refused    1000,10    1.000,10    1,000.10    1.000.000    1 000.10    1e3    -5    ""
//
// Why the field is plain text and not `<input type="number">`: that element
// parses through the browser's locale, and measured in Chromium at pt-BR it
// accepts the first separator typed and silently drops the second, turning
// "1.000,10" into "1.00010" — 1.0001, reported valid, with no error shown. A
// thousandfold understatement that nothing downstream can detect. A text field
// hands over exactly what was typed, so the rule below can answer for it.

/** Why an amount was refused, so the form can name the actual problem. */
export type AmountRefusal =
  | "empty"
  | "separator"
  | "malformed"
  | "not-positive"
  | "not-whole"
  | "too-many-decimals"
  | "no-subunit";

export type ParsedAmount =
  | {
      readonly ok: true;
      /**
       * The string to send. Authoritative, and already in ISO 20022 form: it
       * preserves every digit typed, including precision beyond what a JS number
       * can hold.
       */
      readonly canonical: string;
      /**
       * The same amount as a number, for comparisons and derived display only.
       * Never send this — past ~15 significant digits it is no longer the figure
       * the operator typed.
       */
      readonly value: number;
    }
  | { readonly ok: false; readonly refusal: AmountRefusal };

// ISO 20022: digits, optionally one dot, digits. Deliberately narrower than the
// orchestrator's own big.Rat, which also reads 0x10, 1_000, 1e3 and 1/3 — forms
// nobody means as money. Being the stricter of the two layers is safe; being the
// looser one is how a hex literal becomes a payment.
const ISO20022_AMOUNT = /^\d+(\.\d+)?$/;

/** Parse an operator-typed decimal amount under the ISO 20022 rule. */
export function parseAmount(input: string): ParsedAmount {
  const trimmed = input.trim();
  if (trimmed === "") return { ok: false, refusal: "empty" };

  // Separator problems get their own answer because they are the likely mistake
  // and the fix is a specific one: a comma anywhere, or a second dot, which can
  // only be grouping.
  if (trimmed.includes(",") || (trimmed.match(/\./g) ?? []).length > 1) {
    return { ok: false, refusal: "separator" };
  }

  if (!ISO20022_AMOUNT.test(trimmed)) {
    // A leading minus is a value problem, not a spelling one; say so.
    if (/^-\d+(\.\d+)?$/.test(trimmed)) return { ok: false, refusal: "not-positive" };
    return { ok: false, refusal: "malformed" };
  }

  const value = Number(trimmed);
  if (!(value > 0)) return { ok: false, refusal: "not-positive" };

  return { ok: true, canonical: trimmed, value };
}

/**
 * An amount for a field that carries **base units**, which must be whole.
 *
 * The FX proposal legs are this kind: `FXAgreement.propose` takes `uint256`, the
 * orchestrator converts with `big.Int.SetString(s, 10)`, and every other amount
 * in Scenario A — the HTLC legs, issuance, redemption, the sample tryout — is a
 * raw integer in the same unit. The tryout locks an HTLC of 1000 against an
 * `origin_amount` of 1000: the two must stay 1:1.
 *
 * So this does NOT scale by the token's decimals. fCeBM is an 18-decimal ERC20
 * and the backend mints the string unscaled, which means the scenario already
 * treats the raw base unit as the unit it shows. Scaling here alone would
 * desynchronise a proposal from the HTLC leg that settles it by 10^18. Changing
 * that convention is a scenario-wide decision about units, not one a single form
 * may take.
 */
export function parseBaseUnits(input: string): ParsedAmount {
  const trimmed = input.trim();
  if (trimmed === "") return { ok: false, refusal: "empty" };

  // Digits only. Every other shape gets the same answer, because for a whole
  // base-unit field they are the same mistake, and one accurate sentence beats
  // three diagnoses that each suggest writing something this field also refuses.
  if (!/^\d+$/.test(trimmed)) return { ok: false, refusal: "not-whole" };

  const value = Number(trimmed);
  if (!(value > 0)) return { ok: false, refusal: "not-positive" };

  return { ok: true, canonical: trimmed, value };
}

/** Operator-facing text for a refusal, naming the field it applies to. */
export function amountRefusalMessage(label: string, refusal: AmountRefusal): string {
  switch (refusal) {
    case "empty":
      return `${label} is required.`;
    case "separator":
      return `${label} must use a dot as the decimal separator, with no thousands separator: write 1000.10, not 1000,10 or 1,000.10.`;
    case "malformed":
      return `${label} must be a plain decimal number: digits, optionally followed by a dot and more digits.`;
    case "not-positive":
      return `${label} must be greater than zero.`;
    case "not-whole":
      return `${label} must be a whole number of base units: digits only, with no decimal or thousands separator.`;
    case "too-many-decimals":
      return `${label} carries more decimal places than the currency has.`;
    case "no-subunit":
      return `${label} must be a whole number: this currency has no subunit.`;
  }
}

// ─── ISO 4217 minor units ────────────────────────────────────────────────────
//
// ISO 20022 bounds an amount's fraction digits by the currency's ISO 4217 minor unit,
// and that is emphatically not a flat 2. Two of the currencies this pilot already
// offers have an exponent of ZERO: the Chilean peso and the Paraguayan guaraní have no
// subunit at all, so "1000.50 CLP" is not a small amount — it is not an amount. A fixed
// two-place rule would accept it while looking standards-compliant, which is worse than
// applying no rule.
//
// Only the currencies the forms can actually select are listed. An unlisted code is
// NOT silently given a default: unknownCurrencyMinorUnits returns null and the caller
// decides, because guessing the scale of money is the mistake this whole module exists
// to stop.
const ISO4217_MINOR_UNITS: Readonly<Record<string, number>> = Object.freeze({
  ARS: 2, BOB: 2, BRL: 2, COP: 2, CRC: 2, CUP: 2, DOP: 2, GTQ: 2,
  HNL: 2, MXN: 2, NIO: 2, PAB: 2, PEN: 2, USD: 2, UYU: 2, VES: 2,
  CLP: 0, // no subunit
  PYG: 0, // no subunit
});

/** Minor units for an ISO 4217 code, or null when the code is not one we know. */
export function currencyMinorUnits(code: string | null | undefined): number | null {
  if (!code) return null;
  const key = code.trim().toUpperCase();
  return key in ISO4217_MINOR_UNITS ? ISO4217_MINOR_UNITS[key] : null;
}

/**
 * Parse an amount denominated in a currency, bounding its fraction digits by that
 * currency's ISO 4217 minor unit on top of the ISO 20022 shape.
 *
 * An unknown currency falls back to the shape check alone rather than to a guessed
 * scale — refusing a legitimate amount because we lack a table entry would be worse
 * than accepting one extra decimal place.
 */
export function parseCurrencyAmount(input: string, currencyCode: string | null | undefined): ParsedAmount {
  const parsed = parseAmount(input);
  if (!parsed.ok) return parsed;

  const minor = currencyMinorUnits(currencyCode);
  if (minor === null) return parsed;

  const dot = parsed.canonical.indexOf(".");
  const typedDecimals = dot < 0 ? 0 : parsed.canonical.length - dot - 1;
  if (typedDecimals > minor) {
    return { ok: false, refusal: minor === 0 ? "no-subunit" : "too-many-decimals" };
  }
  return parsed;
}
