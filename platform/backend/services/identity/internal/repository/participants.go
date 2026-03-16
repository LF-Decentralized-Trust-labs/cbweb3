package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

type Participant struct {
	UserID         string
	DID            string
	WalletAddress  string
	Country        string
	BankCode       string
	Role           string
	SignerProvider string
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

type postgresParticipantsRepository struct {
	db *sql.DB
}

func NewPostgresParticipantsRepository(db *sql.DB) ParticipantsRepository {
	return &postgresParticipantsRepository{db: db}
}

func EnsurePostgresSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS participants (
			user_id TEXT PRIMARY KEY,
			did TEXT NOT NULL,
			wallet_address TEXT NOT NULL,
			country TEXT,
			bank_code TEXT,
			role TEXT,
			signer_provider TEXT
		)`,
	)
	return err
}

func (r *postgresParticipantsRepository) Upsert(ctx context.Context, p Participant) error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO participants (user_id, did, wallet_address, country, bank_code, role, signer_provider)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (user_id)
		 DO UPDATE SET did = EXCLUDED.did,
		               wallet_address = EXCLUDED.wallet_address,
		               country = EXCLUDED.country,
		               bank_code = EXCLUDED.bank_code,
		               role = EXCLUDED.role,
		               signer_provider = EXCLUDED.signer_provider`,
		p.UserID, p.DID, p.WalletAddress, p.Country, p.BankCode, p.Role, p.SignerProvider,
	)
	return err
}

func (r *postgresParticipantsRepository) GetByUser(ctx context.Context, userID string) (Participant, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT user_id, did, wallet_address, country, bank_code, role, signer_provider
		 FROM participants WHERE user_id = $1`,
		userID,
	)
	var p Participant
	if err := row.Scan(
		&p.UserID, &p.DID, &p.WalletAddress, &p.Country, &p.BankCode, &p.Role, &p.SignerProvider,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Participant{}, false, nil
		}
		return Participant{}, false, err
	}
	return p, true, nil
}
