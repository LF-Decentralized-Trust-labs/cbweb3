import { useTreasuryStore } from "../stores";

export const useRedemption = () => {
  const { burn, status, error } = useTreasuryStore();
  return { burn, status, error };
};
