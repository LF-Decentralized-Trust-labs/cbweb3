import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label, toast } from "@cbweb3/ui";
import type { AsyncStatus } from "../../../types";

type Step4CompleteProps = {
  completionStatus: AsyncStatus;
  error: string | null;
  clientSecret: string | null;
  walletAddress: string | null;
  txHash: string | null;
  accessToken: string | null;
  pkiLoginError: string | null;
  onGoDashboard: () => void;
};

export function Step4Complete({
  completionStatus,
  error,
  clientSecret,
  walletAddress,
  txHash,
  accessToken,
  pkiLoginError,
  onGoDashboard,
}: Step4CompleteProps) {
  const copySecret = async () => {
    if (!clientSecret) return;
    await navigator.clipboard.writeText(clientSecret);
    toast.success("Client secret copied");
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Step 4: Complete Onboarding</CardTitle>
        <CardDescription>Finalizing onboarding and issuing your credentials.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {completionStatus === "loading" ? <p className="text-sm text-muted-foreground">Completing onboarding...</p> : null}
        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        {clientSecret ? (
          <div className="space-y-3 rounded-md border border-destructive/40 bg-destructive/5 p-3">
            <p className="text-sm font-medium text-destructive">Save this client secret now. It will not be shown again.</p>
            <Label htmlFor="client_secret">Client Secret</Label>
            <div className="flex gap-2">
              <Input id="client_secret" value={clientSecret} readOnly />
              <Button variant="outline" onClick={() => void copySecret()}>
                Copy
              </Button>
            </div>
          </div>
        ) : null}

        <div className="grid gap-3 md:grid-cols-2">
          <div className="space-y-1 rounded-md border border-border p-3 text-sm">
            <p className="text-muted-foreground">Wallet Address</p>
            <p className="break-all font-medium">{walletAddress ?? "—"}</p>
          </div>
          <div className="space-y-1 rounded-md border border-border p-3 text-sm">
            <p className="text-muted-foreground">Transaction Hash</p>
            <p className="break-all font-medium">{txHash ?? "—"}</p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {accessToken ? <Badge variant="default">PKI login validated</Badge> : null}
          {pkiLoginError ? <Badge variant="warning">PKI login pending</Badge> : null}
        </div>

        {pkiLoginError ? <p className="text-sm text-muted-foreground">{pkiLoginError}</p> : null}

        <Button onClick={onGoDashboard} disabled={completionStatus === "loading" || !clientSecret}>
          Go to Dashboard
        </Button>
      </CardContent>
    </Card>
  );
}