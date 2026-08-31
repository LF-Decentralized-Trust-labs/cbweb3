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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Separator,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import type { LiquidityRequestType, OnRampRequestStatus } from "../types";
import { useTokenStore } from "../stores";

const txStatusVariant = (status: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  const variants: Record<string, "warning" | "default" | "success" | "destructive"> = {
    PENDING: "warning",
    LOCKED: "default",
    SETTLED: "success",
    FAILED: "destructive",
    CONFIRMED: "success",
  };
  return variants[status] ?? "outline";
};

const requestStatusVariant = (status: OnRampRequestStatus): "warning" | "default" | "success" | "destructive" | "outline" => {
  const variants: Record<OnRampRequestStatus, "warning" | "default" | "success" | "destructive"> = {
    PENDING: "warning",
    APPROVED: "default",
    REJECTED: "destructive",
    COMPLETED: "success",
  };
  return variants[status] ?? "outline";
};

export function LiquidityTransfersPage() {
  const { balance, transactions, onRampRequests, fetch, requestOnRamp, transfer, status, error } = useTokenStore();
  const [toAddress, setToAddress] = useState("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed");
  const [transferAmount, setTransferAmount] = useState("250");

  const [requestType, setRequestType] = useState<LiquidityRequestType>("ON_RAMP");
  const [requestAmount, setRequestAmount] = useState("1000");
  const [requestProof, setRequestProof] = useState("reserve-proof-001");
  const [requestJustification, setRequestJustification] = useState("Operational liquidity adjustment for settlement window.");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Public Balance</CardDescription>
            <CardTitle>{balance?.publicBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="secondary">Available</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Private Balance</CardDescription>
            <CardTitle>{balance?.privateBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">View-key protected</Badge>
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Bank Scope Notice</CardTitle>
          <CardDescription>
            Treasury controls tCeBM issuance. Bank Portal supports transfers and liquidity adjustment requests.
          </CardDescription>
        </CardHeader>
      </Card>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Transfer tCeBM</CardTitle>
            <CardDescription>Execute public/shielded transfers from existing liquidity</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <Label htmlFor="transfer-to">Recipient address</Label>
              <Input id="transfer-to" value={toAddress} onChange={(event) => setToAddress(event.target.value)} placeholder="Recipient address" />
              <Label htmlFor="transfer-amount">Amount</Label>
              <Input id="transfer-amount" value={transferAmount} onChange={(event) => setTransferAmount(event.target.value)} placeholder="Amount" />
              <div className="flex items-center gap-2">
                <Badge variant="outline">Shielded</Badge>
                <Badge variant="secondary">ZK Ready</Badge>
              </div>
              <Button
                variant="outline"
                onClick={() => void transfer({ toAddress, amount: transferAmount, shielded: true })}
                disabled={status === "loading"}
              >
                Transfer (Shielded)
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Liquidity Adjustment Request</CardTitle>
            <CardDescription>Submit on-ramp/off-ramp request to Treasury</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <Label htmlFor="request-type">Request type</Label>
              <Select value={requestType} onValueChange={(value) => setRequestType(value as LiquidityRequestType)}>
                <SelectTrigger id="request-type">
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ON_RAMP">On-Ramp</SelectItem>
                  <SelectItem value="OFF_RAMP">Off-Ramp</SelectItem>
                </SelectContent>
              </Select>

              <Label htmlFor="request-amount">Amount</Label>
              <Input id="request-amount" value={requestAmount} onChange={(event) => setRequestAmount(event.target.value)} placeholder="Amount" />

              <Label htmlFor="request-proof">Fiat proof reference</Label>
              <Input id="request-proof" value={requestProof} onChange={(event) => setRequestProof(event.target.value)} placeholder="Reserve proof reference" />

              <Label htmlFor="request-justification">Justification</Label>
              <Textarea
                id="request-justification"
                value={requestJustification}
                onChange={(event) => setRequestJustification(event.target.value)}
                placeholder="Explain operational need"
              />

              <Separator />

              <Button
                onClick={() =>
                  void requestOnRamp({
                    type: requestType,
                    amount: requestAmount,
                    fiatProofRef: requestProof,
                    justification: requestJustification,
                  })
                }
                disabled={status === "loading"}
              >
                Submit Request
              </Button>

              <p className="text-xs text-muted-foreground">This submission creates a request. Treasury reviews and processes it.</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Balances</CardTitle>
            <CardDescription>Current spendable liquidity snapshot</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm">Public: {balance?.publicBalance ?? "-"}</p>
            <p className="text-sm">Private: {balance?.privateBalance ?? "-"}</p>
            {error ? <p className="pt-2 text-sm text-red-600">{error}</p> : null}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Liquidity Requests</CardTitle>
          <CardDescription>Track submitted on-ramp/off-ramp requests</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Submitted</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {onRampRequests.map((request) => (
                <TableRow key={request.id}>
                  <TableCell>{request.id}</TableCell>
                  <TableCell>{request.type}</TableCell>
                  <TableCell>{request.amount}</TableCell>
                  <TableCell>
                    <Badge variant={requestStatusVariant(request.status)}>{request.status}</Badge>
                  </TableCell>
                  <TableCell>{new Date(request.createdAt).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Token Transactions</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Type</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {transactions.map((tx) => (
                <TableRow key={tx.id}>
                  <TableCell>{tx.kind}</TableCell>
                  <TableCell>{tx.amount}</TableCell>
                  <TableCell>
                    <Badge variant={txStatusVariant(tx.status)}>{tx.status}</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
