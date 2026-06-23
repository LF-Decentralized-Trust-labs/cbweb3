// SPDX-License-Identifier: Apache-2.0

// Package repository_test verifies the EscrowRepository interface contract using
// MemoryEscrowRepository as the concrete implementation under test. These tests
// document (and protect) the expected behaviour for every EscrowRepository
// implementor, including GormEscrowRepository, without requiring a real database.
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
)

// newDepositRecord returns a minimal valid DepositRecord for test use.
func newDepositRecord(id, requesterID string) domain.DepositRecord {
	return domain.DepositRecord{
		ID:                       id,
		RequesterID:              requesterID,
		RequesterBesuAddress:     "0xABCDEF",
		RequesterPaladinIdentity: requesterID + "@spoke-a-bank-a",
		Amount:                   "1000",
		Status:                   domain.DepositStatusPending,
		CreatedAt:                time.Now().UTC(),
	}
}

func newEscrowRecord(id, requesterID string) domain.EscrowRecord {
	return domain.EscrowRecord{
		ID:                       id,
		RequesterID:              requesterID,
		RequesterBesuAddress:     "0xABCDEF",
		RequesterPaladinIdentity: requesterID + "@spoke-a-bank-a",
		Amount:                   "500",
		Status:                   domain.EscrowStatusPending,
		CreatedAt:                time.Now().UTC(),
	}
}

func newRedeemRecord(id, requesterID string) domain.RedeemRecord {
	return domain.RedeemRecord{
		ID:                       id,
		RequesterID:              requesterID,
		RequesterBesuAddress:     "0xABCDEF",
		RequesterPaladinIdentity: requesterID + "@spoke-a-bank-a",
		Amount:                   "250",
		Status:                   domain.RedeemStatusPending,
		CreatedAt:                time.Now().UTC(),
	}
}

// ─── Deposit ───────────────────────────────────────────────────────────────────

func TestMemoryEscrow_CreateAndGetDeposit(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newDepositRecord("dep-001", "bank-a")

	if err := repo.CreateDeposit(ctx, rec); err != nil {
		t.Fatalf("CreateDeposit: %v", err)
	}

	got, ok, err := repo.GetDeposit(ctx, "dep-001")
	if err != nil {
		t.Fatalf("GetDeposit: %v", err)
	}
	if !ok {
		t.Fatal("expected deposit to be found")
	}
	if got.ID != rec.ID {
		t.Errorf("expected ID %q, got %q", rec.ID, got.ID)
	}
	if got.RequesterID != rec.RequesterID {
		t.Errorf("expected RequesterID %q, got %q", rec.RequesterID, got.RequesterID)
	}
	if got.Amount != rec.Amount {
		t.Errorf("expected Amount %q, got %q", rec.Amount, got.Amount)
	}
}

func TestMemoryEscrow_GetDeposit_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	_, ok, err := repo.GetDeposit(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetDeposit error: %v", err)
	}
	if ok {
		t.Error("expected deposit to not be found")
	}
}

func TestMemoryEscrow_CreateDeposit_DuplicateReturnsError(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newDepositRecord("dep-dup", "bank-a")
	if err := repo.CreateDeposit(ctx, rec); err != nil {
		t.Fatalf("first CreateDeposit: %v", err)
	}
	if err := repo.CreateDeposit(ctx, rec); err == nil {
		t.Error("expected error on duplicate CreateDeposit, got nil")
	}
}

func TestMemoryEscrow_UpdateDeposit(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newDepositRecord("dep-upd", "bank-a")
	if err := repo.CreateDeposit(ctx, rec); err != nil {
		t.Fatalf("CreateDeposit: %v", err)
	}

	rec.Status = domain.DepositStatusApproved
	rec.MintTxHash = "0xMINT123"
	if err := repo.UpdateDeposit(ctx, rec); err != nil {
		t.Fatalf("UpdateDeposit: %v", err)
	}

	got, ok, err := repo.GetDeposit(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("GetDeposit after update: err=%v, ok=%v", err, ok)
	}
	if got.Status != domain.DepositStatusApproved {
		t.Errorf("expected status APPROVED, got %q", got.Status)
	}
	if got.MintTxHash != "0xMINT123" {
		t.Errorf("expected MintTxHash 0xMINT123, got %q", got.MintTxHash)
	}
}

