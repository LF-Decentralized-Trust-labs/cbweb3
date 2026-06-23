// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRepository_ParticipantCRUD(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()

	// Empty userID rejected.
	if err := repo.UpsertParticipant(ctx, Participant{}); err == nil {
		t.Fatal("expected error for empty userID")
	}

	exp := time.Now().Add(time.Hour)
	if err := repo.UpsertParticipant(ctx, Participant{
		UserID: "u1", InstitutionName: "Bank A", BankCode: "BANK-A", Status: "ACTIVE",
		CertificateExpiry: &exp, PopNonceExpiresAt: &exp,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.UpsertParticipant(ctx, Participant{UserID: "u2", BankCode: "BANK-B", Status: "PENDING"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, found, err := repo.GetParticipantByUser(ctx, "u1")
	if err != nil || !found || got.InstitutionName != "Bank A" {
		t.Fatalf("get u1 = %+v found=%v err=%v", got, found, err)
	}

	_, found, _ = repo.GetParticipantByUser(ctx, "missing")
	if found {
		t.Error("expected not found")
	}

	// Filter by status.
	active, _ := repo.ListParticipants(ctx, ParticipantFilter{Status: "ACTIVE"})
	if len(active) != 1 || active[0].UserID != "u1" {
		t.Errorf("status filter = %+v", active)
	}

	// Filter by bank code.
	byCode, _ := repo.ListParticipants(ctx, ParticipantFilter{BankCode: "BANK-B"})
	if len(byCode) != 1 || byCode[0].UserID != "u2" {
		t.Errorf("bankcode filter = %+v", byCode)
	}

	// No filter returns all.
	all, _ := repo.ListParticipants(ctx, ParticipantFilter{})
	if len(all) != 2 {
		t.Errorf("expected 2 participants, got %d", len(all))
	}
}

func TestMemoryRepository_AuditLogs(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := repo.CreateAuditLog(ctx, AuditEntry{ActionType: "A", Category: "SESSION", Severity: "INFO"}); err != nil {
			t.Fatalf("create audit: %v", err)
		}
	}
	if err := repo.CreateAuditLog(ctx, AuditEntry{ActionType: "B", Category: "FREEZE", Severity: "CRITICAL"}); err != nil {
		t.Fatalf("create audit: %v", err)
	}

	// Category + severity filter.
	freeze, _ := repo.GetAuditLogs(ctx, AuditFilter{Category: "FREEZE", Severity: "CRITICAL"})
	if len(freeze) != 1 || freeze[0].ActionType != "B" {
		t.Errorf("freeze filter = %+v", freeze)
	}

	// Default limit (no filter).
	all, _ := repo.GetAuditLogs(ctx, AuditFilter{})
	if len(all) != 4 {
		t.Errorf("expected 4 logs, got %d", len(all))
	}

	// Explicit limit truncates.
	limited, _ := repo.GetAuditLogs(ctx, AuditFilter{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("expected 2 logs with limit, got %d", len(limited))
	}
}

func TestMemoryRepository_SystemParameters(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()

	_, found, err := repo.GetSystemParameter(ctx, "missing")
	if err != nil || found {
		t.Fatalf("expected not found, got found=%v err=%v", found, err)
	}

	if err := repo.UpsertSystemParameter(ctx, SystemParameter{Key: "tx_min", Value: "10", UpdatedBy: "admin"}); err != nil {
		t.Fatalf("upsert param: %v", err)
	}
	// Overwrite.
	if err := repo.UpsertSystemParameter(ctx, SystemParameter{Key: "tx_min", Value: "20", UpdatedBy: "admin2"}); err != nil {
		t.Fatalf("upsert param: %v", err)
	}

	val, found, err := repo.GetSystemParameter(ctx, "tx_min")
	if err != nil || !found || val != "20" {
		t.Errorf("get param = %q found=%v err=%v", val, found, err)
	}
}

func TestMemoryRepository_VolumeRestoreFloorsAtZero(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()
	day := time.Now().UTC()

	// Restore more than accumulated must floor at 0 (not negative).
	if err := repo.DeductTransferVolume(ctx, "bank-a", "BRL", "100", day); err != nil {
		t.Fatalf("deduct: %v", err)
	}
	if err := repo.RestoreTransferVolume(ctx, "bank-a", "BRL", "500", day); err != nil {
		t.Fatalf("restore: %v", err)
	}
	acc, _ := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", day)
	if acc != "0" {
		t.Errorf("expected floored volume 0, got %q", acc)
	}

	// Invalid amountWei in deduct returns an error.
	if err := repo.DeductTransferVolume(ctx, "bank-a", "BRL", "notanumber", day); err == nil {
		t.Error("expected error for invalid amountWei in deduct")
	}
	// Invalid amountWei in restore is a no-op (best-effort).
	if err := repo.RestoreTransferVolume(ctx, "bank-a", "BRL", "notanumber", day); err != nil {
		t.Errorf("restore best-effort should not error, got %v", err)
	}
}

func TestMemoryRepository_FindApplicableLimit_Cascade(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()

	// Global (empty participant + empty currency) limit only.
	if err := repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "g", CentralBankID: "cb-a", IsActive: true, MaxAmount: "1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	l, _ := repo.FindApplicableLimit(ctx, "cb-a", "bank-a", "BRL")
	if l == nil || l.LimitID != "g" {
		t.Fatalf("expected global fallback, got %+v", l)
	}

	// More specific participant+currency match takes priority.
	if err := repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "s", CentralBankID: "cb-a", ParticipantID: "bank-a", Currency: "BRL", IsActive: true, MaxAmount: "2"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	l2, _ := repo.FindApplicableLimit(ctx, "cb-a", "bank-a", "BRL")
	if l2 == nil || l2.LimitID != "s" {
		t.Fatalf("expected specific limit, got %+v", l2)
	}

	// No match for a different CB.
	l3, _ := repo.FindApplicableLimit(ctx, "cb-z", "bank-a", "BRL")
	if l3 != nil {
		t.Errorf("expected nil for unknown CB, got %+v", l3)
	}
}
