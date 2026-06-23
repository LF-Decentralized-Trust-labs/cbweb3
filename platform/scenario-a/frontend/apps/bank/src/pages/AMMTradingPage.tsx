// SPDX-License-Identifier: Apache-2.0

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
  Progress,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useAmmStore } from "../stores";

export function AMMTradingPage() {
  const { pool, quote, refreshPool, getQuote, swap, status, error } = useAmmStore();
  const [tokenIn, setTokenIn] = useState("BRL-tCeBM");
  const [tokenOut, setTokenOut] = useState("ARS-tCeBM");
  const [exactOutputAmount, setExactOutputAmount] = useState("1000");

  useEffect(() => {
    void refreshPool();
  }, [refreshPool]);

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{pool?.tokenA ?? "Token A"}</CardDescription>
            <CardTitle>{pool?.reserveA ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{pool?.tokenB ?? "Token B"}</CardDescription>
            <CardTitle>{pool?.reserveB ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Imbalance</CardDescription>
            <CardTitle>{pool?.imbalanceFlag ? "Alert" : "Normal"}</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={pool?.imbalanceFlag ? "warning" : "success"}>{pool?.imbalanceFlag ? "70/30+" : "Balanced"}</Badge>
          </CardContent>
        </Card>
      </section>

      <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Automated FX Desk</CardTitle>
          <CardDescription>Liquidity pool-based FX workflow</CardDescription>
        </CardHeader>
        <CardContent>
        <Tabs defaultValue="quote">
          <TabsList>
            <TabsTrigger value="quote">Quote</TabsTrigger>
            <TabsTrigger value="execute">Execute</TabsTrigger>
          </TabsList>
          <TabsContent value="quote" className="space-y-2">
            <Label>Token In</Label>
            <Input value={tokenIn} onChange={(event) => setTokenIn(event.target.value)} placeholder="Token In" />
            <Label>Token Out</Label>
            <Input value={tokenOut} onChange={(event) => setTokenOut(event.target.value)} placeholder="Token Out" />
            <Label>Exact Output</Label>
            <Input value={exactOutputAmount} onChange={(event) => setExactOutputAmount(event.target.value)} placeholder="Exact output amount" />
            <Button onClick={() => void getQuote({ tokenIn, tokenOut, exactOutputAmount })} disabled={status === "loading"}>
              Get Quote
            </Button>
          </TabsContent>
          <TabsContent value="execute" className="space-y-2">
            <p className="text-sm text-muted-foreground">Review quote before execution.</p>
            <Button variant="outline" onClick={() => void swap({ tokenIn, tokenOut, exactOutputAmount })} disabled={status === "loading"}>
              Execute Swap
            </Button>
          </TabsContent>
        </Tabs>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Quote & Slippage</CardTitle>
        </CardHeader>
        <CardContent>
        <p className="text-sm">Required input: {quote?.requiredInputAmount ?? "-"}</p>
        <p className="text-sm">Price impact: {quote?.priceImpactPct ?? "-"}%</p>
        <p className="text-sm">Slippage: {quote?.slippagePct ?? "-"}%</p>
        <div className="pt-2">
          <p className="mb-1 text-xs text-muted-foreground">Impact indicator</p>
          <Progress value={Math.min(Number(quote?.priceImpactPct ?? 0), 100)} />
        </div>
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        </CardContent>
      </Card>
      </div>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Pool Status</CardTitle>
        </CardHeader>
        <CardContent>
        <p className="text-sm">{pool?.tokenA ?? "-"} reserve: {pool?.reserveA ?? "-"}</p>
        <p className="text-sm">{pool?.tokenB ?? "-"} reserve: {pool?.reserveB ?? "-"}</p>
        <p className="text-sm">Imbalance alert: {pool?.imbalanceFlag ? "true" : "false"}</p>
        </CardContent>
      </Card>
    </div>
  );
}
