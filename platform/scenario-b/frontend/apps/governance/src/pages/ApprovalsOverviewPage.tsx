// SPDX-License-Identifier: Apache-2.0

import { Card, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";
import { useEffect } from "react";
import { isScenarioB } from "../config/scenario";
import { usePaymentStore } from "../stores";
import { PaymentStatus, normalizePaymentStatus } from "../types";

/**
 * A roll-up of the approval queues a governance operator has to work through. Counts only: in
 * Scenario B the deposit, escrow and redeem approval screens are not part of this portal, so
 * there is nowhere here to link to and nothing to act on from this page.
 *
 * The cards used to carry links to /deposits-approval, /escrows-approval and /redeems-approval.
 * Those routes exist only in the Scenario A route set, so in Scenario B every one of them fell
 * through to the dashboard. Removed rather than left as three dead links — verified on a live
 * stack.
 *
 * This was the Swap Monitor, which also let an operator track a swap by id. That half never
 * worked: a swap read is answered only to the bank that owns the swap, so a governance session
 * got 403 on every lookup, and the screen reported it as "Could not load this swap record."
 * It was removed rather than given a working read path, because a central bank seeing
 * commercial banks' swaps was decided against.
 */
export function ApprovalsOverviewPage() {
  const fetchAllPayments = usePaymentStore((state) => state.fetchAll);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);

  useEffect(() => {
    void fetchAllPayments();
  }, [fetchAllPayments]);

  if (!isScenarioB) {
    return null;
  }

  const pendingDeposits = deposits.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;
  const pendingEscrows = escrows.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;
  const pendingRedeems = redeems.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Approvals Overview</CardTitle>
          <CardDescription>Read-only counts of the pending approval queues.</CardDescription>
        </CardHeader>
      </Card>

      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Deposits</CardDescription>
            <CardTitle>{pendingDeposits}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Escrows</CardDescription>
            <CardTitle>{pendingEscrows}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingRedeems}</CardTitle>
          </CardHeader>
        </Card>
      </section>
    </div>
  );
}
