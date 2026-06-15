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
import { usePaymentStore } from "../../stores";
import { liquidityApi } from "../../services/api/liquidity.api";
import { paymentApi } from "../../services/api";
import type { LpBalanceResponse, PendingCommit } from "../../types/liquidity.types";
import type { BalanceResponse } from "../../types/payment.types";
import { currencyFromTokenSymbol, formatCeBM, formatTokenAmount } from "../../types";
import type { MatchContext } from "./CooperativeLiquidityWizard";
import { CooperativeLiquidityWizard } from "./CooperativeLiquidityWizard";
import { classifyPoolSides, formatRemainingMs, poolSideInfo, remainingMsUntil, sideRoleLabel, truncateAddress } from "./format";
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
  const tokenDecimals = usePaymentStore((state) => state.tokenDecimals) ?? 18;

  const [removeLpId, setRemoveLpId] = useState("");
  const [removePair, setRemovePair] = useState(configuredPoolPair);
  const [removeProviderId, setRemoveProviderId] = useState("central-bank-a");

  // const [approveAmount, setApproveAmount] = useState("");
  // const [recipient, setRecipient] = useState("");
  // const [showMintApprove, setShowMintApprove] = useState(false);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3>(1);
  const [bannerCommit, setBannerCommit] = useState<PendingCommit | null>(null);
  const [matchContext, setMatchContext] = useState<MatchContext | null>(null);

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

  // On-chain CB position (013-amm-lp-shares): live CBW3-LP shares + the CB's own tCeBM balance.
  const [lpBalance, setLpBalance] = useState<LpBalanceResponse | null>(null);
  const [cbTokenBalance, setCbTokenBalance] = useState<BalanceResponse | null>(null);
  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const lp = await liquidityApi.getLpBalance();
        if (active) setLpBalance(lp);
      } catch {
        // endpoint optional (older gateways) — card shows "-"
      }
      try {
        const bal = await paymentApi.getBalance();
        if (active) setCbTokenBalance(bal);
      } catch {
        // payment-orchestrator unavailable — card shows "-"
      }
    };
    void load();
    const id = window.setInterval(load, 15000);
    return () => {
      active = false;
      window.clearInterval(id);
    };
  }, []);

  // National currency is sourced on-chain from the CB's own tCeBM symbol (falls back to env
  // VITE_FIAT_SYMBOL until the balance loads); the foreign side comes from the pair id. This
  // classifies each pool side as National/Foreign relative to the viewing Central Bank.
  const nationalCurrency = currencyFromTokenSymbol(cbTokenBalance?.symbol);
  const poolSides = classifyPoolSides(configuredPoolPair, nationalCurrency);

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
    setMatchContext(null);
  };

  const handleMonitorPendingCommit = () => {
    if (!ownPendingCommit) {
      return;
    }
    setBannerCommit(ownPendingCommit);
    setMatchContext(null);
    setWizardStep(3);
    setWizardOpen(true);
  };

  const handleMatchCounterpart = () => {
    if (!counterpartCommit) {
      return;
    }
    // Route the matcher through Lock-Mint first — their currency differs, so they must
    // mint their own side's tokens before committing. Pre-fill the FX-suggested amount
    // (editable); fall back to free entry when no oracle rate is available.
    setMatchContext({
      poolPair: configuredPoolPair,
      suggestedAmount: counterpartCommit.suggested_match_amount ?? "",
      counterpartAmount: counterpartCommit.amount,
      counterpartSide: counterpartCommit.side,
    });
    setBannerCommit(null);
    setWizardStep(1);
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
    setMatchContext(null);
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
          <p className="text-sm">Reserve — {sideRoleLabel(poolSides.a, { fallbackLabel: "Token A" })}: {poolStatus?.reserve_a ? formatTokenAmount(poolStatus.reserve_a, tokenDecimals) : "-"}</p>
          <p className="text-sm">Reserve — {sideRoleLabel(poolSides.b, { fallbackLabel: "Token B" })}: {poolStatus?.reserve_b ? formatTokenAmount(poolStatus.reserve_b, tokenDecimals) : "-"}</p>
          <p className="text-sm">Current ratio: {poolStatus?.current_ratio ?? "-"}</p>
          <p className="text-xs text-muted-foreground">Updated at: {poolStatus?.updated_at ?? "-"}</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Central Bank On-Chain Position</CardTitle>
          <CardDescription>
            Live from the Hub AMM contract — LP shares (CBW3-LP) are the on-chain source of truth for
            pool ownership.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 sm:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">LP shares (CBW3-LP)</p>
            <p className="text-sm font-medium">
              {lpBalance ? formatTokenAmount(lpBalance.lp_shares, tokenDecimals) : "-"}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Pool ownership</p>
            <p className="text-sm font-medium">
              {lpBalance ? `${lpBalance.share_percentage.toFixed(2)}%` : "-"}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">tCeBM balance</p>
            <p className="text-sm font-medium">
              {cbTokenBalance
                ? formatCeBM(cbTokenBalance.balance, cbTokenBalance.decimals ?? tokenDecimals, cbTokenBalance.symbol)
                : "-"}
            </p>
          </div>
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
              <p className="text-sm">Both sides are funded. Reserves {sideRoleLabel(poolSides.a, { short: true, fallbackLabel: "A" })} {poolStatus?.reserve_a ? formatTokenAmount(poolStatus.reserve_a, tokenDecimals) : "-"} / {sideRoleLabel(poolSides.b, { short: true, fallbackLabel: "B" })} {poolStatus?.reserve_b ? formatTokenAmount(poolStatus.reserve_b, tokenDecimals) : "-"}.</p>
            </div>
          ) : counterpartCommit ? (
            <div className="space-y-3 rounded border border-amber-300 bg-amber-50 p-3">
              <Badge variant="warning">Counterpart waiting on you</Badge>
              <p className="text-sm">
                A counterpart central bank committed <strong>{formatTokenAmount(counterpartCommit.amount, tokenDecimals)} tCeBM</strong> to the{" "}
                <strong>{sideRoleLabel(poolSideInfo(counterpartCommit.side, configuredPoolPair, nationalCurrency), { fallbackLabel: `side ${counterpartCommit.side}` })}</strong> side and is waiting for your matching deposit.
              </p>
              {counterpartCommit.suggested_match_amount ? (
                <p className="text-sm">
                  Suggested match on your side (at current FX rate):{" "}
                  <strong>{formatTokenAmount(counterpartCommit.suggested_match_amount, tokenDecimals)} tCeBM</strong>
                </p>
              ) : (
                <p className="text-xs text-muted-foreground">
                  No FX rate available — you’ll set your own deposit amount.
                </p>
              )}
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
                Your commit {ownPendingCommit.commit_id} ({sideRoleLabel(poolSideInfo(ownPendingCommit.side, configuredPoolPair, nationalCurrency), { fallbackLabel: `side ${ownPendingCommit.side}` })}) is registered and awaiting a
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
          matchContext={matchContext}
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
                <TableHead>{sideRoleLabel(poolSides.a, { fallbackLabel: "Token A" })}</TableHead>
                <TableHead>{sideRoleLabel(poolSides.b, { fallbackLabel: "Token B" })}</TableHead>
                <TableHead>LP Shares (on-chain)</TableHead>
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
                  <TableCell>{formatTokenAmount(position.token_a_contributed, tokenDecimals)}</TableCell>
                  <TableCell>{formatTokenAmount(position.token_b_contributed, tokenDecimals)}</TableCell>
                  <TableCell>{formatTokenAmount(position.lp_shares, tokenDecimals)}</TableCell>
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
