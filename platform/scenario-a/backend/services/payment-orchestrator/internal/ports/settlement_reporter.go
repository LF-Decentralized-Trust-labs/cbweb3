// SPDX-License-Identifier: Apache-2.0

package ports

import (
	"context"
	"time"
)

// SettledLeg is a completed inter-bank PvP settlement leg reported to the Central
// Bank so the receiving bank can see the incoming credit on its statement. A
// receiving bank's own orchestrator holds no record of the leg (the counterparty
// locked it on a different orchestrator, and the amount is private Zeto value),
// so the settling orchestrator forwards the fact of settlement to the CB.
type SettledLeg struct {
	TradeID    string    // FX agreement id this leg belongs to
	ContractID string    // HTLC contract id (unique per leg; used for idempotent upsert)
	Sender     string    // sender Paladin identity (the locking bank's operator)
	Receiver   string    // receiver Paladin identity (credited bank)
	Amount     string    // integer units, as settled
	SettledAt  time.Time // settlement timestamp
}

// SettlementReporter reports a settled PvP leg to the Central Bank. Implementations
// are best-effort: the caller logs failures and never blocks settlement on them.
type SettlementReporter interface {
	ReportSettledLeg(ctx context.Context, leg SettledLeg) error
}
