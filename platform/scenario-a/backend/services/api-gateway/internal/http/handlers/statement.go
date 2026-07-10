// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"sort"
	"strings"
	"time"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/gofiber/fiber/v2"
)

// Token types surfaced on a statement movement.
const (
	tokenFiat  = "fCeBM" // tokenized fiat
	tokenTCeBM = "tCeBM" // tokenized central bank money (reserve)
)

// Movement directions.
const (
	directionCredit = "credit" // money received
	directionDebit  = "debit"  // money sent
)

// MovementSource supplies a commercial bank's persisted payment records so the
// statement handler can consolidate them into a single credit/debit ledger.
// PaymentProxyHandler implements it (records are fetched from the Central Bank,
// scoped to this entity).
type MovementSource interface {
	FetchDeposits(ctx context.Context) ([]paymentadapter.DepositRecord, error)
	FetchEscrows(ctx context.Context) ([]paymentadapter.EscrowRecord, error)
	FetchRedeems(ctx context.Context) ([]paymentadapter.RedeemRecord, error)
}

// Movement is a single credit/debit line on the bank statement.
type Movement struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`        // RFC3339
	Direction string `json:"direction"`        // "credit" (received) | "debit" (sent)
	Token     string `json:"token"`            // "fCeBM" (tokenized fiat) | "tCeBM"
	Amount    string `json:"amount"`           // integer units, as persisted
	Kind      string `json:"kind"`             // "deposit" | "tokenisation" | "redeem"
	Reference string `json:"reference,omitempty"` // settlement tx hash
}

// StatementHandler serves the commercial bank statement (extrato): a
// chronological consolidation of tokenized-fiat and tCeBM movements derived
// from the bank's deposit, reserve-tokenisation and redeem records.
type StatementHandler struct {
	source MovementSource
}

// NewStatementHandler creates a StatementHandler over the given movement source.
func NewStatementHandler(source MovementSource) *StatementHandler {
	return &StatementHandler{source: source}
}

// GetStatement returns the bank's movements, most recent first.
//
//	GET /api/v1/statement -> { "movements": [...], "total": N }
//
// Only settled (approved) records become movements. Each source maps to
// directional legs by operation semantics:
//   - deposit       -> credit fCeBM (bank receives tokenized fiat)
//   - tokenisation  -> debit fCeBM  + credit tCeBM (fCeBM burned, tCeBM minted)
//   - redeem        -> debit tCeBM  + credit fCeBM (tCeBM burned, fCeBM minted)
func (h *StatementHandler) GetStatement(c *fiber.Ctx) error {
	ctx := c.UserContext()

	deposits, err := h.source.FetchDeposits(ctx)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load deposits: " + err.Error()})
	}
	escrows, err := h.source.FetchEscrows(ctx)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load tokenisations: " + err.Error()})
	}
	redeems, err := h.source.FetchRedeems(ctx)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load redeems: " + err.Error()})
	}

	movements := make([]Movement, 0, len(deposits)+2*len(escrows)+2*len(redeems))

	for _, d := range deposits {
		if !isSettled(d.Status) {
			continue
		}
		movements = append(movements, Movement{
			ID:        "deposit:" + d.ID,
			Timestamp: d.CreatedAt,
			Direction: directionCredit,
			Token:     tokenFiat,
			Amount:    d.Amount,
			Kind:      "deposit",
			Reference: d.MintTxHash,
		})
	}

	for _, e := range escrows {
		if !isSettled(e.Status) {
			continue
		}
		movements = append(movements,
			Movement{
				ID:        "tokenisation:" + e.ID + ":fcebm",
				Timestamp: e.CreatedAt,
				Direction: directionDebit,
				Token:     tokenFiat,
				Amount:    e.Amount,
				Kind:      "tokenisation",
				Reference: e.BurnTxHash,
			},
			Movement{
				ID:        "tokenisation:" + e.ID + ":tcebm",
				Timestamp: e.CreatedAt,
				Direction: directionCredit,
				Token:     tokenTCeBM,
				Amount:    e.Amount,
				Kind:      "tokenisation",
				Reference: e.MintTxHash,
			},
		)
	}

	for _, r := range redeems {
		if !isSettled(r.Status) {
			continue
		}
		movements = append(movements,
			Movement{
				ID:        "redeem:" + r.ID + ":tcebm",
				Timestamp: r.CreatedAt,
				Direction: directionDebit,
				Token:     tokenTCeBM,
				Amount:    r.Amount,
				Kind:      "redeem",
				Reference: r.ZetoTransferTxHash,
			},
			Movement{
				ID:        "redeem:" + r.ID + ":fcebm",
				Timestamp: r.CreatedAt,
				Direction: directionCredit,
				Token:     tokenFiat,
				Amount:    r.Amount,
				Kind:      "redeem",
				Reference: r.FiatMintTxHash,
			},
		)
	}

	sort.SliceStable(movements, func(i, j int) bool {
		return parseTimestamp(movements[i].Timestamp).After(parseTimestamp(movements[j].Timestamp))
	})

	return c.JSON(fiber.Map{"movements": movements, "total": len(movements)})
}

// isSettled reports whether a record's status (a proto enum string such as
// "DEPOSIT_STATUS_APPROVED") represents a completed, money-moving operation.
func isSettled(status string) bool {
	return strings.HasSuffix(strings.ToUpper(strings.TrimSpace(status)), "APPROVED")
}

// parseTimestamp parses an RFC3339 timestamp; unparseable/empty values sort last.
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}
