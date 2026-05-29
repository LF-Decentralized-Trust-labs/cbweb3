import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import {
  DashboardPage,
  DepositsApprovalPage,
  EscrowsApprovalPage,
  HTLCMonitorPage,
  LoginPage,
  RedeemsApprovalPage,
  RegistryPage,
} from "../pages";

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
          { path: "registry", element: <RegistryPage /> },
          // { path: "circuit-breaker", element: <CircuitBreakerPage /> },
          { path: "htlc-monitor", element: <HTLCMonitorPage /> },
          // { path: "accounts", element: <AccountsPage /> },
          // { path: "parameters", element: <ParametersPage /> },
          { path: "deposits-approval", element: <DepositsApprovalPage /> },
          { path: "escrows-approval", element: <EscrowsApprovalPage /> },
          { path: "redeems-approval", element: <RedeemsApprovalPage /> },
          // { path: "audit", element: <AuditPage /> },
          // { path: "settings", element: <SettingsPage /> },
        ],
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
];
