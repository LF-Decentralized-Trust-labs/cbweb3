// SPDX-License-Identifier: Apache-2.0

import { currencyMinorUnits } from "@cbweb3/ui";
import { baseToExactDisplay, displayToBase } from "../../types";

/**
 * The scale of every FX agreement amount: an integer amount means hundredths.
 *
 * It is the application scale, not a property of the local token. Every orchestrator in this
 * scenario uses the same one (`amountScaleDecimals` in the payment orchestrator, which is also
 * the `decimals` the balance endpoint reports), because the Zeto lock circuit bounds amounts
 * and an FX leg must match the HTLC leg that settles it, base unit for base unit. So both legs
 * of an agreement, proposed on one spoke and settled on another, are in this scale.
 *
 * Agreement amounts are converted with this constant rather than with the decimals loaded from
 * the balance: those arrive asynchronously, and a value converted before they load would be
 * submitted with a different scale than the one it was shown in.
 */
export const AGREEMENT_AMOUNT_DECIMALS = 2;

/**
 * An agreement amount (base units) as an operator would type it, at full precision, for
 * pre-filling an amount field. Converting it back with agreementInputToBase gives exactly the
 * agreement's figure.
 */
export function agreementAmountToInput(baseAmount: string | null | undefined): string {
  return baseToExactDisplay(baseAmount ?? "", AGREEMENT_AMOUNT_DECIMALS);
}

/** What an operator typed for an agreement amount, as the base-unit integer the API takes. */
export function agreementInputToBase(displayAmount: string): string {
  return displayToBase(displayAmount, AGREEMENT_AMOUNT_DECIMALS);
}

/**
 * An agreement amount for reading, with its currency: "10,000.00 BRL", "1,000 CLP".
 *
 * The fraction is shown to the currency's ISO 4217 minor unit (none for CLP and PYG), and
 * never truncated: a digit beyond the minor unit is still money, so it is shown rather than
 * hidden. An unknown currency gets the agreement scale. The same function serves lists, notices
 * and detail pages, so one agreement reads the same everywhere.
 */
export function formatAgreementAmount(baseAmount: string | null | undefined, currency: string | null | undefined): string {
  const code = (currency ?? "").trim();
  const exact = agreementAmountToInput(baseAmount);
  if (!exact) return code ? `— ${code}` : "—";

  const [whole, frac = ""] = exact.split(".");
  const minor = currencyMinorUnits(code) ?? AGREEMENT_AMOUNT_DECIMALS;
  const shownFrac = frac.length >= minor ? frac : frac.padEnd(minor, "0");
  const grouped = BigInt(whole).toLocaleString("en-US");
  const amount = shownFrac ? `${grouped}.${shownFrac}` : grouped;
  return code ? `${amount} ${code}` : amount;
}
