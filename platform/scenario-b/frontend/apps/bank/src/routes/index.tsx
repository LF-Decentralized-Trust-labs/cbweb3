// SPDX-License-Identifier: Apache-2.0

import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { isScenarioB } from "../config/scenario";
import { AgreementDetailPage } from "../pages/AgreementDetailPage";
import { AgreementInboxPage } from "../pages/AgreementInboxPage";
import { AgreementProposalPage } from "../pages/AgreementProposalPage";
import { AMMTradingPage } from "../pages/AMMTradingPage";
import { ApproveAmmPage } from "../pages/ApproveAmmPage";
import { ComplianceCenterPage } from "../pages/ComplianceCenterPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DepositsPage } from "../pages/DepositsPage";
import { EscrowsPage } from "../pages/EscrowsPage";
import { LiquidityTransfersPage } from "../pages/LiquidityTransfersPage";
import { LoginPage } from "../pages/LoginPage";
import { OnboardingPage } from "../pages/OnboardingPage";
import { RedeemsPage } from "../pages/RedeemsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { SwapPage } from "../pages/SwapPage";
import { TransferPage } from "../pages/TransferPage";

const scenarioAChildren: RouteObject[] = [
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
  { path: "amm", element: <AMMTradingPage /> },
  { path: "compliance", element: <ComplianceCenterPage /> },
  { path: "onboarding", element: <OnboardingPage /> },
  { path: "settings", element: <SettingsPage /> },
];

const scenarioBChildren: RouteObject[] = [
  { index: true, element: <DashboardPage /> },
  // Cross-currency bridge (formerly "Swap"): the SwapPage component drives the
  // sovereign-pool cross-currency flow; the legacy BridgePage was outdated and removed.
  { path: "bridge", element: <SwapPage /> },
  { path: "transfer", element: <TransferPage /> },
  { path: "deposits", element: <DepositsPage /> },
  { path: "escrows", element: <EscrowsPage /> },
  { path: "approve-amm", element: <ApproveAmmPage /> },
  { path: "redeems", element: <RedeemsPage /> },
  { path: "amm", element: <AMMTradingPage /> },
  { path: "compliance", element: <ComplianceCenterPage /> },
  { path: "onboarding", element: <OnboardingPage /> },
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
