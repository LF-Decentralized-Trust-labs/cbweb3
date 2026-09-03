// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { hasTreasuryAccess } from "../../auth/authorization";

export function ProtectedRoute() {
  const { isAuthenticated, initialized, status, profile, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking treasury session...</div>;
  }

  if (!isAuthenticated || !hasTreasuryAccess(profile)) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
