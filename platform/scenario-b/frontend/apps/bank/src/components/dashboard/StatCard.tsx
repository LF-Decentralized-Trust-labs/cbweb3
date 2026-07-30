// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from "react";
import { useLayoutEffect, useRef } from "react";
import { Card, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

type StatCardProps = {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  loading?: boolean;
};

// Font bounds (px) for the auto-fit value. MAX matches Tailwind's text-2xl.
const FIT_MAX_PX = 24;
const FIT_MIN_PX = 12;

// FitValue renders its content on a single line and shrinks the font-size until the
// text fits the available width, so large balances (fiat / tCeBM) never overflow the
// card. It re-measures on container resize (breakpoint changes) and when the value
// changes. DOM style is written directly (no state) to avoid re-render loops.
function FitValue({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLSpanElement>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;

    const fit = () => {
      // Measure natural (unwrapped) width at the max size, then scale down to fit.
      el.style.fontSize = `${FIT_MAX_PX}px`;
      const available = el.clientWidth;
      const needed = el.scrollWidth;
      let size = FIT_MAX_PX;
      if (needed > available && needed > 0) {
        size = Math.max(FIT_MIN_PX, Math.floor((FIT_MAX_PX * available) / needed));
      }
      el.style.fontSize = `${size}px`;
    };

    fit();
    const observer = new ResizeObserver(fit);
    observer.observe(el);
    return () => observer.disconnect();
  }, [children]);

  return (
    <span ref={ref} className="block overflow-hidden whitespace-nowrap">
      {children}
    </span>
  );
}

// StatCard is a compact KPI tile: a description label, a prominent value, and an
// optional smaller hint line beneath it. The value auto-shrinks to fit the card width.
export function StatCard({ label, value, hint, loading = false }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl">{loading ? "…" : <FitValue>{value}</FitValue>}</CardTitle>
        {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      </CardHeader>
    </Card>
  );
}
