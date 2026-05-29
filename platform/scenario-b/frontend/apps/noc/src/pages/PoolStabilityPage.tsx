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
  const { pools, status, fetch } = usePoolStability();

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
      <CardContent>
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
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
