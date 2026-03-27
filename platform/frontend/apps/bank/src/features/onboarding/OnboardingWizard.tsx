import { Card, CardContent, CardHeader, CardTitle, Progress } from "@cbweb3/ui";
import { useEffect, useRef } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useAuthStore } from "../../stores";
import { useOnboardingPolling } from "./hooks/useOnboardingPolling";
import type { InstitutionFormValues } from "./schemas/onboarding.schema";
import { useOnboardingStore } from "./store/useOnboardingStore";
import { Step1OperatorConfirmation } from "./steps/Step1OperatorConfirmation";
import { Step2InstitutionForm } from "./steps/Step2InstitutionForm";
import { Step3KycPending } from "./steps/Step3KycPending";
import { Step4Complete } from "./steps/Step4Complete";

const stepTitles = ["Operator", "Institution", "KYC", "Complete"];

export function OnboardingWizard() {
  const navigate = useNavigate();
  const profile = useAuthStore((state) => state.profile);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

  const {
    currentStep,
    status,
    completionStatus,
    error,
    requestId,
    userId,
    walletAddress,
    requestStatus,
    clientSecret,
    txHash,
    accessToken,
    pkiLoginError,
    start,
    initiate,
    complete,
    reset,
  } = useOnboardingStore();

  const completionTriggered = useRef(false);
  const { pollingStatus, elapsedSeconds, refresh, stopPolling } = useOnboardingPolling(currentStep === 3 && Boolean(requestId));

  useEffect(() => {
    if (currentStep !== 4) {
      completionTriggered.current = false;
      return;
    }

    if (completionTriggered.current || !requestId || !userId || clientSecret) {
      return;
    }

    completionTriggered.current = true;
    stopPolling();
    void complete();
  }, [clientSecret, complete, currentStep, requestId, stopPolling, userId]);

  const onSubmitInstitution = async (values: InstitutionFormValues) => {
    await initiate(values);
  };

  const onGoDashboard = () => {
    reset();
    navigate("/", { replace: true });
  };

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  const operatorId = profile?.bankId ?? profile?.subject ?? "unknown-operator";
  const progress = (currentStep / 4) * 100;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Commercial Bank Onboarding</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
            {stepTitles.map((title, index) => {
              const stepNumber = index + 1;
              return (
                <span key={title} className={stepNumber <= currentStep ? "font-semibold text-foreground" : undefined}>
                  {stepNumber}. {title}
                </span>
              );
            })}
          </div>
          <Progress value={progress} />
        </CardContent>
      </Card>

      {currentStep === 1 ? <Step1OperatorConfirmation operatorId={operatorId} onStart={start} /> : null}

      {currentStep === 2 ? (
        <Step2InstitutionForm
          loading={status === "loading"}
          error={error}
          onSubmit={onSubmitInstitution}
        />
      ) : null}

      {currentStep === 3 && requestId && walletAddress ? (
        <Step3KycPending
          requestId={requestId}
          walletAddress={walletAddress}
          requestStatus={requestStatus}
          pollingStatus={pollingStatus}
          elapsedSeconds={elapsedSeconds}
          error={error}
          onRefresh={refresh}
        />
      ) : null}

      {currentStep === 4 ? (
        <Step4Complete
          completionStatus={completionStatus}
          error={error}
          clientSecret={clientSecret}
          walletAddress={walletAddress}
          txHash={txHash}
          accessToken={accessToken}
          pkiLoginError={pkiLoginError}
          onGoDashboard={onGoDashboard}
        />
      ) : null}
    </div>
  );
}