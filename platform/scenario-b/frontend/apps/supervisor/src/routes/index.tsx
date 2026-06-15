import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AuditVaultPage } from "../pages/AuditVaultPage";
import { DashboardPage } from "../pages/DashboardPage";
import { LiquidityMonitorPage } from "../pages/LiquidityMonitorPage";
import { LoginPage } from "../pages/LoginPage";
import { ParticipantManagementPage } from "../pages/ParticipantManagementPage";
import { SettingsPage } from "../pages/SettingsPage";
import { StabilityControlsPage } from "../pages/StabilityControlsPage";
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
          { path: "liquidity", element: <LiquidityMonitorPage /> },
          { path: "participants", element: <ParticipantManagementPage /> },
          { path: "audit", element: <AuditVaultPage /> },
          { path: "stability", element: <StabilityControlsPage /> },
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
