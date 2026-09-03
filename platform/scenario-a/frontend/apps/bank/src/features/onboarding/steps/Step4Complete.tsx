// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@cbweb3/ui";
import type { AsyncStatus } from "../../../types";

type Step4CompleteProps = {
  completionStatus: AsyncStatus;
  error: string | null;
  walletAddress: string | null;
  txHash: string | null;
  accessToken: string | null;
  pkiLoginError: string | null;
  onGoDashboard: () => void;
};

export function Step4Complete({
  completionStatus,
  error,
  walletAddress,
  txHash,
  accessToken,
  pkiLoginError,
  onGoDashboard,
}: Step4CompleteProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Step 4: Finalizing Onboarding</CardTitle>
        <CardDescription>
          Central Bank approval received. Final activation is in progress.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {completionStatus === "loading" ? (
          <p className="text-sm text-muted-foreground">
            Completing onboarding...
          </p>
        ) : null}
        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <div
          className={
            walletAddress
              ? "grid gap-3 md:grid-cols-2"
              : "grid gap-3 md:grid-cols-1"
          }
        >
          {walletAddress ? (
            <div className="space-y-1 rounded-md border border-border p-3 text-sm">
              <p className="text-muted-foreground">Wallet Address</p>
              <p className="break-all font-medium">{walletAddress}</p>
            </div>
          ) : null}
          <div className="space-y-1 rounded-md border border-border p-3 text-sm">
            <p className="text-muted-foreground">Transaction Hash</p>
            <p className="break-all font-medium">{txHash ?? "—"}</p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {accessToken ? (
            <Badge variant="default">PKI login validated</Badge>
          ) : null}
          {pkiLoginError ? (
            <Badge variant="warning">PKI login pending</Badge>
          ) : null}
        </div>

        {pkiLoginError ? (
          <p className="text-sm text-muted-foreground">{pkiLoginError}</p>
        ) : null}

        <Button
          onClick={onGoDashboard}
          disabled={completionStatus === "loading"}
        >
          Go to Dashboard
        </Button>
      </CardContent>
    </Card>
  );
}
