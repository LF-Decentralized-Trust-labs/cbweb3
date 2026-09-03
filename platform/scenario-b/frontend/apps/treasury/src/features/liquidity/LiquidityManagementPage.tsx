// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Progress,
  Separator,
  cn,
  toast,
} from "@cbweb3/ui";
import { ArrowLeftRight, Copy, ShieldAlert, ShieldCheck, ShieldQuestion } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { usePolling } from "../../hooks/usePolling";
import { circuitBreakerApi, hubLiquidityApi, liquidityApi, paymentApi } from "../../services/api";
import { currencyFromTokenSymbol, formatTokenAmount } from "../../types";
import type { CircuitBreakerStatus } from "../../types/circuit-breaker.types";
import type { PoolStatus } from "../../types/liquidity.types";
import type { HubCurrency, HubPair } from "../../types/hub-liquidity.types";

const POOL_REFRESH_MS = 15_000;

type BadgeVariant = "success" | "warning" | "destructive" | "outline" | "default";

function truncate(value?: string): string {
  if (!value) return "—";
  return value.length <= 12 ? value : `${value.slice(0, 6)}…${value.slice(-4)}`;
}

// codesFromPairId extracts the currency codes encoded in a pair id, in order.
// e.g. "W-tCeBM_BRL-W-tCeBM_COP" -> ["BRL", "COP"]. Used as a fallback when the
// on-chain currency registry does not map a token address to a code.
function codesFromPairId(pairId: string): string[] {
  return (pairId.match(/_([A-Za-z0-9]{2,})/g) ?? []).map((s) => s.slice(1));
}

// reserveNumber converts a wei reserve to a float for proportion math only
// (display values go through formatTokenAmount, which keeps full precision).
function reserveNumber(wei: string, decimals: number): number {
  if (!wei || !/^\d+$/.test(wei)) return 0;
  try {
    return Number(BigInt(wei)) / 10 ** decimals;
  } catch {
    return 0;
  }
}

function poolStatusVariant(status?: string): BadgeVariant {
  switch (status) {
    case "ACTIVE":
      return "success";
    case "PENDING_COUNTERPART":
      return "warning";
    default:
      return "outline";
  }
}

function poolStatusLabel(status?: string): string {
  switch (status) {
    case "ACTIVE":
      return "Active";
    case "PENDING_COUNTERPART":
      return "Awaiting counterpart";
    case "EMPTY":
      return "Empty";
    default:
      return status ?? "Unknown";
  }
}

