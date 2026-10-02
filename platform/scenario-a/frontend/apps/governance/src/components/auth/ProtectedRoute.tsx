// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks";
import {
  GOVERNANCE_ONLY,
  hasPortalAccess,
  isAllowedOnRoute,
  type RouteRequirement,
} from "../../auth/authorization";

interface ProtectedRouteProps {
  /**
   * Roles allowed on this route. Omitted → governance-only. The portal-level gate
   * (hasPortalAccess) only decides whether the operator may sign in at all; it is
   * deliberately NOT sufficient for page access (spec 042 FR-008).
   */
  required?: RouteRequirement;
}

export function ProtectedRoute({ required = GOVERNANCE_ONLY }: ProtectedRouteProps = {}) {
  const { isAuthenticated, initialized, status, profile, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking governance session...</div>;
  }

  // Not signed in, or holds neither profile → back to login.
  if (!isAuthenticated || !hasPortalAccess(profile)) {
    return <Navigate to="/login" replace />;
  }

  // Signed in but not entitled to THIS route (e.g. an Admission-only operator opening a
  // governance-only page). Send them to the onboarding surface rather than the login
  // page: the session is valid, only this surface is not theirs.
  if (!isAllowedOnRoute(profile, required)) {
    return <Navigate to="/registry" replace />;
  }

  return <Outlet />;
}
