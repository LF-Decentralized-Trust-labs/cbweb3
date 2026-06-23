// SPDX-License-Identifier: Apache-2.0

import type { TreasuryEvent } from "../../types";

const messages = [
  { type: "funding.request.updated", message: "Funding queue updated", severity: "INFO" },
  { type: "supply.updated", message: "Supply snapshot recalculated", severity: "INFO" },
  { type: "reconciliation.updated", message: "Spoke-hub delta warning threshold reached", severity: "WARNING" },
  { type: "audit.log.created", message: "New audit event recorded", severity: "INFO" },
] as const;

const makeId = () => `evt_${Math.random().toString(36).slice(2, 10)}`;

export const treasuryEventsService = {
  start(emit: (event: TreasuryEvent) => void) {
    const timer = setInterval(() => {
      const entry = messages[Math.floor(Math.random() * messages.length)];
      emit({
        id: makeId(),
        type: entry.type,
        message: entry.message,
        severity: entry.severity,
        createdAt: new Date().toISOString(),
      });
    }, 12_000);
    return () => clearInterval(timer);
  },
};
