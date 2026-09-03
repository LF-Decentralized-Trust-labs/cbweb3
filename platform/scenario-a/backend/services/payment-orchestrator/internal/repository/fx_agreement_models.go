// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"gorm.io/gorm"
)

// FXAgreementModel is the GORM model for the fx_agreements table.
type FXAgreementModel struct {
	TradeID         string    `gorm:"primaryKey;column:trade_id"`
	Originator      string    `gorm:"not null;column:originator"`
	CounterpartyB   string    `gorm:"not null;column:counterparty_b;index:idx_fx_agreements_counterparty"`
	SettlementAgent string    `gorm:"column:settlement_agent"`
	Custodian       string    `gorm:"column:custodian"`
	Beneficiary     string    `gorm:"column:beneficiary"`
	OriginAmount    string    `gorm:"not null;column:origin_amount"`
	CounterAmount   string    `gorm:"not null;column:counter_amount"`
	OriginCurrency  string    `gorm:"not null;column:origin_currency"`
	CounterCurrency string    `gorm:"not null;column:counter_currency"`
	Rate            string    `gorm:"not null;column:rate"`
	SourceSpokeId  string    `gorm:"column:source_spoke_id"`
	DestSpokeId    string    `gorm:"column:dest_spoke_id"`
	SourceReceiver string    `gorm:"column:source_receiver"`
	DestReceiver   string    `gorm:"column:dest_receiver"`
	ExpiryDate      uint64    `gorm:"not null;column:expiry_date"`
	State           string    `gorm:"not null;column:state;index:idx_fx_agreements_state"`
	OnChainTxHash   string    `gorm:"column:on_chain_tx_hash"`
	GroupID         string    `gorm:"column:group_id"`
	ContractAddress string    `gorm:"column:contract_address"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (FXAgreementModel) TableName() string { return "fx_agreements" }

// FXAgreementEventModel is the GORM model for the fx_agreement_events table.
// Events are append-only and must never be updated or deleted.
type FXAgreementEventModel struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;column:id"`
	TradeID    string    `gorm:"not null;column:trade_id;index:idx_fx_agreement_events_trade_id"`
	FromState  string    `gorm:"not null;column:from_state"`
	ToState    string    `gorm:"not null;column:to_state"`
	Actor      string    `gorm:"not null;column:actor"`
	OccurredAt time.Time `gorm:"not null;column:occurred_at;index:idx_fx_agreement_events_occurred_at"`
	Notes      string    `gorm:"column:notes"`
	TxHash     string    `gorm:"column:tx_hash"`
	Source     string    `gorm:"not null;column:source"`
}

func (FXAgreementEventModel) TableName() string { return "fx_agreement_events" }

