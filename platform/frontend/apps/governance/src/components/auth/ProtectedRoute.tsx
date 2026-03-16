import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks";

export function ProtectedRoute() {
  const { isAuthenticated, initialized, status, user, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking governance session...</div>;
  }

  if (!isAuthenticated || user?.role !== "CENTRAL_BANK_ADMIN") {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
