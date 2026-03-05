import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useStabilityStore } from "../stores";

export function LiquidityMonitorPage() {
  const { pools, alerts, circuitBreakerActive, status, refresh, toggleCircuitBreaker, updateConfig, error } = useStabilityStore();
  const [reason, setReason] = useState("");
  const [feeBps, setFeeBps] = useState(30);
  const [slippageBps, setSlippageBps] = useState(50);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Scenario B · Liquidity Stability Monitor</CardTitle>
          <CardDescription>Monitor pool ratios and trigger emergency controls when thresholds are exceeded.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Pair</TableHead>
                <TableHead>Reserve A</TableHead>
                <TableHead>Reserve B</TableHead>
                <TableHead>Ratio</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pools.map((pool) => (
                <TableRow key={pool.pair}>
                  <TableCell>{pool.pair}</TableCell>
                  <TableCell>{pool.reserveA.toLocaleString()}</TableCell>
                  <TableCell>{pool.reserveB.toLocaleString()}</TableCell>
                  <TableCell>{pool.ratioA}/{pool.ratioB}</TableCell>
                  <TableCell>
                    <Badge variant={pool.isImbalanced ? "destructive" : "success"}>{pool.isImbalanced ? "IMBALANCED" : "HEALTHY"}</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Global Circuit Breaker</CardTitle>
            <CardDescription>Pause or resume swap functionality with an auditable reason.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm">Current status: <Badge variant={circuitBreakerActive ? "destructive" : "success"}>{circuitBreakerActive ? "PAUSED" : "ACTIVE"}</Badge></p>
            <div className="space-y-2">
              <Label htmlFor="cb-reason">Reason</Label>
              <Textarea id="cb-reason" value={reason} onChange={(event) => setReason(event.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button
                disabled={status === "loading" || reason.length < 5}
                onClick={async () => {
                  await toggleCircuitBreaker({ action: "PAUSE", reason });
                  toast("Circuit breaker enabled");
                }}
              >
                Pause Swaps
              </Button>
              <Button
                variant="outline"
                disabled={status === "loading" || reason.length < 5}
                onClick={async () => {
                  await toggleCircuitBreaker({ action: "RESUME", reason });
                  toast("Circuit breaker disabled");
                }}
              >
                Resume Swaps
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>AMM Parameter Tuning</CardTitle>
            <CardDescription>Adjust fee and slippage controls exposed by governance API.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="fee-bps">Fee (bps)</Label>
                <Input id="fee-bps" type="number" value={feeBps} onChange={(event) => setFeeBps(Number(event.target.value))} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="slippage-bps">Slippage (bps)</Label>
                <Input
                  id="slippage-bps"
                  type="number"
                  value={slippageBps}
                  onChange={(event) => setSlippageBps(Number(event.target.value))}
                />
              </div>
            </div>
            <Button
              disabled={status === "loading" || reason.length < 5}
              onClick={async () => {
                await updateConfig({ feeBps, slippageBps, reason });
                toast("AMM configuration updated");
              }}
            >
              Save Configuration
            </Button>
            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Active Alerts</CardTitle>
          <CardDescription>Imbalance notifications generated by monitor rules.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {alerts.map((alert) => (
            <div key={alert.id} className="rounded-md border border-border p-2 text-sm">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{alert.pair}</span>
                <Badge variant={alert.severity === "CRITICAL" ? "destructive" : "warning"}>{alert.severity}</Badge>
              </div>
              <p className="text-muted-foreground">{alert.message}</p>
            </div>
          ))}
          {!alerts.length ? <p className="text-sm text-muted-foreground">No active alerts.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
