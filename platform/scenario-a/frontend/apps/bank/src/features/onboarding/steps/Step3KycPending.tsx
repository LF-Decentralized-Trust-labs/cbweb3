// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Progress } from "@cbweb3/ui";
import type { AsyncStatus, OnboardingRequestStatus } from "../../../types";

type Step3KycPendingProps = {
  requestId: string;
  walletAddress: string | null;
  requestStatus: OnboardingRequestStatus | null;
  pollingStatus: AsyncStatus;
  elapsedSeconds: number;
  error: string | null;
  onRefresh: () => Promise<void>;
};

const statusProgress: Record<OnboardingRequestStatus, number> = {
  NONE: 0,
  PENDING: 34,
  APPROVED: 67,
  CREDENTIAL_REQUESTED: 34,
  KYC_APPROVED: 67,
  ACTIVE: 100,
  FROZEN: 100,
  REVOKED: 100,
  REJECTED: 100,
};

export function Step3KycPending({
  requestId,
  walletAddress,
  requestStatus,
  pollingStatus,
  elapsedSeconds,
  error,
  onRefresh,
}: Step3KycPendingProps) {
  const status = requestStatus ?? "PENDING";
  const waiting = status === "PENDING" || status === "CREDENTIAL_REQUESTED";

  return (
    <Card>
      <CardHeader>
        <CardTitle>Step 3: KYC Approval</CardTitle>
        <CardDescription>Waiting for Central Bank governance approval.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Current status</span>
            <Badge variant={waiting ? "warning" : "default"}>{status}</Badge>
          </div>
          <Progress value={statusProgress[status]} />
          <p className="text-xs text-muted-foreground">PENDING → APPROVED → ACTIVE</p>
        </div>

        <div className="rounded-md border border-border p-3 text-sm">
          <p>
            <span className="font-medium">Request ID:</span> {requestId}
          </p>
          <p>
            <span className="font-medium">Wallet:</span> {walletAddress ?? "—"}
          </p>
          <p>
            <span className="font-medium">Elapsed:</span> {elapsedSeconds}s
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => void onRefresh()} disabled={pollingStatus === "loading"}>
            {pollingStatus === "loading" ? "Refreshing..." : "Refresh status"}
          </Button>
          {waiting ? <p className="text-xs text-muted-foreground">Polling every 5 seconds.</p> : null}
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}