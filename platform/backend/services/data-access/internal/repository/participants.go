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

// Participant is the canonical domain struct for participant data.
type Participant struct {
	UserID          string
	DID             string
	WalletAddress   string
	Country         string
	BankCode        string
	Role            string
	InstitutionName string
}

// KYCCredential holds a VC pointer for a participant.
type KYCCredential struct {
	Subject    string
	ZKPPointer string
	VCJWT      string
	IssuedAt   string
}

// AuditEntry is the domain struct for an audit log record.
type AuditEntry struct {
	ActorSubject  string
	ActorAddress  string
	ActionType    string
	TargetSubject string
	CorrelationID string
	IPAddress     string
	Result        string
	Details       string
}

// ParticipantsRepository defines the persistence operations for participants.
type ParticipantsRepository interface {
	Upsert(ctx context.Context, p Participant) error
	GetByUser(ctx context.Context, userID string) (Participant, bool, error)
	UpsertKYCCredential(ctx context.Context, cred KYCCredential) error
	CreateAuditLog(ctx context.Context, entry AuditEntry) error
}

// --- In-memory implementation (dev / testing) ---

type memoryParticipantsRepository struct {
	mu          sync.RWMutex
	data        map[string]Participant
	credentials map[string]KYCCredential // subject → credential
}

func NewMemoryParticipantsRepository() ParticipantsRepository {
	return &memoryParticipantsRepository{
		data:        map[string]Participant{},
		credentials: map[string]KYCCredential{},
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

func (r *memoryParticipantsRepository) UpsertKYCCredential(_ context.Context, cred KYCCredential) error {
	r.mu.Lock()
	r.credentials[cred.Subject] = cred
	r.mu.Unlock()
	return nil
}

func (r *memoryParticipantsRepository) CreateAuditLog(_ context.Context, _ AuditEntry) error {
	return nil // no-op for in-memory; audit is fire-and-forget
}

// --- Postgres / GORM implementation ---

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
	if err := db.AutoMigrate(&ParticipantModel{}, &KYCCredentialModel{}, &AuditLogModel{}); err != nil {
		return nil, err
	}
	return &gormParticipantsRepository{db: db}, nil
}

func (r *gormParticipantsRepository) Upsert(ctx context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	model := ParticipantModel{
		UserID:          p.UserID,
		DID:             p.DID,
		WalletAddress:   p.WalletAddress,
		CountryCode:     p.Country,
		BankCode:        p.BankCode,
		Role:            p.Role,
		InstitutionName: p.InstitutionName,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"did", "wallet_address", "country_code", "bank_code",
				"participant_role", "institution_name", "updated_at",
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
		UserID:          model.UserID,
		DID:             model.DID,
		WalletAddress:   model.WalletAddress,
		Country:         model.CountryCode,
		BankCode:        model.BankCode,
		Role:            model.Role,
		InstitutionName: model.InstitutionName,
	}, true, nil
}

func (r *gormParticipantsRepository) UpsertKYCCredential(ctx context.Context, cred KYCCredential) error {
	model := KYCCredentialModel{
		Subject:    cred.Subject,
		ZKPPointer: cred.ZKPPointer,
		VCJWT:      cred.VCJWT,
		IssuedAt:   cred.IssuedAt,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "subject"}},
			DoUpdates: clause.AssignmentColumns([]string{"zkp_pointer", "vc_jwt", "issued_at", "updated_at"}),
		}).
		Create(&model).Error
}

func (r *gormParticipantsRepository) CreateAuditLog(ctx context.Context, entry AuditEntry) error {
	model := AuditLogModel{
		ActorSubject:  entry.ActorSubject,
		ActorAddress:  entry.ActorAddress,
		ActionType:    entry.ActionType,
		TargetSubject: entry.TargetSubject,
		CorrelationID: entry.CorrelationID,
		IPAddress:     entry.IPAddress,
		Result:        entry.Result,
		Details:       entry.Details,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}
