import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label } from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { useAuthStore } from "../../stores/auth.store";
import { SOVEREIGN_FLOW_PHASE } from "../../types/liquidity.types";
import type { CommitResult, PendingCommit } from "../../types/liquidity.types";
import { usePolling } from "../../hooks/usePolling";
import { useLiquidityStore } from "./liquidity.store";

const WIZARD_STEPS = [
  { id: 1, label: "Lock-Mint" },
  { id: 2, label: "Commit" },
  { id: 3, label: "Commit Pending/Executed" },
  { id: 4, label: "Pool Active" },
] as const;

type WizardStep = (typeof WIZARD_STEPS)[number]["id"];

type CooperativeLiquidityWizardProps = {
  initialStep?: WizardStep;
  pendingCommit?: PendingCommit | null;
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

function formatRemainingMs(ms: number): string {
  if (ms <= 0) {
    return "Expired";
  }
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return `${hours}h ${minutes}m ${seconds}s`;
}

export function CooperativeLiquidityWizard({
  initialStep = 1,
  pendingCommit = null,
  onDone,
  onSelectLpForRemoval,
}: CooperativeLiquidityWizardProps) {
  const profile = useAuthStore((state) => state.profile);
  const status = useLiquidityStore((state) => state.status);
  const error = useLiquidityStore((state) => state.error);
  const poolStatus = useLiquidityStore((state) => state.poolStatus);
  const activeCommit = useLiquidityStore((state) => state.activeCommit);
  const commitStatus = useLiquidityStore((state) => state.commitStatus);
  const commitError = useLiquidityStore((state) => state.commitError);
  const sovereignPhase = useLiquidityStore((state) => state.sovereignPhase);
  const commitLatencyWarning = useLiquidityStore((state) => state.commitLatencyWarning);
  const operationalHint = useLiquidityStore((state) => state.operationalHint);
  const bridgeLockMintResult = useLiquidityStore((state) => state.bridgeLockMintResult);
  const lpPositions = useLiquidityStore((state) => state.lpPositions);

  const fetchPoolStatus = useLiquidityStore((state) => state.fetchPoolStatus);
  const lockMint = useLiquidityStore((state) => state.lockMint);
  const submitCommit = useLiquidityStore((state) => state.submitCommit);
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
    if (poolStatus?.pool_status === "ACTIVE") {
      setCurrentStep(4);
      setPollingError(null);
      return;
    }
    if (poolStatus?.pool_status === "EMPTY" && remainingMs <= 0) {
      setPollingError("Commit expired before counterpart confirmation.");
    }
  }, [currentStep, poolStatus?.pool_status, remainingMs]);

  usePolling(
    () => {
      if (currentStep !== 3) {
        return;
      }
      void fetchPoolStatus(monitoringPoolPair)
        .then(() => {
          setPollingError(null);
        })
        .catch((pollError) => {
          setPollingError(pollError instanceof Error ? pollError.message : "Unable to refresh pool status");
        });
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
    await lockMint(mintAmount);

    if (useLiquidityStore.getState().status === "idle") {
      setCurrentStep(2);
    }
  };

  const handleCommit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitCommit({
      pool_pair: commitPoolPair,
      amount: commitAmount,
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
            <div className="space-y-1">
              <Label htmlFor="wizard_amount">Amount</Label>
              <Input
                id="wizard_amount"
                value={mintAmount}
                onChange={(event) => setMintAmount(event.target.value)}
                inputMode="numeric"
                pattern="[0-9]+"
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
                onChange={(event) => setCommitAmount(event.target.value)}
                inputMode="numeric"
                pattern="[0-9]+"
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
            {commitLatencyWarning ? (
              <p className="text-sm text-amber-700">Commit still pending after 30s. The watcher may still execute it.</p>
            ) : null}
            {sovereignPhase === SOVEREIGN_FLOW_PHASE.CANCELLED ? (
              <p className="text-sm text-muted-foreground">Commit was cancelled.</p>
            ) : null}
            {sovereignPhase === SOVEREIGN_FLOW_PHASE.TIMEOUT ? (
              <p className="text-sm text-destructive">Operation timed out after 180 seconds. Verify bridge/commit status and retry.</p>
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
              <p>Reserve A: {poolStatus?.reserve_a ?? "-"}</p>
              <p>Reserve B: {poolStatus?.reserve_b ?? "-"}</p>
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
                      <p>token_a_contributed: {position.token_a_contributed}</p>
                      <p>token_b_contributed: {position.token_b_contributed}</p>
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
