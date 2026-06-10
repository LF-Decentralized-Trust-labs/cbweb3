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

export interface BalanceResponse {
  balance: string;
  decimals: number;
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

export interface ApproveDepositRequest {
  deposit_id: string;
}

export interface RejectDepositRequest {
  deposit_id: string;
  reason: string;
}

export interface ApproveEscrowRequest {
  escrow_id: string;
}

export interface ApproveEscrowResponse {
  burn_tx_hash: string;
  mint_tx_hash: string;
}

export interface RejectEscrowRequest {
  escrow_id: string;
  reason: string;
}

export interface ApproveRedeemRequest {
  redeem_id: string;
}

export interface ApproveRedeemResponse {
  fiat_mint_tx_hash: string;
}

export interface RejectRedeemRequest {
  redeem_id: string;
  reason: string;
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

// displayToBase converts a human-readable decimal string to raw base-unit wei string.
// Accepts comma-formatted input (e.g. "1,000.5") as well as plain "1000.5".
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

// formatCeBM converts a raw base-unit amount (wei) to a human-readable tCeBM string.
export function formatCeBM(rawAmount: string, decimals: number): string {
  return `${formatTokenAmount(rawAmount, decimals)} tCeBM`;
}

// formatFiatUnits converts a raw base-unit amount to a human-readable fiat string.
export function formatFiatUnits(rawAmount: string, decimals: number): string {
  return `${formatTokenAmount(rawAmount, decimals)} ${fiatUnitLabel}`;
}

// formatTokenAmount converts a raw base-unit amount string to a display decimal string.
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
