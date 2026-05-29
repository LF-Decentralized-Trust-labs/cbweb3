import { PORTAL_ROUTING } from "../config/portals";

export function resolvePortal(clientId: string): { portalUrl: string; label: string } | null {
  return PORTAL_ROUTING.find((r) => r.match.test(clientId)) ?? null;
}
