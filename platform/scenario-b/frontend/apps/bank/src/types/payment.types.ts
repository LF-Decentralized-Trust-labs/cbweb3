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

export function formatCeBM(rawAmount: string): string {
  if (!rawAmount || rawAmount === "0") {
    return "0 tCeBM";
  }

  const value = BigInt(rawAmount);
  return `${value.toLocaleString("en-US")} tCeBM`;
}

export function formatFiatUnits(rawAmount: string): string {
  if (!rawAmount || rawAmount === "0") {
    return `0 ${fiatUnitLabel}`;
  }

  const value = BigInt(rawAmount);
  return `${value.toLocaleString("en-US")} ${fiatUnitLabel}`;
}
