package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TransferLimitChecker gates outgoing HTLC locks against CB-configured daily limits.
type TransferLimitChecker interface {
	CheckAndDeductTransferLimit(ctx context.Context, payerBankID, currency, amountHuman string) (allowed bool, errorCode, maxAmount string, err error)
	RestoreTransferLimit(ctx context.Context, payerBankID, currency, amountHuman string) error
}

// PaymentHandler exposes the payment-orchestrator operations as REST endpoints.
type PaymentHandler struct {
	payment      *paymentadapter.GRPCAdapter
	bankCode     string // institution fallback when JWT lacks BankID (e.g. user not yet registered in compliance)
	fiatSymbol   string // currency for transfer limit checks (e.g. "BRL", "ARS"); empty = skip check
	limitChecker TransferLimitChecker
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(payment *paymentadapter.GRPCAdapter, bankCode string) *PaymentHandler {
	return &PaymentHandler{payment: payment, bankCode: bankCode}
}

// WithLimitChecker attaches a transfer-limit checker and the fiat currency symbol.
func (h *PaymentHandler) WithLimitChecker(checker TransferLimitChecker, fiatSymbol string) *PaymentHandler {
	h.limitChecker = checker
	h.fiatSymbol = fiatSymbol
	return h
}

// --- HTLC endpoints ---

func (h *PaymentHandler) LockHTLC(c *fiber.Ctx) error {
	var req struct {
		AgreementID string `json:"agreement_id"`
		Receiver    string `json:"receiver"`
		Amount      string `json:"amount"`
		TimeLock    uint64 `json:"time_lock"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Receiver == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "receiver and amount are required"})
	}
	// Apply smart defaults
	if req.AgreementID == "" {
		req.AgreementID = uuid.NewString()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 3600 // 1h — initiator must have longer timelock
	}
	if err := h.checkAndDeductLimit(c, req.Amount); err != nil {
		return err
	}
	result, err := h.payment.LockHTLC(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock)
	if err != nil {
		h.restoreLimit(c, req.Amount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) LockHTLCWithHashLock(c *fiber.Ctx) error {
	var req struct {
		AgreementID string `json:"agreement_id"`
		Receiver    string `json:"receiver"`
		Amount      string `json:"amount"`
		TimeLock    uint64 `json:"time_lock"`
		HashLock    string `json:"hash_lock"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.HashLock == "" || req.Receiver == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hash_lock, receiver and amount are required"})
	}
	// Apply smart defaults
	if req.AgreementID == "" {
		req.AgreementID = uuid.NewString()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 1800 // 30min — responder must have shorter timelock than initiator
	}
	if err := h.checkAndDeductLimit(c, req.Amount); err != nil {
		return err
	}
	result, err := h.payment.LockHTLCWithHashLock(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock, req.HashLock)
	if err != nil {
		h.restoreLimit(c, req.Amount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) SettleHTLC(c *fiber.Ctx) error {
	var req struct {
		ContractID string `json:"contract_id"`
		Secret     string `json:"secret"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.SettleHTLC(c.Context(), req.ContractID, req.Secret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) RefundHTLC(c *fiber.Ctx) error {
	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RefundHTLC(c.Context(), req.ContractID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetHTLCStatus(c *fiber.Ctx) error {
	contractID := c.Params("contractId")
	if contractID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "contractId is required"})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}

	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	result, err := h.payment.GetHTLCStatus(ctx, contractID)
	if err != nil {
		if st, ok2 := status.FromError(err); ok2 && st.Code() == codes.PermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	counterparty, parseErr := isHTLCCounterparty(result.Sender, result.Receiver, callerBankID)
	if parseErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "identity format error: " + parseErr.Error()})
	}
	if !counterparty {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
	}

	return c.JSON(result)
}

func (h *PaymentHandler) SearchHTLC(c *fiber.Ctx) error {
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}

	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	results, err := h.payment.SearchHTLC(ctx,
		c.Query("agreement_id"),
		c.Query("sender"),
		c.Query("receiver"),
		c.Query("state"),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Keep only records where the caller's institution is a counterparty.
	filtered := results[:0]
	for _, r := range results {
		counterparty, parseErr := isHTLCCounterparty(r.Sender, r.Receiver, callerBankID)
		if parseErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "identity format error: " + parseErr.Error()})
		}
		if counterparty {
			filtered = append(filtered, r)
		}
	}

	return c.JSON(fiber.Map{"locks": filtered, "total": len(filtered)})
}

// isHTLCCounterparty reports whether bankID (from JWT claims) exactly matches
// the institution embedded in either Paladin identity (format: name@spoke-{prefix}-{bankID}).
// Exact segment matching is required: "bank-a" must not match "bank-abc".
// Parse errors are returned so callers can log them; on error the check fails closed.
func isHTLCCounterparty(sender, receiver, bankID string) (bool, error) {
	if bankID == "" {
		return false, nil
	}
	senderBank, sErr := bankIDFromIdentity(sender)
	receiverBank, rErr := bankIDFromIdentity(receiver)
	if sErr != nil {
		return false, sErr
	}
	if rErr != nil {
		return false, rErr
	}
	return senderBank == bankID || receiverBank == bankID, nil
}

// bankIDFromIdentity extracts the bank identifier from a Paladin identity string.
// e.g. "funded_operator@spoke-a-bank-a" → "bank-a"
//
// The Paladin identity format "{name}@{spoke-word}-{letter}-{bankID}" is structural
// to this function: the bankID is the third dash-delimited segment after the "@".
// If Paladin changes this naming convention, this function will return an error and
// all authorization checks will fail closed until the implementation is updated.
// Mirrors identity.BankID in the payment-orchestrator; keep both in sync or move to a shared module.
func bankIDFromIdentity(paladinIdentity string) (string, error) {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("bankIDFromIdentity: missing '@' in %q — expected format {name}@{spoke-word}-{letter}-{bankID}", paladinIdentity)
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 3 {
		return "", fmt.Errorf("bankIDFromIdentity: fewer than three dash-segments in %q — expected format spoke-{letter}-{bankID}", parts[1])
	}
	return segs[2], nil
}

// --- Token endpoints ---

func (h *PaymentHandler) MintToken(c *fiber.Ctx) error {
	var req struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.MintToken(c.Context(), req.To, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) BurnToken(c *fiber.Ctx) error {
	var req struct {
		From   string `json:"from"`
		Amount string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.From == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from and amount are required"})
	}
	result, err := h.payment.BurnToken(c.Context(), req.From, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) TransferToken(c *fiber.Ctx) error {
	var req struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.TransferToken(c.Context(), req.To, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetBalance(c *fiber.Ctx) error {
	result, err := h.payment.GetBalance(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetFiatBalance(c *fiber.Ctx) error {
	result, err := h.payment.GetFiatBalance(c.Context())
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// --- Escrow: Deposit endpoints ---

func (h *PaymentHandler) RegisterDeposit(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RegisterDeposit(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveDeposit(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.ApproveDeposit(c.Context(), req.DepositID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "approved"})
}

func (h *PaymentHandler) RejectDeposit(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
		Reason    string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectDeposit(c.Context(), req.DepositID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) RequestFiatExchange(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestFiatExchange(c.Context(), req.DepositID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ListDeposits(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	deposits, err := h.payment.ListDeposits(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"deposits": deposits, "total": len(deposits)})
}

// --- Escrow: Tokenization endpoints ---

func (h *PaymentHandler) RequestEscrow(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestEscrow(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveEscrow(c *fiber.Ctx) error {
	var req struct {
		EscrowID string `json:"escrow_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.ApproveEscrow(c.Context(), req.EscrowID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) RejectEscrow(c *fiber.Ctx) error {
	var req struct {
		EscrowID string `json:"escrow_id"`
		Reason   string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectEscrow(c.Context(), req.EscrowID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) ListEscrows(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	escrows, err := h.payment.ListEscrows(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"escrows": escrows, "total": len(escrows)})
}

// --- Escrow: Redeem endpoints ---

func (h *PaymentHandler) RequestRedeem(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
		ZetoTransferTxHash       string `json:"zeto_transfer_tx_hash"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestRedeem(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount, req.ZetoTransferTxHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveRedeem(c *fiber.Ctx) error {
	var req struct {
		RedeemID string `json:"redeem_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.ApproveRedeem(c.Context(), req.RedeemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) RejectRedeem(c *fiber.Ctx) error {
	var req struct {
		RedeemID string `json:"redeem_id"`
		Reason   string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectRedeem(c.Context(), req.RedeemID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) ListRedeems(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	redeems, err := h.payment.ListRedeems(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"redeems": redeems, "total": len(redeems)})
}

// --- FX Agreement endpoints ---

func (h *PaymentHandler) ProposeFXAgreement(c *fiber.Ctx) error {
	var req struct {
		TradeID         string `json:"trade_id"`
		CounterpartyB   string `json:"counterparty_b"`
		Originator      string `json:"originator"`
		SettlementAgent string `json:"settlement_agent"`
		Custodian       string `json:"custodian"`
		Beneficiary     string `json:"beneficiary"`
		OriginAmount    string `json:"origin_amount"`
		CounterAmount   string `json:"counter_amount"`
		OriginCurrency  string `json:"origin_currency"`
		CounterCurrency string `json:"counter_currency"`
		Rate            string `json:"rate"`
		ExpiryDate      uint64 `json:"expiry_date"`
		OnBehalf        bool   `json:"on_behalf"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CounterpartyB == "" || req.OriginAmount == "" || req.CounterAmount == "" ||
		req.OriginCurrency == "" || req.CounterCurrency == "" || req.Rate == "" || req.ExpiryDate == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "counterparty_b, origin_amount, counter_amount, origin_currency, counter_currency, rate, and expiry_date are required"})
	}
	result, err := h.payment.ProposeFXAgreement(c.Context(), &pb.ProposeFXAgreementRequest{
		TradeId:         req.TradeID,
		CounterpartyB:   req.CounterpartyB,
		Originator:      req.Originator,
		SettlementAgent: req.SettlementAgent,
		Custodian:       req.Custodian,
		Beneficiary:     req.Beneficiary,
		OriginAmount:    req.OriginAmount,
		CounterAmount:   req.CounterAmount,
		OriginCurrency:  req.OriginCurrency,
		CounterCurrency: req.CounterCurrency,
		Rate:            req.Rate,
		ExpiryDate:      req.ExpiryDate,
		OnBehalf:        req.OnBehalf,
	})
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) AcceptFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	var req struct {
		OnBehalf bool `json:"on_behalf"`
	}
	_ = c.BodyParser(&req)
	result, err := h.payment.AcceptFXAgreement(c.Context(), tradeID, req.OnBehalf)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) RejectFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	var req struct {
		OnBehalf bool `json:"on_behalf"`
	}
	_ = c.BodyParser(&req)
	result, err := h.payment.RejectFXAgreement(c.Context(), tradeID, req.OnBehalf)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) CancelFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	result, err := h.payment.CancelFXAgreement(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) SettleFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	result, err := h.payment.SettleFXAgreement(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	result, err := h.payment.GetFXAgreement(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"agreement": result})
}

func (h *PaymentHandler) ListFXAgreements(c *fiber.Ctx) error {
	results, err := h.payment.ListFXAgreements(c.Context(), c.Query("counterparty"), c.Query("state"))
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"agreements": results, "total": len(results)})
}

func (h *PaymentHandler) ListFXAgreementEvents(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	results, err := h.payment.ListFXAgreementEvents(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"events": results, "total": len(results)})
}

// checkAndDeductLimit enforces the CB daily transfer limit before an HTLC lock.
// Returns nil when no checker is configured (limits disabled) or when the transfer is allowed.
func (h *PaymentHandler) checkAndDeductLimit(c *fiber.Ctx, amount string) error {
	if h.limitChecker == nil || h.fiatSymbol == "" {
		return nil
	}
	allowed, errorCode, _, err := h.limitChecker.CheckAndDeductTransferLimit(c.Context(), h.bankCode, h.fiatSymbol, amount)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":      "transfer limit service unavailable",
			"error_code": "LIMIT_SERVICE_UNAVAILABLE",
		})
	}
	if !allowed {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "daily transfer limit exceeded",
			"error_code":         errorCode,
			"recommended_action": "Contact your Central Bank to review or increase the daily transfer limit.",
		})
	}
	return nil
}

// restoreLimit is best-effort: called when an HTLC lock fails after a successful deduction.
func (h *PaymentHandler) restoreLimit(c *fiber.Ctx, amount string) {
	if h.limitChecker == nil || h.fiatSymbol == "" {
		return
	}
	_ = h.limitChecker.RestoreTransferLimit(c.Context(), h.bankCode, h.fiatSymbol, amount)
}

// grpcErrorToHTTP maps gRPC status codes to appropriate HTTP responses.
func grpcErrorToHTTP(c *fiber.Ctx, err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case codes.InvalidArgument:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case codes.FailedPrecondition:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case codes.Unavailable:
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}
