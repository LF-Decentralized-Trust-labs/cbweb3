// SPDX-License-Identifier: Apache-2.0

export type CircuitBreakerMode = "LIVE" | "HALTED";

export type CircuitBreakerState = {
  state: CircuitBreakerMode;
  updatedAt: string;
  updatedBy: string;
};

export type CircuitBreakerPayload = {
  pause: boolean;
  reason: string;
};

export type CircuitBreakerResult = {
  success: boolean;
  state: CircuitBreakerMode;
  updatedAt: string;
  updatedBy: string;
};
