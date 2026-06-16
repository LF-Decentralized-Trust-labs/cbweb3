// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/repository"
)

// SpokesHandler manages spoke CRUD endpoints.
type SpokesHandler struct {
	repo *repository.SpokesRepository
}

// NewSpokesHandler creates a new SpokesHandler.
func NewSpokesHandler(repo *repository.SpokesRepository) *SpokesHandler {
	return &SpokesHandler{repo: repo}
}

// Register mounts routes under the given router group.
func (h *SpokesHandler) Register(g fiber.Router) {
	g.Get("/spokes", h.list)
	g.Post("/spokes", h.create)
	g.Get("/spokes/:id", h.get)
	g.Put("/spokes/:id", h.update)
	g.Delete("/spokes/:id", h.delete)
}

type upsertSpokeRequest struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currency_code"`
	Jurisdiction string `json:"jurisdiction"`
	Active       *bool  `json:"active"`
}

func (h *SpokesHandler) list(c *fiber.Ctx) error {
	spokes, err := h.repo.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": spokes})
}

func (h *SpokesHandler) create(c *fiber.Ctx) error {
	var req upsertSpokeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" || req.CurrencyCode == "" || req.Jurisdiction == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, currency_code and jurisdiction are required"})
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	spoke := &domain.NocSpoke{
		Name:         req.Name,
		CurrencyCode: req.CurrencyCode,
		Jurisdiction: req.Jurisdiction,
		Active:       active,
	}
	if req.ID != "" {
		id, err := uuid.Parse(req.ID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
		}
		spoke.ID = id
	}
	if err := h.repo.Create(spoke); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(spoke)
}

func (h *SpokesHandler) get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	spoke, err := h.repo.GetByID(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "spoke not found"})
	}
	return c.JSON(spoke)
}

func (h *SpokesHandler) update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	spoke, err := h.repo.GetByID(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "spoke not found"})
	}
	var req upsertSpokeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name != "" {
		spoke.Name = req.Name
	}
	if req.CurrencyCode != "" {
		spoke.CurrencyCode = req.CurrencyCode
	}
	if req.Jurisdiction != "" {
		spoke.Jurisdiction = req.Jurisdiction
	}
	if req.Active != nil {
		spoke.Active = *req.Active
	}
	if err := h.repo.Update(spoke); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(spoke)
}

func (h *SpokesHandler) delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.repo.Delete(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
