// SPDX-License-Identifier: Apache-2.0

export type FXAgreementState =
  | "FX_STATE_PROPOSED"
  | "FX_STATE_ACCEPTED"
  | "FX_STATE_REJECTED"
  | "FX_STATE_CANCELLED"
  | "FX_STATE_SETTLED";

export interface FXAgreement {
  trade_id: string;
  originator?: string;
  counterparty_b: string;
  settlement_agent: string;
  custodian: string;
  beneficiary: string;
  origin_amount: string;
  counter_amount: string;
  origin_currency: string;
  counter_currency: string;
  spoke_a_receiver?: string;
  spoke_b_receiver?: string;
  rate: string;
  expiry_date: number;
  state: FXAgreementState;
}

export interface ProposeFXAgreementRequest {
  trade_id?: string;
  counterparty_b: string;
  settlement_agent: string;
  custodian: string;
  beneficiary: string;
  origin_amount: string;
  counter_amount: string;
  origin_currency: string;
  counter_currency: string;
  rate: string;
  expiry_date: number;
  spoke_a_receiver?: string;
  spoke_b_receiver?: string;
}

export interface ProposeFXAgreementResponse {
  tx_hash: string;
  trade_id: string;
}

export interface AcceptRejectFXResponse {
  tx_hash: string;
}
