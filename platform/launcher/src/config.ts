// SPDX-License-Identifier: Apache-2.0
//
// The launcher is deployed PER ENTITY (on that entity's own VPS) and lists only that
// entity's local portals, grouped by scenario. It knows nothing about other
// institutions — there is no global map to hardcode, so adding a new bank/spoke never
// touches an existing launcher (scenario isolation + distributed deployment).
//
// Configuration is loaded at RUNTIME from `config.json` (served next to index.html),
// NOT baked at build time. The image is therefore generic: each entity's deploy drops
// in its own config.json (see gen-config.sh), and adding portals needs no rebuild.

export type Scenario = "A" | "B";

// A single portal the entity exposes: a scenario + role, and the local URL the browser
// navigates to (login happens on that portal, not here).
export interface Portal {
  scenario: Scenario;
  role: string; // governance | treasury | supervisor | noc | bank
  label: string;
  url: string;
}

export interface LauncherConfig {
  // Institution label shown in the header, e.g. "Banco Itaú" or "Central Bank of Brazil".
  entity: string;
  portals: Portal[];
}

// Human-readable scenario headings (the only scenario knowledge the launcher carries —
// text, never code or addresses).
export const SCENARIO_META: Record<Scenario, { name: string; description: string }> = {
  A: { name: "Scenario A", description: "Enhanced Correspondent Banking — dual-layer HTLC settlement." },
  B: { name: "Scenario B", description: "International Hub — FX + AMM cross-currency corridor." },
};

// Each scenario's toolkit drops ONE fragment into the launcher's config dir when its
// apply runs with `launcher: enable`. The launcher fetches all known fragments and
// MERGES them client-side, so no toolkit ever reads another scenario's fragment and a
// missing scenario (404) simply doesn't appear. Adding/removing a scenario needs no
// rebuild or restart — just a fragment appearing/disappearing on the next reload.
const FRAGMENTS = ["configs/config.a.json", "configs/config.b.json"];

// loadConfig fetches every scenario fragment (ignoring absent ones) and unions their
// portals. Relative to BASE_URL so the launcher works under any mount path; no-store
// so a remounted fragment is picked up on reload.
export async function loadConfig(): Promise<LauncherConfig> {
  const base = import.meta.env.BASE_URL;
  const results = await Promise.all(
    FRAGMENTS.map(async (f): Promise<LauncherConfig | null> => {
      try {
        const res = await fetch(`${base}${f}`, { cache: "no-store" });
        if (!res.ok) return null;
        return (await res.json()) as LauncherConfig;
      } catch {
        return null;
      }
    }),
  );
  const present = results.filter((r): r is LauncherConfig => r !== null);
  if (present.length === 0) {
    throw new Error("no scenario deployed (no configs/config.*.json found)");
  }
  return {
    entity: present.find((r) => r.entity)?.entity ?? "CBWeb3 Platform",
    portals: present.flatMap((r) => r.portals ?? []),
  };
}
