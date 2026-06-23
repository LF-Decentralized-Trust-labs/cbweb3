// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type gormHTLCRepository struct {
	db *gorm.DB
}

// NewGormHTLCRepository opens a PostgreSQL connection, runs AutoMigrate for
// the htlcs table, and returns an HTLCRepository.
func NewGormHTLCRepository(dsn string) (ports.HTLCRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&HTLCModel{}); err != nil {
		return nil, err
	}
	return &gormHTLCRepository{db: db}, nil
}

// NewGormHTLCRepositoryFromDB uses an already-open *gorm.DB, runs AutoMigrate,
// and returns an HTLCRepository. Use this when sharing a single DB connection
// across multiple repositories.
func NewGormHTLCRepositoryFromDB(db *gorm.DB) (ports.HTLCRepository, error) {
	if err := db.AutoMigrate(&HTLCModel{}); err != nil {
		return nil, err
	}
	return &gormHTLCRepository{db: db}, nil
}

// CreateHTLC persists a new HTLC record.
func (r *gormHTLCRepository) CreateHTLC(ctx context.Context, record *domain.HTLCRecord) error {
	m := htlcToModel(record)
	return r.db.WithContext(ctx).Create(&m).Error
}

// GetHTLC retrieves an HTLC record by contract_id. Returns (nil, nil) when not found.
func (r *gormHTLCRepository) GetHTLC(ctx context.Context, contractID string) (*domain.HTLCRecord, error) {
	var m HTLCModel
	err := r.db.WithContext(ctx).Where("contract_id = ?", contractID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return htlcFromModel(m), nil
}

// GetHTLCByHashLock retrieves an HTLC record by hash_lock. Returns (nil, nil) when not found.
func (r *gormHTLCRepository) GetHTLCByHashLock(ctx context.Context, hashLock string) (*domain.HTLCRecord, error) {
	var m HTLCModel
	err := r.db.WithContext(ctx).Where("hash_lock = ?", hashLock).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return htlcFromModel(m), nil
}

// UpdateHTLC saves state and relevant field changes to an existing record.
func (r *gormHTLCRepository) UpdateHTLC(ctx context.Context, record *domain.HTLCRecord) error {
	result := r.db.WithContext(ctx).
		Model(&HTLCModel{}).
		Where("contract_id = ?", record.ContractID).
		Updates(map[string]interface{}{
			"state":               string(record.State),
			"secret":              record.Secret,
			"htlc_tx_hash":        record.HTLCTxHash,
			"zeto_tx_hash":        record.ZetoTxHash,
			"counterparty_locked": record.CounterpartyLocked,
			"updated_at":          time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("htlc %s not found", record.ContractID)
	}
	return nil
}

// ListHTLCs returns HTLC records matching the filter, ordered by created_at DESC.
func (r *gormHTLCRepository) ListHTLCs(ctx context.Context, filter ports.HTLCFilter) ([]*domain.HTLCRecord, error) {
	q := r.db.WithContext(ctx).Model(&HTLCModel{})
	if filter.AgreementID != "" {
		q = q.Where("agreement_id = ?", filter.AgreementID)
	}
	if filter.Sender != "" {
		q = q.Where("sender = ?", filter.Sender)
	}
	if filter.Receiver != "" {
		q = q.Where("receiver = ?", filter.Receiver)
	}
	if filter.State != "" {
		q = q.Where("state = ?", filter.State)
	}
	var models []HTLCModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.HTLCRecord, len(models))
	for i, m := range models {
		result[i] = htlcFromModel(m)
	}
	return result, nil
}

// ListNonTerminal returns all HTLC records that are not in a terminal state
// (SETTLED, REFUNDED, or INVALID).
func (r *gormHTLCRepository) ListNonTerminal(ctx context.Context) ([]*domain.HTLCRecord, error) {
	terminal := []string{
		string(domain.HTLCStateSettled),
		string(domain.HTLCStateRefunded),
		string(domain.HTLCStateInvalid),
	}
	var models []HTLCModel
	if err := r.db.WithContext(ctx).
		Where("state NOT IN ?", terminal).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.HTLCRecord, len(models))
	for i, m := range models {
		result[i] = htlcFromModel(m)
	}
	return result, nil
}
