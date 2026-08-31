// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";

type UIStore = {
  sidebarOpen: boolean;
  setSidebarOpen: (open: boolean) => void;
};

export const useUiStore = create<UIStore>((set) => ({
  sidebarOpen: true,
  setSidebarOpen: (open) => set({ sidebarOpen: open }),
}));
