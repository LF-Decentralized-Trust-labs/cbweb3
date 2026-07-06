// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormFXAgreementRepository struct {
	db *gorm.DB
}

// NewGormFXAgreementRepository opens a PostgreSQL connection, runs AutoMigrate for
// fx_agreements, fx_agreement_events, and relay_delivery_records tables,
// and returns an FXAgreementRepository.
func NewGormFXAgreementRepository(dsn string) (ports.FXAgreementRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&FXAgreementModel{}, &FXAgreementEventModel{}, &RelayDeliveryRecordModel{}); err != nil {
		return nil, err
	}
	if err := RunSpokeKeyedMigration(db, slog.Default()); err != nil {
		return nil, err
	}
	return &gormFXAgreementRepository{db: db}, nil
}

// NewGormFXAgreementRepositoryFromDB uses an already-open *gorm.DB, runs AutoMigrate,
// and returns an FXAgreementRepository. Use this when sharing a single DB connection
// across multiple repositories.
func NewGormFXAgreementRepositoryFromDB(db *gorm.DB) (ports.FXAgreementRepository, error) {
	if err := db.AutoMigrate(&FXAgreementModel{}, &FXAgreementEventModel{}, &RelayDeliveryRecordModel{}); err != nil {
		return nil, err
	}
	if err := RunSpokeKeyedMigration(db, slog.Default()); err != nil {
		return nil, err
	}
	return &gormFXAgreementRepository{db: db}, nil
}

// CreateAgreement persists a new FX agreement. Idempotent on trade_id: the synchronous propose
// handler and the FXIndexer (both enabled on commercial-bank nodes — see entityenv.go) can race
// to project the same on-chain agreement, so a trade_id conflict is a benign duplicate, not an
// error — ON CONFLICT DO NOTHING silently no-ops instead of surfacing a unique-violation.
func (r *gormFXAgreementRepository) CreateAgreement(ctx context.Context, rec *domain.FXAgreementRecord) error {
	m := fxAgreementToModel(rec)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
}

// GetAgreement retrieves an FX agreement by trade_id. Returns (nil, nil) when not found.
func (r *gormFXAgreementRepository) GetAgreement(ctx context.Context, tradeID string) (*domain.FXAgreementRecord, error) {
	var m FXAgreementModel
	err := r.db.WithContext(ctx).Where("trade_id = ?", tradeID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fxAgreementFromModel(m), nil
}

// UpdateAgreement saves state and timestamp changes to an existing record.
func (r *gormFXAgreementRepository) UpdateAgreement(ctx context.Context, rec *domain.FXAgreementRecord) error {
	return r.db.WithContext(ctx).
		Model(&FXAgreementModel{}).
		Where("trade_id = ?", rec.TradeID).
		Updates(map[string]interface{}{
			"state":            string(rec.State),
			"on_chain_tx_hash": rec.OnChainTxHash,
			"group_id":         rec.GroupID,
			"contract_address": rec.ContractAddress,
			"updated_at":       rec.UpdatedAt,
		}).Error
}

// ListAgreements returns agreements matching the filter, ordered by created_at DESC.
func (r *gormFXAgreementRepository) ListAgreements(ctx context.Context, f ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	q := r.db.WithContext(ctx).Model(&FXAgreementModel{})
	if f.Counterparty != "" {
		q = q.Where("originator = ? OR counterparty_b = ?", f.Counterparty, f.Counterparty)
	}
	if f.State != "" {
		q = q.Where("state = ?", string(f.State))
	}
	var models []FXAgreementModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.FXAgreementRecord, len(models))
	for i, m := range models {
		result[i] = fxAgreementFromModel(m)
	}
	return result, nil
}

// CreateAuditEvent appends an immutable lifecycle event.
func (r *gormFXAgreementRepository) CreateAuditEvent(ctx context.Context, e *domain.FXAgreementEvent) error {
	m := fxEventToModel(e)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	e.ID = m.ID
	return nil
}

