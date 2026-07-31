// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// RelayWatermarkModel persists the last processed event cursor per relay stream.
// Durable in Postgres (the service's shared DB), it lets the Cacti poller resume
// after a restart instead of resetting to time.Now() and dropping events observed
// during downtime (finding R2-H-11).
type RelayWatermarkModel struct {
	Kind      string    `gorm:"column:kind;primaryKey"`
	Value     int64     `gorm:"column:value;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName pins the table name so it is stable across model renames.
func (RelayWatermarkModel) TableName() string { return "relay_watermarks" }

// GormRelayWatermarkStore is a Postgres-backed ports.RelayWatermarkStore.
type GormRelayWatermarkStore struct {
	db *gorm.DB
}

// NewGormRelayWatermarkStoreFromDB uses an already-open *gorm.DB, runs AutoMigrate,
// and returns a ready store. It mirrors the other *FromDB repository constructors.
func NewGormRelayWatermarkStoreFromDB(db *gorm.DB) (*GormRelayWatermarkStore, error) {
	if err := db.AutoMigrate(&RelayWatermarkModel{}); err != nil {
		return nil, err
	}
	return &GormRelayWatermarkStore{db: db}, nil
}

// Ensure the concrete type satisfies the port.
var _ ports.RelayWatermarkStore = (*GormRelayWatermarkStore)(nil)

// GetWatermark returns the persisted cursor for kind, or found=false when absent.
func (s *GormRelayWatermarkStore) GetWatermark(ctx context.Context, kind string) (int64, bool, error) {
	var m RelayWatermarkModel
	err := s.db.WithContext(ctx).First(&m, "kind = ?", kind).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return m.Value, true, nil
}

// SetWatermark upserts the cursor for kind. On conflict it updates value and
// updated_at so the stored cursor is always the latest processed position.
func (s *GormRelayWatermarkStore) SetWatermark(ctx context.Context, kind string, value int64) error {
	m := RelayWatermarkModel{Kind: kind, Value: value, UpdatedAt: time.Now().UTC()}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kind"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&m).Error
}
