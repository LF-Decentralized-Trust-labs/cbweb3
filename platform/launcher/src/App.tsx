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
// THIS entity's local portals (grouped by scenario). Each portal is a real link, so the
// browser handles navigation natively (Ctrl/Cmd/middle-click opens a new tab, and the
// context menu works). Login happens on the destination portal, so the launcher never
// imports scenario code and holds no other institution's addresses.

const SCENARIO_ORDER: Scenario[] = ["A", "B"];

// Minimal inline icons (no icon dependency in this standalone app).
function BuildingIcon(): React.JSX.Element {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="4" y="2" width="16" height="20" rx="2" />
      <path d="M9 22v-4h6v4M8 6h.01M16 6h.01M8 10h.01M16 10h.01M8 14h.01M16 14h.01" />
    </svg>
  );
}
function DotIcon(): React.JSX.Element {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}

const FEATURES: { title: string; sub: string }[] = [
  { title: "One entry point per entity", sub: "A single launcher lists every portal this institution runs." },
  { title: "Scenario A & B side by side", sub: "Correspondent Banking and International Hub portals in one place." },
  { title: "Login on the portal", sub: "Credentials are entered on the destination portal, never here." },
];

function PortalLink({ p }: { p: Portal }): React.JSX.Element {
  return (
    <a className="portal-btn" href={p.url}>
      <span className="portal-main">
        <span className="portal-label">{p.label}</span>
        <span className="portal-role">{p.role}</span>
      </span>
      <span className="portal-cta" aria-hidden="true">Open →</span>
    </a>
  );
}

export default function App(): React.JSX.Element {
  const [config, setConfig] = useState<LauncherConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [active, setActive] = useState<Scenario>("A");

  useEffect(() => {
    loadConfig()
      .then(setConfig)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  if (error) {
    return (
      <main className="state">
        <div className="state-inner">
          <h1>CBWeb3 Platform</h1>
          <p>Could not load this entity's portal configuration.</p>
          <p className="state-error">{error}</p>
        </div>
      </main>
    );
  }

  if (!config) {
    return (
      <main className="state">
        <div className="state-inner"><p>Loading…</p></div>
      </main>
    );
  }

  return (
    <main className="page">
      <div className="container">
        {/* left: brand panel (mirrors the login's left column) */}
        <section className="brand">
          <span className="badge">LNET · Launcher</span>
          <h1>{config.entity}</h1>
          <p className="brand-lead">
            Entry point for this institution's operator portals. Pick a scenario
            and role to continue — you will sign in on the portal itself.
          </p>
          <div className="features">
            {FEATURES.map((f) => (
              <div className="feature" key={f.title}>
                <DotIcon />
                <div>
                  <p className="feature-title">{f.title}</p>
                  <p className="feature-sub">{f.sub}</p>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* right: the portal list (replaces the login form) */}
        <div className="card">
          <div className="card-header">
            <div className="card-icon"><BuildingIcon /></div>
            <h2 className="card-title">Choose a portal</h2>
            <p className="card-desc">Select a scenario and role to open its portal.</p>
          </div>
          <div className="card-body">
            {(() => {
              const scenarios = SCENARIO_ORDER.filter((s) =>
                config.portals.some((p) => p.scenario === s),
              );
              const current = scenarios.includes(active) ? active : scenarios[0];
              const portals = config.portals.filter((p) => p.scenario === current);
              // Roving-tabindex keyboard nav for the scenario tablist (WAI-ARIA).
              const onTabKey = (e: React.KeyboardEvent): void => {
                const i = scenarios.indexOf(current);
                let next: Scenario | undefined;
                if (e.key === "ArrowRight" || e.key === "ArrowDown") next = scenarios[(i + 1) % scenarios.length];
                else if (e.key === "ArrowLeft" || e.key === "ArrowUp") next = scenarios[(i - 1 + scenarios.length) % scenarios.length];
                else if (e.key === "Home") next = scenarios[0];
                else if (e.key === "End") next = scenarios[scenarios.length - 1];
                if (next && next !== current) {
                  e.preventDefault();
                  const target = next;
                  setActive(target);
                  requestAnimationFrame(() => document.getElementById(`lt-tab-${target}`)?.focus());
                }
              };
              return (
                <>
                  {scenarios.length > 1 ? (
                    <div className="tabs" role="tablist" aria-label="Scenario" onKeyDown={onTabKey}>
                      {scenarios.map((s) => (
                        <button
                          key={s}
                          id={`lt-tab-${s}`}
                          type="button"
                          role="tab"
                          aria-selected={s === current}
                          aria-controls="lt-panel"
                          tabIndex={s === current ? 0 : -1}
                          className="tab"
                          onClick={() => setActive(s)}
                        >
                          {SCENARIO_META[s].name}
                        </button>
                      ))}
                    </div>
                  ) : null}
                  <div id="lt-panel" role="tabpanel" aria-labelledby={`lt-tab-${current}`}>
                    <p className="group-desc">{SCENARIO_META[current].description}</p>
                    <div className="portal-list">
                      {portals.map((p) => (
                        <PortalLink key={`${p.scenario}-${p.role}`} p={p} />
                      ))}
                    </div>
                  </div>
                </>
              );
            })()}
            <hr className="separator" />
            <p className="card-foot">Distributed per-entity launcher · one host, every scenario</p>
          </div>
        </div>
      </div>
    </main>
  );
}