function relativeTime(iso?: string): string {
  if (!iso) return "—";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "—";
  const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.round(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return new Date(then).toLocaleString();
}

async function copyToClipboard(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied`);
  } catch {
    toast.error("Could not copy to clipboard");
  }
}

// The circuit breaker is per-pair (one AMM instance per pair). Treasury only reads it;
// pausing/resuming stays in the governance portal.
function breakerView(state?: string): {
  variant: BadgeVariant;
  label: string;
  Icon: typeof ShieldCheck;
  tone: string;
  blurb: string;
} {
  switch (state) {
    case "LIVE":
      return {
        variant: "success",
        label: "Operational",
        Icon: ShieldCheck,
        tone: "text-emerald-600 dark:text-emerald-400",
        blurb: "Swaps are open on this pool.",
      };
    case "HALTED":
      return {
        variant: "destructive",
        label: "Paused",
        Icon: ShieldAlert,
        tone: "text-red-600 dark:text-red-400",
        blurb: "A Central Bank triggered the circuit breaker. Swaps are suspended.",
      };
    case "RESUME_PENDING":
      return {
        variant: "warning",
        label: "Resume pending",
        Icon: ShieldQuestion,
        tone: "text-amber-600 dark:text-amber-400",
        blurb: "A resume proposal is awaiting the 2-of-N Central Bank quorum.",
      };
    default:
      return {
        variant: "outline",
        label: "Unknown",
        Icon: ShieldQuestion,
        tone: "text-muted-foreground",
        blurb: "Circuit breaker state is not available.",
      };
  }
}

export function LiquidityManagementPage() {
  const [pairs, setPairs] = useState<HubPair[]>([]);
  const [currencies, setCurrencies] = useState<HubCurrency[]>([]);
  const [poolByPair, setPoolByPair] = useState<Record<string, PoolStatus>>({});
  const [breakerByPair, setBreakerByPair] = useState<Record<string, CircuitBreakerStatus>>({});
  const [selectedPairId, setSelectedPairId] = useState("");
  const [nationalCurrency, setNationalCurrency] = useState("");
  const [tokenDecimals, setTokenDecimals] = useState(18);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Registry data (pairs + currencies) + this CB's own token (for the national currency
  // sovereignty filter and reserve decimals) — fetched once.
  useEffect(() => {
    void (async () => {
      try {
        const [pairsRes, currenciesRes] = await Promise.all([
          hubLiquidityApi.listPairs(),
          hubLiquidityApi.listCurrencies(),
        ]);
        setPairs(pairsRes.pairs ?? []);
        setCurrencies(currenciesRes.currencies ?? []);
      } catch {
        setError("Unable to load liquidity pools. Please try again.");
      } finally {
        setLoading(false);
      }
      try {
        const bal = await paymentApi.getBalance();
        setNationalCurrency(currencyFromTokenSymbol(bal.symbol));
        setTokenDecimals(bal.decimals ?? 18);
      } catch {
        /* balance optional — falls back to showing all pools */
      }
    })();
  }, []);

  const addressToCode = useMemo(() => {
    const map = new Map<string, string>();
    for (const c of currencies) map.set(c.token_address.toLowerCase(), currencyFromTokenSymbol(c.symbol));
    return map;
  }, [currencies]);

  const codeOf = (addr: string) => addressToCode.get(addr.toLowerCase()) ?? "";

  const pairLabel = (p: HubPair): { a: string; b: string } => {
    const codes = codesFromPairId(p.pair_id);
    return {
      a: codeOf(p.token_a_address) || codes[0] || truncate(p.token_a_address),
      b: codeOf(p.token_b_address) || codes[1] || truncate(p.token_b_address),
    };
  };

  // Sovereignty filter: only pools that include this Central Bank's national currency
  // (the corridors it provisions). Pairs without it are not managed here.
  const isNationalPair = (p: HubPair) => {
    if (!nationalCurrency) return false;
    const { a, b } = pairLabel(p);
    return a === nationalCurrency || b === nationalCurrency;
  };

  // Filter to the CB's national-currency corridors. If the national currency cannot be
  // determined (balance/symbol unavailable), fall back to showing all pools rather than
  // hiding everything.
  const visiblePairs = useMemo(() => {
    const list = nationalCurrency ? pairs.filter(isNationalPair) : pairs;
    return [...list].sort((x, y) => {
      if (x.status === y.status) return pairLabel(x).a.localeCompare(pairLabel(y).a);
      return x.status === "ACTIVE" ? -1 : 1;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pairs, nationalCurrency, addressToCode]);

  const refreshLiveData = useMemo(
    () => async () => {
      if (visiblePairs.length === 0) return;
      const entries = await Promise.all(
        visiblePairs.map(async (p) => {
          const [pool, breaker] = await Promise.all([
            liquidityApi.getPoolStatus(p.pair_id).catch(() => null),
            circuitBreakerApi.getStatus(p.pair_id).catch(() => null),
          ]);
          return { pairId: p.pair_id, pool, breaker };
        }),
      );
      setPoolByPair((prev) => {
        const next = { ...prev };
        for (const e of entries) if (e.pool) next[e.pairId] = e.pool;
        return next;
      });
      setBreakerByPair((prev) => {
        const next = { ...prev };
        for (const e of entries) if (e.breaker) next[e.pairId] = e.breaker;
        return next;
      });
    },
    [visiblePairs],
  );

  usePolling(() => void refreshLiveData(), POOL_REFRESH_MS, visiblePairs.length > 0);

  useEffect(() => {
    if (visiblePairs.length === 0) {
      setSelectedPairId("");
      return;
    }
    if (!visiblePairs.some((p) => p.pair_id === selectedPairId)) {
      setSelectedPairId(visiblePairs[0].pair_id);
    }
  }, [visiblePairs, selectedPairId]);

  const activeCount = visiblePairs.filter((p) => p.status === "ACTIVE").length;
  const pausedCount = visiblePairs.filter((p) => breakerByPair[p.pair_id]?.state === "HALTED").length;

  const selected = visiblePairs.find((p) => p.pair_id === selectedPairId) ?? null;
  const selectedPool = selected ? poolByPair[selected.pair_id] : undefined;
  const selectedBreaker = selected ? breakerByPair[selected.pair_id] : undefined;

  return (
    <div className="space-y-4">
      {/* Header */}
      <Card>
        <CardHeader>
          <CardTitle>Liquidity Management</CardTitle>
          <CardDescription>
            Cross-currency pools this Central Bank provisions{nationalCurrency ? ` (${nationalCurrency})` : ""} — only
            pairs that include your national currency. Read-only view.
          </CardDescription>
        </CardHeader>
      </Card>

      {/* Summary */}
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pools managed</CardDescription>
            <CardTitle className="text-2xl">{visiblePairs.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active</CardDescription>
            <CardTitle className="text-2xl text-emerald-600 dark:text-emerald-400">{activeCount}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Paused (circuit breaker)</CardDescription>
            <CardTitle className={cn("text-2xl", pausedCount > 0 ? "text-red-600 dark:text-red-400" : "")}>
              {pausedCount}
            </CardTitle>
          </CardHeader>
        </Card>
      </section>

      {error ? (
        <Card>
          <CardContent className="py-6 text-sm text-destructive">{error}</CardContent>
        </Card>
      ) : loading ? (
        <Card>
          <CardContent className="py-6 text-sm text-muted-foreground">Loading pools…</CardContent>
        </Card>
      ) : visiblePairs.length === 0 ? (
        <Card>
          <CardContent className="py-6 text-sm text-muted-foreground">
            {pairs.length === 0
              ? "No liquidity pools have been created on the hub yet."
              : `No pools include your national currency${nationalCurrency ? ` (${nationalCurrency})` : ""} yet. Propose a pair in Liquidity Provisioning.`}
          </CardContent>
        </Card>
      ) : (
        <section className="grid gap-4 lg:grid-cols-[minmax(0,20rem)_1fr]">
          {/* Pool list */}
          <div className="space-y-2">
            {visiblePairs.map((p) => {
              const { a, b } = pairLabel(p);
              const breaker = breakerByPair[p.pair_id];
              const isSelected = p.pair_id === selectedPairId;
              const paused = breaker?.state === "HALTED";
              return (
                <button
                  key={p.pair_id}
                  type="button"
                  onClick={() => setSelectedPairId(p.pair_id)}
                  className={cn(
                    "w-full rounded-lg border p-3 text-left transition-colors",
                    isSelected
                      ? "border-primary bg-primary/5 shadow-sm"
                      : "border-border hover:border-primary/40 hover:bg-accent",
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-1.5 font-medium">
                      {a} <ArrowLeftRight className="h-3.5 w-3.5 text-muted-foreground" /> {b}
                    </span>
                    <Badge variant={poolStatusVariant(p.status)}>{poolStatusLabel(p.status)}</Badge>
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                    {paused ? (
                      <span className="flex items-center gap-1 text-red-600 dark:text-red-400">
                        <ShieldAlert className="h-3.5 w-3.5" /> Paused
                      </span>
                    ) : breaker?.state === "LIVE" ? (
                      <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                        <ShieldCheck className="h-3.5 w-3.5" /> Live
                      </span>
                    ) : (
                      <span>{breaker?.state ?? "…"}</span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>

          {/* Detail panel */}
          {selected ? (
            <PoolDetail
              pair={selected}
              label={pairLabel(selected)}
              pool={selectedPool}
              breaker={selectedBreaker}
              tokenDecimals={tokenDecimals}
            />
          ) : null}
        </section>
      )}
    </div>
  );
}

function PoolDetail({
  pair,
  label,
  pool,
  breaker,
  tokenDecimals,
}: {
  pair: HubPair;
  label: { a: string; b: string };
  pool?: PoolStatus;
  breaker?: CircuitBreakerStatus;
  tokenDecimals: number;
}) {
  const bv = breakerView(breaker?.state);
  const rA = reserveNumber(pool?.reserve_a ?? "0", tokenDecimals);
  const rB = reserveNumber(pool?.reserve_b ?? "0", tokenDecimals);
  const total = rA + rB;
  const pctA = total > 0 ? Math.round((rA / total) * 100) : 0;

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-2xl">
            {label.a} <ArrowLeftRight className="h-5 w-5 text-muted-foreground" /> {label.b}
          </CardTitle>
          <Badge variant={poolStatusVariant(pair.status)}>{poolStatusLabel(pair.status)}</Badge>
        </div>
        <CardDescription className="break-all">Pair id: {pair.pair_id}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Circuit breaker banner */}
        <div
          className={cn(
            "flex items-start gap-3 rounded-lg border p-3",
            breaker?.state === "HALTED"
              ? "border-red-500/30 bg-red-500/5"
              : breaker?.state === "LIVE"
                ? "border-emerald-500/30 bg-emerald-500/5"
                : "border-border bg-muted/30",
          )}
        >
          <bv.Icon className={cn("mt-0.5 h-5 w-5 shrink-0", bv.tone)} />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium">Circuit breaker</span>
              <Badge variant={bv.variant}>{bv.label}</Badge>
            </div>
            <p className="mt-0.5 text-sm text-muted-foreground">{bv.blurb}</p>
            {breaker?.state === "HALTED" ? (
              <p className="mt-1 text-xs text-muted-foreground">
                {breaker.pause_reason ? `Reason: ${breaker.pause_reason}. ` : ""}
                {breaker.pause_initiator ? `Initiated by ${breaker.pause_initiator}.` : ""}
              </p>
            ) : null}
          </div>
        </div>

        {/* Reserves + balance bar */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium">Reserves</span>
            {pool?.imbalance_flag ? <Badge variant="warning">Imbalanced</Badge> : null}
          </div>
          <Progress value={pctA} className="h-2" />
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>
              {label.a}: <span className="font-mono">{pool ? formatTokenAmount(pool.reserve_a, tokenDecimals) : "—"}</span>
            </span>
            <span>
              {label.b}: <span className="font-mono">{pool ? formatTokenAmount(pool.reserve_b, tokenDecimals) : "—"}</span>
            </span>
          </div>
        </div>

        <Separator />

        {/* Metrics grid */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Metric label="Current ratio" value={pool?.current_ratio ?? "—"} />
          <Metric
            label="Fee rate"
            value={pool?.fee_rate_bps != null ? `${(pool.fee_rate_bps / 100).toFixed(2)}%` : "—"}
          />
          <Metric label="Liquidity providers" value={pool?.total_lp_count != null ? String(pool.total_lp_count) : "—"} />
          <Metric label="Pool state" value={poolStatusLabel(pool?.pool_status ?? pair.status)} />
          <Metric label="Last updated" value={relativeTime(pool?.updated_at)} />
        </div>

        <Separator />

        {/* On-chain references */}
        <div className="space-y-2 text-sm">
          <AddressRow label="AMM contract" value={pair.amm_address} />
          <AddressRow label={`${label.a} token`} value={pair.token_a_address} />
          <AddressRow label={`${label.b} token`} value={pair.token_b_address} />
          {pair.proposer_cb ? <KeyVal label="Proposer CB" value={pair.proposer_cb} /> : null}
          {pair.confirmer_cb ? <KeyVal label="Confirmer CB" value={pair.confirmer_cb} /> : null}
        </div>
      </CardContent>
    </Card>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border bg-muted/20 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-0.5 font-mono text-sm font-medium">{value}</p>
    </div>
  );
}

function KeyVal({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}

function AddressRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-muted-foreground">{label}</span>
      <button
        type="button"
        onClick={() => void copyToClipboard(value, label)}
        className="flex items-center gap-1 font-mono text-xs hover:text-primary"
        title={value}
      >
        {truncate(value)}
        <Copy className="h-3 w-3" />
      </button>
    </div>
  );
}
