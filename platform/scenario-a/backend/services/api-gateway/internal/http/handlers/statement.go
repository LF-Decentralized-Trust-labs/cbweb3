// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"sort"
	"strings"
	"time"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/metadata"
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

// Movement kinds.
const (
	kindDeposit      = "deposit"
	kindTokenisation = "tokenisation"
	kindRedeem       = "redeem"
	kindPvP          = "pvp_settlement" // inter-bank HTLC PvP settlement leg
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

// HTLCSource supplies the inter-bank HTLC PvP records visible to this entity's
// payment-orchestrator, so the statement can add the settled PvP legs (sent =
// debit, received = credit) alongside the deposit/tokenisation/redeem movements.
// *paymentadapter.GRPCAdapter implements it.
type HTLCSource interface {
	SearchHTLC(ctx context.Context, agreementID, sender, receiver, state string) ([]paymentadapter.HTLCStatus, error)
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
	source   MovementSource
	htlc     HTLCSource // optional; enables inter-bank PvP settlement movements
	bankCode string     // institution fallback when the JWT lacks BankID
}

// NewStatementHandler creates a StatementHandler over the given movement source.
func NewStatementHandler(source MovementSource) *StatementHandler {
	return &StatementHandler{source: source}
}

// WithHTLCSource attaches the payment-orchestrator HTLC search used to
// consolidate inter-bank PvP settlement legs into the statement. bankCode is the
// institution fallback applied when the caller's JWT carries no BankID.
func (h *StatementHandler) WithHTLCSource(htlc HTLCSource, bankCode string) *StatementHandler {
	h.htlc = htlc
	h.bankCode = bankCode
	return h
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

	pvp, err := h.pvpMovements(c)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load pvp settlements: " + err.Error()})
	}
	movements = append(movements, pvp...)

	sort.SliceStable(movements, func(i, j int) bool {
		return parseTimestamp(movements[i].Timestamp).After(parseTimestamp(movements[j].Timestamp))
	})

	return c.JSON(fiber.Map{"movements": movements, "total": len(movements)})
}

// pvpMovements consolidates this bank's settled inter-bank HTLC PvP legs into
// statement movements: a leg where the bank is the sender is a debit (value
// sent), one where it is the receiver is a credit (value received). Value moves
// as tokenized fiat (fCeBM). Returns an empty slice when no HTLC source is
// configured (e.g. the central-bank gateway, or tests).
func (h *StatementHandler) pvpMovements(c *fiber.Ctx) ([]Movement, error) {
	if h.htlc == nil {
		return nil, nil
	}

	callerBankID := h.bankCode
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok && claims.BankID != "" {
		callerBankID = claims.BankID
	}
	if callerBankID == "" {
		return nil, nil
	}

	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	locks, err := h.htlc.SearchHTLC(ctx, "", "", "", "")
	if err != nil {
		return nil, err
	}

	movements := make([]Movement, 0, len(locks))
	for _, l := range locks {
		if !isHTLCSettled(l.State) {
			continue
		}
		senderBank, sErr := bankIDFromIdentity(l.Sender)
		receiverBank, rErr := bankIDFromIdentity(l.Receiver)
		if sErr != nil || rErr != nil {
			// Unparseable identity: skip rather than misattribute a movement.
			continue
		}
		if senderBank == callerBankID {
			movements = append(movements, Movement{
				ID:        "pvp:" + l.ContractID + ":debit",
				Timestamp: l.CreatedAt,
				Direction: directionDebit,
				Token:     tokenFiat,
				Amount:    l.Amount,
				Kind:      kindPvP,
				Reference: l.ContractID,
			})
		}
		if receiverBank == callerBankID {
			movements = append(movements, Movement{
				ID:        "pvp:" + l.ContractID + ":credit",
				Timestamp: l.CreatedAt,
				Direction: directionCredit,
				Token:     tokenFiat,
				Amount:    l.Amount,
				Kind:      kindPvP,
				Reference: l.ContractID,
			})
		}
	}
	return movements, nil
}

// isSettled reports whether a record's status (a proto enum string such as
// "DEPOSIT_STATUS_APPROVED") represents a completed, money-moving operation.
func isSettled(status string) bool {
	return strings.HasSuffix(strings.ToUpper(strings.TrimSpace(status)), "APPROVED")
}

// isHTLCSettled reports whether an HTLC state (proto enum string such as
// "HTLC_STATE_SETTLED") represents a completed PvP settlement. The transient
// "HTLC_STATE_SETTLING" is excluded (it does not end with "SETTLED").
func isHTLCSettled(state string) bool {
	return strings.HasSuffix(strings.ToUpper(strings.TrimSpace(state)), "SETTLED")
}

// parseTimestamp parses an RFC3339 timestamp; unparseable/empty values sort last.
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}
