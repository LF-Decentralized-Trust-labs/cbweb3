// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

type Step4SuccessProps = {
  walletAddress: string | null;
  onGoDashboard: () => void;
};

export function Step4Success({ walletAddress, onGoDashboard }: Step4SuccessProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Onboarding Complete</CardTitle>
        <CardDescription>Your institution is active and ready to operate.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 flex flex-col items-start">
        <Badge variant="success">ACTIVE</Badge>

        {walletAddress ? (
          <div className="space-y-1 rounded-md border border-border p-3 text-sm">
            <p className="text-muted-foreground">Wallet Address</p>
            <p className="break-all font-medium">{walletAddress}</p>
          </div>
        ) : null}

        <Button onClick={onGoDashboard}>Go to Dashboard</Button>
      </CardContent>
    </Card>
  );
}