// ListAuditEvents returns all events for a trade_id ordered by occurred_at ASC.
func (r *gormFXAgreementRepository) ListAuditEvents(ctx context.Context, tradeID string) ([]*domain.FXAgreementEvent, error) {
	var models []FXAgreementEventModel
	if err := r.db.WithContext(ctx).
		Where("trade_id = ?", tradeID).
		Order("occurred_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.FXAgreementEvent, len(models))
	for i, m := range models {
		result[i] = fxEventFromModel(m)
	}
	return result, nil
}

// ListExpiredNonTerminal returns agreements past their expiry_date still in a non-terminal state.
func (r *gormFXAgreementRepository) ListExpiredNonTerminal(ctx context.Context, nowUnix int64) ([]*domain.FXAgreementRecord, error) {
	nonTerminal := []string{
		string(domain.FXStateProposed),
		string(domain.FXStateAccepted),
	}
	var models []FXAgreementModel
	if err := r.db.WithContext(ctx).
		Where("expiry_date > 0 AND expiry_date < ? AND state IN ?", uint64(nowUnix), nonTerminal). //#nosec G115 -- unix timestamp is always positive and fits uint64
		Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.FXAgreementRecord, len(models))
	for i, m := range models {
		result[i] = fxAgreementFromModel(m)
	}
	return result, nil
}

// RunSpokeKeyedMigration performs an idempotent migration from positional
// spoke_a_receiver/spoke_b_receiver columns to spoke-keyed fields.
// AutoMigrate already added the new columns; this function backfills data
// from old columns and drops them atomically within a transaction.
//
// The logger parameter is used for structured lifecycle logging.
func RunSpokeKeyedMigration(db *gorm.DB, logger *slog.Logger) error {
	if !db.Migrator().HasTable(&FXAgreementModel{}) {
		return nil
	}

	hasA := db.Migrator().HasColumn(&FXAgreementModel{}, "spoke_a_receiver")
	hasB := db.Migrator().HasColumn(&FXAgreementModel{}, "spoke_b_receiver")

	// Already on target schema — nothing to do.
	if !hasA && !hasB {
		return nil
	}

	logger.Info("spoke-keyed migration starting",
		"has_spoke_a_receiver", hasA,
		"has_spoke_b_receiver", hasB,
	)

	return db.Transaction(func(tx *gorm.DB) error {
		// Backfill only when both legacy columns are present (data hasn't
		// been migrated yet). In partial state the backfill already ran.
		if hasA && hasB {
			result := tx.Exec(
				`UPDATE fx_agreements SET source_spoke_id = 'spoke-a', dest_spoke_id = 'spoke-b', source_receiver = COALESCE(spoke_a_receiver, ''), dest_receiver = COALESCE(spoke_b_receiver, '') WHERE source_spoke_id IS NULL OR source_spoke_id = ''`,
			)
			if result.Error != nil {
				return fmt.Errorf("spoke-keyed migration backfill: %w", result.Error)
			}
			logger.Info("spoke-keyed migration backfill complete",
				"rows_affected", result.RowsAffected,
			)
		}

		// Drop legacy columns conditionally — each column is checked
		// independently so partial states are recovered automatically.
		if hasA {
			if err := tx.Exec("ALTER TABLE fx_agreements DROP COLUMN spoke_a_receiver").Error; err != nil {
				return fmt.Errorf("spoke-keyed migration drop spoke_a_receiver: %w", err)
			}
			logger.Info("spoke-keyed migration dropped column", "column", "spoke_a_receiver")
		}

		if hasB {
			if err := tx.Exec("ALTER TABLE fx_agreements DROP COLUMN spoke_b_receiver").Error; err != nil {
				return fmt.Errorf("spoke-keyed migration drop spoke_b_receiver: %w", err)
			}
			logger.Info("spoke-keyed migration dropped column", "column", "spoke_b_receiver")
		}

		logger.Info("spoke-keyed migration complete")
		return nil
	})
}

// nowUTC is a helper for consistent timestamps.
func nowUTC() time.Time { return time.Now().UTC() }
