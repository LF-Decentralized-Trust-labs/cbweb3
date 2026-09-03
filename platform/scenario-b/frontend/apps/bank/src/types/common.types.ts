// SPDX-License-Identifier: Apache-2.0

export type UserRole = "COMMERCIAL_BANK_OPERATOR";

export type AsyncStatus = "idle" | "loading" | "success" | "error";

export type TransactionLifecycle = "PENDING" | "LOCKED" | "SETTLED" | "FAILED" | "REFUNDED";

export interface ApiResponse<T> {
  data: T;
  message?: string;
}

export interface SelectOption {
  label: string;
  value: string;
}
