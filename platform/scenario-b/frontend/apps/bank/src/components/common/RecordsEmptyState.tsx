// SPDX-License-Identifier: Apache-2.0

import { useTrustStore } from "../../stores/trust.store";

type RecordsEmptyStateProps = {
  /** What to say when the list really is empty. */
  emptyLabel: string;
};

/**
 * RecordsEmptyState keeps an empty table from claiming there are no records.
 *
 * The payments store loads its lists with allSettled, so a rejected request yields an empty array and
 * the table reads exactly like a bank with nothing to show. When the central bank has rejected us the
 * lists were never read, and saying so is the difference between "nothing happened" and "we could not
 * ask".
 */
export function RecordsEmptyState({ emptyLabel }: RecordsEmptyStateProps) {
  const blocked = useTrustStore((state) => state.block !== null);

  return (
    <p className="pt-3 text-sm text-muted-foreground">
      {blocked ? "This list could not be loaded: the central bank does not recognize this institution." : emptyLabel}
    </p>
  );
}
