package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Participant struct {
	UserID         string
	DID            string
	WalletAddress  string
	Country        string
	BankCode       string
	Role           string
	SignerProvider string
	KMSKeyID       string
}

type ParticipantsRepository interface {
	Upsert(ctx context.Context, p Participant) error
	GetByUser(ctx context.Context, userID string) (Participant, bool, error)
}

type memoryParticipantsRepository struct {
	mu   sync.RWMutex
	data map[string]Participant
}

func NewMemoryParticipantsRepository() ParticipantsRepository {
	return &memoryParticipantsRepository{
		data: map[string]Participant{},
	}
}

func (r *memoryParticipantsRepository) Upsert(_ context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[p.UserID] = p
	return nil
}

func (r *memoryParticipantsRepository) GetByUser(_ context.Context, userID string) (Participant, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.data[userID]
	return p, ok, nil
}

type gormParticipantsRepository struct {
	db *gorm.DB
}

type PostgresConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func NewGormParticipantsRepository(dsn string) (ParticipantsRepository, error) {
	return NewGormParticipantsRepositoryWithConfig(dsn, PostgresConfig{})
}

func NewGormParticipantsRepositoryWithConfig(dsn string, cfg PostgresConfig) (ParticipantsRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if err := db.AutoMigrate(&ParticipantModel{}); err != nil {
		return nil, err
	}
	return &gormParticipantsRepository{db: db}, nil
}

func (r *gormParticipantsRepository) Upsert(ctx context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	model := ParticipantModel{
		UserID:         p.UserID,
		DID:            p.DID,
		WalletAddress:  p.WalletAddress,
		Country:        p.Country,
		BankCode:       p.BankCode,
		Role:           p.Role,
		SignerProvider: p.SignerProvider,
		KMSKeyID:       p.KMSKeyID,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"did",
				"wallet_address",
				"country",
				"bank_code",
				"role",
				"signer_provider",
				"kms_key_id",
			}),
		}).
		Create(&model).Error
}

func (r *gormParticipantsRepository) GetByUser(ctx context.Context, userID string) (Participant, bool, error) {
	var model ParticipantModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Participant{}, false, nil
	}
	if err != nil {
		return Participant{}, false, err
	}
	return Participant{
		UserID:         model.UserID,
		DID:            model.DID,
		WalletAddress:  model.WalletAddress,
		Country:        model.Country,
		BankCode:       model.BankCode,
		Role:           model.Role,
		SignerProvider: model.SignerProvider,
		KMSKeyID:       model.KMSKeyID,
	}, true, nil
}
