import * as React from "react";
import * as RechartsPrimitive from "recharts";
import { cn } from "../lib/utils";

export type ChartConfig = {
  [k: string]: {
    label?: React.ReactNode;
    icon?: React.ComponentType;
    color?: string;
  };
};

type ChartContextProps = {
  config: ChartConfig;
};

const ChartContext = React.createContext<ChartContextProps | null>(null);

function useChart() {
  const context = React.useContext(ChartContext);
  if (!context) {
    throw new Error("useChart must be used within a <ChartContainer />");
  }
  return context;
}

function ChartContainer({
  id,
  className,
  children,
  config,
  ...props
}: React.ComponentProps<"div"> & {
  config: ChartConfig;
  children: React.ComponentProps<typeof RechartsPrimitive.ResponsiveContainer>["children"];
}) {
  const uniqueId = React.useId();
  const chartId = `chart-${id || uniqueId.replace(/:/g, "")}`;

  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-chart={chartId}
        className={cn(
          "flex aspect-video justify-center text-xs [&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground [&_.recharts-cartesian-grid_line[stroke='#ccc']]:stroke-border/50 [&_.recharts-curve.recharts-tooltip-cursor]:stroke-border [&_.recharts-default-tooltip]:rounded-lg [&_.recharts-default-tooltip]:border-border [&_.recharts-layer]:outline-none",
          className,
        )}
        {...props}
      >
        <ChartStyle id={chartId} config={config} />
        <RechartsPrimitive.ResponsiveContainer>{children}</RechartsPrimitive.ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  );
}

function ChartStyle({ id, config }: { id: string; config: ChartConfig }) {
  const colorConfig = Object.entries(config).filter(([, conf]) => conf.color);

  if (!colorConfig.length) {
    return null;
  }

  return (
    <style
      dangerouslySetInnerHTML={{
        __html: `
[data-chart=${id}] {
${colorConfig
  .map(([key, conf]) => `  --color-${key}: ${conf.color};`)
  .join("\n")}
}
`,
      }}
    />
  );
}

const ChartTooltip = RechartsPrimitive.Tooltip;

function ChartTooltipContent({
  active,
  payload,
  className,
}: {
  active?: boolean;
  payload?: ReadonlyArray<{
    name?: string | number;
    dataKey?: string | number;
    value?: string | number;
    color?: string;
  }>;
  className?: string;
}) {
  const { config } = useChart();

  if (!active || !payload?.length) {
    return null;
  }

  return (
    <div className={cn("grid min-w-[8rem] items-start gap-1.5 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl", className)}>
      {payload.map((item, index: number) => {
        const key = `${item.name || item.dataKey || "value"}`;
        const conf = config[key] ?? config[item.dataKey as string] ?? config.value;
        return (
          <div key={index} className="flex w-full items-center justify-between gap-2">
            <div className="flex items-center gap-1.5">
              <div
                className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                style={{ backgroundColor: item.color || `var(--color-${item.dataKey})` }}
              />
              <span className="text-muted-foreground">{conf?.label ?? item.name}</span>
            </div>
            <span className="font-mono font-medium text-foreground">{item.value?.toLocaleString?.() ?? item.value}</span>
          </div>
        );
      })}
    </div>
  );
}

const ChartLegend = RechartsPrimitive.Legend;

function ChartLegendContent({ payload }: React.ComponentProps<"div"> & { payload?: any[] }) {
  const { config } = useChart();

  if (!payload?.length) {
    return null;
  }

  return (
    <div className="flex items-center justify-center gap-4 pt-3">
      {payload.map((item) => {
        const conf = config[item.dataKey] ?? config[item.value];
        return (
          <div key={item.value} className="flex items-center gap-1.5">
            <div className="h-2.5 w-2.5 rounded-[2px]" style={{ backgroundColor: item.color }} />
            <span className="text-xs text-muted-foreground">{conf?.label ?? item.value}</span>
          </div>
        );
      })}
    </div>
  );
}

export {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
  ChartStyle,
};
