import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";
import { AppLayout } from "../components/layout/AppLayout";
import {
  AccountsPage,
  AuditPage,
  CircuitBreakerPage,
  DashboardPage,
  LoginPage,
  ParametersPage,
  RegistryPage,
  SettingsPage,
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
          { path: "accounts", element: <AccountsPage /> },
          { path: "circuit-breaker", element: <CircuitBreakerPage /> },
          { path: "parameters", element: <ParametersPage /> },
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
