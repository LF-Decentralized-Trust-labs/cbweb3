import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AuditPage } from "../pages/AuditPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DepositsApprovalPage } from "../pages/DepositsApprovalPage";
import { EscrowsApprovalPage } from "../pages/EscrowsApprovalPage";
import { FundingRequestsPage } from "../pages/FundingRequestsPage";
import { HTLCMonitorPage } from "../pages/HTLCMonitorPage";
import { IssuancePage } from "../pages/IssuancePage";
import { LoginPage } from "../pages/LoginPage";
import { ReconciliationPage } from "../pages/ReconciliationPage";
import { RedeemsApprovalPage } from "../pages/RedeemsApprovalPage";
import { RedemptionPage } from "../pages/RedemptionPage";
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
          { path: "funding-requests", element: <FundingRequestsPage /> },
          { path: "issuance", element: <IssuancePage /> },
          { path: "redemption", element: <RedemptionPage /> },
          { path: "deposits-approval", element: <DepositsApprovalPage /> },
          { path: "escrows-approval", element: <EscrowsApprovalPage /> },
          { path: "redeems-approval", element: <RedeemsApprovalPage /> },
          { path: "htlc-monitor", element: <HTLCMonitorPage /> },
          { path: "reconciliation", element: <ReconciliationPage /> },
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
