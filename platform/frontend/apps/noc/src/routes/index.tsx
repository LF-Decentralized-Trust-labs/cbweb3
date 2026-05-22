import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import { AuditPage } from "../pages/AuditPage";
import { DashboardPage } from "../pages/DashboardPage";
import { InfrastructurePage } from "../pages/InfrastructurePage";
import { LoginPage } from "../pages/LoginPage";
import { LogViewerPage } from "../pages/LogViewerPage";
import { PoolStabilityPage } from "../pages/PoolStabilityPage";
import { RelayStatusPage } from "../pages/RelayStatusPage";
import { SettingsPage } from "../pages/SettingsPage";
import { TopologyPage } from "../pages/TopologyPage";

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
          { path: "infrastructure", element: <InfrastructurePage /> },
          { path: "logs/:componentId", element: <LogViewerPage /> },
          { path: "relays", element: <RelayStatusPage /> },
          { path: "pool-stability", element: <PoolStabilityPage /> },
          { path: "topology", element: <TopologyPage /> },
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

