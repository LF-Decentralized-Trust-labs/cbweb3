// SPDX-License-Identifier: Apache-2.0

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

// --- Domain structs ---

type Participant struct {
	UserID              string
	InstitutionName     string
	LegalEntityID       string
	BankCode            string
	CountryCode         string
	Role                string
	WalletAddress       string
	Status              string
	CertificateData     string
	CertificateExpiry   *time.Time
	BlockchainPubKeyHex string
	CsrPem              string
	PopNonce            string
	PopNonceExpiresAt   *time.Time
	RejectionReason     string
}

type AuditEntry struct {
	ActorSubject  string
	ActorAddress  string
	ActionType    string
	TargetSubject string
	CorrelationID string
	IPAddress     string
	Result        string
	Category      string
	Severity      string
	Details       string
}

type AuditFilter struct {
	Category string
	Severity string
	FromDate time.Time
	ToDate   time.Time
	Page     int
	Limit    int
}

type AuditRecord struct {
	LogID         string
	Timestamp     time.Time
	ActorSubject  string
	ActorAddress  string
	ActionType    string
	TargetSubject string
	CorrelationID string
	IPAddress     string
	Result        string
	Category      string
	Severity      string
	Details       string
}

type ParticipantFilter struct {
	Status   string
	Search   string
	BankCode string // exact match filter (takes precedence over Search when set)
}

type SystemParameter struct {
	Key       string
	Value     string
	UpdatedBy string
}

// --- Repository interface ---

type Repository interface {
	UpsertParticipant(ctx context.Context, p Participant) error
	GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error)
	ListParticipants(ctx context.Context, f ParticipantFilter) ([]Participant, error)
	CreateAuditLog(ctx context.Context, entry AuditEntry) error
	GetAuditLogs(ctx context.Context, f AuditFilter) ([]AuditRecord, error)
	GetSystemParameter(ctx context.Context, key string) (string, bool, error)
	UpsertSystemParameter(ctx context.Context, p SystemParameter) error
}

// --- In-memory implementation (dev / testing) ---

type memoryRepository struct {
	mu           sync.RWMutex
	participants map[string]Participant
	auditLogs    []AuditRecord
	params       map[string]SystemParameter
}

func NewMemoryRepository() Repository {
	return &memoryRepository{
		participants: map[string]Participant{},
		params:       map[string]SystemParameter{},
	}
}

func (r *memoryRepository) UpsertParticipant(_ context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participants[p.UserID] = p
	return nil
}

func (r *memoryRepository) GetParticipantByUser(_ context.Context, userID string) (Participant, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.participants[userID]
	return p, ok, nil
}

func (r *memoryRepository) ListParticipants(_ context.Context, f ParticipantFilter) ([]Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Participant
	for _, p := range r.participants {
		if f.Status != "" && p.Status != f.Status {
			continue
		}
		if f.BankCode != "" && p.BankCode != f.BankCode {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *memoryRepository) CreateAuditLog(_ context.Context, entry AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditLogs = append(r.auditLogs, AuditRecord{
		Timestamp:     time.Now(),
		ActorSubject:  entry.ActorSubject,
		ActorAddress:  entry.ActorAddress,
		ActionType:    entry.ActionType,
		TargetSubject: entry.TargetSubject,
		CorrelationID: entry.CorrelationID,
		IPAddress:     entry.IPAddress,
		Result:        entry.Result,
		Category:      entry.Category,
		Severity:      entry.Severity,
		Details:       entry.Details,
	})
	return nil
}

func (r *memoryRepository) GetAuditLogs(_ context.Context, f AuditFilter) ([]AuditRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	var result []AuditRecord
	for _, rec := range r.auditLogs {
		if f.Category != "" && rec.Category != f.Category {
			continue
		}
		if f.Severity != "" && rec.Severity != f.Severity {
			continue
		}
		result = append(result, rec)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (r *memoryRepository) GetSystemParameter(_ context.Context, key string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.params[key]
	if !ok {
		return "", false, nil
	}
	return p.Value, true, nil
}

func (r *memoryRepository) UpsertSystemParameter(_ context.Context, p SystemParameter) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.params[p.Key] = p
	return nil
}

// --- Postgres / GORM implementation ---

type gormRepository struct {
	db *gorm.DB
}

func NewGormRepository(dsn string) (Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&ParticipantModel{},
		&AuditLogModel{},
		&SystemParameterModel{},
	); err != nil {
		return nil, err
	}
	return &gormRepository{db: db}, nil
}

func (r *gormRepository) UpsertParticipant(ctx context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	model := participantToModel(p)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"institution_name", "legal_entity_id", "bank_code", "country_code",
				"participant_role", "wallet_address", "status",
				"certificate_data", "certificate_expiry",
				"blockchain_pub_key_hex", "csr_pem",
				"pop_nonce", "pop_nonce_expires_at", "rejection_reason",
				"updated_at",
			}),
		}).
		Create(&model).Error
}

