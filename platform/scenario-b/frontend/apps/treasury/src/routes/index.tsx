import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AuditPage } from "../pages/AuditPage";
import { DashboardPage } from "../pages/DashboardPage";
import { FundingRequestsPage } from "../pages/FundingRequestsPage";
import { IssuancePage } from "../pages/IssuancePage";
import { KycPage } from "../pages/KycPage";
import { LoginPage } from "../pages/LoginPage";
import { ReconciliationPage } from "../pages/ReconciliationPage";
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
          { path: "reconciliation", element: <ReconciliationPage /> },
          { path: "audit", element: <AuditPage /> },
          { path: "kyc", element: <KycPage /> },
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
