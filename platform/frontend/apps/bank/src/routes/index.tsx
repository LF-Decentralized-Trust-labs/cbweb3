import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AMMTradingPage } from "../pages/AMMTradingPage";
import { ComplianceCenterPage } from "../pages/ComplianceCenterPage";
import { DashboardPage } from "../pages/DashboardPage";
import { HTLCTradingPage } from "../pages/HTLCTradingPage";
import { LoginPage } from "../pages/LoginPage";
import { SettingsPage } from "../pages/SettingsPage";
import { TokenManagementPage } from "../pages/TokenManagementPage";

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
          { path: "tokens", element: <TokenManagementPage /> },
          { path: "htlc", element: <HTLCTradingPage /> },
          { path: "amm", element: <AMMTradingPage /> },
          { path: "compliance", element: <ComplianceCenterPage /> },
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
