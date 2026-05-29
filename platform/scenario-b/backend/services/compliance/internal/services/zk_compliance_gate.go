// Package services provides the ZK compliance gate for Scenario B (FR-025 / FR-058 / SC-016).
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ZKValidationError is returned when a ZK-Pointer fails validation.
type ZKValidationError struct {
	BankID  string
	Message string
}

func (e *ZKValidationError) Error() string {
	return fmt.Sprintf("zk_validation_failed: bank=%s msg=%s", e.BankID, e.Message)
}

// zkPointerRecord is a lightweight struct for querying the compliance DB.
type zkPointerRecord struct {
	PointerID      string
	CommitmentHash string
	State          string
	ExpiresAt      *time.Time
}

func (zkPointerRecord) TableName() string { return "compliance_zk_pointers" }

// ZKComplianceGate implements the ComplianceGate interface by querying the DB for valid ZK-Pointers.
type ZKComplianceGate struct {
	db *gorm.DB
}

// NewZKComplianceGate creates a ZKComplianceGate backed by the compliance database.
func NewZKComplianceGate(db *gorm.DB) *ZKComplianceGate {
	return &ZKComplianceGate{db: db}
}

// ValidateZKPointer checks that the given bank has a valid, non-expired ZK-Pointer
// matching the provided commitment hash (FR-025 / FR-058).
func (g *ZKComplianceGate) ValidateZKPointer(ctx context.Context, bankID, _ string, commitmentHash string) error {
	if bankID == "" || commitmentHash == "" {
		return &ZKValidationError{BankID: bankID, Message: "bankID and commitmentHash are required"}
	}

	var record zkPointerRecord
	err := g.db.WithContext(ctx).
		Table("compliance_zk_pointers").
		Where("bank_id = ? AND commitment_hash = ? AND state = 'VALID'", bankID, commitmentHash).
		First(&record).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &ZKValidationError{BankID: bankID, Message: "no valid ZK-Pointer found"}
	}
	if err != nil {
		return fmt.Errorf("zk compliance db query: %w", err)
	}

	// Check expiry
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return &ZKValidationError{BankID: bankID, Message: "ZK-Pointer has expired"}
	}

	return nil
}
