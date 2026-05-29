// Package services provides ParticipantResolver for looking up bank on-chain addresses.
package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// participantRow mirrors only the columns we need from the compliance service's participants table.
type participantRow struct {
	BankCode      string `gorm:"column:bank_code"`
	WalletAddress string `gorm:"column:wallet_address"`
	Status        string `gorm:"column:status"`
}

func (participantRow) TableName() string { return "participants" }

// ParticipantResolverIface resolves a bank_id to its on-chain wallet address.
type ParticipantResolverIface interface {
	// ResolveWalletAddress returns the on-chain wallet address for the given bank_code.
	// Returns an error if the participant is not found or not ACTIVE.
	ResolveWalletAddress(ctx context.Context, bankCode string) (string, error)
}

// ParticipantResolver looks up on-chain addresses from the local participants table.
// CB-B owns this table (populated by the onboarding/compliance flow), so it is the
// authoritative source for the wallet addresses of its member banks.
type ParticipantResolver struct {
	db *gorm.DB
}

// NewParticipantResolver creates a resolver backed by the given GORM database.
func NewParticipantResolver(db *gorm.DB) *ParticipantResolver {
	return &ParticipantResolver{db: db}
}

// ResolveWalletAddress looks up the on-chain address of a bank by its bank_code.
// Returns an error if the bank is not found, not ACTIVE, or has an empty wallet address.
func (r *ParticipantResolver) ResolveWalletAddress(ctx context.Context, bankCode string) (string, error) {
	if bankCode == "" {
		return "", fmt.Errorf("bank_code is required")
	}

	var row participantRow
	err := r.db.WithContext(ctx).
		Where("bank_code = ?", bankCode).
		First(&row).Error
	if err != nil {
		return "", fmt.Errorf("participant not found for bank_code=%q: %w", bankCode, err)
	}

	if !strings.EqualFold(row.Status, "ACTIVE") {
		return "", fmt.Errorf("participant bank_code=%q is not ACTIVE (status=%s)", bankCode, row.Status)
	}

	addr := strings.TrimSpace(row.WalletAddress)
	if addr == "" {
		return "", fmt.Errorf("participant bank_code=%q has no wallet_address", bankCode)
	}

	return addr, nil
}
