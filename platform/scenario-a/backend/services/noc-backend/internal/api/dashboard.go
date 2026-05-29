package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/service"
)

// DashboardHandler exposes read endpoints for the NOC portal.
type DashboardHandler struct {
	db           *gorm.DB
	spokes        *repository.SpokesRepository
	agents        *repository.AgentsRepository
	components    *repository.ComponentsRepository
	alerts        *service.AlertService
	relayMetrics  *service.RelayMetricsService
}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler(
	db *gorm.DB,
	spokes *repository.SpokesRepository,
	agents *repository.AgentsRepository,
	components *repository.ComponentsRepository,
	alerts *service.AlertService,
	relayMetrics *service.RelayMetricsService,
) *DashboardHandler {
	return &DashboardHandler{
		db:           db,
		spokes:       spokes,
		agents:       agents,
		components:   components,
		alerts:       alerts,
		relayMetrics: relayMetrics,
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
	g.Get("/relays", h.listRelays)
	g.Get("/audit", h.listAudit)
	g.Get("/topology", h.getTopology)
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

	state := c.Query("state", "ACTIVE")
	var (
		alerts []domain.NocAlert
		err    error
	)
	if state == "RESOLVED" {
		alerts, err = h.alerts.ResolvedAlerts(filter)
	} else {
		alerts, err = h.alerts.ActiveAlerts(filter)
	}
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
	claims, _ := middleware.GetClaims(c)
	actor := claims.Subject
	if actor == "" {
		actor = "unknown"
	}
	if err := h.alerts.DismissAlert(c.Params("id"), actor); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DashboardHandler) listRelays(c *fiber.Ctx) error {
	window := 24 * time.Hour
	if w := c.Query("window"); w == "7d" {
		window = 7 * 24 * time.Hour
	}
	metrics, err := h.relayMetrics.ComputeRelayMetrics(window)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": metrics})
}

func (h *DashboardHandler) listAudit(c *fiber.Ctx) error {
	limit := 100
	offset := 0
	if l := c.QueryInt("limit", 100); l > 0 && l <= 500 {
		limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		offset = o
	}

	var entries []domain.NocAuditEntry
	if err := h.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": entries})
}

// TopologyNode is the response shape for a single node in the topology graph.
type TopologyNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`      // BESU | PALADIN | CACTI
	SpokeID   string `json:"spoke_id"`
	SpokeName string `json:"spoke_name"`
	Redundant bool   `json:"redundant"`
	Status    string `json:"status"` // HEALTHY | DEGRADED | DOWN
}

// TopologyEdge is the response shape for a connection between two nodes.
type TopologyEdge struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Healthy bool   `json:"healthy"`
}

func (h *DashboardHandler) getTopology(c *fiber.Ctx) error {
	var components []domain.NocComponent
	if err := h.db.Find(&components).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var spokes []domain.NocSpoke
	if err := h.db.Find(&spokes).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	spokeNames := make(map[string]string, len(spokes))
	for _, s := range spokes {
		spokeNames[s.ID.String()] = s.Name
	}

	// Count components of each (spokeID, type) pair to detect redundancy.
	type spokeTypeKey struct{ spokeID, typ string }
	typeCounts := make(map[spokeTypeKey]int)
	for _, comp := range components {
		typeCounts[spokeTypeKey{comp.SpokeID.String(), comp.Type}]++
	}

	mapKind := func(t string) string {
		if t == "CACTI_RELAY" {
			return "CACTI"
		}
		return t
	}
	mapStatus := func(s string) string {
		if s == "OFFLINE" || s == "UNKNOWN" {
			return "DOWN"
		}
		return s
	}

	// Build nodes.
	nodes := make([]TopologyNode, 0, len(components))
	for _, comp := range components {
		key := spokeTypeKey{comp.SpokeID.String(), comp.Type}
		nodes = append(nodes, TopologyNode{
			ID:        comp.ID.String(),
			Label:     comp.Name,
			Kind:      mapKind(comp.Type),
			SpokeID:   comp.SpokeID.String(),
			SpokeName: spokeNames[comp.SpokeID.String()],
			Redundant: typeCounts[key] > 1,
			Status:    mapStatus(comp.HealthStatus),
		})
	}

	// Build edges: full mesh within each spoke.
	// Group component IDs by spoke.
	spokeComps := make(map[string][]domain.NocComponent)
	for _, comp := range components {
		sid := comp.SpokeID.String()
		spokeComps[sid] = append(spokeComps[sid], comp)
	}

	edges := make([]TopologyEdge, 0)
	for _, comps := range spokeComps {
		for i := 0; i < len(comps); i++ {
			for j := i + 1; j < len(comps); j++ {
				a, b := comps[i], comps[j]
				healthy := mapStatus(a.HealthStatus) == "HEALTHY" && mapStatus(b.HealthStatus) == "HEALTHY"
				edges = append(edges, TopologyEdge{
					ID:      "e-" + a.ID.String() + "-" + b.ID.String(),
					From:    a.ID.String(),
					To:      b.ID.String(),
					Healthy: healthy,
				})
			}
		}
	}

	return c.JSON(fiber.Map{"data": fiber.Map{"nodes": nodes, "edges": edges}})
}
