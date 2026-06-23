// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";
import type { OnboardingRequestStatus } from "../../../types";

type Step4RevokedProps = {
  status: OnboardingRequestStatus;
  onGoDashboard: () => void;
};

const statusLabel: Record<OnboardingRequestStatus, string> = {
  NONE: "NONE",
  PENDING: "PENDING",
  APPROVED: "APPROVED",
  ACTIVE: "ACTIVE",
  FROZEN: "FROZEN",
  REVOKED: "REVOKED",
  REJECTED: "REJECTED",
  CREDENTIAL_REQUESTED: "PENDING",
  KYC_APPROVED: "APPROVED",
};

export function Step4Revoked({ status, onGoDashboard }: Step4RevokedProps) {
  const descriptionByStatus: Record<OnboardingRequestStatus, string> = {
    NONE: "No onboarding request is associated with this account.",
    PENDING: "This request is still pending review.",
    APPROVED: "This request is approved and awaiting activation.",
    ACTIVE: "This institution is active.",
    FROZEN: "Your institution has been frozen by Central Bank governance.",
    REVOKED: "Your institution has been revoked by Central Bank governance.",
    REJECTED: "Your onboarding request has been rejected by Central Bank governance.",
    CREDENTIAL_REQUESTED: "This request is still pending review.",
    KYC_APPROVED: "This request is approved and awaiting activation.",
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Onboarding Status</CardTitle>
        <CardDescription>{descriptionByStatus[status]}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Badge variant="destructive">{statusLabel[status]}</Badge>

        <p className="text-sm text-muted-foreground">
          Contact Central Bank governance support for additional details about this status.
        </p>

        <Button onClick={onGoDashboard}>Go to Dashboard</Button>
      </CardContent>
    </Card>
  );
}