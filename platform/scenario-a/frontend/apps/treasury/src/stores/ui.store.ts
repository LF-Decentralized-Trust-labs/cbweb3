import { create } from "zustand";

type UiState = {
  selectedFundingRequestId: string | null;
  setSelectedFundingRequestId: (requestId: string | null) => void;
};

export const useUiStore = create<UiState>((set) => ({
  selectedFundingRequestId: null,
  setSelectedFundingRequestId: (requestId) => set({ selectedFundingRequestId: requestId }),
}));
