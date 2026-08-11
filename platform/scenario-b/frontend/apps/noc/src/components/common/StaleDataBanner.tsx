// SPDX-License-Identifier: Apache-2.0

import { Badge } from "@cbweb3/ui";

type StaleDataBannerProps = {
  stale: boolean;
};

export function StaleDataBanner({ stale }: StaleDataBannerProps) {
  if (!stale) {
    return null;
  }

  return (
    <div className="mb-3 rounded-md border border-warning/50 bg-warning/10 px-3 py-2 text-sm">
      <div className="flex items-center gap-2">
        <Badge variant="warning">STALE</Badge>
        <span className="text-muted-foreground">
          The last backend refresh failed. Showing the most recent data received.
        </span>
      </div>
    </div>
  );
}
