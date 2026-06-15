// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// zkPointerRow mirrors the columns the gate queries on compliance_zk_pointers.
type zkPointerRow struct {
	PointerID      string `gorm:"column:pointer_id;primaryKey"`
	BankID         string `gorm:"column:bank_id"`
	CommitmentHash string `gorm:"column:commitment_hash"`
	State          string `gorm:"column:state"`
	ExpiresAt      *time.Time
}

func (zkPointerRow) TableName() string { return "compliance_zk_pointers" }

func newGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&zkPointerRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestValidateZKPointer_Allow(t *testing.T) {
	db := newGateDB(t)
	future := time.Now().Add(time.Hour)
	if err := db.Create(&zkPointerRow{
		PointerID:      "p1",
		BankID:         "bank-a",
		CommitmentHash: "0xabc",
		State:          "VALID",
		ExpiresAt:      &future,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	gate := NewZKComplianceGate(db)
	if err := gate.ValidateZKPointer(context.Background(), "bank-a", "tx-1", "0xabc"); err != nil {
		t.Fatalf("expected ALLOW, got error: %v", err)
	}
}

func TestValidateZKPointer_AllowNoExpiry(t *testing.T) {
	db := newGateDB(t)
	if err := db.Create(&zkPointerRow{
		PointerID:      "p-noexp",
		BankID:         "bank-a",
		CommitmentHash: "0xnoexp",
		State:          "VALID",
		ExpiresAt:      nil,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	gate := NewZKComplianceGate(db)
	if err := gate.ValidateZKPointer(context.Background(), "bank-a", "", "0xnoexp"); err != nil {
		t.Fatalf("expected ALLOW for nil expiry, got: %v", err)
	}
}

func TestValidateZKPointer_DenyMissingArgs(t *testing.T) {
	gate := NewZKComplianceGate(newGateDB(t))
	cases := []struct{ bank, commit string }{
		{"", "0xabc"},
		{"bank-a", ""},
		{"", ""},
	}
	for _, c := range cases {
		err := gate.ValidateZKPointer(context.Background(), c.bank, "tx", c.commit)
		var zkErr *ZKValidationError
		if !errors.As(err, &zkErr) {
			t.Fatalf("bank=%q commit=%q: expected ZKValidationError, got %v", c.bank, c.commit, err)
		}
	}
}

func TestValidateZKPointer_DenyNoPointer(t *testing.T) {
	gate := NewZKComplianceGate(newGateDB(t))
	err := gate.ValidateZKPointer(context.Background(), "bank-a", "tx", "0xmissing")
	var zkErr *ZKValidationError
	if !errors.As(err, &zkErr) {
		t.Fatalf("expected ZKValidationError for missing pointer, got %v", err)
	}
	if zkErr.Message != "no valid ZK-Pointer found" {
		t.Errorf("unexpected message: %q", zkErr.Message)
	}
}

func TestValidateZKPointer_DenyWrongState(t *testing.T) {
	db := newGateDB(t)
	// REVOKED pointer must not satisfy the gate (state filter is 'VALID').
	db.Create(&zkPointerRow{PointerID: "p2", BankID: "bank-a", CommitmentHash: "0xrev", State: "REVOKED"})
	gate := NewZKComplianceGate(db)
	err := gate.ValidateZKPointer(context.Background(), "bank-a", "tx", "0xrev")
	var zkErr *ZKValidationError
	if !errors.As(err, &zkErr) {
		t.Fatalf("expected DENY for revoked pointer, got %v", err)
	}
}

func TestValidateZKPointer_DenyExpired(t *testing.T) {
	db := newGateDB(t)
	past := time.Now().Add(-time.Hour)
	db.Create(&zkPointerRow{
		PointerID:      "p3",
		BankID:         "bank-a",
		CommitmentHash: "0xexp",
		State:          "VALID",
		ExpiresAt:      &past,
	})
	gate := NewZKComplianceGate(db)
	err := gate.ValidateZKPointer(context.Background(), "bank-a", "tx", "0xexp")
	var zkErr *ZKValidationError
	if !errors.As(err, &zkErr) {
		t.Fatalf("expected DENY for expired pointer, got %v", err)
	}
	if zkErr.Message != "ZK-Pointer has expired" {
		t.Errorf("unexpected message: %q", zkErr.Message)
	}
}

func TestZKValidationError_Error(t *testing.T) {
	e := &ZKValidationError{BankID: "bank-z", Message: "boom"}
	if got := e.Error(); got != "zk_validation_failed: bank=bank-z msg=boom" {
		t.Errorf("unexpected Error(): %q", got)
	}
}
