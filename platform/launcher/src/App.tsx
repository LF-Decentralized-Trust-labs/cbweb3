// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from "react";
import {
  loadConfig,
  SCENARIO_META,
  type LauncherConfig,
  type Portal,
  type Scenario,
} from "./config";

// The launcher is a thin, scenario-agnostic entry point deployed per entity: it lists
// THIS entity's local portals (grouped by scenario) and redirects (full navigation) to
// the chosen one. Login happens on the destination portal, so the launcher never
// imports scenario code and holds no other institution's addresses.
function go(url: string): void {
  window.location.href = url;
}

const SCENARIO_ORDER: Scenario[] = ["A", "B"];

function PortalButton({ p }: { p: Portal }): React.JSX.Element {
  return (
    <button type="button" style={styles.card} onClick={() => go(p.url)}>
      <span style={styles.cardName}>{p.label}</span>
      <span style={styles.cardRole}>{p.role}</span>
      <span style={styles.cardCta}>Open →</span>
    </button>
  );
}

export default function App(): React.JSX.Element {
  const [config, setConfig] = useState<LauncherConfig | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadConfig()
      .then(setConfig)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  if (error) {
    return (
      <main style={styles.page}>
        <div style={styles.header}>
          <h1 style={styles.title}>CBWeb3 Platform</h1>
          <p style={styles.subtitle}>Could not load this entity's portal configuration.</p>
          <p style={styles.errorText}>{error}</p>
        </div>
      </main>
    );
  }

  if (!config) {
    return (
      <main style={styles.page}>
        <p style={styles.subtitle}>Loading…</p>
      </main>
    );
  }

  return (
    <main style={styles.page}>
      <div style={styles.header}>
        <h1 style={styles.title}>{config.entity}</h1>
        <p style={styles.subtitle}>Choose a scenario and portal to continue.</p>
      </div>
      {SCENARIO_ORDER.map((scenario) => {
        const portals = config.portals.filter((p) => p.scenario === scenario);
        if (portals.length === 0) return null;
        const meta = SCENARIO_META[scenario];
        return (
          <section key={scenario} style={styles.section}>
            <div style={styles.sectionHead}>
              <h2 style={styles.sectionTitle}>{meta.name}</h2>
              <p style={styles.sectionDesc}>{meta.description}</p>
            </div>
            <div style={styles.grid}>
              {portals.map((p) => (
                <PortalButton key={`${p.scenario}-${p.role}`} p={p} />
              ))}
            </div>
          </section>
        );
      })}
    </main>
  );
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    minHeight: "100vh",
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: "2rem",
    fontFamily: "system-ui, -apple-system, Segoe UI, Roboto, sans-serif",
    background: "#0b1220",
    color: "#e5e7eb",
    padding: "2rem",
  },
  header: { textAlign: "center" },
  title: { margin: 0, fontSize: "2rem", fontWeight: 700 },
  subtitle: { margin: "0.5rem 0 0", color: "#94a3b8" },
  errorText: { margin: "0.75rem 0 0", color: "#f87171", fontFamily: "monospace", fontSize: "0.85rem" },
  section: { width: "100%", maxWidth: "720px" },
  sectionHead: { marginBottom: "0.75rem" },
  sectionTitle: { margin: 0, fontSize: "1.1rem", fontWeight: 600 },
  sectionDesc: { margin: "0.25rem 0 0", color: "#94a3b8", fontSize: "0.85rem" },
  grid: {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
    gap: "1rem",
    width: "100%",
  },
  card: {
    display: "flex",
    flexDirection: "column",
    gap: "0.35rem",
    textAlign: "left",
    padding: "1.25rem",
    borderRadius: "12px",
    border: "1px solid #1e293b",
    background: "#111a2e",
    color: "inherit",
    cursor: "pointer",
    font: "inherit",
  },
  cardName: { fontSize: "1.1rem", fontWeight: 600 },
  cardRole: { color: "#94a3b8", fontSize: "0.8rem", textTransform: "uppercase", letterSpacing: "0.05em" },
  cardCta: { marginTop: "0.35rem", color: "#60a5fa", fontWeight: 600 },
};
