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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useState } from "react";
import { usePolling } from "../../hooks/usePolling";
import { useAuthStore } from "../../stores/auth.store";
import type { PendingCommit } from "../../types/liquidity.types";
import { CooperativeLiquidityWizard } from "./CooperativeLiquidityWizard";
import { formatRemainingMs, remainingMsUntil, truncateAddress } from "./format";
import { useLiquidityStore } from "./liquidity.store";

const configuredPoolPair = (import.meta.env.VITE_POOL_PAIR ?? "W-BRL-ARS").trim() || "W-BRL-ARS";

export function LiquidityManagementPage() {
  const poolStatus = useLiquidityStore((state) => state.poolStatus);
  const lpPositions = useLiquidityStore((state) => state.lpPositions);
  const status = useLiquidityStore((state) => state.status);
  const error = useLiquidityStore((state) => state.error);
  const fetchPoolStatus = useLiquidityStore((state) => state.fetchPoolStatus);
  const removeLiquidity = useLiquidityStore((state) => state.removeLiquidity);
  // const mintAndApprove = useLiquidityStore((state) => state.mintAndApprove);
  const getOperationalSummary = useLiquidityStore((state) => state.getOperationalSummary);
  const profile = useAuthStore((state) => state.profile);

  const [removeLpId, setRemoveLpId] = useState("");
  const [removePair, setRemovePair] = useState(configuredPoolPair);
  const [removeProviderId, setRemoveProviderId] = useState("central-bank-a");

  // const [approveAmount, setApproveAmount] = useState("");
  // const [recipient, setRecipient] = useState("");
  // const [showMintApprove, setShowMintApprove] = useState(false);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3>(1);
  const [bannerCommit, setBannerCommit] = useState<PendingCommit | null>(null);
  const [prefillCommit, setPrefillCommit] = useState<{ pool_pair: string; amount: string } | null>(null);

  // Tick once per second so the coordination countdowns stay live between 5s polls.
  const [nowMs, setNowMs] = useState(() => Date.now());

  usePolling(
    () => {
      void fetchPoolStatus(configuredPoolPair);
    },
    5000,
    true,
  );

  useEffect(() => {
    const intervalId = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(intervalId);
  }, []);

  const handleRemoveLiquidity = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await removeLiquidity({
      lp_id: removeLpId,
      pool_pair: removePair,
      provider_bank_id: removeProviderId,
    });
  };

  // const handleMintAndApprove = async (event: FormEvent<HTMLFormElement>) => {
  //   event.preventDefault();
  //   await mintAndApprove({
  //     amount: approveAmount,
  //     recipient: recipient.trim() || undefined,
  //   });
  // };

  const providerId = profile?.bankId ?? "";
  const poolActive = poolStatus?.pool_status === "ACTIVE";
  // Your own open commit (local DB, keyed by provider) — waiting for a counterpart.
  const ownPendingCommit =
    !poolActive
      ? (poolStatus?.pending_commits ?? []).find((commit) => commit.provider_id === providerId) ?? null
      : null;
  // A counterpart CB's on-chain commit on the opposite side — waiting for you to match.
  const counterpartCommit = !poolActive ? poolStatus?.counterpart_commit ?? null : null;

  const handleOpenWizard = () => {
    setWizardOpen(true);
    setWizardStep(1);
    setBannerCommit(null);
    setPrefillCommit(null);
  };

  const handleMonitorPendingCommit = () => {
    if (!ownPendingCommit) {
      return;
    }
    setBannerCommit(ownPendingCommit);
    setPrefillCommit(null);
    setWizardStep(3);
    setWizardOpen(true);
  };

  const handleMatchCounterpart = () => {
    if (!counterpartCommit) {
      return;
    }
    // Pre-fill the Commit step with the counterpart's pool pair and a matching amount.
    setPrefillCommit({ pool_pair: configuredPoolPair, amount: counterpartCommit.amount });
    setBannerCommit(null);
    setWizardStep(2);
    setWizardOpen(true);
  };

  const handleSelectLpForRemoval = (lpId: string, poolPair: string, provider: string) => {
    setRemoveLpId(lpId);
    setRemovePair(poolPair);
    setRemoveProviderId(provider || providerId || "central-bank-a");
  };

  const handleWizardDone = () => {
    setWizardOpen(false);
    setWizardStep(1);
    setBannerCommit(null);
    setPrefillCommit(null);
  };

  const summary = getOperationalSummary();

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Pool Status</CardTitle>
          <CardDescription>Real-time reserve and ratio state for {configuredPoolPair} pool.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {poolStatus?.imbalance_flag ? (
            <Badge variant="warning">Pool imbalance detected</Badge>
          ) : (
            <Badge variant="success">Pool balanced</Badge>
          )}
          <p className="text-sm">Reserve A: {poolStatus?.reserve_a ?? "-"}</p>
          <p className="text-sm">Reserve B: {poolStatus?.reserve_b ?? "-"}</p>
          <p className="text-sm">Current ratio: {poolStatus?.current_ratio ?? "-"}</p>
          <p className="text-xs text-muted-foreground">Updated at: {poolStatus?.updated_at ?? "-"}</p>
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Sovereign Phase</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">{summary.phase}</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Commit Status</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={summary.commitStatus === "EXECUTED" ? "success" : "outline"}>
              {summary.commitStatus}
            </Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Recommended Action</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              {summary.hint ??
                (summary.poolStatus === "ACTIVE"
                  ? "Pool is ACTIVE. Commercial swap flow can proceed."
                  : "Wait for bridge/commit progression before activating the pool.")}
            </p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Liquidity Coordination</CardTitle>
          <CardDescription>Cross-CB commit state for {configuredPoolPair}, read live from the Hub.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {poolActive ? (
            <div className="space-y-1">
              <Badge variant="success">Pool ACTIVE</Badge>
              <p className="text-sm">Both sides are funded. Reserves A {poolStatus?.reserve_a ?? "-"} / B {poolStatus?.reserve_b ?? "-"}.</p>
            </div>
          ) : counterpartCommit ? (
            <div className="space-y-3 rounded border border-amber-300 bg-amber-50 p-3">
              <Badge variant="warning">Counterpart waiting on you</Badge>
              <p className="text-sm">
                A counterpart central bank committed <strong>{counterpartCommit.amount}</strong> to side{" "}
                <strong>{counterpartCommit.side}</strong> and is waiting for your matching deposit.
              </p>
              <p className="text-xs text-muted-foreground">
                Signer {truncateAddress(counterpartCommit.signer_address)} · Expires in{" "}
                {formatRemainingMs(remainingMsUntil(counterpartCommit.expires_at, nowMs))}
              </p>
              <Button onClick={handleMatchCounterpart}>Match & Activate Pool</Button>
            </div>
          ) : ownPendingCommit ? (
            <div className="space-y-3">
              <Badge variant="outline">Waiting for counterpart</Badge>
              <p className="text-sm">
                Your commit {ownPendingCommit.commit_id} (side {ownPendingCommit.side}) is registered and awaiting a
                counterpart deposit.
              </p>
              <p className="text-xs text-muted-foreground">
                Expires in {formatRemainingMs(remainingMsUntil(ownPendingCommit.expires_at, nowMs))}
              </p>
              <Button onClick={handleMonitorPendingCommit}>Monitor Pending Commit</Button>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              No open liquidity intents for this pool. Start a commit to seed your side.
            </p>
          )}
        </CardContent>
      </Card>

      {wizardOpen ? (
        <CooperativeLiquidityWizard
          initialStep={wizardStep}
          pendingCommit={bannerCommit}
          prefillCommit={prefillCommit}
          onDone={handleWizardDone}
          onSelectLpForRemoval={handleSelectLpForRemoval}
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Cooperative Liquidity</CardTitle>
            <CardDescription>Use the guided wizard for Mint & Approve, Commit, Monitor, and Success.</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={handleOpenWizard}>Add Cooperative Liquidity</Button>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Remove Liquidity</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleRemoveLiquidity}>
              <div className="space-y-1">
                <Label htmlFor="lp_id">LP ID</Label>
                <Input id="lp_id" value={removeLpId} onChange={(event) => setRemoveLpId(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="remove_pool_pair">Pool Pair</Label>
                <Input id="remove_pool_pair" value={removePair} onChange={(event) => setRemovePair(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="remove_provider_bank_id">Provider Bank ID</Label>
                <Input id="remove_provider_bank_id" value={removeProviderId} onChange={(event) => setRemoveProviderId(event.target.value)} required />
              </div>
              <Button type="submit" variant="outline" disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Remove Liquidity"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      {/* <Card>
        <CardHeader>
          <CardTitle>MintAndApprove</CardTitle>
          <CardDescription>Pre-fund and approve AMM spending for liquidity operations.</CardDescription>
          <Button variant="outline" onClick={() => setShowMintApprove((value) => !value)}>
            {showMintApprove ? "Hide Panel" : "Show Panel"}
          </Button>
        </CardHeader>
        {showMintApprove ? (
          <CardContent>
            <form className="space-y-3" onSubmit={handleMintAndApprove}>
              <div className="space-y-1">
                <Label htmlFor="approve_amount">Amount</Label>
                <Input id="approve_amount" value={approveAmount} onChange={(event) => setApproveAmount(event.target.value)} inputMode="numeric" pattern="[0-9]+" required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="recipient">Recipient (optional)</Label>
                <Input id="recipient" value={recipient} onChange={(event) => setRecipient(event.target.value)} />
              </div>
              <Button type="submit" disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Submit MintAndApprove"}
              </Button>
            </form>
          </CardContent>
        ) : null}
      </Card> */}

      <Card>
        <CardHeader>
          <CardTitle>LP Positions (Session)</CardTitle>
          <CardDescription>Positions are session-only and populated from Add Liquidity responses.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>LP ID</TableHead>
                <TableHead>Pool Pair</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Token A</TableHead>
                <TableHead>Token B</TableHead>
                <TableHead>LP Shares</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Added At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lpPositions.map((position) => (
                <TableRow key={position.lp_id}>
                  <TableCell className="font-mono text-xs">{position.lp_id}</TableCell>
                  <TableCell>{position.pool_pair}</TableCell>
                  <TableCell>{position.provider_bank_id}</TableCell>
                  <TableCell>{position.token_a_contributed}</TableCell>
                  <TableCell>{position.token_b_contributed}</TableCell>
                  <TableCell>{position.lp_shares}</TableCell>
                  <TableCell>
                    <Badge variant={position.status === "ACTIVE" ? "success" : "outline"}>{position.status}</Badge>
                  </TableCell>
                  <TableCell>{position.added_at}</TableCell>
                </TableRow>
              ))}
              {lpPositions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center text-muted-foreground">
                    No LP positions in this session.
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
          {error === "Position not found or already withdrawn" ? (
            <p className="mt-3 text-sm text-destructive">Position not found or already withdrawn</p>
          ) : null}
          {error && error !== "Position not found or already withdrawn" ? (
            <p className="mt-3 text-sm text-destructive">{error}</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
