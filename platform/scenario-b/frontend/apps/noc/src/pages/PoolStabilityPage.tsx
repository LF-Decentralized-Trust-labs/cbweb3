// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Progress,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { usePoolStability } from "../hooks";

export function PoolStabilityPage() {
  const { pools, failures, status, error, fetch } = usePoolStability();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle>Pool Stability (70/30)</CardTitle>
          <CardDescription>Observe reserve ratio drift and trigger operational rebalancing.</CardDescription>
        </div>
        <Button onClick={() => void fetch()} disabled={status === "loading"}>
          Refresh
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        {error && (
          <div className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm">
            <span className="font-medium">Could not reach the NOC backend.</span>{" "}
            <span className="text-muted-foreground">{error}</span>
          </div>
        )}
        {failures.length > 0 && (
          <div className="rounded-md border border-warning/50 bg-warning/10 px-3 py-2 text-sm">
            <p className="font-medium">
              {failures.length === 1 ? "1 pair could not be read" : `${failures.length} pairs could not be read`} from
              the api-gateway — this is a configuration or connectivity problem, not an empty AMM.
            </p>
            <ul className="mt-1 space-y-0.5 text-xs text-muted-foreground">
              {failures.map((failure) => (
                <li key={failure.pair}>
                  <span className="font-mono">{failure.pair}</span>: {failure.reason}
                </li>
              ))}
            </ul>
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Pool</TableHead>
              <TableHead>Reserve A</TableHead>
              <TableHead>Reserve B</TableHead>
              <TableHead>Split</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Updated At</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pools.map((pool) => (
              <TableRow key={pool.pair}>
                <TableCell className="font-medium">{pool.pair}</TableCell>
                <TableCell>{pool.reserveA}</TableCell>
                <TableCell>{pool.reserveB}</TableCell>
                <TableCell className="w-[220px]">
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">A {pool.ratioA}% / B {pool.ratioB}%</div>
                    <Progress value={pool.ratioA} />
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={pool.breached7030 ? "destructive" : "default"}>
                    {pool.breached7030 ? "BREACHED" : "STABLE"}
                  </Badge>
                </TableCell>
                <TableCell>{new Date(pool.updatedAt).toLocaleString()}</TableCell>
              </TableRow>
            ))}
            {pools.length === 0 && status !== "loading" && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  {failures.length > 0
                    ? "No pool could be read — see the warning above."
                    : "No AMM pools are configured for this deployment."}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
