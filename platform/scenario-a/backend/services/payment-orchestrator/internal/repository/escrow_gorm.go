package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type gormEscrowRepository struct {
	db *gorm.DB
}

// NewGormEscrowRepository opens a PostgreSQL connection, runs AutoMigrate for
// deposits, escrows, and redeems tables, and returns an EscrowRepository.
func NewGormEscrowRepository(dsn string) (ports.EscrowRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&DepositModel{}, &EscrowModel{}, &RedeemModel{}); err != nil {
		return nil, err
	}
	return &gormEscrowRepository{db: db}, nil
}

// NewGormEscrowRepositoryFromDB uses an already-open *gorm.DB, runs AutoMigrate,
// and returns an EscrowRepository. Use this when sharing a single DB connection
// across multiple repositories.
func NewGormEscrowRepositoryFromDB(db *gorm.DB) (ports.EscrowRepository, error) {
	if err := db.AutoMigrate(&DepositModel{}, &EscrowModel{}, &RedeemModel{}); err != nil {
		return nil, err
	}
	return &gormEscrowRepository{db: db}, nil
}

// --- Deposit methods ---

func (r *gormEscrowRepository) CreateDeposit(ctx context.Context, record domain.DepositRecord) error {
	m := depositToModel(record)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *gormEscrowRepository) GetDeposit(ctx context.Context, id string) (domain.DepositRecord, bool, error) {
	var m DepositModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.DepositRecord{}, false, nil
	}
	if err != nil {
		return domain.DepositRecord{}, false, err
	}
	return depositFromModel(m), true, nil
}

func (r *gormEscrowRepository) UpdateDeposit(ctx context.Context, record domain.DepositRecord) error {
	result := r.db.WithContext(ctx).
		Model(&DepositModel{}).
		Where("id = ?", record.ID).
		Updates(map[string]interface{}{
			"status":           string(record.Status),
			"mint_tx_hash":     record.MintTxHash,
			"rejection_reason": record.RejectionReason,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("deposit %s not found", record.ID)
	}
	return nil
}

func (r *gormEscrowRepository) ListDeposits(ctx context.Context, requesterID string) ([]domain.DepositRecord, error) {
	q := r.db.WithContext(ctx).Model(&DepositModel{})
	if requesterID != "" {
		q = q.Where("requester_id = ?", requesterID)
	}
	var models []DepositModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DepositRecord, len(models))
	for i, m := range models {
		result[i] = depositFromModel(m)
	}
	return result, nil
}

// --- Escrow methods ---

func (r *gormEscrowRepository) CreateEscrow(ctx context.Context, record domain.EscrowRecord) error {
	m := escrowToModel(record)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *gormEscrowRepository) GetEscrow(ctx context.Context, id string) (domain.EscrowRecord, bool, error) {
	var m EscrowModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.EscrowRecord{}, false, nil
	}
	if err != nil {
		return domain.EscrowRecord{}, false, err
	}
	return escrowFromModel(m), true, nil
}

func (r *gormEscrowRepository) UpdateEscrow(ctx context.Context, record domain.EscrowRecord) error {
	result := r.db.WithContext(ctx).
		Model(&EscrowModel{}).
		Where("id = ?", record.ID).
		Updates(map[string]interface{}{
			"status":           string(record.Status),
			"burn_tx_hash":     record.BurnTxHash,
			"mint_tx_hash":     record.MintTxHash,
			"rejection_reason": record.RejectionReason,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("escrow %s not found", record.ID)
	}
	return nil
}

func (r *gormEscrowRepository) ListEscrows(ctx context.Context, requesterID string) ([]domain.EscrowRecord, error) {
	q := r.db.WithContext(ctx).Model(&EscrowModel{})
	if requesterID != "" {
		q = q.Where("requester_id = ?", requesterID)
	}
	var models []EscrowModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]domain.EscrowRecord, len(models))
	for i, m := range models {
		result[i] = escrowFromModel(m)
	}
	return result, nil
}

// --- Redeem methods ---

func (r *gormEscrowRepository) CreateRedeem(ctx context.Context, record domain.RedeemRecord) error {
	m := redeemToModel(record)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *gormEscrowRepository) GetRedeem(ctx context.Context, id string) (domain.RedeemRecord, bool, error) {
	var m RedeemModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.RedeemRecord{}, false, nil
	}
	if err != nil {
		return domain.RedeemRecord{}, false, err
	}
	return redeemFromModel(m), true, nil
}

func (r *gormEscrowRepository) UpdateRedeem(ctx context.Context, record domain.RedeemRecord) error {
	result := r.db.WithContext(ctx).
		Model(&RedeemModel{}).
		Where("id = ?", record.ID).
		Updates(map[string]interface{}{
			"status":                string(record.Status),
			"zeto_transfer_tx_hash": record.ZetoTransferTxHash,
			"fiat_mint_tx_hash":     record.FiatMintTxHash,
			"rejection_reason":      record.RejectionReason,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("redeem %s not found", record.ID)
	}
	return nil
}

func (r *gormEscrowRepository) ListRedeems(ctx context.Context, requesterID string) ([]domain.RedeemRecord, error) {
	q := r.db.WithContext(ctx).Model(&RedeemModel{})
	if requesterID != "" {
		q = q.Where("requester_id = ?", requesterID)
	}
	var models []RedeemModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]domain.RedeemRecord, len(models))
	for i, m := range models {
		result[i] = redeemFromModel(m)
	}
	return result, nil
}
