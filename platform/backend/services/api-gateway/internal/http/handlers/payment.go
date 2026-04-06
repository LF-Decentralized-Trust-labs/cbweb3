package handlers

import (
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/gofiber/fiber/v2"
)

// PaymentHandler exposes the payment-orchestrator operations as REST endpoints.
type PaymentHandler struct {
	payment *paymentadapter.GRPCAdapter
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(payment *paymentadapter.GRPCAdapter) *PaymentHandler {
	return &PaymentHandler{payment: payment}
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
	result, err := h.payment.LockHTLC(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock)
	if err != nil {
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
	result, err := h.payment.LockHTLCWithHashLock(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock, req.HashLock)
	if err != nil {
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
	result, err := h.payment.GetHTLCStatus(c.Context(), contractID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) SearchHTLC(c *fiber.Ctx) error {
	results, err := h.payment.SearchHTLC(c.Context(),
		c.Query("agreement_id"),
		c.Query("sender"),
		c.Query("receiver"),
		c.Query("state"),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"locks": results, "total": len(results)})
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
	identity := c.Query("identity")
	if identity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "identity query parameter is required"})
	}
	result, err := h.payment.GetBalance(c.Context(), identity)
	if err != nil {
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
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"reason": req.Reason})
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
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"reason": req.Reason})
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
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) ListRedeems(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	redeems, err := h.payment.ListRedeems(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"redeems": redeems, "total": len(redeems)})
}
