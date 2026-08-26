// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { hasSupervisorAccess } from "../../auth/authorization";
import { useAuth } from "../../hooks/useAuth";

export function ProtectedRoute() {
  const { isAuthenticated, initialized, status, user, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking supervisor session...</div>;
  }

  // Checked here as well as in the store: the store decides at login and session restore,
  // this catches a user reaching the router by any other path.
  if (!isAuthenticated || !hasSupervisorAccess(user)) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
