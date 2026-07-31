// SPDX-License-Identifier: Apache-2.0

// The per-entity launcher URL is injected at build time by each scenario's toolkit as
// VITE_LAUNCHER_URL (never hardcoded). Access import.meta.env defensively: this shared
// package does not include the "vite/client" ambient types (only the apps do), so we
// cast instead of relying on ImportMetaEnv being declared here.
function readLauncherUrl(): string {
  const env = (import.meta as unknown as { env?: Record<string, string | undefined> }).env;
  const raw = env?.VITE_LAUNCHER_URL;
  return typeof raw === "string" ? raw.trim() : "";
}

// getLauncherUrl returns the configured launcher URL, or null when the frontend was built
// without a launcher (VITE_LAUNCHER_URL absent/empty) — callers use null to hide the
// "back to launcher" affordance and keep the previous behaviour.
export function getLauncherUrl(): string | null {
  const url = readLauncherUrl();
  return url ? url : null;
}

// goToLauncher navigates the browser to the launcher when one is configured and reports
// whether it did. Returns false when no launcher URL is set, so the caller can fall back
// (e.g. navigate to the local /login route).
export function goToLauncher(): boolean {
  const url = getLauncherUrl();
  if (!url) return false;
  window.location.assign(url);
  return true;
}
