package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/service"
)

// DashboardHandler exposes read endpoints for the NOC portal.
type DashboardHandler struct {
	spokes     *repository.SpokesRepository
	agents     *repository.AgentsRepository
	components *repository.ComponentsRepository
	alerts     *service.AlertService
}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler(
	spokes *repository.SpokesRepository,
	agents *repository.AgentsRepository,
	components *repository.ComponentsRepository,
	alerts *service.AlertService,
) *DashboardHandler {
	return &DashboardHandler{
		spokes:     spokes,
		agents:     agents,
		components: components,
		alerts:     alerts,
	}
}

// Register mounts read-only dashboard routes (requires JWT auth).
func (h *DashboardHandler) Register(g fiber.Router) {
	g.Get("/spokes", h.listSpokes)
	g.Get("/spokes/:spokeId/agents", h.listAgents)
	g.Get("/spokes/:spokeId/components", h.listComponents)
	g.Get("/components/:id/health", h.componentHealth)
	g.Get("/components/:id/logs", h.componentLogs)
	g.Get("/alerts", h.listAlerts)
	g.Get("/alerts/:id", h.getAlert)
	g.Post("/alerts/:id/acknowledge", h.acknowledgeAlert)
	g.Post("/alerts/:id/dismiss", h.dismissAlert)
}

func (h *DashboardHandler) listSpokes(c *fiber.Ctx) error {
	spokes, err := h.spokes.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": spokes})
}

func (h *DashboardHandler) listAgents(c *fiber.Ctx) error {
	spokeID, err := uuid.Parse(c.Params("spokeId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid spoke id"})
	}
	agents, err := h.agents.ListAgents(&spokeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": agents})
}

func (h *DashboardHandler) listComponents(c *fiber.Ctx) error {
	spokeID, err := uuid.Parse(c.Params("spokeId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid spoke id"})
	}
	components, err := h.components.ListBySpoke(spokeID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": components})
}

func (h *DashboardHandler) componentHealth(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid component id"})
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := h.components.HealthHistory(id, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": events})
}

func (h *DashboardHandler) componentLogs(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid component id"})
	}
	n := 200
	if l := c.Query("tail"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			n = v
		}
	}
	logs, err := h.components.GetRecentLogs(id, n)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": logs})
}

func (h *DashboardHandler) listAlerts(c *fiber.Ctx) error {
	spokeIDStr := c.Query("spoke_id")
	var filter *string
	if spokeIDStr != "" {
		filter = &spokeIDStr
	}
	alerts, err := h.alerts.ActiveAlerts(filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": alerts})
}

func (h *DashboardHandler) getAlert(c *fiber.Ctx) error {
	detail, err := h.alerts.GetAlertDetail(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": detail})
}

func (h *DashboardHandler) acknowledgeAlert(c *fiber.Ctx) error {
	claims, _ := middleware.GetClaims(c)
	username := claims.Subject
	if username == "" {
		username = "unknown"
	}
	if err := h.alerts.AcknowledgeAlert(c.Params("id"), username); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DashboardHandler) dismissAlert(c *fiber.Ctx) error {
	if err := h.alerts.DismissAlert(c.Params("id")); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
