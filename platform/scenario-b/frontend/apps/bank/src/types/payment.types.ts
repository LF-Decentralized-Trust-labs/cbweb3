// SPDX-License-Identifier: Apache-2.0

export const PaymentStatus = {
  PENDING: 0,
  APPROVED: 1,
  REJECTED: 2,
  MINT_FAILED: 3,
} as const;

export const fiatUnitLabel = (import.meta.env.VITE_FIAT_SYMBOL ?? "fiat units").trim() || "fiat units";
const fiatHasSymbol = fiatUnitLabel !== "fiat units";

// currencyFromTokenSymbol extracts the currency code from an on-chain ERC-20 symbol.
// The tCeBM contract is the single source of truth: symbols are "tCeBM_BRL", "W-tCeBM_ARS",
// "fCeBM_BRL", etc. — the currency code is the segment after the last underscore.
// Falls back to the configured VITE_FIAT_SYMBOL while the balance API response is in flight.
export function currencyFromTokenSymbol(tokenSymbol: string | null | undefined): string {
  if (tokenSymbol) {
    const idx = tokenSymbol.lastIndexOf("_");
    if (idx >= 0 && idx < tokenSymbol.length - 1) {
      return tokenSymbol.slice(idx + 1).trim();
    }
  }
  return fiatHasSymbol ? fiatUnitLabel : "";
}

// tCeBM is denominated in the spoke's fiat currency; surface that alongside the token name.
// e.g. "tCeBM (BRL)" for Bank A, "tCeBM (ARS)" for Bank B. The currency is sourced from the
// on-chain token symbol (see currencyFromTokenSymbol); pass the symbol from the balance API.
export function tCeBMUnitLabel(tokenSymbol?: string | null): string {
  const code = currencyFromTokenSymbol(tokenSymbol);
  return code ? `tCeBM (${code})` : "tCeBM";
}

export type PaymentStatus = (typeof PaymentStatus)[keyof typeof PaymentStatus];

export interface DepositRecord {
  id: string;
  requester_id: string;
  requester_besu_address: string;
  amount: string;
  status: PaymentStatus | string | number;
  fiat_mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export const EscrowStatus = {
  PENDING: 0,
  APPROVED: 1,
  REJECTED: 2,
} as const;

export type EscrowStatus = (typeof EscrowStatus)[keyof typeof EscrowStatus];

export interface EscrowRecord {
  id: string;
  requester_id: string;
  besu_address: string;
  deposit_id: string;
  amount: string;
  status: EscrowStatus | string | number;
  burn_tx_hash: string;
  mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export interface RequestEscrowRequest {
  deposit_id: string;
  amount: string;
}

export interface RequestEscrowResponse {
  escrow_id: string;
}

export interface ListEscrowsResponse {
  escrows: EscrowRecord[];
  total: number;
}

export interface RedeemRecord {
  id: string;
  requester_id: string;
  requester_besu_address: string;
  amount: string;
  status: PaymentStatus | string | number;
  mint_tx_hash: string;
  rejection_reason: string;
  created_at: string;
}

export interface RegisterDepositRequest {
  requester_besu_address?: string;
  amount: string;
}

export interface RequestRedeemRequest {
  requester_besu_address?: string;
  amount: string;
}

export interface RegisterDepositResponse {
  deposit_id: string;
}

export interface RequestRedeemResponse {
  redeem_id: string;
}

export interface ListDepositsResponse {
  deposits: DepositRecord[];
  total: number;
}

export interface ListRedeemsResponse {
  redeems: RedeemRecord[];
  total: number;
}

export interface BalanceResponse {
  balance: string;
  decimals: number;
  symbol?: string; // on-chain ERC-20 symbol, e.g. "tCeBM_BRL"; source of truth for the currency code
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
      .replace("REDEEM_STATUS_", "")
      .replace("ESCROW_STATUS_", "");

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

// formatCeBM converts a raw base-unit amount (wei) to a human-readable tCeBM string.
// decimals comes from the token contract (included in the balance API response).
export function formatCeBM(rawAmount: string, decimals: number, tokenSymbol?: string | null): string {
  return `${formatTokenAmount(rawAmount, decimals)} ${tCeBMUnitLabel(tokenSymbol)}`;
}

// fiatCurrencyLabel renders the fiat currency code (e.g. "BRL" / "ARS") for the spoke, sourced
// from the on-chain fCeBM token symbol (see currencyFromTokenSymbol). Falls back to VITE_FIAT_SYMBOL.
export function fiatCurrencyLabel(tokenSymbol?: string | null): string {
  return currencyFromTokenSymbol(tokenSymbol) || fiatUnitLabel;
}

// formatFiatUnits converts a raw base-unit amount to a human-readable fiat string.
// Pass the on-chain fCeBM symbol (from /token/fiat-balance) so the currency code is contract-sourced.
export function formatFiatUnits(rawAmount: string, decimals: number, tokenSymbol?: string | null): string {
  return `${formatTokenAmount(rawAmount, decimals)} ${fiatCurrencyLabel(tokenSymbol)}`;
}

// formatTokenAmount converts a raw base-unit amount string to a display decimal string.
// Divides by 10^decimals and shows up to 6 fractional digits (trailing zeros trimmed).
export function formatTokenAmount(rawAmount: string, decimals: number): string {
  if (!rawAmount || rawAmount === "0") return "0";
  try {
    const divisor = 10n ** BigInt(decimals);
    const value = BigInt(rawAmount);
    const whole = value / divisor;
    const frac = (value % divisor).toString().padStart(decimals, "0").slice(0, 6).replace(/0+$/, "");
    return frac
      ? `${whole.toLocaleString("en-US")}.${frac}`
      : whole.toLocaleString("en-US");
  } catch {
    return rawAmount;
  }
}

// displayToBase converts a human-readable decimal string entered by the user into
// the raw base-unit integer string expected by the API (multiplies by 10^decimals).
// Accepts comma-formatted input (e.g. "1,000.5") as well as plain "1000.5".
// e.g. displayToBase("1,000.5", 18) → "1000500000000000000000"
export function displayToBase(displayAmount: string, decimals: number): string {
  if (!displayAmount || displayAmount === "0") return "0";
  const clean = displayAmount.replace(/,/g, "");
  const [whole, frac = ""] = clean.split(".");
  const fracPadded = frac.padEnd(decimals, "0").slice(0, decimals);
  const combined = `${whole}${fracPadded}`.replace(/^0+/, "") || "0";
  return combined;
}

// formatAmountInput formats a numeric string with thousand-separator commas for display
// inside an input field (e.g. "1000000.5" → "1,000,000.5").
// Feed it a clean string (digits + at most one dot); it preserves the fractional part as-is.
export function formatAmountInput(value: string): string {
  if (!value) return "";
  const [whole = "", frac] = value.split(".");
  const formatted = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return frac !== undefined ? `${formatted}.${frac}` : formatted;
}

// parseAmountInput strips thousand-separator commas so the result can be passed to displayToBase.
export function parseAmountInput(value: string): string {
  return value.replace(/,/g, "");
}
