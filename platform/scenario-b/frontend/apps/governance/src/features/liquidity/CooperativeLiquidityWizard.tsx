import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label } from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { useAuthStore } from "../../stores/auth.store";
import { usePaymentStore } from "../../stores";
import { SOVEREIGN_FLOW_PHASE } from "../../types/liquidity.types";
import type { CommitResult, CommitSide, PendingCommit } from "../../types/liquidity.types";
import { currencyFromTokenSymbol, displayToBase, formatAmountInput, formatTokenAmount, parseAmountInput } from "../../types";
import { usePolling } from "../../hooks/usePolling";
import { formatRemainingMs, poolSideInfo, sideRoleLabel } from "./format";
import { useLiquidityStore } from "./liquidity.store";

const WIZARD_STEPS = [
  { id: 1, label: "Lock-Mint" },
  { id: 2, label: "Commit" },
  { id: 3, label: "Commit Pending/Executed" },
  { id: 4, label: "Pool Active" },
] as const;

type WizardStep = (typeof WIZARD_STEPS)[number]["id"];

// MatchContext routes the matcher through Lock-Mint first (their currency differs from
// the counterpart, so they must mint their own side's amount before committing). The
// suggested amount is FX-derived and editable; the counterpart figures are reference only.
export type MatchContext = {
  poolPair: string;
  suggestedAmount: string;
  counterpartAmount: string;
  counterpartSide: CommitSide;
};

type CooperativeLiquidityWizardProps = {
  initialStep?: WizardStep;
  pendingCommit?: PendingCommit | null;
  // When set, starts at Lock-Mint with the FX-suggested amount pre-filled (editable),
  // showing the counterpart's open commit as reference. Used by "Match & Activate".
  matchContext?: MatchContext | null;
  onDone?: () => void;
  onSelectLpForRemoval?: (lpId: string, poolPair: string, providerId: string) => void;
};

function toActiveCommitFromPending(pendingCommit: PendingCommit): CommitResult {
  return {
    commit_id: pendingCommit.commit_id,
    on_chain_commit_id: null,
    pool_pair: "W-BRL-ARS",
    side: pendingCommit.side,
    amount: pendingCommit.amount,
    status: "PENDING",
    expires_at: pendingCommit.expires_at,
    lp_ids: null,
  };
}

