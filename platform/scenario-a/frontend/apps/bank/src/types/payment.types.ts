// SPDX-License-Identifier: Apache-2.0

export const PaymentStatus = {
  PENDING: 0,
  APPROVED: 1,
  REJECTED: 2,
  MINT_FAILED: 3,
} as const;

export const fiatUnitLabel = (import.meta.env.VITE_FIAT_SYMBOL ?? "fiat units").trim() || "fiat units";

export type PaymentStatus = (typeof PaymentStatus)[keyof typeof PaymentStatus];

export interface DepositRecord {
  id: string;
  requester_id: string;
  requester_besu_address: string;
  requester_paladin_identity: string;
  amount: string;
  status: PaymentStatus | string | number;
  mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export interface EscrowRecord {
  id: string;
  requester_id: string;
  requester_besu_address: string;
  requester_paladin_identity: string;
  amount: string;
  status: PaymentStatus | string | number;
  burn_tx_hash: string;
  mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export interface RedeemRecord {
  id: string;
  requester_id: string;
  requester_besu_address: string;
  requester_paladin_identity: string;
  amount: string;
  status: PaymentStatus | string | number;
  zeto_transfer_tx_hash: string;
  fiat_mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export interface CreateAmountRequest {
  amount: string;
}

export interface RegisterDepositResponse {
  deposit_id: string;
}

export interface RequestEscrowResponse {
  escrow_id: string;
}

export interface RequestRedeemResponse {
  redeem_id: string;
}

export interface ListDepositsResponse {
  deposits: DepositRecord[];
  total: number;
}

export interface ListEscrowsResponse {
  escrows: EscrowRecord[];
  total: number;
}

export interface ListRedeemsResponse {
  redeems: RedeemRecord[];
  total: number;
}

export interface BalanceResponse {
  balance: string;
  /**
   * Base-unit scale of the balance. Required to render it: without the scale a
   * balance of "1000" is indistinguishable between 1000 wei and 1000 whole units,
   * which is precisely what the portal used to get wrong (ADR-009).
   */
  decimals: number;
  /** Token symbol from the contract for fCeBM, or the fixed "tCeBM" for the Zeto note. */
  symbol: string;
}

export const paymentStatusLabel: Record<PaymentStatus, string> = {
  [PaymentStatus.PENDING]: "PENDING",
  [PaymentStatus.APPROVED]: "APPROVED",
  [PaymentStatus.REJECTED]: "REJECTED",
  [PaymentStatus.MINT_FAILED]: "MINT FAILED",
};

export const paymentStatusVariant: Record<PaymentStatus, "warning" | "success" | "destructive" | "outline"> = {
  [PaymentStatus.PENDING]: "warning",
  [PaymentStatus.APPROVED]: "success",
  [PaymentStatus.REJECTED]: "destructive",
  [PaymentStatus.MINT_FAILED]: "destructive",
};

export function normalizePaymentStatus(status: unknown): PaymentStatus | null {
  if (typeof status === "number") {
    if (
      status === PaymentStatus.PENDING ||
      status === PaymentStatus.APPROVED ||
      status === PaymentStatus.REJECTED ||
      status === PaymentStatus.MINT_FAILED
    ) {
      return status;
    }
    return null;
  }

  if (typeof status === "string") {
    const normalized = status
      .trim()
      .toUpperCase()
      .replace("PAYMENT_STATUS_", "")
      .replace("DEPOSIT_STATUS_", "")
      .replace("ESCROW_STATUS_", "")
      .replace("REDEEM_STATUS_", "");

    if (/^\d+$/.test(normalized)) {
      return normalizePaymentStatus(Number(normalized));
    }

    if (normalized === "PENDING") return PaymentStatus.PENDING;
    if (normalized === "APPROVED") return PaymentStatus.APPROVED;
    if (normalized === "REJECTED") return PaymentStatus.REJECTED;
    if (normalized === "MINT_FAILED") return PaymentStatus.MINT_FAILED;
  }

  return null;
}

export function getPaymentStatusLabel(status: unknown): string {
  const normalized = normalizePaymentStatus(status);
  return normalized !== null ? paymentStatusLabel[normalized] : "UNKNOWN";
}

export function getPaymentStatusVariant(status: unknown): "warning" | "success" | "destructive" | "outline" {
  const normalized = normalizePaymentStatus(status);
  return normalized !== null ? paymentStatusVariant[normalized] : "outline";
}

// Money is displayed with exactly the minor units of the currency: two places for the
// currencies in this pilot (ISO 4217). The tokens hold 18 decimals, so this is a
// presentation decision — the value on the wire stays a base-unit integer.
export const CURRENCY_DISPLAY_DECIMALS = 2;

/**
 * Render a base-unit amount for reading.
 *
 * TRUNCATES rather than rounds, so a balance never reads as more than it is, and a
 * non-zero holding too small to appear at two places reads "< 0.01" instead of "0" —
 * showing a zero for money someone holds is the same class of defect as showing the
 * wrong figure.
 *
 * Never use this to prefill an amount input: truncating a value that becomes a
 * transaction changes the transaction. Use baseToExactDisplay for that.
 */
export function formatBaseUnits(
  rawAmount: string,
  decimals: number,
  displayDecimals: number = CURRENCY_DISPLAY_DECIMALS,
): string {
  if (!rawAmount || !/^\d+$/.test(rawAmount)) return (0).toFixed(displayDecimals);
  const divisor = 10n ** BigInt(decimals);
  const value = BigInt(rawAmount);
  const whole = value / divisor;
  const frac = (value % divisor).toString().padStart(decimals, "0").slice(0, displayDecimals);
  if (whole === 0n && BigInt(frac || "0") === 0n && value > 0n) {
    return `< 0.${"0".repeat(Math.max(displayDecimals - 1, 0))}1`;
  }
  return displayDecimals > 0
    ? `${whole.toLocaleString("en-US")}.${frac}`
    : whole.toLocaleString("en-US");
}

/**
 * Full precision, for a value that will be typed back into an amount field — a
 * suggested maximum, a matched amount. Trailing zeros trimmed.
 */
export function baseToExactDisplay(rawAmount: string, decimals: number): string {
  if (!rawAmount || !/^\d+$/.test(rawAmount)) return "";
  const divisor = 10n ** BigInt(decimals);
  const value = BigInt(rawAmount);
  const whole = value / divisor;
  const frac = (value % divisor).toString().padStart(decimals, "0").replace(/0+$/, "");
  return frac ? `${whole}.${frac}` : whole.toString();
}

/**
 * Convert what an operator typed into the base-unit integer the API takes.
 *
 * Expects the canonical ISO 20022 form that parseAmount produces: digits, optionally
 * one dot, digits. It does not clean its input, deliberately — a converter that
 * silently strips characters is how "1000,10" became 100010 in the other scenario.
 */
export function displayToBase(displayAmount: string, decimals: number): string {
  if (!displayAmount) return "0";
  const [whole, frac = ""] = displayAmount.trim().split(".");
  const fracPadded = frac.padEnd(decimals, "0").slice(0, decimals);
  return `${whole}${fracPadded}`.replace(/^0+/, "") || "0";
}

export function formatCeBM(rawAmount: string, decimals: number): string {
  return `${formatBaseUnits(rawAmount, decimals)} tCeBM`;
}

export function formatFiatUnits(rawAmount: string, decimals: number, symbol?: string | null): string {
  return `${formatBaseUnits(rawAmount, decimals)} ${symbol?.trim() || fiatUnitLabel}`;
}

// The two below take what a FORM holds — the string the operator typed — and convert
// before formatting. Handing a display value straight to formatCeBM/formatFiatUnits
// divides it twice: at 18 decimals a typed 1500 renders as "0.00". That is the defect
// the confirmation panels in the other scenario shipped with, and the reason
// __tests__/confirmation-amount-units.test.ts exists here.
export function formatCeBMDisplay(displayAmount: string, decimals: number): string {
  return formatCeBM(displayToBase(displayAmount, decimals), decimals);
}

export function formatFiatDisplayUnits(displayAmount: string, decimals: number, symbol?: string | null): string {
  return formatFiatUnits(displayToBase(displayAmount, decimals), decimals, symbol);
}