func (r *gormRepository) GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error) {
	var model ParticipantModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Participant{}, false, nil
	}
	if err != nil {
		return Participant{}, false, err
	}
	return participantFromModel(model), true, nil
}

func (r *gormRepository) ListParticipants(ctx context.Context, f ParticipantFilter) ([]Participant, error) {
	q := r.db.WithContext(ctx).Model(&ParticipantModel{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.BankCode != "" {
		q = q.Where("bank_code = ?", f.BankCode)
	} else if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("institution_name ILIKE ? OR legal_entity_id ILIKE ?", like, like)
	}
	var models []ParticipantModel
	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]Participant, len(models))
	for i, m := range models {
		result[i] = participantFromModel(m)
	}
	return result, nil
}

func (r *gormRepository) CreateAuditLog(ctx context.Context, entry AuditEntry) error {
	return r.db.WithContext(ctx).Create(&AuditLogModel{
		ActorSubject:  entry.ActorSubject,
		ActorAddress:  entry.ActorAddress,
		ActionType:    entry.ActionType,
		TargetSubject: entry.TargetSubject,
		CorrelationID: entry.CorrelationID,
		IPAddress:     entry.IPAddress,
		Result:        entry.Result,
		Category:      entry.Category,
		Severity:      entry.Severity,
		Details:       entry.Details,
	}).Error
}

func (r *gormRepository) GetAuditLogs(ctx context.Context, f AuditFilter) ([]AuditRecord, error) {
	q := r.db.WithContext(ctx).Model(&AuditLogModel{}).Order("timestamp DESC")
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if !f.FromDate.IsZero() {
		q = q.Where("timestamp >= ?", f.FromDate)
	}
	if !f.ToDate.IsZero() {
		q = q.Where("timestamp <= ?", f.ToDate)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	q = q.Limit(limit).Offset((page - 1) * limit)

	var models []AuditLogModel
	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]AuditRecord, len(models))
	for i, m := range models {
		result[i] = AuditRecord{
			LogID:         m.LogID,
			Timestamp:     m.Timestamp,
			ActorSubject:  m.ActorSubject,
			ActorAddress:  m.ActorAddress,
			ActionType:    m.ActionType,
			TargetSubject: m.TargetSubject,
			CorrelationID: m.CorrelationID,
			IPAddress:     m.IPAddress,
			Result:        m.Result,
			Category:      m.Category,
			Severity:      m.Severity,
			Details:       m.Details,
		}
	}
	return result, nil
}

func (r *gormRepository) GetSystemParameter(ctx context.Context, key string) (string, bool, error) {
	var model SystemParameterModel
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return model.Value, true, nil
}

func (r *gormRepository) UpsertSystemParameter(ctx context.Context, p SystemParameter) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_by", "updated_at"}),
		}).
		Create(&SystemParameterModel{
			Key:       p.Key,
			Value:     p.Value,
			UpdatedBy: p.UpdatedBy,
		}).Error
}

// --- Mapping helpers ---

func participantToModel(p Participant) ParticipantModel {
	return ParticipantModel{
		UserID:              p.UserID,
		InstitutionName:     p.InstitutionName,
		LegalEntityID:       p.LegalEntityID,
		BankCode:            p.BankCode,
		CountryCode:         p.CountryCode,
		Role:                p.Role,
		WalletAddress:       p.WalletAddress,
		Status:              p.Status,
		CertificateData:     p.CertificateData,
		CertificateExpiry:   p.CertificateExpiry,
		BlockchainPubKeyHex: p.BlockchainPubKeyHex,
		CsrPem:              p.CsrPem,
		PopNonce:            p.PopNonce,
		PopNonceExpiresAt:   p.PopNonceExpiresAt,
		RejectionReason:     p.RejectionReason,
	}
}

func participantFromModel(m ParticipantModel) Participant {
	return Participant{
		UserID:              m.UserID,
		InstitutionName:     m.InstitutionName,
		LegalEntityID:       m.LegalEntityID,
		BankCode:            m.BankCode,
		CountryCode:         m.CountryCode,
		Role:                m.Role,
		WalletAddress:       m.WalletAddress,
		Status:              m.Status,
		CertificateData:     m.CertificateData,
		CertificateExpiry:   m.CertificateExpiry,
		BlockchainPubKeyHex: m.BlockchainPubKeyHex,
		CsrPem:              m.CsrPem,
		PopNonce:            m.PopNonce,
		PopNonceExpiresAt:   m.PopNonceExpiresAt,
		RejectionReason:     m.RejectionReason,
	}
}
