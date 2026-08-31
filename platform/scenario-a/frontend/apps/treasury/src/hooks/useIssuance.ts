// SPDX-License-Identifier: Apache-2.0

import { useTreasuryStore } from "../stores";

export const useIssuance = () => {
  const { mint, status, error } = useTreasuryStore();
  return { mint, status, error };
};
