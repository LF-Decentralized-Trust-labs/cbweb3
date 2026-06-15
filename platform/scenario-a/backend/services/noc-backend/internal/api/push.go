// SPDX-License-Identifier: Apache-2.0

package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/service"
)

// PushHandler handles inbound push payloads from NOC agents.
type PushHandler struct {
	db         *gorm.DB
	agents     *repository.AgentsRepository
	components *repository.ComponentsRepository
	alert      *service.AlertService
}

// NewPushHandler creates a PushHandler.
func NewPushHandler(
	db *gorm.DB,
	agents *repository.AgentsRepository,
	components *repository.ComponentsRepository,
	alert *service.AlertService,
) *PushHandler {
	return &PushHandler{db: db, agents: agents, components: components, alert: alert}
}

// Register mounts the internal push route.
func (h *PushHandler) Register(app fiber.Router) {
	app.Post("/internal/v1/push", middleware.AgentAuth(h.db), h.push)
}

// componentPayload mirrors the agent's JSON structure.
type componentPayload struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Endpoint    string  `json:"endpoint"`
	Status      string  `json:"status"`
	BlockNumber *int64  `json:"block_number,omitempty"`
	Diagnostic  *string `json:"diagnostic,omitempty"`
}

// transactionEventPayload mirrors the agent's tx event JSON.
type transactionEventPayload struct {
	TxID          string    `json:"tx_id"`
	EventType     string    `json:"event_type"`
	OccurredAt    time.Time `json:"occurred_at"`
	ParticipantID *string   `json:"participant_id,omitempty"`
	ContractID    *string   `json:"contract_id,omitempty"`
	ErrorCode     *string   `json:"error_code,omitempty"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
}

type logLinePayload struct {
	ComponentName string    `json:"component_name"`
	Stream        string    `json:"stream"`
	LogLine       string    `json:"log_line"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type pushPayload struct {
	SpokeID             string                    `json:"spoke_id"`
	AgentName           string                    `json:"agent_name"`
	PushIntervalSeconds int                       `json:"push_interval_seconds"`
	CollectedAt         time.Time                 `json:"collected_at"`
	Components          []componentPayload        `json:"components"`
	TransactionEvents   []transactionEventPayload `json:"transaction_events,omitempty"`
	LogLines            []logLinePayload          `json:"log_lines,omitempty"`
}

func (h *PushHandler) push(c *fiber.Ctx) error {
	agent, ok := middleware.GetAgent(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
	}

	var payload pushPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Validate spoke_id matches the provisioned key's binding
	spokeID, err := uuid.Parse(payload.SpokeID)
	if err != nil || spokeID != agent.SpokeID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "spoke_id mismatch"})
	}

	now := time.Now()

	// Update agent last_seen
	if err := h.agents.UpdateLastSeen(agent.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	// Build component name→ID index for log routing
	nameToComponentID := make(map[string]uuid.UUID)

	// Upsert components + record health events
	healthEvents := make([]domain.NocHealthEvent, 0, len(payload.Components))
	for _, cp := range payload.Components {
		comp := &domain.NocComponent{
			AgentID:         agent.ID,
			SpokeID:         spokeID,
			Name:            cp.Name,
			Type:            cp.Type,
			Endpoint:        cp.Endpoint,
			HealthStatus:    cp.Status,
			LastCheckedAt:   &now,
			LastBlockNumber: cp.BlockNumber,
		}
		if err := h.components.UpsertComponent(comp); err != nil {
			continue
		}
		nameToComponentID[cp.Name] = comp.ID

		prevStatus, _ := h.components.GetLastHealthStatus(comp.ID)

		event := domain.NocHealthEvent{
			ComponentID: comp.ID,
			Status:      cp.Status,
			OccurredAt:  payload.CollectedAt,
			ReceivedAt:  now,
			Diagnostic:  cp.Diagnostic,
			BlockNumber: cp.BlockNumber,
		}
		healthEvents = append(healthEvents, event)

		// Capture snapshot on transition to OFFLINE or UNKNOWN
		if (cp.Status == "OFFLINE" || cp.Status == "UNKNOWN") && prevStatus != cp.Status {
			_ = h.components.CaptureLogSnapshot(comp.ID, cp.Status)
		}

		// Evaluate and fire alerts
		h.alert.Evaluate(c.Context(), comp, cp.Status, prevStatus)
	}

	_ = h.components.BulkRecordHealthEvents(healthEvents)

	// Store transaction events
	if len(payload.TransactionEvents) > 0 {
		txEvents := make([]domain.NocTransactionEvent, 0, len(payload.TransactionEvents))
		for _, te := range payload.TransactionEvents {
			txEvents = append(txEvents, domain.NocTransactionEvent{
				TxID:          te.TxID,
				EventType:     te.EventType,
				OccurredAt:    te.OccurredAt,
				SpokeID:       spokeID,
				ParticipantID: te.ParticipantID,
				ContractID:    te.ContractID,
				ErrorCode:     te.ErrorCode,
				ErrorMessage:  te.ErrorMessage,
			})
		}
		h.db.Create(&txEvents)
	}

	// Store log lines
	if len(payload.LogLines) > 0 {
		// Group by component
		grouped := make(map[uuid.UUID][]domain.NocContainerLog)
		for _, ll := range payload.LogLines {
			compID, ok := nameToComponentID[ll.ComponentName]
			if !ok {
				continue
			}
			grouped[compID] = append(grouped[compID], domain.NocContainerLog{
				ComponentID: compID,
				Stream:      ll.Stream,
				LogLine:     ll.LogLine,
				OccurredAt:  ll.OccurredAt,
			})
		}
		for compID, lines := range grouped {
			_ = h.components.AppendLogs(compID, lines)
		}
	}

	return c.JSON(fiber.Map{"status": "accepted", "components": len(payload.Components)})
}
