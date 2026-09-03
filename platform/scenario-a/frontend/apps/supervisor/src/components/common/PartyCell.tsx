// SPDX-License-Identifier: Apache-2.0

import { CopyableValue } from "./CopyableValue";

interface PartyCellProps {
  /** Raw on-chain address or subject/identity string. */
  address: string;
  /** Resolved institution name, if any. */
  name?: string;
  /** Characters shown before truncating the address. */
  truncate?: number;
}

/**
 * Renders an involved party as "institution name + address". When the name
 * cannot be resolved, falls back to showing only the address (previous
 * behaviour). Used by the Auditor Portal transaction/audit listings.
 */
export function PartyCell({ address, name, truncate = 14 }: PartyCellProps) {
  if (!address) return <span className="text-muted-foreground">—</span>;
  return (
    <div className="flex flex-col gap-0.5">
      {name ? <span className="text-sm font-medium">{name}</span> : null}
      <CopyableValue value={address} truncate={truncate} className={name ? "text-muted-foreground" : ""} />
    </div>
  );
}
