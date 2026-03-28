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
