// SPDX-License-Identifier: Apache-2.0

import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AuditVaultPage } from "../pages/AuditVaultPage";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";
import { ComplianceRegistryPage } from "../pages/ComplianceRegistryPage";
import { SettingsPage } from "../pages/SettingsPage";
import { SettlementHealthPage } from "../pages/SettlementHealthPage";
import { InvestigationPage } from "../pages/InvestigationPage";

export const routes: RouteObject[] = [
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: "participants", element: <ComplianceRegistryPage /> },
          { path: "audit", element: <AuditVaultPage /> },
          { path: "stability", element: <SettlementHealthPage /> },
          { path: "investigation", element: <InvestigationPage /> },
          { path: "settings", element: <SettingsPage /> },
        ],
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
];
