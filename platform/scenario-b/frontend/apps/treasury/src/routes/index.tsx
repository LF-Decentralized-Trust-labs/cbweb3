import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { LiquidityManagementPage } from "../features/liquidity/LiquidityManagementPage";
import { AuditPage } from "../pages/AuditPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DepositsApprovalPage } from "../pages/DepositsApprovalPage";
import { EscrowsApprovalPage } from "../pages/EscrowsApprovalPage";
import { FundingRequestsPage } from "../pages/FundingRequestsPage";
import { IssuancePage } from "../pages/IssuancePage";
import { LoginPage } from "../pages/LoginPage";
import { ReconciliationPage } from "../pages/ReconciliationPage";
import { RedemptionPage } from "../pages/RedemptionPage";
import { RedeemsApprovalPage } from "../pages/RedeemsApprovalPage";
import { SettingsPage } from "../pages/SettingsPage";
import { TransferLimitsPage } from "../pages/TransferLimitsPage";

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
          { path: "funding-requests", element: <FundingRequestsPage /> },
          { path: "issuance", element: <IssuancePage /> },
          { path: "redemption", element: <RedemptionPage /> },
          { path: "deposits-approval", element: <DepositsApprovalPage /> },
          { path: "escrows-approval", element: <EscrowsApprovalPage /> },
          { path: "redeems-approval", element: <RedeemsApprovalPage /> },
          { path: "reconciliation", element: <ReconciliationPage /> },
          { path: "liquidity", element: <LiquidityManagementPage /> },
          { path: "transfer-limits", element: <TransferLimitsPage /> },
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
