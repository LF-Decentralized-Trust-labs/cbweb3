// SPDX-License-Identifier: Apache-2.0

// The country an onboarding form should start on.
//
// It used to start on the literal "BR", in every portal, including an Argentine
// bank's. The country is registered against the institution at its central bank, so
// a default that is silently wrong writes a wrong jurisdiction onto a compliance
// record — and the wizard gave no sign the value had been assumed rather than chosen.
//
// The portal does know where it is: the toolkit bakes VITE_FIAT_SYMBOL with the
// spoke's sovereign currency, and a spoke has exactly one. So the currency is the
// signal, and this maps it.
//
// The map is deliberately explicit and deliberately incomplete. An unknown currency
// yields an EMPTY default, not a guess: an empty field makes the operator choose,
// while a wrong one is accepted in silence. That asymmetry is the whole point of this
// file — adding a fallback "BR" here would restore the defect.

/** ISO-4217 currency → ISO-3166-1 alpha-2 country, for the sovereign currencies in play. */
const COUNTRY_BY_CURRENCY: Readonly<Record<string, string>> = {
  BRL: "BR",
  ARS: "AR",
  COP: "CO",
  CLP: "CL",
  PEN: "PE",
  UYU: "UY",
  MXN: "MX",
  PYG: "PY",
  BOB: "BO",
  CRC: "CR",
};

/**
 * The two-letter country for a sovereign currency code, or "" when it is not known.
 *
 * Case- and whitespace-insensitive, because the value arrives from a build-time
 * environment variable rather than from a typed source.
 */
export function defaultCountryForCurrency(currencySymbol: string | undefined | null): string {
  const symbol = (currencySymbol ?? "").trim().toUpperCase();
  if (symbol === "") return "";
  return COUNTRY_BY_CURRENCY[symbol] ?? "";
}
