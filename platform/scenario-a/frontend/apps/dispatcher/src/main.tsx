// SPDX-License-Identifier: Apache-2.0

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toaster } from "@cbweb3/ui";
import "@cbweb3/ui/styles.css";
import App from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Toaster />
    <App />
  </StrictMode>,
);