// RelayDeliveryRecordModel tracks cross-spoke FX event forwarding with retry semantics.
// Ensures idempotent delivery and exponential backoff on failure.
type RelayDeliveryRecordModel struct {
	ID             int64      `gorm:"primaryKey;autoIncrement;column:id"`
	IdempotencyKey string     `gorm:"not null;column:idempotency_key;uniqueIndex:idx_relay_delivery_records_idempotency_key"`
	TradeID        string     `gorm:"not null;column:trade_id;index:idx_relay_delivery_records_trade_id"`
	EventType      string     `gorm:"not null;column:event_type"`
	SourceSpoke    string     `gorm:"column:source_spoke"`
	TargetSpoke    string     `gorm:"column:target_spoke"`
	Status         string     `gorm:"not null;column:status;index:idx_relay_delivery_records_status_next_retry"`
	AttemptCount   int        `gorm:"not null;column:attempt_count;default:0"`
	NextRetryAt    *time.Time `gorm:"column:next_retry_at;index:idx_relay_delivery_records_status_next_retry"`
	LastError      string     `gorm:"column:last_error"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

func (RelayDeliveryRecordModel) TableName() string { return "relay_delivery_records" }

// --- Model ↔ Domain converters ---

func fxAgreementToModel(r *domain.FXAgreementRecord) FXAgreementModel {
	return FXAgreementModel{
		TradeID:         r.TradeID,
		Originator:      r.Originator,
		CounterpartyB:   r.CounterpartyB,
		SettlementAgent: r.SettlementAgent,
		Custodian:       r.Custodian,
		Beneficiary:     r.Beneficiary,
		OriginAmount:    r.OriginAmount,
		CounterAmount:   r.CounterAmount,
		OriginCurrency:  r.OriginCurrency,
		CounterCurrency: r.CounterCurrency,
		Rate:            r.Rate,
		SourceSpokeId:   r.SourceSpokeId,
		DestSpokeId:     r.DestSpokeId,
		SourceReceiver:  r.SourceReceiver,
		DestReceiver:    r.DestReceiver,
		ExpiryDate:      r.ExpiryDate,
		State:           string(r.State),
		OnChainTxHash:   r.OnChainTxHash,
		GroupID:         r.GroupID,
		ContractAddress: r.ContractAddress,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func fxAgreementFromModel(m FXAgreementModel) *domain.FXAgreementRecord {
	return &domain.FXAgreementRecord{
		TradeID:         m.TradeID,
		Originator:      m.Originator,
		CounterpartyB:   m.CounterpartyB,
		SettlementAgent: m.SettlementAgent,
		Custodian:       m.Custodian,
		Beneficiary:     m.Beneficiary,
		OriginAmount:    m.OriginAmount,
		CounterAmount:   m.CounterAmount,
		OriginCurrency:  m.OriginCurrency,
		CounterCurrency: m.CounterCurrency,
		Rate:            m.Rate,
		SourceSpokeId:   m.SourceSpokeId,
		DestSpokeId:     m.DestSpokeId,
		SourceReceiver:  m.SourceReceiver,
		DestReceiver:    m.DestReceiver,
		ExpiryDate:      m.ExpiryDate,
		State:           domain.FXState(m.State),
		OnChainTxHash:   m.OnChainTxHash,
		GroupID:         m.GroupID,
		ContractAddress: m.ContractAddress,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func fxEventToModel(e *domain.FXAgreementEvent) FXAgreementEventModel {
	return FXAgreementEventModel{
		TradeID:    e.TradeID,
		FromState:  string(e.FromState),
		ToState:    string(e.ToState),
		Actor:      e.Actor,
		OccurredAt: e.OccurredAt,
		Notes:      e.Notes,
		TxHash:     e.TxHash,
		Source:     string(e.Source),
	}
}

func fxEventFromModel(m FXAgreementEventModel) *domain.FXAgreementEvent {
	return &domain.FXAgreementEvent{
		ID:         m.ID,
		TradeID:    m.TradeID,
		FromState:  domain.FXState(m.FromState),
		ToState:    domain.FXState(m.ToState),
		Actor:      m.Actor,
		OccurredAt: m.OccurredAt,
		Notes:      m.Notes,
		TxHash:     m.TxHash,
		Source:     domain.EventSource(m.Source),
	}
}

// Ensure GORM doesn't try to soft-delete events — they are append-only.
func (FXAgreementEventModel) BeforeDelete(*gorm.DB) error {
	return gorm.ErrInvalidData // blocks all deletions
}

// relayDeliveryToModel converts domain relay delivery record to GORM model.
func relayDeliveryToModel(r *domain.RelayDeliveryRecord) RelayDeliveryRecordModel {
	return RelayDeliveryRecordModel{
		ID:             r.ID,
		IdempotencyKey: r.IdempotencyKey,
		TradeID:        r.TradeID,
		EventType:      r.EventType,
		SourceSpoke:    r.SourceSpoke,
		TargetSpoke:    r.TargetSpoke,
		Status:         r.Status,
		AttemptCount:   r.AttemptCount,
		NextRetryAt:    r.NextRetryAt,
		LastError:      r.LastError,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// relayDeliveryFromModel converts GORM model to domain relay delivery record.
func relayDeliveryFromModel(m RelayDeliveryRecordModel) *domain.RelayDeliveryRecord {
	return &domain.RelayDeliveryRecord{
		ID:             m.ID,
		IdempotencyKey: m.IdempotencyKey,
		TradeID:        m.TradeID,
		EventType:      m.EventType,
		SourceSpoke:    m.SourceSpoke,
		TargetSpoke:    m.TargetSpoke,
		Status:         m.Status,
		AttemptCount:   m.AttemptCount,
		NextRetryAt:    m.NextRetryAt,
		LastError:      m.LastError,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