func TestMemoryEscrow_UpdateDeposit_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newDepositRecord("dep-missing", "bank-a")
	if err := repo.UpdateDeposit(ctx, rec); err == nil {
		t.Error("expected error when updating non-existent deposit")
	}
}

func TestMemoryEscrow_ListDeposits_AllAndFiltered(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	if err := repo.CreateDeposit(ctx, newDepositRecord("dep-A1", "bank-a")); err != nil {
		t.Fatalf("CreateDeposit A1: %v", err)
	}
	if err := repo.CreateDeposit(ctx, newDepositRecord("dep-A2", "bank-a")); err != nil {
		t.Fatalf("CreateDeposit A2: %v", err)
	}
	if err := repo.CreateDeposit(ctx, newDepositRecord("dep-B1", "bank-b")); err != nil {
		t.Fatalf("CreateDeposit B1: %v", err)
	}

	all, err := repo.ListDeposits(ctx, "")
	if err != nil {
		t.Fatalf("ListDeposits (all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 deposits, got %d", len(all))
	}

	bankA, err := repo.ListDeposits(ctx, "bank-a")
	if err != nil {
		t.Fatalf("ListDeposits (bank-a): %v", err)
	}
	if len(bankA) != 2 {
		t.Errorf("expected 2 deposits for bank-a, got %d", len(bankA))
	}

	bankB, err := repo.ListDeposits(ctx, "bank-b")
	if err != nil {
		t.Fatalf("ListDeposits (bank-b): %v", err)
	}
	if len(bankB) != 1 {
		t.Errorf("expected 1 deposit for bank-b, got %d", len(bankB))
	}

	bankC, err := repo.ListDeposits(ctx, "bank-c")
	if err != nil {
		t.Fatalf("ListDeposits (bank-c): %v", err)
	}
	if len(bankC) != 0 {
		t.Errorf("expected 0 deposits for bank-c, got %d", len(bankC))
	}
}

// ─── Escrow ────────────────────────────────────────────────────────────────────

func TestMemoryEscrow_CreateAndGetEscrow(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newEscrowRecord("esc-001", "bank-a")
	if err := repo.CreateEscrow(ctx, rec); err != nil {
		t.Fatalf("CreateEscrow: %v", err)
	}

	got, ok, err := repo.GetEscrow(ctx, "esc-001")
	if err != nil {
		t.Fatalf("GetEscrow: %v", err)
	}
	if !ok {
		t.Fatal("expected escrow to be found")
	}
	if got.ID != rec.ID {
		t.Errorf("expected ID %q, got %q", rec.ID, got.ID)
	}
}

func TestMemoryEscrow_GetEscrow_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	_, ok, err := repo.GetEscrow(ctx, "esc-nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected escrow not to be found")
	}
}

func TestMemoryEscrow_CreateEscrow_DuplicateReturnsError(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newEscrowRecord("esc-dup", "bank-a")
	if err := repo.CreateEscrow(ctx, rec); err != nil {
		t.Fatalf("first CreateEscrow: %v", err)
	}
	if err := repo.CreateEscrow(ctx, rec); err == nil {
		t.Error("expected duplicate CreateEscrow to return error")
	}
}

