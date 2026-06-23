// SPDX-License-Identifier: Apache-2.0

import { useTreasuryStore } from "../stores";

export const useIssuance = () => {
  const { mint, validateBurnToMint, validation, status, error } = useTreasuryStore();
  return { mint, validateBurnToMint, validation, status, error };
};
