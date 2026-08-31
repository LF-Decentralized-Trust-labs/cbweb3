// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";

export default defineConfig({
  base: process.env.VITE_BASE_PATH || "/",
  plugins: [react()],
  server: {
    port: 5900,
    strictPort: false,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "../../packages/ui/src"),
    },
  },
});
