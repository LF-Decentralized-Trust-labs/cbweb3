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
} from "@cbweb3/ui";
import { useEffect } from "react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { isScenarioB } from "../config/scenario";
import { useSwapMonitorStore } from "../features/swap-monitor/swap-monitor.store";
import { usePaymentStore } from "../stores";
import { PaymentStatus, normalizePaymentStatus } from "../types";

function statusVariant(status: string): "default" | "warning" | "success" | "destructive" | "outline" {
  if (status === "COMPLETED") {
    return "success";
  }
  if (status === "BRIDGE_OUT_FAILED") {
    return "destructive";
  }
  if (status.endsWith("_PROGRESS") || status === "PENDING") {
    return "warning";
  }
  return "outline";
}

export function SwapMonitorPage() {
  const fetchAllPayments = usePaymentStore((state) => state.fetchAll);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);

  const swapIds = useSwapMonitorStore((state) => state.swapIds);
  const swapRecords = useSwapMonitorStore((state) => state.swapRecords);
  const fetchStatus = useSwapMonitorStore((state) => state.fetchStatus);
  const fetchSwapRecord = useSwapMonitorStore((state) => state.fetchSwapRecord);
  const addSwapId = useSwapMonitorStore((state) => state.addSwapId);

  const [swapIdInput, setSwapIdInput] = useState("");

  useEffect(() => {
    void fetchAllPayments();
  }, [fetchAllPayments]);

  useEffect(() => {
    swapIds.forEach((swapId) => {
      if (!fetchStatus[swapId]) {
        void fetchSwapRecord(swapId);
      }
    });
  }, [fetchStatus, fetchSwapRecord, swapIds]);

  if (!isScenarioB) {
    return null;
  }

  const pendingDeposits = deposits.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;
  const pendingEscrows = escrows.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;
  const pendingRedeems = redeems.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;

  const onTrackSwap = () => {
    const normalized = swapIdInput.trim();
    if (!normalized) {
      return;
    }

    addSwapId(normalized);
    setSwapIdInput("");
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Scenario B Swap Monitor</CardTitle>
          <CardDescription>Read-only operational visibility for pending approvals and tracked swap operations.</CardDescription>
        </CardHeader>
      </Card>

      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Deposits</CardDescription>
            <CardTitle>{pendingDeposits}</CardTitle>
          </CardHeader>
          <CardContent>
            <Link className="text-sm underline" to="/deposits-approval">Open issuance approvals</Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Escrows</CardDescription>
            <CardTitle>{pendingEscrows}</CardTitle>
          </CardHeader>
          <CardContent>
            <Link className="text-sm underline" to="/escrows-approval">Open tokenisation approvals</Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingRedeems}</CardTitle>
          </CardHeader>
          <CardContent>
            <Link className="text-sm underline" to="/redeems-approval">Open redeem approvals</Link>
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Swap History</CardTitle>
          <CardDescription>Track swaps by ID from session operations.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-2 md:grid-cols-[1fr_auto]">
            <div className="space-y-1">
              <Label htmlFor="swap-id-input">Swap ID</Label>
              <Input
                id="swap-id-input"
                value={swapIdInput}
                onChange={(event) => setSwapIdInput(event.target.value)}
                placeholder="swap-op-..."
              />
            </div>
            <div className="flex items-end">
              <Button type="button" onClick={onTrackSwap}>Track Swap</Button>
            </div>
          </div>

          {swapIds.length === 0 ? <p className="text-sm text-muted-foreground">No swap records in this session</p> : null}

          {swapIds.map((swapId) => {
            const status = fetchStatus[swapId] ?? "loading";
            const record = swapRecords[swapId];

            return (
              <Card key={swapId}>
                <CardHeader>
                  <CardTitle className="text-base">Swap {swapId}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-1 text-sm">
                  {status === "error" ? (
                    <p className="text-destructive">Could not load this swap record.</p>
                  ) : null}
                  {status !== "error" && !record ? (
                    <p className="text-muted-foreground">Loading swap record...</p>
                  ) : null}
                  {record ? (
                    <>
                      <p>
                        Status: <Badge variant={statusVariant(record.status)}>{record.status}</Badge>
                      </p>
                      <p>Payer Bank: {record.payer_bank_id}</p>
                      <p>Beneficiary Bank: {record.beneficiary_bank_id}</p>
                      <p>Bridge In Position: {record.bridge_in_position_id ?? "-"}</p>
                      <p>Swap Tx Ref: {record.swap_tx_ref ?? "-"}</p>
                      <p>Bridge Out Position: {record.bridge_out_position_id ?? "-"}</p>
                      <p>Created At: {record.created_at ? new Date(record.created_at).toLocaleString() : "-"}</p>
                      <p>Updated At: {record.updated_at ? new Date(record.updated_at).toLocaleString() : "-"}</p>
                    </>
                  ) : null}
                </CardContent>
              </Card>
            );
          })}
        </CardContent>
      </Card>
    </div>
  );
}
