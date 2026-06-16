// Package services provides the ZK pointer gate for supervisor verification (FR-SUP-002 / US2).
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ZKPointerRecord is the result of a ZK-pointer lookup for supervisor verification.
type ZKPointerRecord struct {
	PointerID      string
	CommitmentHash string
	State          string
	ExpiresAt      *time.Time
}

// ZKPointerGate queries the compliance_zk_pointers table directly for supervisor read access.
// It provides richer lookup semantics than ValidateZKPointer — returning the full record so
// the HTTP handler can distinguish VALID, EXPIRED, and NOT_FOUND responses.
type ZKPointerGate struct {
	db *gorm.DB
}

// NewZKPointerGate creates a ZKPointerGate backed by the given DB.
func NewZKPointerGate(db *gorm.DB) *ZKPointerGate {
	return &ZKPointerGate{db: db}
}

// ErrZKPointerNotFound is returned when no matching record exists.
var ErrZKPointerGateNotFound = errors.New("no ZK-Pointer record found")

// GetZKPointerRecord looks up the most recent ZK pointer for the given bank and commitment hash.
// Returns ErrZKPointerGateNotFound when no record matches.
func (g *ZKPointerGate) GetZKPointerRecord(ctx context.Context, bankID, commitmentHash string) (*ZKPointerRecord, error) {
	var row ZKPointerRecord
	err := g.db.WithContext(ctx).
		Table("compliance_zk_pointers").
		Select("pointer_id, commitment_hash, state, expires_at").
		Where("bank_id = ? AND commitment_hash = ?", bankID, commitmentHash).
		Order("created_at DESC").
		Limit(1).
		Scan(&row).Error

	if err != nil {
		return nil, fmt.Errorf("zk pointer gate query: %w", err)
	}
	if row.PointerID == "" {
		return nil, ErrZKPointerGateNotFound
	}

	// Derive EXPIRED state if DB says VALID but expiry has passed.
	if row.State == "VALID" && row.ExpiresAt != nil && time.Now().After(*row.ExpiresAt) {
		row.State = "EXPIRED"
	}

	result := row // copy so caller owns the pointer
	return &result, nil
}