func TestMemoryEscrow_UpdateEscrow(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newEscrowRecord("esc-upd", "bank-a")
	if err := repo.CreateEscrow(ctx, rec); err != nil {
		t.Fatalf("CreateEscrow: %v", err)
	}

	rec.Status = domain.EscrowStatusApproved
	rec.BurnTxHash = "0xBURN"
	rec.MintTxHash = "0xMINT"
	if err := repo.UpdateEscrow(ctx, rec); err != nil {
		t.Fatalf("UpdateEscrow: %v", err)
	}

	got, ok, err := repo.GetEscrow(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("GetEscrow after update: err=%v, ok=%v", err, ok)
	}
	if got.Status != domain.EscrowStatusApproved {
		t.Errorf("expected APPROVED, got %q", got.Status)
	}
	if got.BurnTxHash != "0xBURN" {
		t.Errorf("expected BurnTxHash 0xBURN, got %q", got.BurnTxHash)
	}
	if got.MintTxHash != "0xMINT" {
		t.Errorf("expected MintTxHash 0xMINT, got %q", got.MintTxHash)
	}
}

func TestMemoryEscrow_UpdateEscrow_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newEscrowRecord("esc-missing", "bank-a")
	if err := repo.UpdateEscrow(ctx, rec); err == nil {
		t.Error("expected error when updating non-existent escrow")
	}
}

func TestMemoryEscrow_UpdateEscrow_RejectionReason(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newEscrowRecord("esc-reject", "bank-a")
	if err := repo.CreateEscrow(ctx, rec); err != nil {
		t.Fatalf("CreateEscrow: %v", err)
	}

	rec.Status = domain.EscrowStatusRejected
	rec.RejectionReason = "insufficient funds"
	if err := repo.UpdateEscrow(ctx, rec); err != nil {
		t.Fatalf("UpdateEscrow: %v", err)
	}

	got, _, _ := repo.GetEscrow(ctx, rec.ID)
	if got.RejectionReason != "insufficient funds" {
		t.Errorf("expected rejection reason, got %q", got.RejectionReason)
	}
}

func TestMemoryEscrow_ListEscrows_AllAndFiltered(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	for _, id := range []string{"e1", "e2"} {
		if err := repo.CreateEscrow(ctx, newEscrowRecord(id, "bank-a")); err != nil {
			t.Fatalf("CreateEscrow %s: %v", id, err)
		}
	}
	if err := repo.CreateEscrow(ctx, newEscrowRecord("e3", "bank-b")); err != nil {
		t.Fatalf("CreateEscrow e3: %v", err)
	}

	all, err := repo.ListEscrows(ctx, "")
	if err != nil {
		t.Fatalf("ListEscrows all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	bankA, err := repo.ListEscrows(ctx, "bank-a")
	if err != nil {
		t.Fatalf("ListEscrows bank-a: %v", err)
	}
	if len(bankA) != 2 {
		t.Errorf("expected 2 for bank-a, got %d", len(bankA))
	}
}

// ─── Redeem ────────────────────────────────────────────────────────────────────

func TestMemoryEscrow_CreateAndGetRedeem(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newRedeemRecord("rdm-001", "bank-a")
	if err := repo.CreateRedeem(ctx, rec); err != nil {
		t.Fatalf("CreateRedeem: %v", err)
	}

	got, ok, err := repo.GetRedeem(ctx, "rdm-001")
	if err != nil {
		t.Fatalf("GetRedeem: %v", err)
	}
	if !ok {
		t.Fatal("expected redeem to be found")
	}
	if got.ID != rec.ID {
		t.Errorf("expected ID %q, got %q", rec.ID, got.ID)
	}
}

func TestMemoryEscrow_GetRedeem_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	_, ok, err := repo.GetRedeem(ctx, "rdm-nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected redeem not to be found")
	}
}

func TestMemoryEscrow_CreateRedeem_DuplicateReturnsError(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newRedeemRecord("rdm-dup", "bank-a")
	if err := repo.CreateRedeem(ctx, rec); err != nil {
		t.Fatalf("first CreateRedeem: %v", err)
	}
	if err := repo.CreateRedeem(ctx, rec); err == nil {
		t.Error("expected duplicate CreateRedeem to return error")
	}
}

