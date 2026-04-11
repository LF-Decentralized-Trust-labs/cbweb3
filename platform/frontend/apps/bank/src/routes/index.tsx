import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AgreementDetailPage } from "../pages/AgreementDetailPage";
import { AgreementInboxPage } from "../pages/AgreementInboxPage";
import { AgreementProposalPage } from "../pages/AgreementProposalPage";
import { AMMTradingPage } from "../pages/AMMTradingPage";
import { ComplianceCenterPage } from "../pages/ComplianceCenterPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DepositsPage } from "../pages/DepositsPage";
import { EscrowsPage } from "../pages/EscrowsPage";
import { HTLCDetailPage } from "../pages/HTLCDetailPage";
import { HTLCHistoryPage } from "../pages/HTLCHistoryPage";
import { HTLCNewPage } from "../pages/HTLCNewPage";
import { LiquidityTransfersPage } from "../pages/LiquidityTransfersPage";
import { LoginPage } from "../pages/LoginPage";
import { OnboardingPage } from "../pages/OnboardingPage";
import { RedeemsPage } from "../pages/RedeemsPage";
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
          { path: "liquidity", element: <LiquidityTransfersPage /> },
          { path: "deposits", element: <DepositsPage /> },
          { path: "escrows", element: <EscrowsPage /> },
          { path: "redeems", element: <RedeemsPage /> },
          {
            path: "agreements",
            children: [
              { index: true, element: <AgreementInboxPage /> },
              { path: "new", element: <AgreementProposalPage /> },
              { path: ":tradeId", element: <AgreementDetailPage /> },
            ],
          },
          {
            path: "htlc",
            children: [
              { index: true, element: <HTLCHistoryPage /> },
              { path: "new", element: <HTLCNewPage /> },
              { path: ":contractId", element: <HTLCDetailPage /> },
            ],
          },
          { path: "amm", element: <AMMTradingPage /> },
          { path: "compliance", element: <ComplianceCenterPage /> },
          { path: "onboarding", element: <OnboardingPage /> },
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
