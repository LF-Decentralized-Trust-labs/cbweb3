import { Badge } from "@cbweb3/ui";
import { NavLink } from "react-router-dom";
import { useCircuitBreaker } from "../../hooks";

const navItems = [
  { to: "/", label: "Dashboard" },
  { to: "/registry", label: "Registry" },
  { to: "/circuit-breaker", label: "Circuit Breaker" },
  { to: "/accounts", label: "Accounts" },
  { to: "/parameters", label: "Parameters" },
  { to: "/audit", label: "Audit" },
  { to: "/settings", label: "Settings" },
];

export function Sidebar() {
  const { circuitBreaker } = useCircuitBreaker();
  const isHalted = circuitBreaker?.state === "HALTED";

  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">System State</span>
        <Badge variant={isHalted ? "destructive" : "default"}>{circuitBreaker?.state ?? "LIVE"}</Badge>
      </div>
      <nav className="grid gap-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors ${
                isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-muted-foreground hover:bg-accent hover:text-foreground"
              }`
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