func TestMemoryEscrow_UpdateRedeem(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newRedeemRecord("rdm-upd", "bank-a")
	if err := repo.CreateRedeem(ctx, rec); err != nil {
		t.Fatalf("CreateRedeem: %v", err)
	}

	rec.Status = domain.RedeemStatusApproved
	rec.ZetoTransferTxHash = "0xZETO"
	rec.FiatMintTxHash = "0xFIAT"
	if err := repo.UpdateRedeem(ctx, rec); err != nil {
		t.Fatalf("UpdateRedeem: %v", err)
	}

	got, ok, err := repo.GetRedeem(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("GetRedeem after update: err=%v, ok=%v", err, ok)
	}
	if got.Status != domain.RedeemStatusApproved {
		t.Errorf("expected APPROVED, got %q", got.Status)
	}
	if got.ZetoTransferTxHash != "0xZETO" {
		t.Errorf("expected ZetoTransferTxHash 0xZETO, got %q", got.ZetoTransferTxHash)
	}
	if got.FiatMintTxHash != "0xFIAT" {
		t.Errorf("expected FiatMintTxHash 0xFIAT, got %q", got.FiatMintTxHash)
	}
}

func TestMemoryEscrow_UpdateRedeem_NotFound(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newRedeemRecord("rdm-missing", "bank-a")
	if err := repo.UpdateRedeem(ctx, rec); err == nil {
		t.Error("expected error when updating non-existent redeem")
	}
}

func TestMemoryEscrow_UpdateRedeem_RejectionReason(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	rec := newRedeemRecord("rdm-reject", "bank-a")
	if err := repo.CreateRedeem(ctx, rec); err != nil {
		t.Fatalf("CreateRedeem: %v", err)
	}

	rec.Status = domain.RedeemStatusRejected
	rec.RejectionReason = "kyc failed"
	if err := repo.UpdateRedeem(ctx, rec); err != nil {
		t.Fatalf("UpdateRedeem: %v", err)
	}

	got, _, _ := repo.GetRedeem(ctx, rec.ID)
	if got.RejectionReason != "kyc failed" {
		t.Errorf("expected rejection reason, got %q", got.RejectionReason)
	}
}

func TestMemoryEscrow_ListRedeems_AllAndFiltered(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	ctx := context.Background()

	for _, id := range []string{"r1", "r2"} {
		if err := repo.CreateRedeem(ctx, newRedeemRecord(id, "bank-a")); err != nil {
			t.Fatalf("CreateRedeem %s: %v", id, err)
		}
	}
	if err := repo.CreateRedeem(ctx, newRedeemRecord("r3", "bank-b")); err != nil {
		t.Fatalf("CreateRedeem r3: %v", err)
	}

	all, err := repo.ListRedeems(ctx, "")
	if err != nil {
		t.Fatalf("ListRedeems all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	bankA, err := repo.ListRedeems(ctx, "bank-a")
	if err != nil {
		t.Fatalf("ListRedeems bank-a: %v", err)
	}
	if len(bankA) != 2 {
		t.Errorf("expected 2 for bank-a, got %d", len(bankA))
	}

	bankB, err := repo.ListRedeems(ctx, "bank-b")
	if err != nil {
		t.Fatalf("ListRedeems bank-b: %v", err)
	}
	if len(bankB) != 1 {
		t.Errorf("expected 1 for bank-b, got %d", len(bankB))
	}
}

// ─── Interface satisfaction ────────────────────────────────────────────────────

// TestMemoryEscrowRepository_ImplementsInterface confirms that
// *MemoryEscrowRepository satisfies ports.EscrowRepository at compile time.
// This is already asserted in escrow.go itself, but the test makes the intent
// explicit for readers of the test suite.
func TestMemoryEscrowRepository_ImplementsInterface(t *testing.T) {
	repo := repository.NewMemoryEscrowRepository()
	if repo == nil {
		t.Fatal("NewMemoryEscrowRepository returned nil")
	}
	// The compile-time assertion `var _ ports.EscrowRepository = (*MemoryEscrowRepository)(nil)`
	// in escrow.go guarantees this. This test is a documentation artefact.
}
