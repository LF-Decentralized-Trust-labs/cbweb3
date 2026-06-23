// SPDX-License-Identifier: Apache-2.0

export const PORTAL_ROUTING = [
  {
    match: /^bank-a-/i,
    portalUrl: import.meta.env.VITE_BANK_A_PORTAL_URL ?? "http://localhost:5173",
    label: "Bank A",
  },
  {
    match: /^bank-b-/i,
    portalUrl: import.meta.env.VITE_BANK_B_PORTAL_URL ?? "http://localhost:5174",
    label: "Bank B",
  },
  {
    match: /^bank-c-/i,
    portalUrl: import.meta.env.VITE_BANK_C_PORTAL_URL ?? "http://localhost:5175",
    label: "Bank C",
  },
  {
    match: /^bank-d-/i,
    portalUrl: import.meta.env.VITE_BANK_D_PORTAL_URL ?? "http://localhost:5176",
    label: "Bank D",
  },
  {
    match: /^central-bank-a-/i,
    portalUrl: import.meta.env.VITE_CENTRAL_BANK_A_PORTAL_URL ?? "http://localhost:5177",
    label: "Central Bank A",
  },
  {
    match: /^central-bank-b-/i,
    portalUrl: import.meta.env.VITE_CENTRAL_BANK_B_PORTAL_URL ?? "http://localhost:5178",
    label: "Central Bank B",
  },
];
