// SPDX-License-Identifier: Apache-2.0

import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { GOVERNANCE_ONLY, GOVERNANCE_OR_ADMISSION } from "../auth/authorization";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { isScenarioB } from "../config/scenario";
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
  ApprovalsOverviewPage,
  TransferLimitsPage,
} from "../pages";

// Per-route authorization (spec 042 FR-008). The onboarding surface is shared by the
// governance and Admission profiles; every other page stays governance-only. The
// governance-only pages sit behind a second, pathless ProtectedRoute so a page added
// there inherits the stricter requirement by default — the frontend counterpart of the
// router's "relaxing a group widens everything inside it" hazard.
const onboardingChildren: RouteObject[] = [{ path: "registry", element: <RegistryPage /> }];

const scenarioAGovernanceOnlyChildren: RouteObject[] = [
  { index: true, element: <DashboardPage /> },
  { path: "circuit-breaker", element: <CircuitBreakerPage /> },
  { path: "transfer-limits", element: <TransferLimitsPage /> },
  { path: "htlc-monitor", element: <HTLCMonitorPage /> },
  { path: "accounts", element: <AccountsPage /> },
  { path: "parameters", element: <ParametersPage /> },
  { path: "deposits-approval", element: <DepositsApprovalPage /> },
  { path: "escrows-approval", element: <EscrowsApprovalPage /> },
  { path: "redeems-approval", element: <RedeemsApprovalPage /> },
  { path: "audit", element: <AuditPage /> },
  { path: "settings", element: <SettingsPage /> },
];

const scenarioBGovernanceOnlyChildren: RouteObject[] = [
  { index: true, element: <DashboardPage /> },
  { path: "accounts", element: <AccountsPage /> },
  { path: "approvals-overview", element: <ApprovalsOverviewPage /> },
  { path: "circuit-breaker", element: <CircuitBreakerPage /> },
  { path: "transfer-limits", element: <TransferLimitsPage /> },
  { path: "oversight", element: <OversightPage /> },
  { path: "audit", element: <AuditPage /> },
  { path: "settings", element: <SettingsPage /> },
];

const governanceOnlyChildren = isScenarioB
  ? scenarioBGovernanceOnlyChildren
  : scenarioAGovernanceOnlyChildren;

export const routes: RouteObject[] = [
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    // Portal gate: signed in AND holding either profile.
    element: <ProtectedRoute required={GOVERNANCE_OR_ADMISSION} />,
    children: [
      {
        element: <AppLayout />,
        children: [
          ...onboardingChildren,
          {
            // Inner gate: governance-only pages. An Admission-only operator opening one
            // of these is redirected to the onboarding surface instead of being shown a
            // page whose every request would 403.
            element: <ProtectedRoute required={GOVERNANCE_ONLY} />,
            children: governanceOnlyChildren,
          },
        ],
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
];
