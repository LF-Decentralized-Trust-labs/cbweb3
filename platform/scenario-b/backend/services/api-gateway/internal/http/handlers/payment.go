// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PaymentHandler exposes the payment-orchestrator operations as REST endpoints.
type PaymentHandler struct {
	payment *paymentadapter.GRPCAdapter
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(payment *paymentadapter.GRPCAdapter) *PaymentHandler {
	return &PaymentHandler{payment: payment}
}

// --- Token balance endpoint ---

func (h *PaymentHandler) GetBalance(c *fiber.Ctx) error {
	result, err := h.payment.GetBalance(c.Context())
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// --- Deposit endpoints (Central Bank side) ---

func (h *PaymentHandler) RegisterDeposit(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress string `json:"requester_besu_address"`
		Amount               string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RegisterDeposit(c.Context(), req.RequesterBesuAddress, req.Amount)
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
	result, err := h.payment.ApproveDeposit(c.Context(), req.DepositID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "approved", "fiat_mint_tx_hash": result.FiatMintTxHash})
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

// --- Fiat balance endpoint ---

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

// --- Escrow endpoints (Central Bank side) ---

// RequestEscrow is called from the internal relay (commercial bank proxy) to register a tokenization request.
func (h *PaymentHandler) RequestEscrow(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress string `json:"requester_besu_address"`
		DepositID            string `json:"deposit_id"`
		Amount               string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestEscrow(c.Context(), req.RequesterBesuAddress, req.DepositID, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// ListEscrows returns all escrow records, optionally filtered by requester_id query param.
func (h *PaymentHandler) ListEscrows(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	escrows, err := h.payment.ListEscrows(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"escrows": escrows, "total": len(escrows)})
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

// --- Redeem endpoints (Central Bank side) ---

func (h *PaymentHandler) RequestRedeem(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress string `json:"requester_besu_address"`
		Amount               string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestRedeem(c.Context(), req.RequesterBesuAddress, req.Amount)
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
