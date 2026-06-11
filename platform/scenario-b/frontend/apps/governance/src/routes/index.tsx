import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { isScenarioB } from "../config/scenario";
import { LiquidityManagementPage } from "../features/liquidity/LiquidityManagementPage";
import { OversightPage } from "../features/oversight/OversightPage";
import {
  AccountsPage,
  AuditPage,
  CircuitBreakerPage,
  DashboardPage,
  DepositsApprovalPage,
  EscrowsApprovalPage,
  HTLCMonitorPage,
  LoginPage,
  ParametersPage,
  RedeemsApprovalPage,
  RegistryPage,
  SettingsPage,
  SwapMonitorPage,
  TransferLimitsPage,
} from "../pages";

const scenarioAChildren: RouteObject[] = [
  { index: true, element: <DashboardPage /> },
  { path: "registry", element: <RegistryPage /> },
  { path: "circuit-breaker", element: <CircuitBreakerPage /> },
  { path: "htlc-monitor", element: <HTLCMonitorPage /> },
  { path: "accounts", element: <AccountsPage /> },
  { path: "parameters", element: <ParametersPage /> },
  { path: "deposits-approval", element: <DepositsApprovalPage /> },
  { path: "escrows-approval", element: <EscrowsApprovalPage /> },
  { path: "redeems-approval", element: <RedeemsApprovalPage /> },
  { path: "audit", element: <AuditPage /> },
  { path: "settings", element: <SettingsPage /> },
];

const scenarioBChildren: RouteObject[] = [
  { index: true, element: <DashboardPage /> },
  { path: "registry", element: <RegistryPage /> },
  { path: "liquidity", element: <LiquidityManagementPage /> },
  { path: "deposits-approval", element: <DepositsApprovalPage /> },
  { path: "escrows-approval", element: <EscrowsApprovalPage /> },
  { path: "redeems-approval", element: <RedeemsApprovalPage /> },
  { path: "swap-monitor", element: <SwapMonitorPage /> },
  { path: "circuit-breaker", element: <CircuitBreakerPage /> },
  { path: "oversight", element: <OversightPage /> },
  { path: "transfer-limits", element: <TransferLimitsPage /> },
  { path: "audit", element: <AuditPage /> },
  { path: "settings", element: <SettingsPage /> },
];

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
        children: isScenarioB ? scenarioBChildren : scenarioAChildren,
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
];
