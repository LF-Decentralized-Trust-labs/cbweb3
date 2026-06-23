// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks";
import { hasGovernanceAccess } from "../../auth/authorization";

export function ProtectedRoute() {
  const { isAuthenticated, initialized, status, profile, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking governance session...</div>;
  }

  if (!isAuthenticated || !hasGovernanceAccess(profile)) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
