package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
)

// AlertService evaluates component health transitions and records alerts/incidents.
type AlertService struct {
	db *gorm.DB
}

// NewAlertService creates a new AlertService.
func NewAlertService(db *gorm.DB) *AlertService {
	return &AlertService{db: db}
}

// Evaluate fires or resolves alerts based on component state transitions.
func (s *AlertService) Evaluate(ctx context.Context, comp *domain.NocComponent, newStatus, prevStatus string) {
	if newStatus == prevStatus {
		return
	}

	if newStatus == "HEALTHY" {
		// Resolve any ACTIVE alerts for this component
		now := time.Now()
		s.db.Model(&domain.NocAlert{}).
			Where("component_id = ? AND state = 'ACTIVE'", comp.ID).
			Updates(map[string]any{"state": "RESOLVED", "resolved_at": now})
		s.resolveIncidentsIfNeeded(comp)
		return
	}

	severity := resolveSeverity(comp.Type, newStatus)
	if severity == "" {
		return
	}

	sig := rootCauseSig(comp.ID.String(), newStatus)
	title := fmt.Sprintf("[%s] %s %s", severity, comp.Name, newStatus)

	// Upsert incident
	incident := &domain.NocIncident{}
	s.db.Where("root_cause_sig = ? AND state = 'OPEN'", sig).First(incident)
	if incident.ID.String() == "00000000-0000-0000-0000-000000000000" {
		incident = &domain.NocIncident{
			RootCauseSig: sig,
			State:        "OPEN",
			Title:        title,
			SpokeID:      &comp.SpokeID,
		}
		s.db.Create(incident)
	}

	// Create alert
	alert := &domain.NocAlert{
		ComponentID:  comp.ID,
		IncidentID:   &incident.ID,
		Severity:     severity,
		State:        "ACTIVE",
		Title:        title,
		RootCauseSig: sig,
	}
	s.db.Create(alert)
}

func (s *AlertService) resolveIncidentsIfNeeded(comp *domain.NocComponent) {
	sig := rootCauseSig(comp.ID.String(), "")
	now := time.Now()
	s.db.Model(&domain.NocIncident{}).
		Where("root_cause_sig LIKE ? AND state = 'OPEN'", sig+"%").
		Updates(map[string]any{"state": "RESOLVED", "resolved_at": now})
}

// resolveSeverity returns alert severity based on component type and new status.
func resolveSeverity(componentType, status string) string {
	switch status {
	case "OFFLINE":
		switch componentType {
		case "BESU", "PALADIN":
			return "CRITICAL"
		case "CACTI_RELAY":
			return "HIGH"
		default:
			return "HIGH"
		}
	case "DEGRADED":
		return "WARNING"
	case "UNKNOWN":
		return "HIGH"
	}
	return ""
}

func rootCauseSig(componentID, status string) string {
	return fmt.Sprintf("comp:%s:%s", componentID, status)
}

// ActiveAlerts returns currently ACTIVE alerts, optionally filtered by spoke.
func (s *AlertService) ActiveAlerts(spokeID *string) ([]domain.NocAlert, error) {
	q := s.db.Model(&domain.NocAlert{}).Where("noc_alerts.state = 'ACTIVE'").Order("noc_alerts.created_at DESC")
	if spokeID != nil && *spokeID != "" {
		q = q.Joins("JOIN noc_components ON noc_components.id = noc_alerts.component_id").
			Where("noc_components.spoke_id = ?", *spokeID)
	}
	var alerts []domain.NocAlert
	if err := q.Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

// GetAlertDetail returns a single alert with its associated component.
func (s *AlertService) GetAlertDetail(id string) (*domain.NocAlertDetail, error) {
	var alert domain.NocAlert
	if err := s.db.Where("id = ?", id).First(&alert).Error; err != nil {
		return nil, err
	}
	var comp domain.NocComponent
	if err := s.db.Where("id = ?", alert.ComponentID).First(&comp).Error; err != nil {
		return nil, err
	}
	return &domain.NocAlertDetail{NocAlert: alert, Component: comp}, nil
}

// AcknowledgeAlert marks an alert as acknowledged by the given username.
func (s *AlertService) AcknowledgeAlert(id, username string) error {
	result := s.db.Model(&domain.NocAlert{}).
		Where("id = ? AND state = 'ACTIVE'", id).
		Update("acknowledged_by", username)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("alert not found or already resolved")
	}
	return nil
}

// DismissAlert force-resolves an alert regardless of component state.
func (s *AlertService) DismissAlert(id string) error {
	now := time.Now()
	result := s.db.Model(&domain.NocAlert{}).
		Where("id = ?", id).
		Updates(map[string]any{"state": "RESOLVED", "resolved_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("alert not found")
	}
	return nil
}
