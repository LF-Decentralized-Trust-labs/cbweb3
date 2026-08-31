// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useFxAgreementStore } from "../stores/fx-agreement.store";
import type { FXAgreementState } from "../types";

const shortState = (state: FXAgreementState) => {
  const s = state.replace("FX_STATE_", "");
  if (s === "PROPOSED") return "Proposed";
  if (s === "ACCEPTED") return "Accepted";
  if (s === "REJECTED") return "Rejected";
  if (s === "CANCELLED") return "Cancelled";
  if (s === "SETTLED") return "Settled";
  return s;
};

const statusVariant = (state: FXAgreementState): "warning" | "success" | "destructive" | "outline" => {
  if (state === "FX_STATE_PROPOSED") return "warning";
  if (state === "FX_STATE_ACCEPTED") return "success";
  if (state === "FX_STATE_REJECTED") return "destructive";
  if (state === "FX_STATE_CANCELLED") return "outline";
  if (state === "FX_STATE_SETTLED") return "success";
  return "outline";
};

const truncate = (id: string | undefined, len = 12) => {
  if (!id) return "—";
  return id.length > len ? `${id.slice(0, len)}...` : id;
};

export function AgreementInboxPage() {
  const navigate = useNavigate();
  const fetchAll = useFxAgreementStore((s) => s.fetchAll);
  const agreements = useFxAgreementStore((s) => s.agreements);
  const status = useFxAgreementStore((s) => s.status);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Trade Agreements</h1>
          <p className="text-sm text-muted-foreground">View and manage FX trade agreements.</p>
        </div>
        <Button asChild>
          <Link to="/agreements/new">New Agreement</Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Agreements</CardTitle>
          <CardDescription>Total: {agreements.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Trade ID</TableHead>
                <TableHead>Originator</TableHead>
                <TableHead>Send</TableHead>
                <TableHead>Receive</TableHead>
                <TableHead>Rate</TableHead>
                <TableHead>State</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agreements.map((a) => (
                <TableRow key={a.trade_id} className="cursor-pointer" onClick={() => navigate(`/agreements/${a.trade_id}`)}>
                  <TableCell className="font-medium">{truncate(a.trade_id)}</TableCell>
                  <TableCell>{truncate(a.originator ?? a.counterparty_b)}</TableCell>
                  <TableCell>
                    {a.origin_amount} {a.origin_currency}
                  </TableCell>
                  <TableCell>
                    {a.counter_amount} {a.counter_currency}
                  </TableCell>
                  <TableCell>{a.rate}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(a.state)}>{shortState(a.state)}</Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); navigate(`/agreements/${a.trade_id}`); }}>
                      View
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {status === "loading" ? <p className="pt-3 text-sm text-muted-foreground">Loading...</p> : null}
          {status !== "loading" && !agreements.length ? (
            <p className="pt-3 text-sm text-muted-foreground">No trade agreements found.</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
