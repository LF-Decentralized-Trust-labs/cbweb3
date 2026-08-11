// SPDX-License-Identifier: Apache-2.0

import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";
import { ShieldAlert } from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import { useTrustStore } from "../../stores/trust.store";

const ONBOARDING_PATH_RE = /^\/onboarding(\/|$)/;

/**
 * TrustNotice explains a central bank rejection wherever the operator happens to be.
 *
 * It renders nothing until the HTTP layer reports one, and it sits in the layout rather than in each
 * page because the cause is shared: the same missing registration breaks payments, the bridge and the
 * pools at once.
 */
export function TrustNotice() {
  const block = useTrustStore((state) => state.block);
  const checking = useTrustStore((state) => state.checking);
  const recheck = useTrustStore((state) => state.recheck);
  const { pathname } = useLocation();

  // The wizard already is the answer; repeating it there, with a button pointing at the current page,
  // would only take space away from the form the operator needs to fill in.
  if (!block || ONBOARDING_PATH_RE.test(pathname)) {
    return null;
  }

  return (
    <Card className="mb-4 border-destructive/40 bg-destructive/5">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base text-destructive">
          <ShieldAlert className="h-4 w-4 shrink-0" />
          {block.title}
        </CardTitle>
        <CardDescription>{block.description}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        {block.showOnboardingLink ? (
          <Button asChild size="sm">
            <Link to="/onboarding">Go to onboarding</Link>
          </Button>
        ) : null}
        <Button variant="outline" size="sm" onClick={() => void recheck()} disabled={checking}>
          {checking ? "Checking..." : "Check again"}
        </Button>
      </CardContent>
    </Card>
  );
}
