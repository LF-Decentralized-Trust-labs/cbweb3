// SPDX-License-Identifier: Apache-2.0

import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { LiquidityManagementPage } from "../features/liquidity/LiquidityManagementPage";
import { AuditPage } from "../pages/AuditPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DepositsApprovalPage } from "../pages/DepositsApprovalPage";
import { EscrowsApprovalPage } from "../pages/EscrowsApprovalPage";
import { LiquidityProvisioningPage } from "../pages/LiquidityProvisioningPage";
import { LoginPage } from "../pages/LoginPage";
import { RedeemsApprovalPage } from "../pages/RedeemsApprovalPage";
import { SettingsPage } from "../pages/SettingsPage";

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
          { path: "deposits-approval", element: <DepositsApprovalPage /> },
          { path: "escrows-approval", element: <EscrowsApprovalPage /> },
          { path: "redeems-approval", element: <RedeemsApprovalPage /> },
          { path: "liquidity", element: <LiquidityManagementPage /> },
          { path: "liquidity-provisioning", element: <LiquidityProvisioningPage /> },
          { path: "audit", element: <AuditPage /> },
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
