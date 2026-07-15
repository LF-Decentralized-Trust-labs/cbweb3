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

// HTLCSource supplies the inter-bank HTLC PvP records held by this entity's own
// payment-orchestrator. An orchestrator only holds the legs it locked (records
// where it is the sender), so this source surfaces the caller's *sent* legs
// (debits). The receiver side never lands here — each bank runs its own
// orchestrator — so credits are sourced from the Central Bank instead (see
// PvPCreditSource). *paymentadapter.GRPCAdapter implements it.
type HTLCSource interface {
	SearchHTLC(ctx context.Context, agreementID, sender, receiver, state string) ([]paymentadapter.HTLCStatus, error)
}

// PvPCredit is one incoming inter-bank PvP settlement leg for which this bank is
// the receiver, as aggregated by the Central Bank from a SETTLED FX agreement.
type PvPCredit struct {
	Reference string `json:"reference"`  // FX agreement trade id
	Amount    string `json:"amount"`     // integer units (tCeBM), the leg amount
	SettledAt string `json:"settled_at"` // RFC3339 of the FX agreement SETTLED transition
}

// PvPCreditSource supplies the settled PvP legs on which this bank is the
// receiver. A receiving bank's own orchestrator has no record of an incoming
// leg (the counterparty locked it on a different orchestrator, and the amount is
// private Zeto value), so the credits are derived at the Central Bank from the
// aggregated FX agreements it settles, scoped to this bank. PaymentProxyHandler
// implements it.
type PvPCreditSource interface {
	FetchPvPCredits(ctx context.Context, bankID string) ([]PvPCredit, error)
}

// Movement is a single credit/debit line on the bank statement.
type Movement struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`           // RFC3339
	Direction string `json:"direction"`           // "credit" (received) | "debit" (sent)
	Token     string `json:"token"`               // "fCeBM" (tokenized fiat) | "tCeBM" (reserve)
	Amount    string `json:"amount"`              // integer units, as persisted
	Kind      string `json:"kind"`                // "deposit" | "tokenisation" | "redeem" | "pvp_settlement"
	Reference string `json:"reference,omitempty"` // settlement tx hash or FX trade id
}

// StatementHandler serves the commercial bank statement (extrato): a
// chronological consolidation of tokenized-fiat and tCeBM movements derived
// from the bank's deposit, reserve-tokenisation and redeem records.
type StatementHandler struct {
	source     MovementSource
	htlc       HTLCSource      // optional; surfaces the caller's sent PvP legs (debits)
	pvpCredits PvPCreditSource // optional; surfaces the caller's received PvP legs (credits)
	bankCode   string          // institution fallback when the JWT lacks BankID
}

// NewStatementHandler creates a StatementHandler over the given movement source.
func NewStatementHandler(source MovementSource) *StatementHandler {
	return &StatementHandler{source: source}
}

// WithHTLCSource attaches the payment-orchestrator HTLC search used to surface
// this bank's *sent* inter-bank PvP legs (debits). bankCode is the institution
// fallback applied when the caller's JWT carries no BankID.
func (h *StatementHandler) WithHTLCSource(htlc HTLCSource, bankCode string) *StatementHandler {
	h.htlc = htlc
	h.bankCode = bankCode
	return h
}

// WithPvPCreditSource attaches the Central Bank-derived source of this bank's
// *received* inter-bank PvP legs (credits). bankCode is the institution fallback
// applied when the caller's JWT carries no BankID.
func (h *StatementHandler) WithPvPCreditSource(src PvPCreditSource, bankCode string) *StatementHandler {
	h.pvpCredits = src
	if bankCode != "" {
		h.bankCode = bankCode
	}
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

	callerBankID := h.callerBankID(c)

	// Sent PvP legs (debits) — this bank's own orchestrator holds the legs it locked.
	pvpDebits, err := h.pvpDebitMovements(c, callerBankID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load pvp settlements: " + err.Error()})
	}
	movements = append(movements, pvpDebits...)

	// Received PvP legs (credits) — derived at the Central Bank from settled FX
	// agreements, scoped to this bank (the receiving side is never on the local
	// orchestrator).
	pvpCredits, err := h.pvpCreditMovements(c, callerBankID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "statement: load pvp credits: " + err.Error()})
	}
	movements = append(movements, pvpCredits...)

	sort.SliceStable(movements, func(i, j int) bool {
		return parseTimestamp(movements[i].Timestamp).After(parseTimestamp(movements[j].Timestamp))
	})

	return c.JSON(fiber.Map{"movements": movements, "total": len(movements)})
}

// callerBankID resolves the bank the statement is scoped to: the JWT BankID when
// present, else the configured institution fallback. Empty means unscoped.
func (h *StatementHandler) callerBankID(c *fiber.Ctx) string {
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok && claims.BankID != "" {
		return claims.BankID
	}
	return h.bankCode
}

// pvpDebitMovements surfaces this bank's *sent* inter-bank PvP legs as debits.
// The bank's own payment-orchestrator only holds the legs it locked (Sender ==
// this bank), so only the sender branch ever matches here; the received side is
// sourced from the Central Bank (see pvpCreditMovements). An inter-bank PvP
// settlement moves tokenized reserve value, so legs are denominated in tCeBM (not
// tokenized fiat). Returns nil when no HTLC source is configured (e.g. the
// central-bank gateway, or tests).
func (h *StatementHandler) pvpDebitMovements(c *fiber.Ctx, callerBankID string) ([]Movement, error) {
	if h.htlc == nil || callerBankID == "" {
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
		senderBank, err := bankIDFromIdentity(l.Sender)
		if err != nil {
			// Unparseable identity: skip rather than misattribute a movement.
			continue
		}
		if senderBank != callerBankID {
			continue
		}
		movements = append(movements, Movement{
			ID:        "pvp:" + l.ContractID + ":debit",
			Timestamp: l.CreatedAt,
			Direction: directionDebit,
			Token:     tokenTCeBM,
			Amount:    l.Amount,
			Kind:      kindPvP,
			Reference: l.ContractID,
		})
	}
	return movements, nil
}

// pvpCreditMovements surfaces this bank's *received* inter-bank PvP legs as
// credits, sourced from the Central Bank (which aggregates the settled FX
// agreements and derives the receiver legs for this bank). Returns nil when no
// credit source is configured (e.g. the central-bank gateway, or tests).
func (h *StatementHandler) pvpCreditMovements(c *fiber.Ctx, callerBankID string) ([]Movement, error) {
	if h.pvpCredits == nil || callerBankID == "" {
		return nil, nil
	}

	credits, err := h.pvpCredits.FetchPvPCredits(c.UserContext(), callerBankID)
	if err != nil {
		return nil, err
	}

	movements := make([]Movement, 0, len(credits))
	for _, cr := range credits {
		movements = append(movements, Movement{
			ID:        "pvp:" + cr.Reference + ":credit",
			Timestamp: cr.SettledAt,
			Direction: directionCredit,
			Token:     tokenTCeBM,
			Amount:    cr.Amount,
			Kind:      kindPvP,
			Reference: cr.Reference,
		})
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