export function CooperativeLiquidityWizard({
  initialStep = 1,
  pendingCommit = null,
  matchContext = null,
  onDone,
  onSelectLpForRemoval,
}: CooperativeLiquidityWizardProps) {
  const profile = useAuthStore((state) => state.profile);
  const tokenDecimals = usePaymentStore((state) => state.tokenDecimals) ?? 18;
  // National currency from the CB's own (on-chain) tCeBM symbol; env fallback until loaded.
  const nationalCurrency = currencyFromTokenSymbol(usePaymentStore((state) => state.tokenSymbol));
  const status = useLiquidityStore((state) => state.status);
  const error = useLiquidityStore((state) => state.error);
  const poolStatus = useLiquidityStore((state) => state.poolStatus);
  const activeCommit = useLiquidityStore((state) => state.activeCommit);
  const commitStatus = useLiquidityStore((state) => state.commitStatus);
  const commitError = useLiquidityStore((state) => state.commitError);
  const sovereignPhase = useLiquidityStore((state) => state.sovereignPhase);
  const operationalHint = useLiquidityStore((state) => state.operationalHint);
  const bridgeLockMintResult = useLiquidityStore((state) => state.bridgeLockMintResult);
  const lpPositions = useLiquidityStore((state) => state.lpPositions);

  const fetchPoolStatus = useLiquidityStore((state) => state.fetchPoolStatus);
  const lockMint = useLiquidityStore((state) => state.lockMint);
  const submitCommit = useLiquidityStore((state) => state.submitCommit);
  const refreshCommitStatus = useLiquidityStore((state) => state.refreshCommitStatus);
  const cancelActiveCommit = useLiquidityStore((state) => state.cancelActiveCommit);
  const fetchLpPositions = useLiquidityStore((state) => state.fetchLpPositions);
  const clearCommit = useLiquidityStore((state) => state.clearCommit);

  const [currentStep, setCurrentStep] = useState<WizardStep>(initialStep);
  const [pollingError, setPollingError] = useState<string | null>(null);

  const [mintAmount, setMintAmount] = useState("");
  // const [mintRecipient, setMintRecipient] = useState("");

  const [commitPoolPair, setCommitPoolPair] = useState("W-BRL-ARS");
  const [commitAmount, setCommitAmount] = useState("");

  const [nowMs, setNowMs] = useState(0);

  const commitForDisplay = activeCommit ?? (pendingCommit ? toActiveCommitFromPending(pendingCommit) : null);
  const monitoringPoolPair = commitForDisplay?.pool_pair ?? commitPoolPair;

  const remainingMs = useMemo(() => {
    if (!commitForDisplay?.expires_at) {
      return 0;
    }
    const expiryMs = Date.parse(commitForDisplay.expires_at);
    if (Number.isNaN(expiryMs)) {
      return 0;
    }
    return Math.max(0, expiryMs - nowMs);
  }, [commitForDisplay?.expires_at, nowMs]);

  useEffect(() => {
    setCurrentStep(initialStep);
  }, [initialStep]);

  useEffect(() => {
    if (matchContext) {
      // Match & Activate: the matcher must lock-mint their OWN side first. Pre-fill the
      // FX-suggested amount (editable) on the Lock-Mint step; carry the pool pair forward.
      setCommitPoolPair(matchContext.poolPair);
      // suggestedAmount arrives as raw wei from the API — convert to display format.
      setMintAmount(formatAmountInput(formatTokenAmount(matchContext.suggestedAmount, tokenDecimals)));
    }
  }, [matchContext, tokenDecimals]);

  useEffect(() => {
    if (currentStep !== 4 || !profile?.bankId) {
      return;
    }
    void fetchLpPositions("W-BRL-ARS", profile.bankId);
  }, [currentStep, profile?.bankId, fetchLpPositions]);

  useEffect(() => {
    if (currentStep !== 3) {
      return;
    }

    setNowMs(Date.now());

    const intervalId = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [currentStep]);

  useEffect(() => {
    if (currentStep !== 3) {
      return;
    }
    if (poolStatus?.pool_status === "ACTIVE" || activeCommit?.status === "EXECUTED") {
      setCurrentStep(4);
      setPollingError(null);
    }
  }, [currentStep, poolStatus?.pool_status, activeCommit?.status]);

  usePolling(
    () => {
      if (currentStep !== 3) {
        return;
      }
      // Reflect server-driven coordination state: pool reserves + this commit's status.
      // No hard timeout — the commit lifecycle completes server-side regardless of the UI.
      void fetchPoolStatus(monitoringPoolPair)
        .then(() => {
          setPollingError(null);
        })
        .catch((pollError) => {
          setPollingError(pollError instanceof Error ? pollError.message : "Unable to refresh pool status");
        });
      const monitoredCommitId = commitForDisplay?.commit_id;
      if (monitoredCommitId) {
        void refreshCommitStatus(monitoredCommitId);
      }
    },
    5000,
    currentStep === 3 && sovereignPhase !== SOVEREIGN_FLOW_PHASE.CANCELLED,
  );

  const handleStepClick = (stepId: WizardStep) => {
    if (stepId <= currentStep) {
      setCurrentStep(stepId);
    }
  };

  const handleLockMint = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await lockMint(displayToBase(parseAmountInput(mintAmount), tokenDecimals));

    if (useLiquidityStore.getState().status === "idle") {
      // Default the commit amount to what was just minted (the user can still adjust it).
      setCommitAmount(mintAmount);
      setCurrentStep(2);
    }
  };

  const handleCommit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitCommit({
      pool_pair: commitPoolPair,
      amount: displayToBase(parseAmountInput(commitAmount), tokenDecimals),
    });

    const latestCommit = useLiquidityStore.getState().activeCommit;
    const latestError = useLiquidityStore.getState().commitError;

    if (latestError) {
      return;
    }
    if (latestCommit?.status === "EXECUTED") {
      setCurrentStep(4);
      return;
    }
    if (latestCommit?.status === "PENDING") {
      setCurrentStep(3);
      return;
    }
  };

  const handleCancelCommit = async () => {
    const commitId = commitForDisplay?.commit_id;
    if (!commitId) {
      setPollingError("Missing commit context to cancel this pending commit.");
      return;
    }
    await cancelActiveCommit(commitId);
    if (useLiquidityStore.getState().commitStatus === "idle") {
      setCurrentStep(1);
      setCommitAmount("");
      setPollingError(null);
    }
  };

  const handleManualRefresh = async () => {
    try {
      await fetchPoolStatus(monitoringPoolPair);
      setPollingError(null);
    } catch (refreshError) {
      setPollingError(refreshError instanceof Error ? refreshError.message : "Unable to refresh pool status");
    }
  };

  const handleAddMoreLiquidity = () => {
    clearCommit();
    setCommitAmount("");
    setCurrentStep(1);
  };

  const handleDone = () => {
    clearCommit();
    setPollingError(null);
    setCurrentStep(1);
    onDone?.();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Cooperative Liquidity Wizard</CardTitle>
        <CardDescription>Run sovereign flow: lock-mint, commit, watcher execution, and pool ACTIVE validation.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
          {WIZARD_STEPS.map((step) => {
            const isCurrent = currentStep === step.id;
            const isCompleted = currentStep > step.id;
            return (
              <Button
                key={step.id}
                type="button"
                variant={isCurrent ? "default" : "outline"}
                className={isCompleted ? "border-green-500 text-green-700" : ""}
                onClick={() => handleStepClick(step.id)}
                disabled={step.id > currentStep}
              >
                {step.id}. {step.label}
              </Button>
            );
          })}
        </div>

        {currentStep === 1 ? (
          <form className="space-y-3" onSubmit={handleLockMint}>
            <h3 className="text-sm font-medium">Step 1: Bridge Lock-Mint</h3>
            {matchContext ? (
              <div className="rounded border border-blue-300 bg-blue-50 p-2 text-sm text-blue-800">
                <strong>Matching a counterpart.</strong> They committed{" "}
                <strong>{formatTokenAmount(matchContext.counterpartAmount, tokenDecimals)} tCeBM</strong> to the{" "}
                <strong>{sideRoleLabel(poolSideInfo(matchContext.counterpartSide, matchContext.poolPair, nationalCurrency), { fallbackLabel: `side ${matchContext.counterpartSide}` })}</strong> side. The amount below is the FX-suggested
                equivalent on <strong>your</strong> side — adjust it if your rate differs, then lock-mint
                your own tokens before committing.
              </div>
            ) : null}
            <div className="rounded border border-amber-300 bg-amber-50 p-2 text-sm text-amber-800">
              <strong>Note:</strong> If your spoke wallet does not already hold enough native tCeBM, this step will{" "}
              <strong>auto-mint</strong> the required amount on your spoke chain, lock it in the SpokeBridge, and mint
              the wrapped equivalent on the Hub. Minting issues new sovereign central-bank money — only proceed with an
              amount you intend to back.
            </div>
            <div className="space-y-1">
              <Label htmlFor="wizard_amount">Amount</Label>
              <Input
                id="wizard_amount"
                value={mintAmount}
                onChange={(event) => {
                  const cleaned = event.target.value.replace(/[^0-9.]/g, "").replace(/(\..*)\./g, "$1");
                  setMintAmount(formatAmountInput(cleaned));
                }}
                inputMode="decimal"
                placeholder="0.00"
                required
              />
            </div>
            {bridgeLockMintResult ? (
              <div className="rounded border p-2 text-sm">
                <p>Position ID: {bridgeLockMintResult.position_id}</p>
                <p>Bridge state: {bridgeLockMintResult.bridge_state}</p>
                <p>Spoke network: {bridgeLockMintResult.spoke_network}</p>
              </div>
            ) : null}
            {/* <div className="space-y-1">
              <Label htmlFor="wizard_recipient">Recipient (optional)</Label>
              <Input
                id="wizard_recipient"
                value={mintRecipient}
                onChange={(event) => setMintRecipient(event.target.value)}
              />
            </div> */}
            {error ? <p className="text-sm text-destructive">{error}</p> : null}
            <Button type="submit" disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Submit Bridge Lock-Mint"}
            </Button>
          </form>
        ) : null}

        {currentStep === 2 ? (
          <form className="space-y-3" onSubmit={handleCommit}>
            <h3 className="text-sm font-medium">Step 2: Commit</h3>
            <div className="space-y-1">
              <Label htmlFor="wizard_commit_pool_pair">Pool Pair</Label>
              <Input
                id="wizard_commit_pool_pair"
                value={commitPoolPair}
                onChange={(event) => setCommitPoolPair(event.target.value)}
                required
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="wizard_commit_amount">Amount</Label>
              <Input
                id="wizard_commit_amount"
                value={commitAmount}
                onChange={(event) => {
                  const cleaned = event.target.value.replace(/[^0-9.]/g, "").replace(/(\..*)\./g, "$1");
                  setCommitAmount(formatAmountInput(cleaned));
                }}
                inputMode="decimal"
                placeholder="0.00"
                required
              />
            </div>
            {commitError ? <p className="text-sm text-destructive">{commitError}</p> : null}
            <Button type="submit" disabled={commitStatus === "loading"}>
              {commitStatus === "loading" ? "Submitting..." : "Submit Commit"}
            </Button>
            {activeCommit ? (
              <div className="rounded border p-2 text-sm">
                <p>commit_id: {activeCommit.commit_id}</p>
                <p>
                  on_chain_commit_id: {activeCommit.on_chain_commit_id || "pending on-chain confirmation"}
                </p>
              </div>
            ) : null}
          </form>
        ) : null}

        {currentStep === 3 ? (
          <div className="space-y-3">
            <h3 className="text-sm font-medium">Step 3: Monitor</h3>
            <div className="rounded border border-blue-300 bg-blue-50 p-2 text-sm text-blue-800">
              Your commit is registered and coordination is running server-side. The pool activates
              automatically once a counterpart matches — this can take a while and need not be synchronous.
              <strong> You can safely close this page</strong> and re-open “Monitor Pending Commit” later.
            </div>
            <div className="space-y-1 text-sm">
              <p>Commit ID: {commitForDisplay?.commit_id ?? "-"}</p>
              <p>
                On-chain commit ID: {commitForDisplay?.on_chain_commit_id || "pending on-chain confirmation"}
              </p>
              <p>Pool Pair: {monitoringPoolPair}</p>
              <p>Expires in: {formatRemainingMs(remainingMs)}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Badge variant={poolStatus?.pool_status === "ACTIVE" ? "success" : "outline"}>
                Pool status: {poolStatus?.pool_status ?? "UNKNOWN"}
              </Badge>
              <Badge variant="outline">Polling every 5s</Badge>
              <Badge variant="outline">Phase: {sovereignPhase}</Badge>
            </div>
            {operationalHint ? <p className="text-sm text-muted-foreground">{operationalHint}</p> : null}
            {sovereignPhase === SOVEREIGN_FLOW_PHASE.CANCELLED ? (
              <p className="text-sm text-muted-foreground">Commit was cancelled.</p>
            ) : null}
            {pollingError ? <p className="text-sm text-destructive">{pollingError}</p> : null}
            {commitError ? <p className="text-sm text-destructive">{commitError}</p> : null}
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={handleManualRefresh}>
                Retry Polling
              </Button>
              {sovereignPhase !== SOVEREIGN_FLOW_PHASE.CANCELLED ? (
                <Button type="button" variant="destructive" onClick={handleCancelCommit} disabled={commitStatus === "loading"}>
                  {commitStatus === "loading" ? "Cancelling..." : "Cancel Commit"}
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}

        {currentStep === 4 ? (
          <div className="space-y-4">
            <h3 className="text-sm font-medium">Step 4: Pool ACTIVE</h3>
            <div className="space-y-1 text-sm">
              <p>Reserve A: {poolStatus?.reserve_a ? formatTokenAmount(poolStatus.reserve_a, tokenDecimals) : "-"}</p>
              <p>Reserve B: {poolStatus?.reserve_b ? formatTokenAmount(poolStatus.reserve_b, tokenDecimals) : "-"}</p>
              <p>Current ratio: {poolStatus?.current_ratio ?? "-"}</p>
              <p>Fee rate (bps): {poolStatus?.fee_rate_bps ?? "-"}</p>
              <p>Total LP count: {poolStatus?.total_lp_count ?? "-"}</p>
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium">LP Positions</p>
              {lpPositions.length > 0 ? (
                <ul className="space-y-2">
                  {lpPositions.map((position) => (
                    <li key={position.lp_id} className="space-y-1 rounded border p-2 text-sm">
                      <p>lp_id: {position.lp_id}</p>
                      <p>deposit_side: {position.deposit_side}</p>
                      <p>token_a_contributed: {formatTokenAmount(position.token_a_contributed, tokenDecimals)}</p>
                      <p>token_b_contributed: {formatTokenAmount(position.token_b_contributed, tokenDecimals)}</p>
                      <p>commit_id: {position.commit_id || "-"}</p>
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => onSelectLpForRemoval?.(position.lp_id, monitoringPoolPair, position.provider_bank_id)}
                      >
                        Remove Liquidity
                      </Button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">No positions found for this pool.</p>
              )}
            </div>

            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={handleAddMoreLiquidity}>
                Add More Liquidity
              </Button>
              <Button type="button" onClick={handleDone}>
                Done
              </Button>
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
