// SPDX-License-Identifier: Apache-2.0

import { httpClientV2 } from "./http-client";

/** What this central bank still owes one bank, across that bank's in-flight payments. */
export interface BankExposure {
  owner_bank_id: string;
  amount: string;
  positions: number;
}

/**
 * A position this CB's own records already flag as needing attention.
 *
 * Listed for the operator, NOT deducted from `unexplained`: a stuck residue is normally still
 * counted inside its parent's in-flight amount, so netting it as well would report the same money
 * twice. `direction` is what says whether the value is sitting on the address (a stuck burn, OUT)
 * or never reached it (a stuck mint, IN).
 */
export interface StrandedPosition {
  position_id: string;
  owner_bank_id: string;
  leg: string;
  direction: string;
  amount: string;
  bridge_state: string;
}

/**
 * The Hub balance this central bank holds for its banks, against what its records account for.
 *
 * The banks never hold W-token themselves — it is minted to the CB, spent by the CB in the AMM
 * trade and burned by the CB when the unspent part goes back. So this balance is the CB's
 * OBLIGATION, and the payment side of it is zero at rest. `unexplained` is therefore the number
 * that matters: value on-chain not attributable to any bank payment. It is signed — a negative
 * figure is a SHORTFALL against the records, the more serious of the two findings.
 *
 * It also includes this CB's OWN liquidity resting on the same address (W-token it minted and
 * deployed into a pool, which nothing burns back down). That is excluded from the expectation on
 * purpose: no per-position record of pool deployments exists, and inferring one from balances is
 * the heuristic this report replaces.
 */
export interface HubReconciliationReport {
  w_token: string;
  holder_address: string;
  on_chain_balance: string;
  expected_in_flight: string;
  unexplained: string;
  stranded_total: string;
  per_bank: BankExposure[];
  stranded?: StrandedPosition[];
  balanced: boolean;
}

export const hubReconciliationApi = {
  get: async (): Promise<HubReconciliationReport> => {
    const response = await httpClientV2.get<HubReconciliationReport>("/amm/hub-reconciliation");
    return response.data;
  },
};
