import { useEffect } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function ProtectedRoute() {
  const { isAuthenticated, initialized, status, checkSession } = useAuth();

  useEffect(() => {
    if (!initialized) {
      void checkSession();
    }
  }, [initialized, checkSession]);

  if (!initialized || status === "loading") {
    return <div className="p-6 text-sm text-muted-foreground">Checking treasury session...</div>;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
