// SPDX-License-Identifier: Apache-2.0

import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { GOVERNANCE_ONLY, GOVERNANCE_OR_ADMISSION } from "../auth/authorization";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import {
  AccountsPage,
  AuditPage,
  DashboardPage,
  LoginPage,
  RegistryPage,
  SettingsPage,
} from "../pages";

export const routes: RouteObject[] = [
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    // Portal gate: signed in AND holding either profile.
    element: <ProtectedRoute required={GOVERNANCE_OR_ADMISSION} />,
    children: [
      {
        element: <AppLayout />,
        children: [
          // Shared onboarding surface: reachable by governance OR admission (FR-003a).
          { path: "registry", element: <RegistryPage /> },
          {
            // Governance-only pages behind a second, pathless gate so a page added here
            // inherits the stricter requirement by default. An Admission-only operator
            // is redirected to /registry instead of being shown a page whose every
            // request would 403.
            element: <ProtectedRoute required={GOVERNANCE_ONLY} />,
            children: [
              { index: true, element: <DashboardPage /> },
              { path: "accounts", element: <AccountsPage /> },
              { path: "audit", element: <AuditPage /> },
              { path: "settings", element: <SettingsPage /> },
            ],
          },
        ],
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
];
